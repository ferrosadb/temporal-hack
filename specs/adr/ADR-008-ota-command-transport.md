---
title: ADR-008 OTA command/ack transport
status: accepted
date: 2026-05-05
related: [D-04, D-09]
executive_summary: |
  OTA commands and acks ride the same MQTT broker as telemetry, on
  separate topic prefixes (cmd/, ack/). Persistent sessions and QoS 1
  give us "delivers when the robot reconnects" semantics for free,
  matching the intermittent connectivity model. Temporal workflows
  wait on signals delivered by an MQTT-to-signal bridge; no Temporal
  components run on the robot.
---

# ADR-008 OTA command/ack transport

## Decision

OTA messages share the MQTT broker with telemetry, with these topics:

| Direction | Topic                  | Payload                           | QoS |
|-----------|------------------------|-----------------------------------|-----|
| Cloud→Bot | `cmd/{robot_id}/ota`   | OTACommand JSON                   | 1   |
| Bot→Cloud | `ack/{robot_id}/ota`   | OTAAck JSON (one per phase)       | 1   |

The cloud-side `ota-worker` runs both:

- A **Temporal worker** that hosts the OTA workflows + activities
- An **MQTT bridge** that subscribes to `ack/+/ota` and translates
  each ack into a `Temporal.SignalWorkflow` call targeting a
  deterministic workflow ID (`{rolloutID}-robot-{robotID}` or its
  `-rollback` variant).

## Why not gRPC?

gRPC requires the robot to maintain an open connection to the cloud,
which conflicts with D-04 (intermittent connectivity). MQTT QoS 1 +
persistent session delivers commands when the robot reconnects, no
extra retry logic.

## Why not a separate broker?

Operating two brokers doubles the on-prem footprint without benefit.
Topic isolation by prefix is sufficient. Authorization (S5) will
restrict `cmd/+/ota` writes to the control plane only.

## Why JSON, not protobuf, on the wire (v1)?

Protobuf bindings for the bridge node + agent require generation
machinery and a packaging story. JSON keeps the v1 path readable and
debuggable in `mosquitto_sub` and the EMQX dashboard. Switch to
protobuf wire format when polyglot consumers (web UI, partner
integrations) come online.

## Properties

- **Idempotent commands.** The agent records the rollout_id of the
  most recently processed command per robot; duplicate deliveries
  are dropped unless `force=true`.
- **Per-phase acks.** Cloud workflows receive distinct signals for
  pulled / swapped / healthy / failed / rolled-back; intermediate
  states are never inferred.
- **Rollback signals route to a sibling workflow.** Rollback acks
  carry the same rollout_id but target the `*-rollback` workflow ID.
