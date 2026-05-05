---
title: Project Plan — v1 MVP
status: draft
phase: 6-project-plan
date: 2026-05-05
inputs: [specs/decisions.md, specs/overview.md, specs/threat-model.md]
team_size: 4 (midpoint of 3-5)
duration: 6 months / 13 two-week sprints
executive_summary: |
  Eight-sprint plan for v1 MVP (Telemetry + OTA) plus a buffer sprint,
  fitting within the 6-month / 3–5 engineer envelope from D-16. Sprint
  0 builds the on-prem installer (forced by D-08). Telemetry MVP lands
  in Sprint 2; OTA MVP lands in Sprint 4. Identity (R-01 mitigation,
  P1 gate) starts Sprint 5 and must complete before customer ship.
  Critical-path risk is Sprint 0 (installer effort underestimated)
  and Sprint 5–6 (identity scope expands once we touch every surface).
  Velocity assumes 4 engineers averaging ~80% sprint capacity.
---

# Project Plan

Capacity model: 4 engineers × 13 sprints × 80% productive = **42
engineer-weeks effective.** Plan below allocates ~38 of those, leaving
~4 weeks buffer for incidents and customer-prep churn.

## Sprint map

| # | Sprint | Theme | Duration | Engineers | Status | Outputs |
|---|--------|-------|----------|-----------|--------|---------|
| 0 | S0 | Foundations + installer | 2 wk | 3 | scaffolded | Installer, Go monorepo, CI, Postgres + Temporal + EMQX + registry up in lab |
| 1 | S1 | Telemetry plumbing | 2 wk | 3 | scaffolded (bridge gRPC client TODO) | MQTT broker installed; Go agent; bridge node; stub telemetry path |
| 2 | S2 | Telemetry MVP | 2 wk | 3 | scaffolded | SQLite buffer with WAL; TimescaleDB hypertable; operator read API |
| 3 | S3 | OTA workflow + cohort + commands | 2 wk | 3 | landed | Temporal rollout/single/rollback workflows; MQTT command/ack bridge; cohort resolution; sim container |
| 4 | S4 | OTA MVP — swap + rollback | 2 wk | 4 | landed | Agent docker swap (blue-green); per-phase acks; canary→25%→rest; failure budget |
| 5 | S5 | Identity foundation | 2 wk | 2 | pending | mTLS to broker; control-plane mTLS; certificate issuance flow |
| 6 | S6 | Identity completion | 2 wk | 2 | pending | Hardware-backed key story; image signing + verification; cert rotation |
| 7 | S7 | Lab fleet integration | 2 wk | 4 | pending | Run on 5–10 robot lab fleet; harden under intermittent network; runbooks |
| 8 | S8 | Customer-prep hardening | 2 wk | 3 | pending | Installer polish; HA topology docs; DR runbook; first customer onboarding plan |
| 9–12 | (buffer) | Slack | 8 wk | varies | — | Incident response, mission-dispatch v1.5 prep, contingency |

S5 + S6 = 4 weeks identity work. This matches the 2-sprint estimate in
`todo/identity-mtls.md`. Identity MUST close before any customer ship
(see threat model — six CRITICAL/HIGH threats depend on it).

## Priority tiers (from blueprint instructions)

### Priority 1 — Sprint 0–4

Critical threats and core MVP functionality:
- S0: Installer (R-02 mitigation; without it, nothing ships)
- S1–S2: Telemetry MVP (D-02 scope)
- S3–S4: OTA MVP (D-02 scope)
- All Sprint 4 outputs gated by:
  - Telemetry: 95% delivery under 8h disconnect / 5min reconnect window
  - OTA: 100% rollback success on simulated update failure

### Priority 2 — Sprint 5–6

R-01 mitigation. Cannot ship without:
- mTLS on every authenticated surface (T-1, S-1, S-2, I-2)
- Image signature verification (T-1, E-2)
- Cert rotation tested without robot downtime
- Operator audit log (R-1) — folded in here

### Priority 3 — Sprint 7–8

Customer prep:
- Lab fleet integration test
- Installer polish + HA docs (R-04 closes)
- DR runbook
- First-customer onboarding playbook

### Priority 4 — Backlog (sprint 9+)

- Mission dispatch (v1.5)
- Multi-robot coordination
- Sim-in-CI investment
- Web UI beyond operator dashboard
- Telemetry retention / cardinality optimization

## Threat-model item coverage

