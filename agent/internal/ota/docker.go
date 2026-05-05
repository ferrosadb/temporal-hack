package ota

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// DockerCLI is a thin wrapper over the local `docker` command. We use
// the CLI rather than the Go SDK to keep the agent binary small and
// avoid pinning a docker SDK version against the customer's daemon
// version. The CLI is a contract that's far more stable than the SDK.
//
// The container we manage is named "robot-app" by convention. v1
// runs only one application container per robot.
type DockerCLI struct {
	ContainerName string // default "robot-app"
	RunArgs       []string // extra args passed at `docker run` (volumes, devices, etc.)
}

func NewDockerCLI() *DockerCLI {
	return &DockerCLI{ContainerName: "robot-app"}
}

// Pull invokes `docker pull <ref>`.
func (d *DockerCLI) Pull(ctx context.Context, ref string) error {
	return run(ctx, "docker", "pull", ref)
}

// CurrentDigest returns the image digest of the running container, or
// an empty string if no container is running.
func (d *DockerCLI) CurrentDigest(ctx context.Context) (string, error) {
	out, err := capture(ctx, "docker", "inspect", "--format", "{{.Image}}", d.ContainerName)
	if err != nil {
		if strings.Contains(err.Error(), "No such") {
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
	if err := run(ctx, "docker", args...); err != nil {
		return "", fmt.Errorf("docker run new: %w", err)
	}
	// Verify it's actually running.
	state, _ := capture(ctx, "docker", "inspect", "--format", "{{.State.Status}}", tmpName)
	if strings.TrimSpace(state) != "running" {
		_ = run(ctx, "docker", "rm", "-f", tmpName)
		return prevDigest, fmt.Errorf("new container not running (state=%s)", state)
	}
	// Stop+remove old (best effort).
	_ = run(ctx, "docker", "rm", "-f", d.ContainerName)
	if err := run(ctx, "docker", "rename", tmpName, d.ContainerName); err != nil {
		_ = run(ctx, "docker", "rm", "-f", tmpName)
		return prevDigest, fmt.Errorf("docker rename: %w", err)
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
	_ = run(ctx, "docker", "rm", "-f", d.ContainerName)
	args := []string{"run", "-d", "--name", d.ContainerName}
	args = append(args, d.RunArgs...)
	args = append(args, prevDigest)
	return run(ctx, "docker", args...)
}

// Exec runs a smoke command inside the named container and returns
// the command's stdout+stderr; non-zero exit code returns an error.
func (d *DockerCLI) Exec(ctx context.Context, command string) error {
	if command == "" {
		return nil // no smoke check configured; treat as healthy
	}
	return run(ctx, "docker", "exec", d.ContainerName, "sh", "-c", command)
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
