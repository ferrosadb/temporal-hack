# Onboarding

This file is meant to be fed to Claude Code (or read by a human) when
setting up a fresh dev box for `temporal-hack`. It walks through every
dependency and verifies each one before moving on.

When run by Claude, follow each section in order. Stop at the first
failed verification, fix it, and re-verify before continuing.

When run by a human, copy each verification command and run it
yourself.

---

## Section 0 — Operating system

Supported:
- Linux (Ubuntu 22.04+ recommended; Fedora and Arch known to work).
- macOS 13+ (Apple Silicon and Intel).
- WSL2 on Windows (Ubuntu image).

Native Windows is not supported — the lab stack and sim depend on Linux
container semantics.

```bash
uname -srm
```

---

## Section 1 — Required CLI tools

You need all of:

| Tool          | Min version | Why                                        |
|---------------|-------------|--------------------------------------------|
| git           | 2.40        | Source control                             |
| make          | 4.0         | Build orchestrator                         |
| Go            | 1.22        | Cloud + agent modules                      |
| Python        | 3.10        | ROS 2 bridge node                          |
| Docker OR podman | latest   | Lab stack and sim containers               |
| docker compose / podman compose | latest | Compose v2 plugin            |
| protoc        | 25          | Protobuf code generation                   |
| protoc-gen-go | latest      | Go protobuf bindings                       |
| protoc-gen-go-grpc | latest | Go gRPC bindings                           |
| pre-commit    | 3.6         | Git hooks                                  |
| netcat (nc)   | any         | Port probing in `make lab-status`          |
| curl          | any         | Health probes                              |
| jq            | any         | Inspecting API output                      |

### 1.1 Verify each

```bash
git --version
make --version | head -1
go version            # want 1.22+
python3 --version     # want 3.10+
docker --version || podman --version
docker compose version || podman compose version
protoc --version
which protoc-gen-go
which protoc-gen-go-grpc
pre-commit --version
nc -h 2>&1 | head -1
curl --version | head -1
jq --version
```

### 1.2 Install hints

**macOS (Homebrew):**

```bash
brew install go python protobuf jq pre-commit
brew install --cask docker             # or: brew install podman
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

**Ubuntu / Debian:**

```bash
sudo apt-get update
sudo apt-get install -y \
    git make protobuf-compiler python3 python3-pip \
    netcat-openbsd curl jq
# Go: prefer the official tarball (apt is usually too old)
curl -sSLO https://go.dev/dl/go1.22.6.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.6.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
# Docker or podman:
sudo apt-get install -y docker.io   # or: sudo apt-get install -y podman podman-compose
# pre-commit:
pip3 install --user pre-commit
# protoc plugins:
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Make sure `$(go env GOPATH)/bin` is in `$PATH` so `protoc-gen-go` is
discoverable.

---

## Section 2 — Container engine

The Makefile auto-detects docker vs podman. Verify the choice:

```bash
make container-info
```

Expected output (one of):

```
engine:    docker
compose:   docker compose
sock:      /var/run/docker.sock
```

or

```
engine:    podman
compose:   podman compose            # or podman-compose
sock:      /run/user/1000/podman/podman.sock
```

### 2.1 If you see `engine: none`

Install docker or podman per Section 1.

### 2.2 Podman-specific setup

Enable the rootless socket once per user:

```bash
systemctl --user enable --now podman.socket
ls "$XDG_RUNTIME_DIR/podman/podman.sock"   # must exist
```

If `podman compose version` fails, install the legacy fallback:

```bash
pip3 install --user podman-compose
```

### 2.3 Docker-specific setup

Confirm the daemon is reachable without sudo:

```bash
docker info >/dev/null
```

If that requires sudo, add yourself to the `docker` group and re-login.

---

## Section 3 — Repository

```bash
git clone git@github.com:ferrosadb/temporal-hack.git
cd temporal-hack
git submodule update --init --recursive   # safe even if no submodules today
```

Confirm:

```bash
ls -la specs/ cloud/ agent/ bridge/ docker/sim/ installer/
```

You should see the directories above. If any are missing, your clone
is incomplete; fetch again.

---

## Section 4 — Git hooks

Hooks are checked into `.git-hooks/` and activated automatically the
first time you run any `make` target (the Makefile sets
`core.hooksPath` at parse time). No `pre-commit install` step
needed.

```bash
make hooks-install     # explicit reinstall (idempotent; auto-runs on every make)
ls .git-hooks/         # pre-commit, pre-push, commit-msg + helpers
git config core.hooksPath   # should print: .git-hooks
```

### 4.1 What runs

- **pre-commit** — go fmt + go vet on staged Go files; pre-commit
  framework's pre-commit hooks if installed (ruff, codespell, etc.).
- **commit-msg** — conventional commit prefix check.
- **pre-push** — full CI parity: `go test -race` for both modules,
  bridge ruff, and the CI cluster smoke test
  (`make ci-up` → port probe → `make ci-down`).

### 4.2 Bypass envelope

```bash
SKIP_CI_SMOKE=1 git push       # skip the long installer-smoke step
SKIP_CI_HOOK=1 git push        # skip every hook step (emergency)
git push --no-verify           # ultimate bypass (do not abuse)
```

### 4.3 Optional: pre-commit framework

The hooks call `pre-commit run` if the `pre-commit` binary is
installed, picking up extras from `.pre-commit-config.yaml` like
codespell, ruff, secrets detection, and YAML formatting. Without it,
the core Go + smoke checks still fire — `pre-commit` is a nice-to-
have, not a hard dep.

