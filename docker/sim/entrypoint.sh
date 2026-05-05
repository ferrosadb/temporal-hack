#!/usr/bin/env bash
set -eo pipefail

# ROS 2's setup.bash references variables it doesn't always pre-set
# (AMENT_TRACE_SETUP_FILES, etc.). Disable nounset for the source
# step only; turn it back on for our own logic afterwards.
set +u
source /opt/ros/humble/setup.bash
set -u

TURTLEBOT3_MODEL="${TURTLEBOT3_MODEL:-burger}"
WORLD="${WORLD:-empty_world.launch.py}"
BRIDGE_SOCKET="${BRIDGE_SOCKET:-/run/bridge/temporal-hack-bridge.sock}"
HEADLESS="${HEADLESS:-0}"   # default 0: show the GUI via noVNC.
                             # Set HEADLESS=1 to skip the X stack entirely.
DISPLAY_NUM="${DISPLAY_NUM:-1}"
SCREEN_GEOMETRY="${SCREEN_GEOMETRY:-1280x800x24}"

mkdir -p "$(dirname "$BRIDGE_SOCKET")"

# Process tracker so we can clean up children on SIGTERM.
PIDS=()
shutdown() {
    echo "[sim] shutting down"
    for pid in "${PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
    exit 0
}
trap shutdown SIGINT SIGTERM

if [ "$HEADLESS" = "1" ]; then
    echo "[sim] HEADLESS=1; running gzserver only (no display)"
    ros2 launch turtlebot3_gazebo "$WORLD" gui:=false &
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

    echo "[sim] launching Gazebo with GUI (DISPLAY=$DISPLAY)"
    ros2 launch turtlebot3_gazebo "$WORLD" gui:=true &
    PIDS+=($!)

    echo
    echo "  ┌────────────────────────────────────────────────────────────┐"
    echo "  │  Gazebo GUI:  http://localhost:14680/vnc.html?autoconnect=1│"
    echo "  │  Raw VNC:     localhost:14900   (no password)              │"
    echo "  └────────────────────────────────────────────────────────────┘"
    echo
fi

# Synthetic battery (TurtleBot3 sim doesn't emit /battery_state).
sleep 6
python3 -m bridge_node.sim_battery &
PIDS+=($!)

# Bridge: blocks foreground; this is the canonical lifetime of the
# container.
echo "[sim] starting bridge on $BRIDGE_SOCKET"
exec python3 -m bridge_node.server --socket "$BRIDGE_SOCKET"