| Threat | Severity | Sprint mitigated |
|--------|----------|------------------|
| S-1 (robot impersonation) | CRITICAL | S5–S6 |
| S-2 (control-plane impersonation) | CRITICAL | S5–S6 |
| T-1 (OTA tamper) | CRITICAL | S6 |
| E-1 (agent container escape) | CRITICAL | S4 (AppArmor profile) + S6 (signed images) |
| E-2 (image RCE) | CRITICAL | S6 |
| S-3 (operator impersonation) | HIGH | S5 |
| T-3 (agent binary tamper) | HIGH | Out of v1 (requires secure boot) |
| T-4 (Temporal workflow tamper) | HIGH | S0 (namespace ACL) |
| I-1 (telemetry disclosure) | HIGH | S5 (TLS) + S2 (data classification) |
| I-2 (image disclosure) | HIGH | S6 (registry auth) |
| D-1 (broker DoS) | HIGH | S2 (rate limits) |
| E-3 (API authz bypass) | HIGH | S0 + S5 (authz tests) |
| MEDIUM/LOW | | Backlog |

## Risks (carrying R-01..R-06 forward)

| Risk | This plan's response |
|------|----------------------|
| R-01 Identity deferred | S5–S6 dedicated; non-negotiable before customer ship |
| R-02 On-prem ops tax | S0 + S8 absorb the installer + DR cost |
| R-03 Phobos maintenance | Out of plan scope (developer toolchain only); pin versions |
| R-04 MQTT broker durability | ADR-001 in S1; HA pattern documented in S8 |
| R-05 Customer DC connectivity | Confirm at first customer engagement (S8) |
| R-06 Rust→Go skill swap | Resolved in Phase 0 |

## Plan-level risks (new, this phase)

| ID | Risk | Severity | Mitigation |
|----|------|----------|------------|
| PR-01 | Sprint 0 installer estimate too small | HIGH | Re-evaluate at end of S0; if slipping, push S2 by one sprint |
| PR-02 | Identity scope expands when touched | HIGH | S5–S6 has firm scope (mTLS + signing + rotation only); SCEP / ACME punt to v1.5 if needed |
| PR-03 | Lab-fleet bring-up reveals undocumented robot variance | MEDIUM | S7 sized 2 weeks specifically as integration sprint |
| PR-04 | Phase 0 assumed connectivity defaults wrong | MEDIUM | If first customer is air-gapped, add 1-sprint cost for offline artifact bundles |

## Deliverable per sprint

S0: 
- repo skeleton with linting + unit test scaffolding
- installer reproducibly brings up Postgres + Temporal + MQTT broker + registry on a 3-node lab cluster
- CI pipeline runs on PRs

S1:
- Robot agent connects to MQTT broker, sends a single heartbeat
- Bridge node subscribes to a single ROS 2 topic and republishes via gRPC

S2:
- Agent buffers telemetry across simulated 30-min disconnect with 100% delivery
- Operator dashboard shows telemetry from one robot

S3:
- Image push to registry; agent pulls + caches
- Temporal workflow can issue a "send command to robot N" and receive ack

S4:
- Single-robot OTA: pull → swap → smoke check → ack
- Cohort rollout: canary 1 robot, then 25%, then 100%; rollback on failure

S5:
- mTLS bootstrap (PSK → cert exchange) functional in lab
- Control plane and broker enforce mTLS

S6:
- Image signing in CI; agent verifies before apply
- Cert rotation script + automation; tested without downtime

S7:
- 5–10 robots running v1 build for 1 week with simulated network interruption
- All HIGH/CRITICAL threats verified mitigated

S8:
- HA + backup + DR runbooks reviewed
- Installer documented for customer's SRE
- First-customer onboarding playbook complete

## Stop conditions for v1 ship

Customer deployment **must not** proceed until:

1. All sprints 0–7 outputs verified
2. R-01 (identity) and all CRITICAL threats mitigated
3. Lab fleet has run for 1 continuous week with simulated network interruption and 0 unrecovered failures
4. Customer-side runbook exists for incidents we cannot remote-debug

## Phase 6 status

This plan is the Phase 6 deliverable. It is consistent with Phase 0
decisions and Phase 3 threats. It does not yet incorporate Phase 2
(DSM) or Phase 4 (FMEA) findings; both are scheduled to run after
S1–S2 produce concrete components to analyze. Re-run the relevant
phases at end of S2 and update this plan if findings change priority.
