---
title: Phase 0 Decision Record
status: confirmed
phase: 0-grill-me
date: 2026-05-05
executive_summary: |
  Sixteen architectural decisions confirmed through Phase 0 grilling for a
  robotics fleet management platform. Stack: ROS 2 on robot, Go for control
  plane and robot agent, self-hosted Temporal in customer DC, MQTT for
  telemetry transport, Docker/OCI images for OTA. v1 MVP is Telemetry + OTA
  only; mission dispatch and teleop are deferred. Six risks tracked, two
  HIGH (identity deferral, on-prem ops cost). Two assumptions defaulted
  (customer DC connectivity, fleet topology) pending first-customer signal.
---

# Phase 0 Decision Record

This file is the authoritative record of decisions confirmed during the
Phase 0 plan interrogation. Subsequent phases consume these as
pre-resolved items; do not re-litigate without amending this file.

## Confirmed decisions

### D-01: Robot middleware
**Decision:** ROS 2 (Humble or Jazzy LTS).
**Rationale:** ROS 1 (Noetic) reached EOL May 2025. Any new platform must
target ROS 2's DDS-based stack, lifecycle nodes, and SROS2 security model.
**Implications:** rclcpp / rclpy on robot; DDS discovery and QoS apply;
SROS2 available when identity work begins.

### D-02: v1 MVP scope
**Decision:** **Telemetry + OTA**. Mission dispatch deferred to v1.5;
remote teleop deferred to v2 as best-effort-when-connected.
**Rationale:** Four-subsystem v1 (telemetry, OTA, mission, teleop) is a
12+ engineer-month effort. Team capacity is 3–5 engineers / 6 months.
Telemetry establishes observability; OTA establishes update capability.
Mission dispatch and teleop build on both.
**Implications:** Sprint 1–3 focus is telemetry pipeline + OTA orchestrator.
Mission dispatch and teleop are explicit non-goals for v1.

### D-03: Users and scale
**Decision:** Single-customer SaaS-equivalent: 10–100 robots, production
SLA, 12-month target.
**Rationale:** Scope clarified during grilling.
**Implications:** No multi-tenancy in v1. Architecture must still support
RBAC for customer's internal operators.

### D-04: Connectivity model
**Decision:** Intermittent / store-and-forward as the steady-state
assumption. Teleop is best-effort when connectivity is good.
**Rationale:** Robots run autonomously; cloud is consulted opportunistically.
**Implications:** Robot agent buffers telemetry locally during disconnect;
MQTT QoS 1/2 with persistent sessions; Temporal workflows orchestrate
cloud-side state machines, not direct robot calls.

### D-05: Cloud Temporal worker language
**Decision:** **Go** (revised from Rust during Phase 0).
**Rationale:** Temporal's Go SDK is first-class. The Rust SDK is community,
pre-1.0, with feature gaps. Go removes a HIGH risk identified mid-grill.
**Implications:** Project skill swapped from `rust` to `go`. Cloud control
plane and Temporal workers are Go.

### D-06: Robot agent language
**Decision:** **Go**, with a thin ROS 2 bridge node in Python or C++.
**Rationale:** "All in Go" was the user preference, but Go cannot natively
be a production-credible ROS 2 client (rclgo is sparsely maintained).
The pragmatic split: Go agent handles cloud transport, OTA, telemetry
buffering; a small ROS 2 bridge node (Python or C++) subscribes to DDS
topics and republishes via gRPC over Unix domain socket.
**Implications:** Two processes on the robot. Bridge node ships in same
container or sibling container; defined gRPC contract between agent and
bridge; agent never imports ROS.

### D-07: Temporal deployment
**Decision:** Self-hosted Temporal cluster in the customer DC.
**Rationale:** Customer DC deployment (D-08) precludes managed Temporal
Cloud. Postgres or Cassandra backend; we operate frontend, history,
matching services.
**Implications:** ~0.25 FTE ongoing ops. Upgrade story must be part of
the platform installer. Document HA topology and shard count from the
start.

### D-08: Cloud provider
**Decision:** None. Deployment target is the customer's data center.
**Rationale:** Single-customer scope (D-03) and customer requirement.
**Implications:** No managed services available (RDS, S3, IoT Core, etc.).
Every dependency ships in our installer bundle. The platform itself
needs an update mechanism. Disaster recovery runbook is our spec to
write.

### D-09: Telemetry transport
**Decision:** MQTT broker (cloud-side, persistent sessions, QoS 1/2).
**Rationale:** MQTT is the standard for intermittent / unreliable links
in IoT; persistent sessions and queued QoS deliver store-and-forward
semantics natively.
**Implications:** Pick a battle-tested broker (EMQX or VerneMQ
candidates). Bridge MQTT to TSDB and to Temporal where workflow events
are needed. Document HA / clustering pattern for the broker.

