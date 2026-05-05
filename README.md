# temporal-hack

Robotics fleet management platform — Telemetry + OTA MVP, on-prem at
customer DC, ROS 2 + Temporal + MQTT.

This is the v1 codebase. The architecture, decision record, and
project plan live in [`specs/`](specs/). Read those first.

## Layout

```
cloud/        Go control plane (API + telemetry-ingest)
agent/        Go robot agent (MQTT publisher + local SQLite buffer)
bridge/       Python ROS 2 bridge node (DDS → gRPC for the agent)
proto/        protobuf contracts (telemetry + agent↔bridge)
installer/    docker-compose (lab) and helm (prod stub)
deploy/       service config baked into the installer
specs/        blueprint artifacts (decisions, threats, plan, ADRs)
ops/          runbooks
```

## Quickstart (lab)

Requires: Go 1.22+, Docker with compose, Python 3.10+ (for the bridge).

```bash
# 1) Bring up the lab stack: Postgres + Temporal + EMQX + registry
make lab-up
make lab-status

# 2) Build the Go binaries
make build

# 3) Run the telemetry ingester (separate terminal)
TSDB_DSN="postgres://temporal:temporal@localhost:5432/telemetry?sslmode=disable" \
  ./bin/telemetry-ingest

# 4) Run an agent (separate terminal)
ROBOT_ID=lab-robot-01 ./bin/agent

# 5) Run the control plane API (separate terminal)
./bin/controlplane

# 6) Query telemetry
curl -s http://localhost:8081/v1/robots
curl -s "http://localhost:8081/v1/robots/lab-robot-01/telemetry?stream=battery&limit=20"
```

The bridge node requires a working ROS 2 install. For S1/S2 you can
exercise the data path without it — the agent's ingest loop falls
back to a stub source when the bridge is unreachable.

## Sprint status

| Sprint | Theme | Status |
|--------|-------|--------|
| S0 | Foundations + installer | scaffolding landed; needs hands-on lab bring-up verification |
| S1 | Telemetry plumbing | agent + ingester wired through MQTT; bridge stubbed |
| S2 | Telemetry MVP | TSDB integration + read API present; durability test pending |

See [`specs/project-plan.md`](specs/project-plan.md) for the full plan.
