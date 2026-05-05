---
title: ADR-001 MQTT broker selection
status: accepted
date: 2026-05-05
deciders: blueprint Phase 0/1 (auto)
related: [D-09, R-04]
executive_summary: |
  Adopt EMQX as the v1 MQTT broker. Open-source edition is sufficient
  for v1 scale (10–100 robots), supports persistent sessions backed by
  RocksDB, has native clustering for the S8 production target, and
  ships an inspectable dashboard useful in lab. VerneMQ is the closest
  runner-up; reconsidered only if EMQX licensing posture changes or
  production load reveals a clear deficiency.
---

# ADR-001 MQTT broker selection

## Context

D-09 selected MQTT as the telemetry transport. Phase 1 left the
specific broker open. v1 broker requirements:

- Persistent sessions for store-and-forward (D-04 connectivity model)
- QoS 1 with broker-side durable queueing
- Native clustering / HA (S8 production target; D-08 on-prem)
- No managed-service dependency
- Inspectable for ops staff

## Options

- **EMQX 5.x (Apache-2.0).** Persistent sessions via RocksDB, clustered
  by default, dashboard, MQTT 5 features, large operator base.
- **VerneMQ.** Apache-2.0, Erlang-based, mature, smaller dashboard
  surface, less active community.
- **Mosquitto cluster.** Simpler single-node story, no first-class
  clustering — typically paired with HAProxy + a queue.
- **HiveMQ Community.** Apache-2.0 community edition; enterprise
  features behind a paid license. Licensing posture is a v1 risk.

## Decision

Adopt **EMQX 5.7.x**. The clustering story matches the S8 production
target without re-architecting; persistent sessions are first-class;
the dashboard is operationally useful from day one.

## Consequences

- v1 install bundle ships an EMQX container.
- HA topology in S8 uses EMQX clustering rather than active-passive.
- Auth integration (S5–S6) uses EMQX's built-in mTLS plus an
  authenticator plugin against the customer-controlled CA.
- Session storage on local disk: account for ~1 GB per 1k robots in
  installer sizing (R-04).

## Status

Accepted. Re-evaluate if (a) EMQX licensing changes (Apache-2.0
revocation in core), or (b) S2 measured load reveals a sizing problem
the broker cannot recover from with vertical scaling.
