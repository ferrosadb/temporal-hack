---
title: OTA image signature verification before swap
priority: P1
source: build-S4
status: todo
sprint: S6
related_decisions: [D-11]
related_threats: [T-1, E-2]
related_adr: [ADR-007]
date: 2026-05-05
executive_summary: |
  v1 OTA pulls images over TLS to the registry but does not verify
  signatures. T-1 and E-2 are CRITICAL threats that this gap leaves
  unmitigated. Add cosign-style signature verification on the agent
  before `docker run`. Gated behind D-11 identity work because the
  signing key custody story belongs there.
---

# OTA image signature verification

## Why

Threats T-1 (OTA image tampered) and E-2 (compromised image gets
root via Docker daemon) are CRITICAL. v1 trusts the registry's TLS
cert; that's not enough — anyone with registry write access can
push a malicious image. Image signatures verified on the robot
close this.

## What

- Sign images in CI at push time using cosign (or notation; pick one
  in S6).
- Pin a customer-controlled verification key in the agent config (or
  load from filesystem alongside the mTLS bundle).
- Before `docker run` of a new image, the agent verifies the
  signature against the pinned key. Fail closed.
- A new `image_digest` field in the OTA command is checked against
  the verified digest.

## Constraints

- Verification must work offline (robots may be disconnected at OTA
  time). cosign `verify --key` works offline; do not use online
  Rekor lookups in v1.
- Key rotation story: agents must accept signatures from any of N
  configured keys (allow rolling rotation without simultaneous
  reconfiguration).

## Acceptance

- An image signed by the customer key applies; an unsigned or
  wrong-key image fails the rollout with `PHASE_FAILED` and a
  clear `detail` string.
- The integration test in S7 includes a "rogue image" case that
  must fail.
