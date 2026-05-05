---
title: Threat Model — STRIDE
status: draft
phase: 3-threat-model
date: 2026-05-05
inputs: [specs/decisions.md, specs/overview.md]
methodology: STRIDE
executive_summary: |
  STRIDE-based threat model for v1 architecture. Eight trust boundaries
  identified; the most consequential are robot↔cloud (network) and
  control-plane↔Docker-daemon (root-on-robot). Identity deferral
  (D-11) drives nearly every CRITICAL and HIGH threat; mitigations
  pointing at "identity work" cluster on the same P1 work item. Twenty
  threats catalogued; six are CRITICAL or HIGH and constitute the
  must-resolve set before customer deployment.
---

# Threat Model

## Trust boundaries

| ID | Boundary | Crosses |
|----|----------|---------|
| TB-1 | Internet ↔ customer DC ingress | Network (potentially hostile) |
| TB-2 | Customer DC ↔ robot fleet | Customer-managed network |
| TB-3 | Robot agent ↔ MQTT broker | TB-2 |
| TB-4 | Robot agent ↔ ROS 2 bridge | Local IPC (Unix socket / gRPC) |
| TB-5 | ROS 2 bridge ↔ DDS | Local network or shared memory |
| TB-6 | Robot agent ↔ Docker daemon | Local socket (privileged) |
| TB-7 | Operator ↔ control plane API | TB-1 |
| TB-8 | Control plane ↔ Temporal frontend | Internal customer DC network |

## Threats

CRITICAL means immediate ownership of robot or fleet. HIGH means
significant operational or data impact. MEDIUM is recoverable. LOW is
nuisance-class.

### Spoofing (S)

| ID | Threat | Surface | Severity | Mitigation |
|----|--------|---------|----------|------------|
| S-1 | Attacker impersonates robot to broker | TB-3 | **CRITICAL** | mTLS robot→broker (gated by D-11); v1 PSK is dev-only |
| S-2 | Attacker impersonates control plane to robot | TB-3 | **CRITICAL** | Robot pins broker cert; image signatures verified before apply |
| S-3 | Attacker impersonates operator | TB-7 | HIGH | Control plane requires authenticated session; SSO recommended |
| S-4 | Rogue ROS node on robot publishes fake telemetry to bridge | TB-5 | MEDIUM | SROS2 inside robot if customer enables; v1 trusts local DDS |

### Tampering (T)

| ID | Threat | Surface | Severity | Mitigation |
|----|--------|---------|----------|------------|
| T-1 | OTA image tampered in transit or at rest | TB-1, registry | **CRITICAL** | Signed images; agent verifies signature with customer-controlled key (gated by D-11) |
| T-2 | Telemetry mutated in flight | TB-3 | MEDIUM | TLS to broker; broker auth limits write claims |
| T-3 | Robot agent binary tampered on disk | Robot OS | HIGH | Secure boot / verified boot recommended; not in v1 (no safety class per D-15) |
| T-4 | Temporal workflow definitions tampered | TB-8 | HIGH | Workflow code review; namespace-level access control |

### Repudiation (R)

| ID | Threat | Surface | Severity | Mitigation |
|----|--------|---------|----------|------------|
| R-1 | Operator denies issuing OTA rollout | TB-7 | MEDIUM | Audit log on every API mutation; sign with operator identity |
| R-2 | Robot denies receiving update | TB-3 | LOW | MQTT QoS 1/2 semantics + Temporal workflow record provide an audit trail |

### Information disclosure (I)

| ID | Threat | Surface | Severity | Mitigation |
|----|--------|---------|----------|------------|
| I-1 | Telemetry leaks sensitive operational data over network | TB-3 | HIGH | TLS to broker; data classification before v1 ship |
| I-2 | Container image leaks customer code | TB-1, registry | HIGH | Registry auth (gated by D-11); private registry only |
| I-3 | Logs leak credentials | Cloud control plane | MEDIUM | Log scrubber; no secrets in URLs / queries |
| I-4 | DDS topics readable by other LAN devices | TB-5 | MEDIUM | Customer-managed; SROS2 is customer's call |

### Denial of service (D)

| ID | Threat | Surface | Severity | Mitigation |
|----|--------|---------|----------|------------|
| D-1 | MQTT broker overload (too many connections / topics) | TB-3 | HIGH | Connection / message rate limits; broker resource sizing in installer |
| D-2 | Temporal task queue starvation | TB-8 | MEDIUM | Per-namespace rate limits; alert on backlog |
| D-3 | Robot agent OOM during long disconnect (telemetry buffer) | Robot | MEDIUM | Bounded ring buffer with disk cap; drop-oldest policy |
| D-4 | OTA image too large to transfer over slow link | Robot | LOW | Per-cohort size budget; resumable downloads |

### Elevation of privilege (E)

| ID | Threat | Surface | Severity | Mitigation |
|----|--------|---------|----------|------------|
| E-1 | Robot agent escapes container to host | Robot OS | **CRITICAL** | Agent runs unprivileged; Docker socket access is the riskiest single surface — restrict via auditd + AppArmor profile |
| E-2 | Compromised image gets root via Docker daemon | Robot | **CRITICAL** | Signed images verified before apply (T-1 mitigation); image scanning in CI |
| E-3 | Control-plane API authz bypass | TB-7 | HIGH | RBAC at API; deny-by-default; integration tests for authz |
| E-4 | Temporal worker assumes more permissions than needed | TB-8 | MEDIUM | Per-workflow scoped credentials |

## Surface rankings (by aggregated risk)

1. **Robot ↔ broker (TB-3) and image-signing chain** — the path that
   gets root on every robot in the fleet. S-1, S-2, T-1, E-1, E-2.
   This is the v1 attack surface.
2. **Operator ↔ control plane (TB-7)** — single point that can issue
   destructive rollouts. S-3, R-1, E-3.
3. **DC ingress (TB-1)** — customer-managed but ours to design for.
4. Everything else is downstream.

## Identity deferral cost

Six of twenty threats are CRITICAL or HIGH and have mitigations that
point at the deferred identity work (D-11): **S-1, S-2, T-1, T-3, I-1,
I-2.** The threat model does not validate v1 ship to a customer
without that work item closed. This confirms R-01 in `decisions.md`.

## Work items emitted

| File | From | Priority |
|------|------|----------|
| `todo/identity-mtls.md` | (existing, expanded by this phase) | P1 |
| `todo/operator-audit-log.md` | R-1 | P2 |
| `todo/agent-apparmor-profile.md` | E-1 | P2 |
| `todo/image-signature-verification.md` | T-1, E-2 | P1 (subset of identity work) |
| `todo/broker-rate-limits.md` | D-1 | P2 |

These will be created when starting Phase 6 (project plan), so each
sprint can claim them.

## Phase 4 feed (FMEA)

The threat model identifies *adversarial* failure modes. Phase 4
(FMEA) will identify *operational* failure modes. Combined, they form
the test target list for Phase 7. Phase 4 is deferred per the
recommendation in `overview.md` — it is most useful after first sprint
yields concrete components.
