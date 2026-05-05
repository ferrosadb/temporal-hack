# Robot container — always-on ROS infrastructure for the simulated
# rover.
#
# What's in vs. out:
#   IN:  bridge_node (rclpy → gRPC for the agent), sim_battery
#   OUT: gz sim, ros_gz_bridge, GUI stack — those live in docker/gazebo/.
#        OTA-swappable controllers — those live as separate
#        sim/controllers/* images managed by the agent.
#
# This container is *not* the OTA target. It hosts the robot's
# always-on ROS plumbing. The OTA-swappable software runs as a
# sibling container (drive-circle, drive-figure-eight, ...) on the
# same lab network and ROS DDS domain.
FROM ros:humble-ros-base

ENV DEBIAN_FRONTEND=noninteractive
ENV ROS_DOMAIN_ID=42
ENV RMW_IMPLEMENTATION=rmw_cyclonedds_cpp
ENV PYTHONUNBUFFERED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        ros-humble-rmw-cyclonedds-cpp \
        ros-humble-sensor-msgs \
        python3-pip \
        python3-grpcio \
        python3-grpc-tools \
    && rm -rf /var/lib/apt/lists/*

# Bridge package (rclpy → gRPC) and sim_battery publisher.
WORKDIR /opt/bridge
COPY bridge/ /opt/bridge/
RUN python3 -m pip install --no-cache-dir --upgrade "pip>=23.0" \
 && python3 -m pip install --no-cache-dir --break-system-packages /opt/bridge

COPY docker/robot/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# bridge_node gRPC server (TCP) — agent connects here.
EXPOSE 50051

ENTRYPOINT ["/entrypoint.sh"]
