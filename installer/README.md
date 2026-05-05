# Installer

Two delivery targets share the same parameter set:

- `docker-compose/` — single-host lab bring-up (S0 acceptance target).
- `helm/` — production Helm chart (target customer DC; **stub** in S0).

The `helm/` directory is intentionally empty in S0. Sprint 8
(customer-prep hardening) is when it gets fleshed out. In S0 the only
goal is for `docker compose up -d` from `docker-compose/` to bring up
Postgres + Temporal + EMQX + a registry, and for `make lab-status`
to report all four healthy.

## Lab quickstart

```bash
make lab-up
make lab-status   # all four services should report up
make lab-down     # stop containers, keep state
make lab-reset    # stop + wipe state
```

## CI / smoke cluster (alternate ports)

```bash
make ci-up        # full stack on the 2xxxx port range
make ci-status
make ci-down      # tears down AND wipes state
```

`make ci-up` waits for every healthcheck to pass before returning
(uses `compose up -d --wait`). The pre-push installer-smoke hook
calls this target.

## Container engine

`make` auto-detects `docker` or `podman` (in that order) and picks the
right compose command. Inspect the choice with:

```bash
make container-info
```

Force a specific engine when both are installed:

```bash
make lab-up CONTAINER=podman
```

### Podman notes

- **Rootless** is the default. The container socket is at
  `$XDG_RUNTIME_DIR/podman/podman.sock`. The Makefile passes this to
  `docker-compose.sim.yml` via the `CONTAINER_SOCK` env so the agent
  container can OTA sibling containers.
- Enable the API socket:
  `systemctl --user enable --now podman.socket`
- If `podman compose` is not available (older builds), the Makefile
  falls back to `podman-compose` (`pip install podman-compose`).
- Some compose features used by `temporalio/auto-setup` (named
  healthcheck dependencies) require `podman ≥ 4.4`.

Default ports:

| Service           | Lab port | CI port |
|-------------------|----------|---------|
| Postgres          | 14432    | 25432   |
| Temporal frontend | 14733    | 27233   |
| Temporal UI       | 14080    | 28080   |
| MQTT              | 14883    | 21883   |
| MQTT dashboard    | 14093    | 28083   |
| Registry          | 14050    | 25050   |

The two clusters run under separate Compose project names
(`temporal-hack-lab` and `temporal-hack-ci`) so they can be brought up
**simultaneously**. `make ci-up` is what the GitHub Actions
`installer-smoke` job and the local `pre-push` hook both run.

### Sim-only ports (only `make sim-up`, not `make lab-up`)

| Service               | Port  |
|-----------------------|-------|
| Gazebo noVNC          | 14680 |
| Gazebo VNC            | 14900 |
| Robot bridge gRPC     | 50051 |

### Host-side processes (Go binaries via `make`, not in compose)

| Process       | Port  |
|---------------|-------|
| Control plane | 8081  |
| ota-worker    | (none, connects to Temporal :14733) |
| collision-worker | (none, connects to Temporal :14733) |
| agent         | (none, connects to MQTT :14883 + bridge :50051) |

## Production-target gaps (tracked, not v1)

| Gap | When | Notes |
|-----|------|-------|
| Helm chart for multi-node deploy | S8 | Maps the compose services 1:1 to Kubernetes resources |
| TLS termination | S5 (with identity) | Today everything is plaintext on a flat network |
| HA topology for EMQX | S8 | EMQX cluster, persistent session shard config |
| HA topology for Postgres | S8 | Patroni or Crunchy operator candidate |
| Backup / DR runbook | S8 | See `ops/runbook-dr.md` |
