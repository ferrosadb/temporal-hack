# Robot simulation container — Moon world + Ingenuity helicopter.
#
# Base: ros:humble-ros-base (multi-arch — amd64 + arm64 native).
# Sim:  Gazebo Harmonic (LTS 2024-2028) from packages.osrfoundation.org.
#       Replaces Ignition Fortress; same plugin family (gz-sim-*).
# GUI:  Xvfb + fluxbox + x11vnc + noVNC + websockify, browser-served.
#
# Why Harmonic, not Fortress: the AndrejOrsula moon world targets
# gz-sim-* plugin filenames. Harmonic ships them and is arm64-native.
# Why we don't run their Blender procgen here: Blender 4.3 is x86_64-
# only on Linux, and their amd64 image hits a /bin/sh portability bug
# under buildah. The procgen step is a clean follow-up; for now we
# ship a hand-authored moon.sdf + ingenuity.sdf.
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
        ros-humble-sensor-msgs \
        ros-humble-ros-gz-bridge \
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

# Bridge package + deps.
WORKDIR /opt/bridge
COPY bridge/ /opt/bridge/
RUN python3 -m pip install --no-cache-dir --upgrade "pip>=23.0" \
 && python3 -m pip install --no-cache-dir --break-system-packages /opt/bridge

# Sim worlds + models. GZ_SIM_RESOURCE_PATH is read by gz-sim to
# resolve `model://...` URIs in include directives.
COPY docker/sim/worlds  /opt/sim/worlds
COPY docker/sim/models  /opt/sim/models
ENV GZ_SIM_RESOURCE_PATH="/opt/sim/models:/opt/sim/worlds"

# Sim launch script.
COPY docker/sim/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

VOLUME ["/run/bridge"]

# noVNC (browser GUI):  http://localhost:14680
# Raw VNC (any client): localhost:14900
EXPOSE 5900 6080

ENTRYPOINT ["/entrypoint.sh"]
