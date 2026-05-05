#!/usr/bin/env bash
# robot container entrypoint:
#   sim_battery (rclpy)        — synthetic /battery_state publisher
#   bridge_node.server (rclpy) — gRPC peer for the agent on TCP 50051
set -eo pipefail

set +u; source /opt/ros/humble/setup.bash; set -u

LISTEN="${BRIDGE_LISTEN:-0.0.0.0:50051}"

PIDS=()
shutdown() {
    echo "[robot] shutting down"
    for pid in "${PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
    exit 0
}
trap shutdown SIGINT SIGTERM

# sim_battery: stand-in /battery_state publisher (Gazebo doesn't
# emit one for our model). Real robots publish their own; this is
# only present in the dev sim.
echo "[robot] starting sim_battery"
python3 -m bridge_node.sim_battery &
PIDS+=($!)

# collision_publisher: ROS /contacts -> MQTT events/{robot_id}/collision.
# The cloud's CollisionResponse workflow listens on the MQTT side.
echo "[robot] starting collision_publisher"
python3 -m bridge_node.collision_publisher &
PIDS+=($!)

# twist_subscriber: MQTT cmd/{robot_id}/twist -> ROS /cmd_vel.
# The cloud workflow publishes Twist messages here to drive the rover.
echo "[robot] starting twist_subscriber"
python3 -m bridge_node.twist_subscriber &
PIDS+=($!)

echo "[robot] starting bridge gRPC on $LISTEN"
exec python3 -m bridge_node.server --listen "$LISTEN"
