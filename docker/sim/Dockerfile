# Robot simulation container.
#
# Bundles ROS 2 Humble + Gazebo Classic + TurtleBot3 + the Python
# bridge node (rclpy → gRPC) + a small launch script that brings up
# Gazebo with a TurtleBot3 model and the bridge in one process tree.
#
# This is the "basic robot" used end-to-end:
#   - publishes /battery_state and /odom from the simulated robot
#   - bridge subscribes and republishes via gRPC on a Unix socket
#   - the Go agent (in a sibling container) consumes that gRPC stream
#
# Image is large (~3-4 GB) because Gazebo + ROS 2 + Python ML wheels
# pull in a lot. Acceptable for v1; sim is dev-only per D-13.
FROM osrf/ros:humble-desktop

ENV DEBIAN_FRONTEND=noninteractive
ENV TURTLEBOT3_MODEL=burger
ENV ROS_DOMAIN_ID=42

# TurtleBot3 + Gazebo bindings + Python build essentials for the bridge.
# Also Xvfb + x11vnc + noVNC + a minimal WM so the Gazebo GUI can be
# served over a browser-accessible VNC port. This is the standard
# trick for "sim with GUI on macOS / Windows / headless Linux without
# X11 forwarding".
RUN apt-get update && apt-get install -y --no-install-recommends \
        ros-humble-turtlebot3 \
        ros-humble-turtlebot3-gazebo \
        ros-humble-turtlebot3-msgs \
        ros-humble-gazebo-ros-pkgs \
        ros-humble-rmw-cyclonedds-cpp \
        python3-pip \
        python3-grpcio \
        python3-grpc-tools \
        xvfb \
        x11vnc \
        novnc \
        websockify \
        fluxbox \
        net-tools \
    && rm -rf /var/lib/apt/lists/*

# Use Cyclone DDS — slightly more reliable defaults for sim than Fast DDS.
ENV RMW_IMPLEMENTATION=rmw_cyclonedds_cpp

# Bridge: copy the source from the repo (build context is repo root).
# osrf/ros:humble-desktop ships pip 22.x. We install into the system
# site-packages so rclpy stays importable from the bridge process
# without PYTHONPATH gymnastics. Upgrade pip first to get the
# --break-system-packages flag (PEP 668), then install.
WORKDIR /opt/bridge
COPY bridge/ /opt/bridge/
RUN python3 -m pip install --no-cache-dir --upgrade "pip>=23.0" \
 && python3 -m pip install --no-cache-dir --break-system-packages /opt/bridge

# Sim launch script.
COPY docker/sim/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Bridge listens on a host-mounted socket so the agent (sibling
# container) can attach. See installer/docker-compose/docker-compose.sim.yml.
VOLUME ["/run/bridge"]

# noVNC (browser GUI):  http://localhost:14680
# Raw VNC (any client): localhost:14900
EXPOSE 5900 6080

ENTRYPOINT ["/entrypoint.sh"]
