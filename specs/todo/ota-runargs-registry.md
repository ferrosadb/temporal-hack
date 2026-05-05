---
title: Per-robot OTA RunArgs in the fleet registry
priority: P2
source: build-S4
status: todo
sprint: S5
related_adr: [ADR-007]
date: 2026-05-05
executive_summary: |
  The agent currently has empty `DockerCLI.RunArgs`. Real robots need
  `--device`, `--privileged` (or specific caps), volume mounts, and
  network mode. Static-in-the-binary doesn't scale. Move per-robot
  run args into the fleet registry (single source of truth) and have
  the agent fetch them at startup.
---

# Per-robot OTA RunArgs in the fleet registry

## Why

`DockerCLI.RunArgs` is empty in v1. As soon as we OTA a robot that
needs `/dev/ttyUSB0`, a host-mounted ROS workspace, or `host`
networking, the swap will boot a container that can't talk to its
hardware.

## What

- Schema: extend the fleet registry with a `run_args` JSONB column
  per robot (or per robot class).
- API: agent fetches its run args at startup and on heartbeat (so
  changes propagate without restart).
- The OTA executor uses the fetched args at swap time.

## Constraints

- Must support per-robot overrides AND per-class defaults.
- Customer operators must be able to edit run args from the
  control plane (this implies a write API, currently absent).
- Audit log every change (R-1 mitigation).

## Acceptance

- A rollout to two robots with different run_args produces correctly-
  configured containers on each.
- Changing run_args mid-fleet without restarting any agent is
  observable in the heartbeat-reported config version.
