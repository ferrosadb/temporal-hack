# Gazebo (simulator) container.
#
# Runs Ignition Fortress + ros_gz_bridge + an Xvfb/x11vnc/noVNC stack
# for browser-served GUI access. NOTHING ROS-application-specific
# lives here — the rover's controllers (drive-circle, drive-figure-eight)
# and the agent's bridge_node live in their own containers and reach
# this one over the lab network's ROS DDS domain.
#
# What's in vs. out:
#   IN:  gz sim, ros_gz_bridge, Xvfb/x11vnc/novnc/fluxbox, mesa,
#        worlds/, models/
#   OUT: bridge_node (rclpy → agent gRPC), sim_battery, controllers
FROM ros:humble-ros-base

ENV DEBIAN_FRONTEND=noninteractive
ENV ROS_DOMAIN_ID=42
ENV RMW_IMPLEMENTATION=rmw_cyclonedds_cpp

RUN apt-get update && apt-get install -y --no-install-recommends \
        wget gnupg lsb-release ca-certificates \
    && wget -qO /usr/share/keyrings/pkgs-osrf-archive-keyring.gpg \
        https://packages.osrfoundation.org/gazebo.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/pkgs-osrf-archive-keyring.gpg] http://packages.osrfoundation.org/gazebo/ubuntu-stable $(lsb_release -cs) main" \
        > /etc/apt/sources.list.d/gazebo-stable.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ros-humble-rmw-cyclonedds-cpp \
        ros-humble-ros-gz-bridge \
        ignition-fortress \
        libgl1-mesa-dri \
        libgl1-mesa-glx \
        mesa-utils \
        xvfb \
        x11vnc \
        novnc \
        websockify \
        fluxbox \
        net-tools \
    && rm -rf /var/lib/apt/lists/*

# Sim worlds + models. GZ_SIM_RESOURCE_PATH lets gz-sim resolve
# `model://...` includes at runtime.
COPY docker/gazebo/worlds /opt/sim/worlds
COPY docker/gazebo/models /opt/sim/models
ENV GZ_SIM_RESOURCE_PATH="/opt/sim/models:/opt/sim/worlds"

COPY docker/gazebo/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# noVNC (browser GUI):  http://localhost:14680
# Raw VNC (any client): localhost:14900
EXPOSE 5900 6080
ENTRYPOINT ["/entrypoint.sh"]
