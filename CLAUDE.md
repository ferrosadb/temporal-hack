# CLAUDE.md — temporal-hack

This file gives Claude Code (and any other coding agent that reads
`CLAUDE.md` / `AGENTS.md`) the context it needs to be productive in
this repository on the first interaction. Read it before doing
anything substantive.

## What this is

A robotics fleet management platform. v1 MVP is **Telemetry + OTA**
for a single customer with **10–100 ROS 2 robots** over intermittent
links, deployed **on-prem in the customer DC** (no managed cloud
services). See `specs/overview.md` for the full architecture diagram.

## How to find context, in order of authority

1. **`specs/decisions.md`** — sixteen confirmed Phase 0 decisions.
   These are constraints, not opinions. Do not contradict them
   without amending the file (and probably running `/blueprint update`).
2. **`specs/overview.md`** — Phase 1 architecture: components, data
   flows, ADR placeholders.
3. **`specs/adr/`** — closed-out architectural decisions
   (broker, registry, TSDB, OTA swap strategy, OTA transport, bridge
   language).
4. **`specs/threat-model.md`** — STRIDE threats. Six are CRITICAL or
   HIGH and gate customer deployment.
5. **`specs/project-plan.md`** — sprint map and current status.
6. **`specs/in-process/`** — work-in-flight items.
7. **`specs/todo/`** — known-incomplete items.
8. **`ONBOARDING.md`** — environment setup and end-to-end smoke walkthrough.

When the user asks an architectural or scope question, **read these
first** before searching code. They are the source of truth for
"why" — code only tells you "what."

## Repo layout (skim once)

```
cloud/        Go control plane: API, telemetry-ingest, OTA worker
agent/        Go robot agent: MQTT publisher, SQLite buffer, OTA executor
bridge/       Python ROS 2 bridge: rclpy → gRPC over Unix socket
docker/sim/   Gazebo + TurtleBot3 + bridge container (dev sim only)
proto/        Protobuf contracts (telemetry, agent↔bridge, OTA)
deploy/       Service config baked into the installer
installer/    docker-compose (lab) + helm (prod, stub)
ops/          Runbooks
specs/        Blueprint artifacts (read these first)
.git-hooks/   Local pre-push helpers
```

## Languages and constraints

- **Go 1.22** (cloud + agent). Both modules are in `go.work`.
- **Python 3.10+** (bridge). Uses `rclpy` from the system ROS 2
  install; pinned versions are not in `pyproject.toml` because they
  ship with ROS distros.
- **No Rust** despite an early Phase 0 detour (D-05 changed Rust→Go
  to avoid the alpha Temporal Rust SDK). If you see a Rust import,
  it is a mistake.
- **Cgo is required** for the agent (`go-sqlite3`). A C toolchain must
  be available for `go test` / `go build` of the agent module.

## Always-on rules

These come from the user's global safety rules and the Phase 0
blueprint. They are enforced both by hooks and by review:

1. **Never commit on `main` or `master`.** A pre-tool-use hook in
   `.claude/hooks/require-branch.sh` will block edits if you try.
   Use `git checkout -b feature/<name>` first.
2. **Never destructive git without asking.** No `git reset --hard`,
   `git clean -fd`, `git push --force`, `git branch -D`, etc., unless
   the user explicitly authorizes the specific command.
3. **Never bypass hooks.** No `--no-verify` on commits or pushes.
   If a hook fails, fix the underlying issue. The pre-push hook runs
   the full CI surface for a reason.
4. **Fail loud.** No silent fallbacks, no `recover()` that swallows
   errors, no "return zero value on failure" paths. Crash with
   context. The user's global feedback file documents an Erlang/OTP
   philosophy: a stack trace is debuggable, a silent wrong answer is
   not.
5. **No mTLS shortcuts.** Identity is deferred to S5–S6
   (`specs/in-process/identity-mtls.md`). Every authenticated
   surface must be designed with an mTLS-shaped seam, even if v1
   ships pre-shared tokens behind it. Adding auth later must not
   require re-architecting.
6. **No managed-service dependencies.** D-08 forbids them. Anything
   you add must ship in the installer bundle.

## Always-on practices

- Prefer editing existing files; do not create new ones unless the
  task plainly requires it.
- Default to writing **no comments**. Add a comment only when the
  *why* is non-obvious (a hidden constraint, a workaround, a subtle
  invariant). Don't restate what the code does. Don't reference the
  current task or commit; that belongs in the PR description.
- **Trust internal code and framework guarantees.** Do not add
  parameter validation or fallbacks for impossible states. Validate
  only at system boundaries (HTTP handlers, MQTT message handlers,
  external APIs).
- **Don't generalize prematurely.** Three similar lines is fine.
  Two different things sharing 80% of their shape is fine. Extract
  only when a third caller has the same shape and the abstraction
  has a name.
- **Tests run on real dependencies where possible.** The buffer test
  uses real SQLite. The store tests should use a real Postgres
  spawned by the lab compose (do not mock pgx).

## Build, test, run

The Makefile is the canonical interface. Discover targets with `make`
or `make help`.

```bash
# Building
make tidy          # go mod tidy across both modules
make build         # builds bin/{controlplane,telemetry-ingest,ota-worker,agent}

# Linting / testing
make lint          # go vet + (optional) staticcheck on both modules
make test          # go test -race -count=1 ./...

# Lab stack (auto-detects docker vs podman)
make container-info        # confirm the engine + compose command
make lab-up                # Postgres + Temporal + MQTT + registry
make lab-status            # probe ports
make lab-down              # stop, keep state
make lab-reset             # stop + wipe state

# Sim (Gazebo + TurtleBot3 + bridge + agent in containers)
make sim-up
make sim-logs
make sim-down

# Protobuf regeneration
make proto         # requires protoc + protoc-gen-go + protoc-gen-go-grpc
```

