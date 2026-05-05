#!/usr/bin/env bash
# Standalone smoke test for a robot-app controller image.
#
# Spins up the controller container plus a throwaway listener container
# on an isolated docker network, waits for one /cmd_vel message, and
# reports PASS/FAIL. No lab stack, no Gazebo, no MQTT, no Temporal.
#
# Iteration loop:
#   1. edit sim/controllers/<name>/controller.py
#   2. docker buildx build --platform=linux/amd64 \
#        -t localhost:14050/robot-app:<name>-vN \
#        -f sim/controllers/<name>/Dockerfile sim/controllers/<name>
#   3. ./sim/controllers/_smoke.sh localhost:14050/robot-app:<name>-vN
#
# Total cycle time once images are cached: ~10 s.
#
# Note: for smoke we override the controller's baked-in cyclonedds RMW
# with FastDDS so the listener doesn't need cyclonedds installed. The
# real sim path uses cyclonedds for Gazebo interop reasons (D-01).
set -euo pipefail

IMAGE="${1:?usage: $0 <controller-image-ref>}"
NETWORK="ros-controller-smoke-$$"
DOMAIN_ID=99
TIMEOUT_S=8

cleanup() {
  docker rm -f smoke-controller smoke-listener >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[smoke] image=$IMAGE network=$NETWORK domain=$DOMAIN_ID"

docker network create "$NETWORK" >/dev/null

docker run -d --rm --network "$NETWORK" --name smoke-controller \
  -e ROS_DOMAIN_ID="$DOMAIN_ID" \
  -e RMW_IMPLEMENTATION=rmw_fastrtps_cpp \
  "$IMAGE" >/dev/null

# DDS discovery converges in ~1-2s on a fresh network.
sleep 2

echo "[smoke] listening for one /cmd_vel message (${TIMEOUT_S}s timeout)..."
if docker run --rm --network "$NETWORK" --name smoke-listener \
     -e ROS_DOMAIN_ID="$DOMAIN_ID" \
     ros:humble-ros-base \
     bash -c "set +u && source /opt/ros/humble/setup.bash && timeout ${TIMEOUT_S} ros2 topic echo /cmd_vel --once"
then
  echo "[smoke] PASS — controller is publishing /cmd_vel"
  exit 0
else
  echo "[smoke] FAIL — no /cmd_vel within ${TIMEOUT_S}s"
  echo "[smoke] controller logs:"
  docker logs smoke-controller 2>&1 | sed 's/^/  /'
  exit 1
fi
