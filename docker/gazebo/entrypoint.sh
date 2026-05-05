#!/usr/bin/env bash
# gazebo container entrypoint:
#   Xvfb -> fluxbox -> x11vnc -> websockify  (browser GUI pipe)
#   ign gazebo -r -v 4 <world>                (sim engine + GUI)
#   ros_gz_bridge                              (subscribes to /cmd_vel
#                                               on ROS DDS, forwards to
#                                               the gz cmd_vel topic)
set -eo pipefail

# ROS 2's setup.bash references vars under nounset; wrap.
set +u; source /opt/ros/humble/setup.bash; set -u

SIM_WORLD="${SIM_WORLD:-moon.sdf}"
HEADLESS="${HEADLESS:-0}"
DISPLAY_NUM="${DISPLAY_NUM:-1}"
SCREEN_GEOMETRY="${SCREEN_GEOMETRY:-1920x1200x24}"

PIDS=()
shutdown() {
    echo "[gazebo] shutting down"
    for pid in "${PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
    exit 0
}
trap shutdown SIGINT SIGTERM

if [ "$HEADLESS" = "1" ]; then
    echo "[gazebo] HEADLESS=1; running ign gazebo server only ($SIM_WORLD)"
    ign gazebo -s -r -v 4 "$SIM_WORLD" &
    PIDS+=($!)
else
    echo "[gazebo] starting Xvfb on :$DISPLAY_NUM ($SCREEN_GEOMETRY)"
    Xvfb ":$DISPLAY_NUM" -screen 0 "$SCREEN_GEOMETRY" -nolisten tcp &
    PIDS+=($!)
    sleep 1
    export DISPLAY=":$DISPLAY_NUM"

    fluxbox 2>/dev/null &
    PIDS+=($!)

    x11vnc -display "$DISPLAY" -forever -shared -nopw -quiet -rfbport 5900 -bg -o /tmp/x11vnc.log

    websockify --web=/usr/share/novnc 6080 localhost:5900 &
    PIDS+=($!)

    echo "[gazebo] launching ign gazebo with GUI: $SIM_WORLD"
    export LIBGL_ALWAYS_SOFTWARE=1
    export OGRE2_RTSHADERSYSTEM_WRITE_SHADERS_TO_DISK=0
    ign gazebo -r -v 4 "$SIM_WORLD" &
    PIDS+=($!)

    echo
    echo "  ┌──────────────────────────────────────────────────────────────────────────────┐"
    echo "  │  Gazebo GUI:  http://localhost:14680/vnc.html?autoconnect=1&resize=scale     │"
    echo "  │  Raw VNC:     localhost:14900   (no password)                                │"
    echo "  └──────────────────────────────────────────────────────────────────────────────┘"
    echo
fi

# Bridge ROS 2 /cmd_vel and odom for the perseverance rover. The
# diff-drive plugin in moon.sdf listens on /model/perseverance/cmd_vel;
# remap so external ROS 2 controllers can publish to plain /cmd_vel.
sleep 4
case "$SIM_WORLD" in
    moon.sdf)
        echo "[gazebo] starting ros_gz_bridge: /cmd_vel + odom + /perseverance/contacts"
        ros2 run ros_gz_bridge parameter_bridge \
            /model/perseverance/cmd_vel@geometry_msgs/msg/Twist@ignition.msgs.Twist \
            /model/perseverance/odometry@nav_msgs/msg/Odometry@ignition.msgs.Odometry \
            /perseverance/contacts@ros_gz_interfaces/msg/Contacts@ignition.msgs.Contacts \
            --ros-args \
            -r /model/perseverance/cmd_vel:=/cmd_vel \
            -r /model/perseverance/odometry:=/odom \
            -r /perseverance/contacts:=/contacts &
        PIDS+=($!)
        ;;
    diff_drive.sdf|diff_drive_skid.sdf)
        echo "[gazebo] starting ros_gz_bridge: /cmd_vel ↔ /model/vehicle_blue/cmd_vel"
        ros2 run ros_gz_bridge parameter_bridge \
            /model/vehicle_blue/cmd_vel@geometry_msgs/msg/Twist@ignition.msgs.Twist \
            /model/vehicle_blue/odometry@nav_msgs/msg/Odometry@ignition.msgs.Odometry \
            --ros-args \
            -r /model/vehicle_blue/cmd_vel:=/cmd_vel \
            -r /model/vehicle_blue/odometry:=/odom &
        PIDS+=($!)
        ;;
    *)
        echo "[gazebo] no ros_gz_bridge wired for world '$SIM_WORLD'"
        ;;
esac

# Wait for any background process to exit (or signal).
wait -n