End-to-end OTA walkthrough lives in `ONBOARDING.md` Section 9.

## Git workflow

- Branch from `main`: `git checkout -b feature/<thing>`.
- Conventional commit messages: `feat:`, `fix:`, `chore:`, `docs:`,
  `refactor:`, `test:`, `ci:`. The pre-commit `commit-msg` hook
  enforces this.
- The remote is **`upstream`** (`git@github.com:ferrosadb/temporal-hack.git`),
  not `origin`. PRs go against `upstream/main`.
- Use the `commit-it` skill (`/commit-it`) for guided, hook-aware
  commits. It checks branch, stages files, runs hooks, drafts the
  message.
- Use `push-it` (`/push-it`) for guided pushes that include the PR
  body template.

## Pre-push CI parity

`pre-commit` is configured so that **everything CI runs locally before
push**:

| CI job             | Local hook stage        |
|--------------------|-------------------------|
| `go cloud`         | pre-commit (vet, fmt) + pre-push (test) |
| `go agent`         | pre-commit (vet, fmt) + pre-push (test) |
| `python-lint`      | pre-commit (ruff)       |
| `installer-smoke`  | pre-push (`installer-smoke` hook → `.git-hooks/installer-smoke.sh`) |

If `pre-push` is slow (~5 min on cold image cache), use:

```bash
SKIP=installer-smoke git push
```

…but expect to fix it on the next push, not skip permanently.

## Skills available in this repo

Run a skill with `/<skill-name>` or invoke the `Skill` tool. The
following are linked under `.claude/skills/`, `.opencode/skills/`,
and `.agents/skills/` (so all three agent harnesses see them):

**Language / runtime**

- `go` — idioms, stdlib patterns, error handling, testing.
- `docker-dev` — container patterns; we use docker XOR podman.
- `ci-cd` — GitHub Actions patterns.

**Architecture and analysis**

- `architect` — ADRs, Mermaid diagrams, specs/ workflow. Run when
  designing a new component.
- `blueprint` — full Phase 0–12 grilling and analysis pipeline. Run
  `/blueprint update` after major code changes to refresh decisions
  and threat model.

**Repo workflow** (always available)

- `commit-it`, `push-it`, `ship-it` — guided git operations.
- `instruct`, `manage-precommit`, `new-project`, `op-init` — meta.

When a skill matches the user's intent, **invoke it** rather than
re-implementing its instructions inline.

## Common tasks

### "Add a new telemetry stream"

1. Decide on the stream name and payload schema; record in
   `specs/adr/` if non-trivial.
2. Add the topic→stream mapping in
   `bridge/bridge_node/server.py::STREAM_TO_TOPIC`.
3. (If a new ROS topic) extend the bridge subscriber.
4. The agent's pump loop will pick up the new stream automatically;
   the topic on MQTT is `tlm/{robot_id}/{stream}`.
5. Update `cloud/internal/api/router.go` if a new operator endpoint
   is needed.

### "Add a new OTA workflow phase"

1. Update `proto/ota.proto` with the new `Phase` enum value.
2. Mirror the JSON form in `agent/internal/ota/types.go::Ack`.
3. Add the phase wait in
   `cloud/internal/ota/workflow_singlerobot.go::OTASingleRobot`.
4. Emit the corresponding `publishAck` from
   `agent/internal/ota/executor.go`.
5. Document in `specs/adr/ADR-008-ota-command-transport.md`.

### "I need a new sprint of work"

`specs/project-plan.md` is the source of truth for sprint scope. The
S5–S6 (identity) gates are non-negotiable before customer ship; do
not start S7+ work without those landed unless the user explicitly
re-prioritizes.

## Anti-patterns reviewers will catch

- Importing `rclpy` from Go-side code. The bridge is the only place
  ROS appears — see ADR-004 and D-06.
- Adding a managed-cloud dependency (S3, RDS, IoT Core, etc.) — D-08
  forbids it.
- Catching all errors with `if err != nil { return nil }` — fail
  loud rule.
- Using `--no-verify` to bypass a hook — never; fix the hook.
- Adding `// removed unused …` comments or backwards-compat shims —
  delete the dead code instead.
- Long docstrings or planning documents committed to the repo —
  intermediate analysis belongs in chat, decisions belong in
  `specs/`.

## When to ask the user

- Anything that touches identity / mTLS / signing — these are S5–S6
  scope and the user has explicit gating policy.
- Adding a new external dependency (Go module, Python package,
  container image) — confirm before adding.
- Changes to `specs/decisions.md` — these are confirmed via the
  grilling process; don't unilaterally amend.
- Anything that would push directly to `main` — the branch hook
  blocks edits there for a reason.

## Out of scope (don't propose)

- Multi-tenancy — v1 is single-customer.
- Public-cloud deployment — v1 is on-prem.
- Mission dispatch — deferred to v1.5.
- Remote teleop beyond best-effort — deferred to v2.
- Sim in CI — `make sim-up` is dev-only.
- Functional safety certification — no ISO 13482 / 10218 in scope.

When the user asks about any of the above, point them at the
relevant decision in `specs/decisions.md` rather than starting work.
