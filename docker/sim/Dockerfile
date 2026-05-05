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
# Multi-arch base. osrf/ros:humble-desktop is amd64-only, which forced
# qemu user-mode emulation on arm64 hosts (Apple Silicon) — and
# Gazebo's OGRE/LLVM JIT crashes under qemu with "64-bit code requested
# on a subtarget that doesn't support it". library/ros:humble-ros-base
# ships native amd64 + arm64 manifests. We add the gazebo + turtlebot3
# packages explicitly below since ros-base doesn't pre-install them.
FROM ros:humble-ros-base

ENV DEBIAN_FRONTEND=noninteractive
ENV TURTLEBOT3_MODEL=burger
ENV ROS_DOMAIN_ID=42

# Packages, by source:
#  - ros-humble-* from packages.ros.org (arm64 + amd64 published)
#  - ignition-fortress from packages.osrfoundation.org (arm64 + amd64)
#  - X stack and Mesa from Ubuntu jammy main
#
# Why Ignition Fortress and not Gazebo Classic: Gazebo Classic is
# amd64-only in apt; the only way to run it on arm64 is qemu user-
# mode emulation, where OGRE's LLVM JIT crashes. Fortress is the
# modern Gazebo (renamed gz-sim from Garden onward), LTS 2022-2027,
# with native arm64 builds.
RUN apt-get update && apt-get install -y --no-install-recommends \
        wget \
        gnupg \
        lsb-release \
        ca-certificates \
    && wget -qO /usr/share/keyrings/pkgs-osrf-archive-keyring.gpg \
        https://packages.osrfoundation.org/gazebo.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/pkgs-osrf-archive-keyring.gpg] http://packages.osrfoundation.org/gazebo/ubuntu-stable $(lsb_release -cs) main" \
        > /etc/apt/sources.list.d/gazebo-stable.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ros-humble-rmw-cyclonedds-cpp \
        ros-humble-sensor-msgs \
        ignition-fortress \
        libgl1-mesa-dri \
        libgl1-mesa-glx \
        mesa-utils \
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
