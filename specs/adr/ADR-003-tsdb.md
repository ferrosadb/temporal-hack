---
title: ADR-003 Telemetry storage backend
status: accepted
date: 2026-05-05
deciders: blueprint Phase 0/1 (auto)
related: [D-08]
executive_summary: |
  Adopt TimescaleDB (Postgres extension) as the v1 telemetry store.
  Reuses the Postgres dependency already required by Temporal,
  collapsing the operational surface from two database systems to
  one. Hypertables and retention policies cover v1 query patterns.
  Re-evaluate if cardinality or query latency becomes a problem at
  fleet scale > 500 robots.
---

# ADR-003 Telemetry storage backend

## Context

D-08 (on-prem deployment) forbids managed services. We need a
time-series store on customer hardware. We already require Postgres
for Temporal.

## Options

- **TimescaleDB.** Postgres extension. Hypertables, automatic
  partitioning, native retention. Same backup tooling as Postgres.
- **VictoriaMetrics.** Faster ingest, lower cardinality cost, but
  another database to install, version, and back up.
- **Prometheus + Mimir.** Familiar but cardinality-sensitive and
  long-term storage requires Mimir, which is yet another component.

## Decision

Adopt **TimescaleDB**. Same Postgres instance hosts Temporal databases
and the `telemetry` hypertable.

## Consequences

- Installer ships `timescale/timescaledb-ha` instead of stock Postgres
  in production.
- Data plane writer (telemetry-ingest) does plain SQL `INSERT` — no
  TSDB-specific client needed.
- Retention is handled by Timescale's `add_retention_policy`. Default
  30 days; adjust in S2 once cardinality is measured.
- One database to back up, monitor, and patch. Reduces R-02 (on-prem
  ops tax) by one moving part.

## Status

Accepted. Re-evaluate if measured cardinality at fleet scale > 500
robots blows past hypertable recommendations, or if Temporal's
Postgres needs different tuning than our telemetry workload tolerates.
