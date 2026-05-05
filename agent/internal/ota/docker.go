package ota

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// DockerCLI is a thin wrapper over the local container-engine
// command. We use the CLI rather than a Go SDK to keep the agent
// binary small and avoid pinning a docker/podman SDK version against
// the customer's daemon version. The CLI surface (`pull`, `run`,
// `inspect`, `rm`, `rename`, `exec`) is identical between docker and
// podman, so the same code drives either engine — we just resolve
// which binary is on PATH at startup.
//
// The container we manage is named "robot-app" by convention. v1
// runs only one application container per robot.
//
// Engine selection (in order):
//  1. AGENT_CONTAINER_BIN env var (explicit override)
//  2. `docker` on PATH
//  3. `podman` on PATH
type DockerCLI struct {
	ContainerName string   // default "robot-app"
	RunArgs       []string // extra args passed at `<bin> run` (volumes, devices, network, env, etc.)
	bin           string   // resolved engine binary: "docker" or "podman"
}

func NewDockerCLI(runArgs []string) *DockerCLI {
	bin := os.Getenv("AGENT_CONTAINER_BIN")
	if bin == "" {
		if _, err := exec.LookPath("docker"); err == nil {
			bin = "docker"
		} else if _, err := exec.LookPath("podman"); err == nil {
			bin = "podman"
		} else {
			// Last-ditch default; calls will fail loudly with
			// `executable file not found` when the agent first OTAs.
			bin = "docker"
		}
	}
	return &DockerCLI{ContainerName: "robot-app", RunArgs: runArgs, bin: bin}
}

// Bin returns the resolved engine binary ("docker" or "podman").
// Useful for logs.
func (d *DockerCLI) Bin() string { return d.bin }

// Pull invokes `<bin> pull <ref>`.
//
// Podman defaults to HTTPS-only registry traffic; override with
// --tls-verify=false so the local lab registry on a plain-HTTP
// localhost:14050 works without registries.conf surgery. Docker
// uses its daemon-side `insecure-registries` config (set
// per-environment) and does not accept the same flag.
func (d *DockerCLI) Pull(ctx context.Context, ref string) error {
	args := []string{"pull"}
	if d.bin == "podman" {
		args = append(args, "--tls-verify=false")
	}
	args = append(args, ref)
	return run(ctx, d.bin, args...)
}

// CurrentDigest returns the image digest of the running container, or
// an empty string if no container is running.
func (d *DockerCLI) CurrentDigest(ctx context.Context) (string, error) {
	out, err := capture(ctx, d.bin, "inspect", "--format", "{{.Image}}", d.ContainerName)
	if err != nil {
		if strings.Contains(err.Error(), "No such") || strings.Contains(err.Error(), "no such") {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Swap performs a blue-green swap: start the new container under a
// temporary name, verify it's running, then stop+remove the old one
// and rename the new one to the canonical name. Returns the previous
// image digest so it can be used for rollback.
func (d *DockerCLI) Swap(ctx context.Context, ref string) (prevDigest string, err error) {
	prevDigest, _ = d.CurrentDigest(ctx)
	tmpName := d.ContainerName + "-new"
	args := []string{"run", "-d", "--name", tmpName}
	args = append(args, d.RunArgs...)
	args = append(args, ref)
	if err := run(ctx, d.bin, args...); err != nil {
		return "", fmt.Errorf("%s run new: %w", d.bin, err)
	}
	// Verify it's actually running.
	state, _ := capture(ctx, d.bin, "inspect", "--format", "{{.State.Status}}", tmpName)
	if strings.TrimSpace(state) != "running" {
		_ = run(ctx, d.bin, "rm", "-f", tmpName)
		return prevDigest, fmt.Errorf("new container not running (state=%s)", state)
	}
	// Stop+remove old (best effort).
	_ = run(ctx, d.bin, "rm", "-f", d.ContainerName)
	if err := run(ctx, d.bin, "rename", tmpName, d.ContainerName); err != nil {
		_ = run(ctx, d.bin, "rm", "-f", tmpName)
		return prevDigest, fmt.Errorf("%s rename: %w", d.bin, err)
	}
	return prevDigest, nil
}

// Rollback restores a previous image digest by recreating the
// container from it. The agent is responsible for remembering
// prevDigest from the last successful swap.
func (d *DockerCLI) Rollback(ctx context.Context, prevDigest string) error {
	if prevDigest == "" {
		return fmt.Errorf("no previous digest to roll back to")
	}
	_ = run(ctx, d.bin, "rm", "-f", d.ContainerName)
	args := []string{"run", "-d", "--name", d.ContainerName}
	args = append(args, d.RunArgs...)
	args = append(args, prevDigest)
	return run(ctx, d.bin, args...)
}

// Exec runs a smoke command inside the named container and returns
// the command's stdout+stderr; non-zero exit code returns an error.
func (d *DockerCLI) Exec(ctx context.Context, command string) error {
	if command == "" {
		return nil // no smoke check configured; treat as healthy
	}
	return run(ctx, d.bin, "exec", d.ContainerName, "sh", "-c", command)
}

// run executes a command, discarding output unless it fails.
func run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// capture runs a command and returns its stdout. stderr is folded in
// on error.
func capture(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return "", err
	}
	b, _ := io.ReadAll(stdout)
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return string(b), nil
}
