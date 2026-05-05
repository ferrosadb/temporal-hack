---
title: Wire the Go agent's ingest loop to the bridge gRPC client
priority: P1
source: build-S1
status: todo
sprint: S1
related_decisions: [D-06]
date: 2026-05-05
executive_summary: |
  The agent's ingest loop currently emits a stub battery sample every
  5 seconds. Replace it with a real gRPC client that calls
  Bridge.Subscribe on the bridge node's Unix socket and forwards
  TopicEvent payloads into the local SQLite buffer.
---

# Wire the Go agent's ingest loop to the bridge gRPC client

## Why

Sprint 1 acceptance is end-to-end telemetry from a real ROS 2 topic
to the cloud TSDB. The bridge node and the agent both exist; the
gRPC client side is the missing seam.

## What

1. Run `make proto` to generate Go bindings from `proto/agent_bridge.proto`.
2. Implement `agent/internal/bridge/client.go`:
   - Dial via `grpc.Dial("unix://...", grpc.WithTransportCredentials(insecure.NewCredentials()))`
   - Reconnect on stream loss with capped exponential backoff
   - Surface a `Subscribe(ctx, streams) <-chan TopicEvent` channel API
3. Replace the stub in `agent/internal/telemetry/pump.go::runIngest`
   with the real subscriber.
4. Add an integration test that runs both processes (bridge stubbed
   without rclpy) and verifies samples arrive in the buffer.

## Acceptance

- `go test ./agent/...` includes a passing test that exercises the
  bridge → agent → buffer path with a fake bridge server.
- Manual run on a host with ROS 2 Humble + a `/battery_state`
  publisher produces samples in the cloud TSDB visible via
  `GET /v1/robots/{id}/telemetry`.
