# OTA-swappable robot-app: drives the TurtleBot3 in a circle.
#
# Build (target linux/amd64 to match the sim image; M-series builds default
# to arm64 which the emulated sim cannot run):
#
#   docker buildx build --platform=linux/amd64 \
#     -t localhost:14050/robot-app:circle-v1 \
#     -f sim/controllers/drive-circle/Dockerfile \
#     sim/controllers/drive-circle
#   docker push localhost:14050/robot-app:circle-v1
#
# Run standalone (without OTA, for smoke testing against a live sim):
#
#   docker run --rm \
#     --network=temporal-hack-lab_default \
#     -e ROS_DOMAIN_ID=42 \
#     -e RMW_IMPLEMENTATION=rmw_cyclonedds_cpp \
#     localhost:14050/robot-app:circle-v1
#
# Trigger via OTA (once agent RunArgs plumbing lands):
#
#   curl -X POST http://localhost:8081/v1/ota/rollouts \
#     -H "content-type: application/json" \
#     -d '{
#       "image_ref": "localhost:14050/robot-app:circle-v1",
#       "smoke_command": "true",
#       "cohort_selector": {"robot_ids": ["sim-robot-01"]}
#     }'
FROM ros:humble-ros-base

ENV ROS_DOMAIN_ID=42
ENV RMW_IMPLEMENTATION=rmw_cyclonedds_cpp
ENV PYTHONUNBUFFERED=1

# rmw_cyclonedds is in apt; the ros-base image only ships the default RMW.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ros-humble-rmw-cyclonedds-cpp \
    && rm -rf /var/lib/apt/lists/*

COPY controller.py /controller.py

# `set +u` around the ROS source: setup.bash references unset vars
# (AMENT_TRACE_SETUP_FILES) and trips strict mode otherwise.
CMD ["bash", "-c", "set +u && source /opt/ros/humble/setup.bash && exec python3 /controller.py"]
