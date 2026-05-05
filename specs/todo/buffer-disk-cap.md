---
title: Bound agent buffer by disk size, not just sample count
priority: P2
source: build-S2
status: todo
sprint: S2
related_decisions: [D-04]
related_threats: [D-3]
date: 2026-05-05
executive_summary: |
  The local SQLite buffer is currently bounded only by sample count
  (default 100k). Variable-size payloads (e.g., diagnostics with
  attached images) can OOM the disk during a long disconnect. Add a
  byte-budget check alongside the row count and prune by oldest until
  both budgets hold.
---

# Bound agent buffer by disk size, not just sample count

## Why

Threat D-3 (robot agent OOM during long disconnect). Today the buffer
caps row count at 100k, but with no payload size cap a few large
samples can fill the disk. The agent must guarantee bounded disk
usage across any disconnect duration.

## What

- Add `MaxBytes` to the buffer config (default 1 GiB).
- On `Append`, after the row-count eviction, evict by oldest until
  total payload size ≤ MaxBytes.
- Track running size in a single-row meta table to avoid full scans.
- Test: append samples summing to > MaxBytes and verify oldest are
  dropped first.

## Acceptance

- `go test ./agent/internal/buffer/...` includes a byte-budget test.
- Manual: agent run with disk near full does not crash; old samples
  are dropped with a single warning log.
