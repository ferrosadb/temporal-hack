#!/usr/bin/env bash
set -eo pipefail

# ROS 2's setup.bash references variables it doesn't always pre-set
# (AMENT_TRACE_SETUP_FILES, etc.). Disable nounset for the source
# step only; turn it back on for our own logic afterwards.
set +u
source /opt/ros/humble/setup.bash
set -u

BRIDGE_SOCKET="${BRIDGE_SOCKET:-/run/bridge/temporal-hack-bridge.sock}"
HEADLESS="${HEADLESS:-0}"   # default 0: show the GUI via noVNC.
                             # Set HEADLESS=1 to skip the X stack entirely.
DISPLAY_NUM="${DISPLAY_NUM:-1}"
SCREEN_GEOMETRY="${SCREEN_GEOMETRY:-1920x1200x24}"

mkdir -p "$(dirname "$BRIDGE_SOCKET")"

# Process tracker so we can clean up children on SIGTERM.
PIDS=()
shutdown() {
    echo "[sim] shutting down"
    for pid in "${PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
    exit 0
}
trap shutdown SIGINT SIGTERM

SIM_WORLD="${SIM_WORLD:-moon.sdf}"

if [ "$HEADLESS" = "1" ]; then
    echo "[sim] HEADLESS=1; running ign gazebo server only ($SIM_WORLD)"
    ign gazebo -s -v 4 "$SIM_WORLD" &
    PIDS+=($!)
else
    # Start a virtual X display, a minimal window manager, a VNC
    # server bound to that display, and a websockify→noVNC bridge so
    # the GUI is reachable over a browser at port 6080 (mapped to
    # 14680 on the host by docker-compose.sim.yml).
    echo "[sim] starting Xvfb on :$DISPLAY_NUM ($SCREEN_GEOMETRY)"
    Xvfb ":$DISPLAY_NUM" -screen 0 "$SCREEN_GEOMETRY" -nolisten tcp &
    PIDS+=($!)
    sleep 1
    export DISPLAY=":$DISPLAY_NUM"

    echo "[sim] starting fluxbox window manager"
    fluxbox 2>/dev/null &
    PIDS+=($!)

    echo "[sim] starting x11vnc on :5900"
    x11vnc -display "$DISPLAY" -forever -shared -nopw -quiet -rfbport 5900 -bg -o /tmp/x11vnc.log
    # x11vnc -bg already daemonised; nothing to push to PIDS.

    echo "[sim] starting noVNC websockify on :6080"
    # Debian's `novnc` package ships /usr/share/novnc/ + a launcher.
    websockify --web=/usr/share/novnc 6080 localhost:5900 &
    PIDS+=($!)

    echo "[sim] launching Ignition Fortress with GUI: $SIM_WORLD"
    # Software OpenGL is the only path that works in Xvfb (no GPU).
    # OGRE2 will use Mesa's llvmpipe and render to the virtual fb.
    export LIBGL_ALWAYS_SOFTWARE=1
    export OGRE2_RTSHADERSYSTEM_WRITE_SHADERS_TO_DISK=0
    # -r: start running (not paused). Without this Fortress opens
    # paused and the user has to click Play before any movement
    # commands have effect.
    ign gazebo -r -v 4 "$SIM_WORLD" &
    PIDS+=($!)

    echo
    echo "  ┌──────────────────────────────────────────────────────────────────────────────┐"
    echo "  │  Gazebo GUI:  http://localhost:14680/vnc.html?autoconnect=1&resize=scale     │"
    echo "  │  Raw VNC:     localhost:14900   (no password)                                │"
    echo "  └──────────────────────────────────────────────────────────────────────────────┘"
    echo
fi

# ROS 2 ↔ Ignition bridge for the diff_drive demo world only. That
# world's `vehicle_blue` model subscribes to /model/vehicle_blue/cmd_vel;
# external ROS 2 controllers publish on /cmd_vel, so we remap. Skip
# this for moon.sdf (no vehicle_blue there) — the bridge would just
# sit waiting for a non-existent topic.
case "$SIM_WORLD" in
    diff_drive.sdf|diff_drive_skid.sdf)
        sleep 4
        echo "[sim] starting ros_gz_bridge: /cmd_vel ↔ /model/vehicle_blue/cmd_vel"
        ros2 run ros_gz_bridge parameter_bridge \
            /model/vehicle_blue/cmd_vel@geometry_msgs/msg/Twist@ignition.msgs.Twist \
            /model/vehicle_blue/odometry@nav_msgs/msg/Odometry@ignition.msgs.Odometry \
            --ros-args \
            -r /model/vehicle_blue/cmd_vel:=/cmd_vel \
            -r /model/vehicle_blue/odometry:=/odom &
        PIDS+=($!)
        ;;
    moon.sdf)
        sleep 4
        echo "[sim] starting ros_gz_bridge: /cmd_vel ↔ /model/perseverance/cmd_vel"
        ros2 run ros_gz_bridge parameter_bridge \
            /model/perseverance/cmd_vel@geometry_msgs/msg/Twist@ignition.msgs.Twist \
            /model/perseverance/odometry@nav_msgs/msg/Odometry@ignition.msgs.Odometry \
            --ros-args \
            -r /model/perseverance/cmd_vel:=/cmd_vel \
            -r /model/perseverance/odometry:=/odom &
        PIDS+=($!)
        ;;
    *)
        echo "[sim] no ros_gz_bridge wired for world '$SIM_WORLD'"
        ;;
esac

# Synthetic battery (TurtleBot3 sim doesn't emit /battery_state).
sleep 6
python3 -m bridge_node.sim_battery &
PIDS+=($!)

# Bridge: blocks foreground; this is the canonical lifetime of the
# container.
echo "[sim] starting bridge on $BRIDGE_SOCKET"
exec python3 -m bridge_node.server --socket "$BRIDGE_SOCKET"