### D-10: OTA artifact format
**Decision:** Container images (Docker / OCI).
**Rationale:** Atomic rollback, layer caching, mature tooling.
**Implications:** Robot runs a container runtime (Docker per D-14).
Customer DC hosts a container registry. Robot agent pulls images,
swaps containers, validates health, rolls back on failure. Image
signing is part of the deferred identity work (D-11).

### D-11: Identity and enrollment
**Decision:** **DEFERRED.** Pre-shared tokens for development; full
mTLS with hardware-backed keys (TPM or secure element) **before any
customer deployment.**
**Rationale:** User chose to defer despite explicit warning. Recorded
as a HIGH risk and a P1 work item in `todo/`.
**Implications:** This is a hard gate before production. Auth retrofit
touches MQTT broker auth, OTA artifact signing, agent enrollment,
teleop session signaling, telemetry ingest. Cost-of-delay is real; do
not accept v1 sign-off without this item closed.

### D-12: URDF toolchain
**Decision:** Blender + Phobos plugin. Pin known-good Blender / Phobos
version pair.
**Rationale:** Phobos is the standard Blender → URDF flow; team is
Blender-fluent.
**Implications:** Phobos maintenance is uneven (last meaningful release
~2022). MEDIUM risk recorded. Reserve engineering capacity to fork
Phobos if upstream stalls. Document the blessed Blender version in
the developer setup guide.

### D-13: Simulator role
**Decision:** Gazebo for developer workstation iteration only.
**Rationale:** No CI sim, no headless sim farm, no synthetic data
pipeline in v1.
**Implications:** Significant de-scope. Provide a known-good URDF +
world file in the repo; document `make sim` workflow; no GPU pool
or sim-in-the-loop CI investment.

### D-14: Robot OS / runtime baseline
**Decision:** Ubuntu 22.04 + Docker.
**Rationale:** ROS 2 Humble's reference OS; matches D-10 (container OTA).
**Implications:** Docker daemon is a dependency; robot agent has
permission to pull and swap containers; logs and metrics from container
runtime feed telemetry pipeline.

### D-15: Safety classification
**Decision:** None (R&D / soft real-time).
**Rationale:** No ISO 13482, ISO 10218, or similar regulated context in v1.
**Implications:** No formal V-model, no certified components, no audit
trail requirements. Reassess if customer use case changes.

### D-16: Team and timeline
**Decision:** 3–5 engineers, 6 months for v1 MVP (Telemetry + OTA).
**Rationale:** Confirmed during grilling.
**Implications:** Project plan must be ruthlessly scoped. Sprint 0 is
installer + foundations (no avoidable scope). Mission dispatch and
teleop are explicit v1.5 / v2 items, not v1.

## Defaulted assumptions

These were not confirmed during grilling and should be revisited at
first customer engagement.

### A-01: Customer DC connectivity (default: networked egress-only)
The customer DC has outbound internet access for fetching updates
from us, but we do not have inbound access from our office. Architecture
additionally tolerates fully air-gapped operation (signed offline
artifact bundles, manual install media), but air-gap support is not
built in v1 unless required.

### A-02: Fleet topology (default: variable)
Architecture supports both single-site (broker co-located with robots)
and multi-site (broker at central DC, optional site-local relays).
Robot agent connects to the closest available broker.

## Risk register (preliminary)

| ID | Risk | Severity | Owner | Mitigation |
|----|------|----------|-------|------------|
| R-01 | Identity deferred (D-11) | HIGH | TBD | Hard gate before any customer deploy; P1 work item; tracked in `todo/` |
| R-02 | On-prem ops tax (D-08) | HIGH | TBD | Sprint 0 installer; budget +30% eng time for ops surface |
| R-03 | Phobos plugin maintenance (D-12) | MEDIUM | TBD | Pin versions; reserve fork capacity |
| R-04 | MQTT broker durability on customer infra (D-09) | MEDIUM | TBD | Pick EMQX or VerneMQ; document HA topology |
| R-05 | Customer DC connectivity unconfirmed (A-01) | MEDIUM | TBD | Confirm at first customer engagement; design accommodates air-gap if needed |
| R-06 | rust skill replaced with go (D-05) | LOW | resolved | Skill swapped; no further action |

## Decisions deferred to later phases

- Specific MQTT broker vendor (EMQX vs VerneMQ vs Mosquitto). Phase 2.
- Telemetry storage backend (TimescaleDB / VictoriaMetrics / Prometheus). Phase 2.
- Container registry choice (Harbor / Distribution / Zot). Phase 2.
- Web UI scope and stack. Out of v1 MVP; revisit Phase 6.
- Multi-robot coordination model. Out of v1; defer to v1.5 with mission dispatch.
- Update cadence and rollout policy. Phase 2 / Phase 6.
