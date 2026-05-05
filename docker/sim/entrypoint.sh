#!/usr/bin/env bash
set -euo pipefail

# Source ROS 2 + the TurtleBot3 launch tree.
source /opt/ros/humble/setup.bash

# Choose which world to launch. TURTLEBOT3_MODEL controls the robot;
# WORLD selects the Gazebo world. Defaults work for the lab.
TURTLEBOT3_MODEL="${TURTLEBOT3_MODEL:-burger}"
WORLD="${WORLD:-empty_world.launch.py}"
BRIDGE_SOCKET="${BRIDGE_SOCKET:-/run/bridge/temporal-hack-bridge.sock}"
HEADLESS="${HEADLESS:-1}"   # 1 = no GUI; default in containers without an X server

mkdir -p "$(dirname "$BRIDGE_SOCKET")"

# 1) Start Gazebo + TurtleBot3 in the background.
if [ "$HEADLESS" = "1" ]; then
    # gzserver only — no GUI, no GL stack required.
    ros2 launch turtlebot3_gazebo "$WORLD" gui:=false &
else
    ros2 launch turtlebot3_gazebo "$WORLD" &
fi
SIM_PID=$!

# Give the simulator a moment to start publishing topics.
sleep 8

# 2) Start the synthetic battery publisher (TurtleBot3 sim does not
#    emit /battery_state by default). Sim-only.
python3 -m bridge_node.sim_battery &
BATT_PID=$!

# 3) Start the bridge node. It will block until SIGTERM.
echo "starting bridge on $BRIDGE_SOCKET"
exec python3 -m bridge_node.server --socket "$BRIDGE_SOCKET"
