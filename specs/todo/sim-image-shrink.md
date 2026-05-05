---
title: Shrink the sim container image
priority: P3
source: build-S3
status: todo
sprint: S7
related_decisions: [D-13]
date: 2026-05-05
executive_summary: |
  The sim image is built from osrf/ros:humble-desktop which is ~3.5 GB.
  Most of it is unused. Build from `ros:humble-ros-base` plus only
  the gazebo + turtlebot3 packages we need; target ~1.5 GB.
---

# Shrink the sim container image

## Why

Faster `make sim-up` rebuild, less local disk, faster CI cache pulls.

## What

- Switch base from `osrf/ros:humble-desktop` to `ros:humble-ros-base`.
- Add only: `ros-humble-turtlebot3-gazebo` and its transitive deps.
- Drop the desktop GUI tooling (rviz, ros1_bridge, etc.).

## Acceptance

- `docker images temporal-hack/sim:dev` reports < 1.8 GB.
- `make sim-up` then `make sim-logs` shows the bridge node accepting
  connections within 30s of `docker compose up`.