```bash
pip install --user pre-commit  # macOS / Linux
```

---

## Section 5 — Build the Go binaries

```bash
cd cloud && go mod tidy && cd ..
cd agent && go mod tidy && cd ..
make build
ls bin/
```

Expected: `controlplane`, `telemetry-ingest`, `ota-worker`, `agent`.

### 5.1 If `cd agent && go mod tidy` fails

The agent imports `github.com/mattn/go-sqlite3`, which is cgo. Install
a C toolchain:

```bash
# macOS:
xcode-select --install
# Ubuntu:
sudo apt-get install -y build-essential
```

Re-run `go mod tidy`.

---

## Section 6 — Lab stack smoke test

```bash
make lab-up
make lab-status
```

All four services should report `up` on the 14xxx port range:

```
Temporal UI:    up   (:14080)
MQTT broker:    up   (:7883)
Postgres:       up   (:14432)
Registry:       up   (:14050)
```

Tear down:

```bash
make lab-down
```

If a service stays `down`, look at logs:

```bash
cd installer/docker-compose
docker compose -p temporal-hack-lab logs <service>
# or: podman compose -p temporal-hack-lab logs <service>
```

### 6.1 Bring up the CI cluster too (optional)

The same stack on alternate 2xxxx ports, isolated by Compose project
name. Used by `installer-smoke` in CI and pre-push:

```bash
make ci-up
make ci-status     # 25432 / 27233 / 28080 / 21883 / 25050
make ci-down
```

`make lab-up` and `make ci-up` can run **at the same time** on the
same host.

---

## Section 7 — Run the platform end-to-end

In four terminals:

```bash
# Terminal 1 — lab stack already up (Section 6)
make lab-up

# Terminal 2 — telemetry ingester
TSDB_DSN="postgres://temporal:temporal@localhost:5432/telemetry?sslmode=disable" \
  ./bin/telemetry-ingest

# Terminal 3 — control plane API
./bin/controlplane

# Terminal 4 — OTA worker (Temporal worker + MQTT bridge)
./bin/ota-worker

# Then start an agent (a fifth terminal, or detached)
ROBOT_ID=lab-robot-01 ./bin/agent
```

Verify operator reads:

```bash
curl -s http://localhost:8081/healthz
curl -s http://localhost:8081/v1/robots | jq
```

---

## Section 8 — Sim container (optional, requires ~5 GB disk)

```bash
make sim-up           # builds Gazebo + ROS 2 + bridge + agent
make sim-gui          # opens the Gazebo GUI in your browser
make sim-up-headless  # no GUI; gzserver only
make sim-logs         # tail sim and agent
make sim-down
```

First build is 10–20 minutes; subsequent builds are minutes.

The Gazebo GUI is exposed over noVNC, so it works in any browser
(Safari, Chrome, Firefox) without X11 setup:
- `http://localhost:14680/vnc.html?autoconnect=1&resize=scale` — browser GUI
- `localhost:14900` — raw VNC for native clients

---

## Section 9 — Trigger a test OTA

With the lab stack and worker up:

```bash
# Build and push the lab dummy robot image (Alpine; stays running for swap checks)
docker build -t localhost:5001/robot-app:v1 -f docker/dummy-robot/Dockerfile docker/dummy-robot
docker push localhost:5001/robot-app:v1

# Start a rollout
curl -X POST http://localhost:8081/v1/ota/rollouts \
  -H "content-type: application/json" \
  -d '{
    "image_ref": "localhost:5001/robot-app:v1",
    "smoke_command": "true",
    "cohort_selector": {"robot_ids": ["lab-robot-01"]}
  }'

# Watch the rollout progress
curl -s http://localhost:8081/v1/ota/rollouts | jq
```

The OTA worker logs should show signaled phase transitions
(`PHASE_PULLED` → `PHASE_SWAPPED` → `PHASE_HEALTHY`).

---

## Section 10 — Final verification

Run the full local CI surface:

```bash
make lint
make test
SKIP=installer-smoke pre-commit run --all-files --hook-stage pre-push
```

If all three are green, you're set up. Welcome.

---

## Troubleshooting fast paths

| Symptom                                | Likely cause                        | Fix                                                        |
|----------------------------------------|-------------------------------------|------------------------------------------------------------|
| `make lab-up` errors `no such image`   | Pull failed or rate-limited         | `docker login` / wait, retry                               |
| Postgres exits code 3 at lab-up        | Wrong image (stock instead of TSDB) | We pin `timescale/timescaledb-ha`; rebase                  |
| EMQX unhealthy                         | Port 1883 already in use            | `lsof -i :1883`; stop the other broker                     |
| Agent crashes at startup               | Buffer dir not writable             | `chmod` the path or override `BUFFER_PATH`                 |
| Bridge node import error               | `rclpy` not on `PYTHONPATH`         | `source /opt/ros/humble/setup.bash` first                  |
| OTA stuck at PHASE_PULLED              | Robot can't reach registry          | Check robot's network to the registry hostname/port        |
| `pre-commit` hangs in installer-smoke  | Slow image pull on first run        | Run once manually: `bash .git-hooks/installer-smoke.sh`    |

---

## Where to look next

- `specs/decisions.md` — every architectural decision and why
- `specs/overview.md` — system architecture
- `specs/threat-model.md` — STRIDE threats
- `specs/project-plan.md` — sprint plan + status
- `specs/todo/` — open work items
- `specs/in-process/identity-mtls.md` — the gating P1 before customer ship
