# OTA-swappable robot-app: drives the TurtleBot3 in a figure-8.
#
# Build / push / run identical to drive-circle — see that Dockerfile
# for the canonical commands. Only the tag and the source dir differ:
#
#   docker buildx build --platform=linux/amd64 \
#     -t localhost:14050/robot-app:figure-eight-v1 \
#     -f sim/controllers/drive-figure-eight/Dockerfile \
#     sim/controllers/drive-figure-eight
#   docker push localhost:14050/robot-app:figure-eight-v1
FROM ros:humble-ros-base

ENV ROS_DOMAIN_ID=42
ENV RMW_IMPLEMENTATION=rmw_cyclonedds_cpp
ENV PYTHONUNBUFFERED=1

RUN apt-get update \
    && apt-get install -y --no-install-recommends ros-humble-rmw-cyclonedds-cpp \
    && rm -rf /var/lib/apt/lists/*

COPY controller.py /controller.py

CMD ["bash", "-c", "set +u && source /opt/ros/humble/setup.bash && exec python3 /controller.py"]
