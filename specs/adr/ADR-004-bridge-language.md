---
title: ADR-004 ROS 2 bridge node language
status: accepted
date: 2026-05-05
related: [D-06]
executive_summary: |
  Adopt Python (rclpy) for the v1 ROS 2 bridge node. Faster delivery,
  smaller surface, lower bar for autonomy-engineer contribution. C++
  (rclcpp) is the migration path if measured CPU or jitter exceeds
  budget. The bridge is intentionally thin — Python's overhead is
  acceptable because the bridge does not run autonomy.
---

# ADR-004 ROS 2 bridge node language

## Decision

Use **Python (rclpy)** for the v1 bridge node.

## Why

- The bridge is a thin process: subscribe to a small set of DDS topics
  and republish via gRPC. No control loop, no real-time deadlines.
- Python keeps the autonomy team's contribution surface low.
- A C++ rewrite is a one-week port if the v1 implementation hits
  measured CPU or latency budget.

## Constraint

The bridge must remain thin. If the bridge starts to grow business
logic — message transformations, on-robot decisioning, state
machines — that is a signal to either move logic to the Go agent
(via the gRPC contract) or rewrite the bridge in C++.
