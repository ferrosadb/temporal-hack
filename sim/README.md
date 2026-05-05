# sim/ — robot behaviors and worlds

Repo-controlled assets for the Gazebo dev sim. Per **D-13**, this directory
exists so the sim is reproducible from a clean clone and the demo runs
clean-room on a fresh laptop.

## Layout

```
sim/
├── README.md
├── controllers/                          # OTA-swappable robot-app images
│   ├── _smoke.sh                         # standalone test harness
│   ├── drive-circle/
│   │   ├── Dockerfile
│   │   └── controller.py
│   └── drive-figure-eight/
│       ├── Dockerfile
│       └── controller.py
└── worlds/                               # Gazebo SDF world files (TODO)
```

## What lives here

**`controllers/`** — each subdirectory is a self-contained ROS 2 node
packaged as a `robot-app` image. The agent's OTA executor pulls one of
these images by ref, swaps it under the `robot-app` container name, and
the new behavior takes effect. Multiple controllers exist so the demo
can show a live OTA-triggered behavior swap.

**`worlds/`** — Gazebo SDF world files. The default
`empty_world.launch.py` from the apt-installed
`turtlebot3_gazebo` package shows a featureless plane; for the demo
we want at least a few static obstacles so motion is visible.

## Demo narrative

1. `make sim-up` — Gazebo + bridge + agent + a `robot-app` container
   already running its v1 controller (`drive-circle`).
2. Rep opens **<http://localhost:14680/vnc.html?autoconnect=1>** in a
   browser. Gazebo renders in the page (PR #4: noVNC). No XQuartz, no
   X11 forwarding, no host-side display setup — just a URL.
3. Operator (rep) triggers an OTA rollout via the control plane API to
   a different `robot-app` image (e.g. `drive-figure-eight`).
4. Temporal UI steps through `PHASE_PULLED → PHASE_SWAPPED → PHASE_HEALTHY`.
5. **Simultaneously**, the robot's path in the browser-rendered Gazebo
   changes — closed loop visible to the eye.

## Building and pushing a controller

The sim image runs `linux/amd64` (the upstream `osrf/ros:humble-desktop`
image has no arm64 manifest, so it's emulated on Apple Silicon). Build
controllers for the same platform so OTA can swap them in:

```bash
docker buildx build --platform=linux/amd64 \
  -t localhost:14050/robot-app:circle-v1 \
  -f sim/controllers/drive-circle/Dockerfile \
  sim/controllers/drive-circle
docker push localhost:14050/robot-app:circle-v1
```

The lab registry runs at `localhost:14050` (per `make lab-up`).

## Iteration loops

Three loops, each a different speed/fidelity tradeoff. Use the fastest
one that gives you the signal you need.

### (1) Standalone smoke — fastest, no sim

`sim/controllers/_smoke.sh` brings up the controller plus a throwaway
listener on an isolated docker network and verifies it publishes
`/cmd_vel`. No lab stack, no Gazebo, no MQTT.

```bash
docker buildx build --platform=linux/amd64 \
  -t localhost:14050/robot-app:circle-v1 \
  -f sim/controllers/drive-circle/Dockerfile \
  sim/controllers/drive-circle

./sim/controllers/_smoke.sh localhost:14050/robot-app:circle-v1
# → [smoke] PASS — controller is publishing /cmd_vel
```

Cycle time once images are cached: ~10 s. This is the right loop while
you're tuning velocities or experimenting with new motion patterns —
you do not need Gazebo to verify the controller is publishing the
right Twist values.

### (2) Live sim, no OTA — visual confirmation

Once a controller smoke-tests clean, point it at the running sim's
ROS network to see actual robot motion in Gazebo. With PR #4's noVNC
stack landed, viewing is browser-based — no XQuartz / X11 setup:

```bash
make sim-up                              # if not already up
open http://localhost:14680/vnc.html?autoconnect=1   # Gazebo in the browser

docker push localhost:14050/robot-app:circle-v1

docker run --rm \
  --network=temporal-hack-lab_default \
  -e ROS_DOMAIN_ID=42 \
  -e RMW_IMPLEMENTATION=rmw_cyclonedds_cpp \
  localhost:14050/robot-app:circle-v1
```

The controller container shares the lab network so DDS discovery
reaches the sim's ROS topics; the burger starts moving immediately
and the motion is visible in the browser tab. This is the right loop
for screenshots and demo video — and it's also the path the rep takes
during the live demo, so make sure motion looks right here before
trusting it for OTA-triggered swap.

### (3) Full OTA path — end-to-end demo verification

The closed-loop path: operator triggers a rollout, Temporal orchestrates
phases, agent swaps the `robot-app` container, motion changes in
Gazebo. **Blocked on partner-track work** (see "Outstanding wiring"
below). Use only once that lands.

## Authoring a new controller

Copy `drive-circle/` to `<your-name>/`. Edit `controller.py`:
publish whatever combination of `geometry_msgs/Twist` fields produces
the motion. Build with a fresh tag, push, then trigger via OTA (or
smoke-test as above).

Interesting patterns to try:

- **drive-figure-eight** — alternate `angular.z` sign every N seconds.
- **patrol** — square trajectory: drive straight, turn 90°, repeat.
- **stop** — publish all zeros (useful to verify the "robot stops on
  failed deploy" rollback story).
- **reactive** — subscribe to `/scan`, slow down or turn when an
  obstacle is close. Needs lidar; works because the burger has an LDS.

## Worlds

Author SDF world files into `worlds/`. The simplest path is to fork
one from the upstream `turtlebot3_gazebo` package:

```bash
docker run --rm temporal-hack/sim:dev \
  cat /opt/ros/humble/share/turtlebot3_gazebo/worlds/turtlebot3_world.world \
  > sim/worlds/obstacle-course.world
```

…then prune to just the obstacles you want.

To use a custom world, point `WORLD` env at it in
`docker-compose.sim.yml` (currently it's `empty_world.launch.py`,
which loads the default empty world from the launch tree). Switching
to a custom file requires editing the launch invocation in
`docker/sim/entrypoint.sh` — note in your PR if you do this, since
that's a partner-track file.

## Outstanding wiring (partner track)

For OTA-triggered behavior swap to work end-to-end, the agent's
`RunArgs` must include `--network=temporal-hack-lab_default` and
`-e ROS_DOMAIN_ID=42` so the swapped container lands on the sim's
DDS network. Currently `agent/internal/ota/docker.go:52` does
`docker run -d --name <tmp>` with no network args. Coordinate with
the partner before relying on the OTA path; standalone smoke testing
(above) is unblocked today.

## Toolchain

- ROS 2 Humble (per D-01) for both the sim and controller images.
- Python 3.10 (Humble's reference Python; provided by the ROS image).
- No host-side dependencies on the rep's laptop beyond Docker — the
  whole stack is `make sim-up`.
