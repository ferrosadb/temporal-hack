---
title: ADR-007 OTA container swap strategy
status: accepted
date: 2026-05-05
related: [D-10, D-14]
executive_summary: |
  v1 swaps containers blue-green: pull → start under temporary name →
  verify running → stop+remove old → rename new to canonical. Rollback
  recreates from the previous image digest. The agent retains the
  previous digest in memory for self-rollback after smoke check
  failure; cloud-initiated rollback uses the same path.
---

# ADR-007 OTA container swap strategy

## Decision

**Blue-green with rename**:

1. `docker pull <ref>`
2. `docker run -d --name robot-app-new <ref>`
3. Inspect `State.Status == "running"`; abort if not
4. `docker rm -f robot-app` (best effort)
5. `docker rename robot-app-new robot-app`

Rollback restores by recreating from the previous digest, which the
agent records in memory at swap time.

## Why not `docker compose up -d --no-deps` style?

The agent runs as a single binary on the robot, not a compose stack.
Binding to a compose tool would impose a deployment-shape decision on
every customer. Direct `docker` CLI is simpler and more debuggable
on-robot.

## Why not pause-and-swap?

Pause-and-swap (briefly hold both containers, then atomic switch) gives
zero data-plane interruption but requires the application to handle
double-binding to the same DDS topic / hardware port. Most ROS 2
applications cannot, so the simplest correct strategy wins.

## Constraints

- Robot application uses one canonical container name (`robot-app`).
- Hardware passthrough (USB, /dev/...) goes via `RunArgs` configured
  in the agent. v1 keeps this static; per-robot run args belong in
  the registry once enrollment lands.
- Smoke check is a command executed inside the new container. If the
  container doesn't have a meaningful smoke command, the operator can
  set `smoke_command=""` and the agent treats that as healthy. This
  is recorded in the rollout spec for audit.

## Open

- **Image signature verification before pull/run** — gated by D-11
  (identity work in S5–S6). v1 trusts TLS to the registry only.
- **Persistent volumes / state migration** — robots that store data
  inside the container will lose it on swap. Robots that store data
  in mounted volumes are fine. Document this for the customer in S8.
