---
title: Swap docker-compose Postgres image for timescaledb-ha
priority: P2
source: build-S0
status: todo
sprint: S0
related_decisions: [D-08]
related_adr: [ADR-003]
date: 2026-05-05
executive_summary: |
  Lab compose currently uses stock postgres:15-alpine. The TimescaleDB
  init script silently no-ops when the extension is missing, which
  hides the production gap. Swap to `timescale/timescaledb-ha` and
  verify hypertable creation succeeds at compose-up.
---

# Swap docker-compose Postgres image for timescaledb-ha

## Why

The lab stack currently runs vanilla Postgres. `init-telemetry.sql`
attempts `CREATE EXTENSION IF NOT EXISTS timescaledb`, which succeeds
on a TimescaleDB image and silently no-ops on stock Postgres. That
gap will surface in production when the hypertable insert path
diverges from what we tested in lab.

## What

- Change `installer/docker-compose/docker-compose.yml`:
  - `postgres.image: timescale/timescaledb-ha:pg15-latest`
- Re-run `make lab-reset && make lab-up` and verify:
  - `psql ... -d telemetry -c '\\dx timescaledb'` shows the extension
  - The `telemetry` table is registered as a hypertable

## Acceptance

`SELECT * FROM timescaledb_information.hypertables;` returns one row
named `telemetry` after a fresh `lab-reset`.
