---
title: ADR-002 Container registry
status: accepted
date: 2026-05-05
related: [D-08, D-10]
executive_summary: |
  v1 ships the upstream `registry:2` (Distribution) container in the
  installer. It is the simplest production-credible OCI registry,
  Apache-2.0, and supports the read/write paths we need without
  introducing a database. Harbor and Zot remain candidates if v1
  reveals a need for image scanning or replication; both are S8 or
  later considerations.
---

# ADR-002 Container registry

## Decision

Adopt **`registry:2`** (CNCF Distribution) for v1.

## Why

- Apache-2.0, no auth-by-default but easy to gate behind mTLS once
  D-11 lands.
- No database dependency — file/blob storage on disk is sufficient
  for a single-customer fleet.
- The official, well-documented OCI registry implementation.

## Why not Harbor?

Harbor adds image scanning, replication, RBAC, and a Postgres
dependency. None of those are v1 needs. The cost (operational
surface, learning curve, additional moving part) is not justified
until S8 or later.

## Why not Zot?

Zot is a credible alternative (Apache-2.0, single binary, good OCI
1.1 support). The chief reason to pick `registry:2` is reach: more
operators have run it in production. We can swap to Zot later if
operations prefer it.

## Consequences

- Installer ships `registry:2` on host port 5001 in lab (maps to container port 5000); production target
  exposes it on a routable address with TLS termination at the
  ingress.
- Image signatures (ADR-007 / `todo/ota-image-signing.md`) verified
  by cosign at the agent, not at the registry. The registry stays
  signature-agnostic.
- No image scanning in v1. Add Trivy in CI or move to Harbor in
  S8 if customer requires it.
