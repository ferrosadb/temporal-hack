---
title: Identity & enrollment — mTLS with hardware-backed keys
priority: P1
source: grill-me
status: todo
gate: pre-customer-deployment
phase_origin: 0
related_decisions: [D-11]
related_risks: [R-01]
date: 2026-05-05
executive_summary: |
  Production identity (mTLS with TPM/SE-backed keys) must replace the v1
  pre-shared-token bootstrap before any customer deployment. Touches
  every authenticated surface: MQTT broker auth, OTA image signing,
  agent enrollment, telemetry ingest. Deferred during Phase 0 grilling;
  recorded as HIGH risk.
---

# Identity & enrollment — mTLS with hardware-backed keys

## Why this is P1

Phase 0 deferred identity in favor of pre-shared tokens to accelerate
v1. Every protocol surface (MQTT auth, OTA image signing, agent
enrollment, telemetry ingest, future teleop signaling) needs a
production identity story before customer deployment. Auth retrofit
costs grow non-linearly: each authenticated surface needs migration,
each version of the protocol needs an upgrade story, and deployed
robots in the field with weak auth are an active risk.

## Hard gate

This work item must be **closed before any customer deployment.**
Development and lab use can proceed on pre-shared tokens. Any production
ship without this resolved is an unauthorized deployment.

## Scope

- Robot identity model: per-robot X.509 cert with key in TPM or
  hardware secure element. Document fallback for boards without
  hardware roots of trust.
- Cloud identity model: control-plane services authenticate to each
  other and to the broker via mTLS.
- Enrollment flow: how a new robot gets its first cert. Options to
  evaluate: SCEP, ACME for IoT, custom enrollment endpoint signed by
  customer-controlled CA.
- Cert rotation policy: lifetime, renewal cadence, revocation handling.
- MQTT broker integration: mTLS authentication for clients.
- OTA artifact signing: image signatures verified by agent before
  apply; key custody.
- Telemetry ingest: pipeline authenticates writers.

## Constraints

- Customer runs their own PKI in their DC (D-08). Plan for customer-
  controlled CA, not a vendor-provided one.
- Robot OS is Ubuntu 22.04 (D-14); hardware roots vary by chassis.
- Connectivity is intermittent (D-04); enrollment cannot assume
  always-on.

## Acceptance

- mTLS enforced on every authenticated surface in the architecture.
- Enrollment flow documented and exercised end-to-end at lab.
- Cert rotation tested without robot downtime.
- OTA artifact verification tested with rotated keys.
- Threat model updated; revisit Phase 3 deliverable.

## Estimated effort

~2 sprints (2 engineers × 4 weeks). Cost-of-delay grows with each
authenticated surface added in v1.
