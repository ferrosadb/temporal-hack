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

Default ports:

| Service           | Port  |
|-------------------|-------|
| Postgres          | 5432  |
| Temporal frontend | 7233  |
| Temporal UI       | 8080  |
| MQTT              | 1883  |
| MQTT dashboard    | 18083 |
| Registry          | 5000  |

## Production-target gaps (tracked, not v1)

| Gap | When | Notes |
|-----|------|-------|
| Helm chart for multi-node deploy | S8 | Maps the compose services 1:1 to Kubernetes resources |
| TLS termination | S5 (with identity) | Today everything is plaintext on a flat network |
| HA topology for EMQX | S8 | EMQX cluster, persistent session shard config |
| HA topology for Postgres | S8 | Patroni or Crunchy operator candidate |
| Backup / DR runbook | S8 | See `ops/runbook-dr.md` |
