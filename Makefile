SHELL := /usr/bin/env bash

CLOUD_PKG := ./cloud/...
AGENT_PKG := ./agent/...

# -----------------------------------------------------------------------------
# Auto-install local git hooks on every `make` invocation. Idempotent:
# if core.hooksPath is already .git-hooks this runs nothing. If it's
# missing or pointed elsewhere, we set it. Done at parse time so it
# fires for every target, including `make help`.
#
# Hooks live in .git-hooks/ (committed). Skip a step ad-hoc with:
#   SKIP_CI_SMOKE=1 git push       # skip the installer smoke test
#   SKIP_CI_HOOK=1 git push        # skip everything (emergency only)
#   git push --no-verify           # ultimate bypass
# -----------------------------------------------------------------------------
ifeq ($(shell git rev-parse --is-inside-work-tree 2>/dev/null),true)
  _CURRENT_HOOKS := $(shell git config core.hooksPath 2>/dev/null)
  ifneq ($(_CURRENT_HOOKS),.git-hooks)
    _BOOTSTRAP := $(shell git config core.hooksPath .git-hooks && echo bootstrapped)
    ifeq ($(_BOOTSTRAP),bootstrapped)
      $(info [hooks] core.hooksPath set to .git-hooks)
    endif
  endif
endif

# Container engine detection. Order: explicit override -> docker -> podman.
# Override with `make lab-up CONTAINER=podman` if both are installed and
# you want the non-default. The choice picks both the compose command
# and the path to the container runtime socket used by the agent.
ifdef CONTAINER
  CONTAINER_ENGINE := $(CONTAINER)
else ifneq (,$(shell command -v docker 2>/dev/null))
  CONTAINER_ENGINE := docker
else ifneq (,$(shell command -v podman 2>/dev/null))
  CONTAINER_ENGINE := podman
else
  CONTAINER_ENGINE := none
endif

# Resolve the compose command for the chosen engine. `docker compose`
# (v2 plugin) and `podman compose` are the modern paths; `podman-compose`
# is the older standalone Python tool. Prefer the integrated subcommand.
ifeq ($(CONTAINER_ENGINE),docker)
  COMPOSE := docker compose
  CONTAINER_SOCK := /var/run/docker.sock
else ifeq ($(CONTAINER_ENGINE),podman)
  ifeq (,$(shell podman compose version 2>/dev/null))
    ifneq (,$(shell command -v podman-compose 2>/dev/null))
      COMPOSE := podman-compose
    else
      COMPOSE := podman compose
    endif
  else
    COMPOSE := podman compose
  endif
  # Ask podman directly. On macOS the host doesn't have /run/podman/
  # at all — the socket lives inside the podman machine VM at the
  # path podman reports. Strip a unix:// prefix if present.
  # Falls back to the canonical rootless path if podman info fails.
  _PODMAN_SOCK := $(shell podman info --format '{{.Host.RemoteSocket.Path}}' 2>/dev/null)
  ifneq (,$(_PODMAN_SOCK))
    CONTAINER_SOCK := $(patsubst unix://%,%,$(_PODMAN_SOCK))
  else
    CONTAINER_SOCK := $(if $(XDG_RUNTIME_DIR),$(XDG_RUNTIME_DIR)/podman/podman.sock,/run/podman/podman.sock)
  endif
else
  COMPOSE := /bin/false
  CONTAINER_SOCK :=
endif

.PHONY: hooks-install
hooks-install: ## Re-point git at .git-hooks/ (auto-runs on every make invocation)
	git config core.hooksPath .git-hooks
	@echo "git hooks: .git-hooks/{pre-commit,pre-push,commit-msg}"
	@ls -1 .git-hooks/

.PHONY: hooks-uninstall
hooks-uninstall: ## Restore default .git/hooks/ path (rare; for debugging)
	git config --unset core.hooksPath || true

.PHONY: container-info
container-info: ## Show detected container engine and compose command
	@echo "engine:    $(CONTAINER_ENGINE)"
	@echo "compose:   $(COMPOSE)"
	@echo "sock:      $(CONTAINER_SOCK)"

.PHONY: container-check
container-check:
	@if [ "$(CONTAINER_ENGINE)" = "none" ]; then \
	  echo "ERROR: no container engine found (docker or podman required)"; \
	  exit 1; \
	fi

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk -F':.*?## ' '{printf "  %-22s %s\n", $$1, $$2}'

.PHONY: tidy
tidy: ## go mod tidy across all modules
	cd cloud && go mod tidy
	cd agent && go mod tidy

.PHONY: build
build: build-cloud build-agent ## Build all Go binaries

.PHONY: build-cloud
build-cloud:
	cd cloud && go build -o ../bin/controlplane ./cmd/controlplane
	cd cloud && go build -o ../bin/telemetry-ingest ./cmd/telemetry-ingest
	cd cloud && go build -o ../bin/ota-worker ./cmd/ota-worker
	cd cloud && go build -o ../bin/collision-worker ./cmd/collision-worker

# =============================================================================
# Workflow workers — host-side Go binaries that connect to the lab
# Temporal frontend + MQTT broker. Without these running, the
# Temporal UI shows no Workers / no in-flight Workflows.
# =============================================================================

WORKER_TEMPORAL_ADDR ?= localhost:14733
WORKER_BROKER_URL   ?= tcp://localhost:14883
WORKER_TSDB_DSN     ?= postgres://temporal:temporal@localhost:14432/telemetry?sslmode=disable

# Native agent — runs as a Go binary on the host (macOS). Avoids
# bind-mounting the container-runtime socket into a sibling
# container (rootless podman idmap makes that path painful) and
# uses the host's docker/podman CLI directly when an OTA fires.
AGENT_ROBOT_ID    ?= sim-robot-01
AGENT_BROKER_URL  ?= tcp://localhost:14883
AGENT_BRIDGE_ADDR ?= localhost:50051
AGENT_BUFFER_PATH ?= .run/agent-buffer.db
AGENT_OTA_RUN_ARGS ?= --network=temporal-hack-lab_default,-e,ROS_DOMAIN_ID=42,-e,RMW_IMPLEMENTATION=rmw_cyclonedds_cpp

.PHONY: agent-up
agent-up: build-agent ## Start the agent natively on the host (preferred for local dev)
	@mkdir -p .run
	@ROBOT_ID=$(AGENT_ROBOT_ID) \
	  BROKER_URL=$(AGENT_BROKER_URL) \
	  BRIDGE_ADDR=$(AGENT_BRIDGE_ADDR) \
	  BUFFER_PATH=$(AGENT_BUFFER_PATH) \
	  OTA_RUN_ARGS="$(AGENT_OTA_RUN_ARGS)" \
	  nohup ./bin/agent > .run/agent.log 2>&1 & echo $$! > .run/agent.pid
	@sleep 1
	@echo "agent             PID $$(cat .run/agent.pid 2>/dev/null)             log .run/agent.log"

.PHONY: agent-down
agent-down: ## Stop the native agent
	@[ -f .run/agent.pid ] && kill "$$(cat .run/agent.pid)" 2>/dev/null && rm -f .run/agent.pid && echo "stopped agent" || true

.PHONY: agent-status
agent-status: ## Show native agent status
	@pid="$$(cat .run/agent.pid 2>/dev/null || echo '')"; \
	 if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then echo "agent: running (pid $$pid)"; \
	 else echo "agent: not running"; fi

.PHONY: workers-up
workers-up: build-cloud ## Start telemetry-ingest + ota-worker + collision-worker in the background
	@mkdir -p .run
	@BROKER_URL=$(WORKER_BROKER_URL) TSDB_DSN="$(WORKER_TSDB_DSN)" \
	  nohup ./bin/telemetry-ingest > .run/telemetry-ingest.log 2>&1 & echo $$! > .run/telemetry-ingest.pid
	@TEMPORAL_ADDR=$(WORKER_TEMPORAL_ADDR) BROKER_URL=$(WORKER_BROKER_URL) \
	  TSDB_DSN="$(WORKER_TSDB_DSN)" \
	  nohup ./bin/ota-worker > .run/ota-worker.log 2>&1 & echo $$! > .run/ota-worker.pid
	@TEMPORAL_ADDR=$(WORKER_TEMPORAL_ADDR) BROKER_URL=$(WORKER_BROKER_URL) \
	  nohup ./bin/collision-worker > .run/collision-worker.log 2>&1 & echo $$! > .run/collision-worker.pid
	@sleep 1
	@echo "telemetry-ingest  PID $$(cat .run/telemetry-ingest.pid 2>/dev/null)  log .run/telemetry-ingest.log"
	@echo "ota-worker        PID $$(cat .run/ota-worker.pid 2>/dev/null)        log .run/ota-worker.log"
	@echo "collision-worker  PID $$(cat .run/collision-worker.pid 2>/dev/null)  log .run/collision-worker.log"

.PHONY: workers-down
workers-down: ## Stop the workflow workers
	@for f in .run/telemetry-ingest.pid .run/ota-worker.pid .run/collision-worker.pid; do \
	  [ -f $$f ] && kill "$$(cat $$f)" 2>/dev/null && rm -f $$f && echo "stopped $$f" || true; \
	done

.PHONY: workers-status
workers-status: ## Show status of running workflow workers
	@for n in telemetry-ingest ota-worker collision-worker; do \
	  pid="$$(cat .run/$$n.pid 2>/dev/null || echo '')"; \
	  if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
	    echo "$$n: running (pid $$pid)"; \
	  else \
	    echo "$$n: not running"; \
	  fi; \
	done

# Publish a fake collision event to trigger a CollisionResponse workflow.
# Uses the lab broker directly via the robot container's paho client.
.PHONY: collide
collide: ## Publish a fake collision event for sim-robot-01 (triggers Temporal workflow)
	@$(CONTAINER_ENGINE) exec temporal-hack-lab-robot-1 python3 -c \
	  "import paho.mqtt.publish as p, time, json; p.single('events/sim-robot-01/collision', json.dumps({'robot_id':'sim-robot-01','at':time.time(),'count':1,'partner':'manual-trigger'}), hostname='mqtt', port=1883, qos=1)"
	@echo "published events/sim-robot-01/collision; check Temporal UI for collision-* workflow"

# =============================================================================
# Control plane (HTTP API for OTA rollouts) — host-side binary, not in
# compose. Required to start a rollout via /v1/ota/rollouts. Same
# pattern as workers-up / workers-down.
# =============================================================================

CP_LISTEN_ADDR ?= :8081

.PHONY: controlplane-up
controlplane-up: build-cloud ## Start the control plane HTTP API in the background
	@mkdir -p .run
	@LISTEN_ADDR=$(CP_LISTEN_ADDR) \
	  TEMPORAL_ADDR=$(WORKER_TEMPORAL_ADDR) \
	  TSDB_DSN="$(WORKER_TSDB_DSN)" \
	  nohup ./bin/controlplane > .run/controlplane.log 2>&1 & echo $$! > .run/controlplane.pid
	@sleep 1
	@echo "controlplane      PID $$(cat .run/controlplane.pid 2>/dev/null)      log .run/controlplane.log"
	@echo "  POST http://localhost$(CP_LISTEN_ADDR)/v1/ota/rollouts to start an OTA"

.PHONY: controlplane-down
controlplane-down: ## Stop the control plane API
	@[ -f .run/controlplane.pid ] && kill "$$(cat .run/controlplane.pid)" 2>/dev/null && rm -f .run/controlplane.pid && echo "stopped controlplane" || true

.PHONY: controlplane-status
controlplane-status: ## Show control plane status
	@pid="$$(cat .run/controlplane.pid 2>/dev/null || echo '')"; \
	 if [ -n "$$pid" ] && kill -0 "$$pid" 2>/dev/null; then \
	   echo "controlplane: running (pid $$pid) at http://localhost$(CP_LISTEN_ADDR)"; \
	 else echo "controlplane: not running"; fi

# =============================================================================
# OTA demo helpers — build the controller image, push it to the lab
# registry, fire a rollout. One command per controller.
# =============================================================================

OTA_REGISTRY    ?= localhost:14050
OTA_ROBOT_ID    ?= sim-robot-01
OTA_CP_HOST     ?= http://localhost:8081

# Internal: build + push a single controller. Args: $(1)=name (matches sim/controllers/<name>),
# $(2)=tag suffix.
define _ota_build_push
	@echo "[ota] building sim/controllers/$(1) → $(OTA_REGISTRY)/robot-app:$(2)"
	$(CONTAINER_ENGINE) build \
	  -t $(OTA_REGISTRY)/robot-app:$(2) \
	  -f sim/controllers/$(1)/Dockerfile \
	  sim/controllers/$(1)
	$(CONTAINER_ENGINE) push --tls-verify=false $(OTA_REGISTRY)/robot-app:$(2)
endef

# Internal: POST a rollout for the given image tag.
define _ota_rollout
	@echo "[ota] starting rollout for $(OTA_REGISTRY)/robot-app:$(1) on $(OTA_ROBOT_ID)"
	@curl -sS -X POST $(OTA_CP_HOST)/v1/ota/rollouts \
	  -H "content-type: application/json" \
	  -d '{ \
	    "image_ref": "$(OTA_REGISTRY)/robot-app:$(1)", \
	    "smoke_command": "true", \
	    "smoke_timeout_sec": 10, \
	    "cohort_selector": {"robot_ids": ["$(OTA_ROBOT_ID)"]} \
	  }' && echo
endef

.PHONY: ota-circle
ota-circle: ## Build, push, and OTA-roll the drive-circle controller
	$(call _ota_build_push,drive-circle,circle-v1)
	$(call _ota_rollout,circle-v1)
	@echo "watch the rover at http://localhost:14680 — should start driving in a circle"
	@echo "rollout status:   curl -s $(OTA_CP_HOST)/v1/ota/rollouts | jq"

.PHONY: ota-figure-eight
ota-figure-eight: ## Build, push, and OTA-roll the drive-figure-eight controller
	$(call _ota_build_push,drive-figure-eight,figure-eight-v1)
	$(call _ota_rollout,figure-eight-v1)
	@echo "watch the rover at http://localhost:14680 — should start tracing a figure 8"

.PHONY: ota-status
ota-status: ## List recent OTA rollouts (requires controlplane-up)
	@curl -sS $(OTA_CP_HOST)/v1/ota/rollouts | (command -v jq >/dev/null && jq || cat)

.PHONY: build-agent
build-agent:
	cd agent && go build -o ../bin/agent ./cmd/agent

.PHONY: lint
lint: ## Run linters
	cd cloud && go vet ./...
	cd agent && go vet ./...
	@command -v staticcheck >/dev/null && (cd cloud && staticcheck ./...) || echo "staticcheck not installed; skipping"
	@command -v staticcheck >/dev/null && (cd agent && staticcheck ./...) || echo "staticcheck not installed; skipping"

.PHONY: test
test: ## Run unit tests
	cd cloud && go test -race -count=1 ./...
	cd agent && go test -race -count=1 ./...

.PHONY: proto
proto: container-check ## Regenerate Go + Python protobuf bindings via containerized protoc
	@echo "[$(CONTAINER_ENGINE)] generating Go bindings"
	$(CONTAINER_ENGINE) run --rm -v "$(PWD)":/work -w /work \
	  rvolosatovs/protoc:4.0.0 \
	  -I=. \
	  --go_out=. --go_opt=module=github.com/example/temporal-hack \
	  --go-grpc_out=. --go-grpc_opt=module=github.com/example/temporal-hack \
	  proto/agent_bridge.proto proto/telemetry.proto proto/ota.proto
	@echo "[$(CONTAINER_ENGINE)] generating Python bindings"
	$(CONTAINER_ENGINE) run --rm --entrypoint sh \
	  -v "$(PWD)":/work -w /work \
	  python:3.11-slim \
	  -c "pip install --quiet grpcio-tools && python -m grpc_tools.protoc -I=. --python_out=bridge/bridge_node/proto --grpc_python_out=bridge/bridge_node/proto proto/agent_bridge.proto proto/telemetry.proto proto/ota.proto"
	@echo "fixing python relative imports"
	@for f in bridge/bridge_node/proto/*.py; do \
	  python3 -c "import re,sys; p=sys.argv[1]; s=open(p).read(); s=re.sub(r'^from proto import (\w+_pb2)', r'from . import \1', s, flags=re.M); open(p,'w').write(s)" "$$f"; \
	done

# =============================================================================
# Lab cluster (dev / validation) — default ports in the 14xxx range so
# it can coexist with both other local services AND the CI cluster
# (2xxxx range below).
#   Postgres       14432
#   Temporal       14733
#   Temporal UI    14080
#   MQTT           14883
#   MQTT dashboard 14093
#   Registry       14050
#
# Override any of POSTGRES_PORT TEMPORAL_PORT TEMPORAL_UI_PORT MQTT_PORT
# MQTT_DASHBOARD_PORT REGISTRY_PORT to relocate.
# =============================================================================

LAB_PROJECT := temporal-hack-lab

.PHONY: lab-up
lab-up: container-check ## Bring up local lab stack (Postgres + Temporal + MQTT + registry)
	@echo "[$(CONTAINER_ENGINE)] bringing up lab stack via '$(COMPOSE)'"
	cd installer/docker-compose && CONTAINER_SOCK=$(CONTAINER_SOCK) \
	  $(COMPOSE) -p $(LAB_PROJECT) up -d
	@echo "Lab stack starting. Run 'make lab-status' to check."

.PHONY: lab-down
lab-down: container-check ## Tear down local lab stack
	cd installer/docker-compose && $(COMPOSE) -p $(LAB_PROJECT) down

.PHONY: lab-status
lab-status: ## Probe lab services
	@LAB_TEMPORAL_UI_PORT="$${TEMPORAL_UI_PORT:-14080}"; \
	 LAB_MQTT_PORT="$${MQTT_PORT:-14883}"; \
	 LAB_POSTGRES_PORT="$${POSTGRES_PORT:-14432}"; \
	 LAB_REGISTRY_PORT="$${REGISTRY_PORT:-14050}"; \
	 echo "--- engine: $(CONTAINER_ENGINE) (compose: $(COMPOSE)) ---"; \
	 nc -z localhost "$$LAB_TEMPORAL_UI_PORT" 2>/dev/null && echo "Temporal UI:    up   (:$$LAB_TEMPORAL_UI_PORT)" || echo "Temporal UI:    down (:$$LAB_TEMPORAL_UI_PORT)"; \
	 nc -z localhost "$$LAB_MQTT_PORT"        2>/dev/null && echo "MQTT broker:    up   (:$$LAB_MQTT_PORT)"        || echo "MQTT broker:    down (:$$LAB_MQTT_PORT)"; \
	 nc -z localhost "$$LAB_POSTGRES_PORT"    2>/dev/null && echo "Postgres:       up   (:$$LAB_POSTGRES_PORT)"    || echo "Postgres:       down (:$$LAB_POSTGRES_PORT)"; \
	 nc -z localhost "$$LAB_REGISTRY_PORT"    2>/dev/null && echo "Registry:       up   (:$$LAB_REGISTRY_PORT)"    || echo "Registry:       down (:$$LAB_REGISTRY_PORT)"

.PHONY: lab-reset
lab-reset: container-check ## Wipe lab state and restart
	cd installer/docker-compose && $(COMPOSE) -p $(LAB_PROJECT) down -v
	$(MAKE) lab-up

.PHONY: dummy-robot-image
dummy-robot-image: container-check ## Build and push lab OTA placeholder (Alpine, PID 1 sleeps forever)
	$(CONTAINER_ENGINE) build -t localhost:5001/robot-app:v1 -f docker/dummy-robot/Dockerfile docker/dummy-robot
	$(CONTAINER_ENGINE) push localhost:5001/robot-app:v1

.PHONY: sim-up
sim-up: container-check ## Bring up the lab stack + a Gazebo robot sim (GUI on :14680) + agent
	@echo "[$(CONTAINER_ENGINE)] bringing up sim stack via '$(COMPOSE)'"
	cd installer/docker-compose && CONTAINER_SOCK=$(CONTAINER_SOCK) \
	  $(COMPOSE) -p $(LAB_PROJECT) -f docker-compose.yml -f docker-compose.sim.yml up -d --build
	@echo
	@echo "  Gazebo GUI:  http://localhost:14680/vnc.html?autoconnect=1&resize=scale"
	@echo "  Raw VNC:     localhost:14900  (no password)"
	@echo "  Tail logs:   make sim-logs"

.PHONY: sim-up-headless
sim-up-headless: container-check ## Same as sim-up, but no GUI (gzserver only)
	@echo "[$(CONTAINER_ENGINE)] bringing up sim stack (headless) via '$(COMPOSE)'"
	cd installer/docker-compose && CONTAINER_SOCK=$(CONTAINER_SOCK) SIM_HEADLESS=1 \
	  $(COMPOSE) -p $(LAB_PROJECT) -f docker-compose.yml -f docker-compose.sim.yml up -d --build

.PHONY: sim-down
sim-down: container-check ## Tear down the lab stack + sim
	cd installer/docker-compose && \
	  $(COMPOSE) -p $(LAB_PROJECT) -f docker-compose.yml -f docker-compose.sim.yml down

.PHONY: sim-logs
sim-logs: container-check ## Tail logs from sim + agent
	cd installer/docker-compose && \
	  $(COMPOSE) -p $(LAB_PROJECT) -f docker-compose.yml -f docker-compose.sim.yml logs -f sim agent

.PHONY: sim-gui
sim-gui: ## Open the Gazebo GUI in the default browser
	@URL="http://localhost:14680/vnc.html?autoconnect=1&resize=scale"; \
	 echo "$$URL"; \
	 (command -v open  >/dev/null && open  "$$URL") || \
	 (command -v xdg-open >/dev/null && xdg-open "$$URL") || \
	 echo "Open it manually."

# Drive helpers for the perseverance rover in the moon world. These
# publish straight to the in-container Ignition topic (no ROS install
# on the host required). Override LX / AZ to vary speed.
LX ?= 0.5
AZ ?= 0.0
SIM_CONTAINER ?= temporal-hack-lab-sim-1
GZ_DRIVE_TOPIC ?= /model/perseverance/cmd_vel

.PHONY: sim-drive-fwd
sim-drive-fwd: ## Drive the rover forward (LX m/s, override LX=)
	@$(CONTAINER_ENGINE) exec $(SIM_CONTAINER) bash -c \
	  'ign topic -t $(GZ_DRIVE_TOPIC) -m ignition.msgs.Twist -p "linear: {x: $(LX)}"'

.PHONY: sim-drive-back
sim-drive-back: ## Drive the rover backward
	@$(CONTAINER_ENGINE) exec $(SIM_CONTAINER) bash -c \
	  'ign topic -t $(GZ_DRIVE_TOPIC) -m ignition.msgs.Twist -p "linear: {x: -$(LX)}"'

.PHONY: sim-drive-left
sim-drive-left: ## Spin in place, left
	@$(CONTAINER_ENGINE) exec $(SIM_CONTAINER) bash -c \
	  'ign topic -t $(GZ_DRIVE_TOPIC) -m ignition.msgs.Twist -p "angular: {z: 0.5}"'

.PHONY: sim-drive-right
sim-drive-right: ## Spin in place, right
	@$(CONTAINER_ENGINE) exec $(SIM_CONTAINER) bash -c \
	  'ign topic -t $(GZ_DRIVE_TOPIC) -m ignition.msgs.Twist -p "angular: {z: -0.5}"'

.PHONY: sim-drive-stop
sim-drive-stop: ## Stop the rover
	@$(CONTAINER_ENGINE) exec $(SIM_CONTAINER) bash -c \
	  'ign topic -t $(GZ_DRIVE_TOPIC) -m ignition.msgs.Twist -p "linear: {x: 0}, angular: {z: 0}"'

# Teleport the rover back to the origin. Useful when an OTA controller
# runs the rover off the world or into the boulder. Override
# TELEPORT_X / TELEPORT_Y / TELEPORT_Z to land somewhere else.
SIM_GAZEBO_CONTAINER ?= temporal-hack-lab-gazebo-1
SIM_WORLD_NAME ?= moon
ROBOT_MODEL ?= perseverance
TELEPORT_X ?= 0
TELEPORT_Y ?= 0
TELEPORT_Z ?= 0.30

.PHONY: sim-teleport
sim-teleport: ## Teleport the rover back to the origin (override TELEPORT_X/Y/Z)
	@echo "[sim-teleport] $(ROBOT_MODEL) -> ($(TELEPORT_X), $(TELEPORT_Y), $(TELEPORT_Z))"
	@# Stop the rover first so its old velocity doesn't get carried over.
	@$(CONTAINER_ENGINE) exec $(SIM_GAZEBO_CONTAINER) bash -c \
	  'ign topic -t $(GZ_DRIVE_TOPIC) -m ignition.msgs.Twist -p "linear: {x: 0}, angular: {z: 0}"' >/dev/null 2>&1 || true
	@# set_pose service from inside the gazebo container. Orientation
	@# is identity quaternion (w=1) — rover faces +x.
	@$(CONTAINER_ENGINE) exec $(SIM_GAZEBO_CONTAINER) bash -c \
	  'ign service -s /world/$(SIM_WORLD_NAME)/set_pose \
	     --reqtype ignition.msgs.Pose --reptype ignition.msgs.Boolean \
	     --timeout 2000 \
	     --req "name: \"$(ROBOT_MODEL)\", position: {x: $(TELEPORT_X), y: $(TELEPORT_Y), z: $(TELEPORT_Z)}, orientation: {w: 1.0}"'

# =============================================================================
# Web console (operator UI) — Vite + React. Dev server proxies /v1 to
# the controlplane (default localhost:8081). Override CONTROLPLANE_URL=
# to point at a different backend.
# =============================================================================

WEB_DIR ?= web
WEB_PID := .run/web.pid

.PHONY: web-install
web-install: ## Install web console deps
	cd $(WEB_DIR) && npm install

.PHONY: web-up
web-up: ## Start the web console dev server (http://127.0.0.1:5173)
	@mkdir -p .run
	@if [ -f $(WEB_PID) ] && kill -0 $$(cat $(WEB_PID)) 2>/dev/null; then \
	  echo "[web] already running (pid $$(cat $(WEB_PID)))"; exit 0; fi
	@if [ ! -d $(WEB_DIR)/node_modules ]; then $(MAKE) web-install; fi
	cd $(WEB_DIR) && nohup npm run dev >../.run/web.log 2>&1 & echo $$! >$(WEB_PID)
	@echo "[web] started → http://127.0.0.1:5173/  (logs: .run/web.log)"

.PHONY: web-down
web-down: ## Stop the web console dev server
	@if [ -f $(WEB_PID) ]; then \
	  kill $$(cat $(WEB_PID)) 2>/dev/null || true; rm -f $(WEB_PID); \
	  echo "[web] stopped"; \
	else echo "[web] not running"; fi

.PHONY: web-status
web-status: ## Show whether the web dev server is up
	@if [ -f $(WEB_PID) ] && kill -0 $$(cat $(WEB_PID)) 2>/dev/null; then \
	  echo "[web] running (pid $$(cat $(WEB_PID)))"; \
	else echo "[web] not running"; fi

.PHONY: web-build
web-build: ## Production-build the web console (web/dist/)
	cd $(WEB_DIR) && npm run build

# =============================================================================
# CI cluster (smoke / pre-push parity) — alternate ports so it can run
# alongside `make lab-up` on the same host. Used by .git-hooks/installer-smoke.sh
# and by .github/workflows/ci.yml.
#   Postgres       25432
#   Temporal       27233
#   Temporal UI    28080
#   MQTT           21883
#   MQTT dashboard 28083
#   Registry       25050
# =============================================================================

CI_PROJECT := temporal-hack-ci
CI_FILES   := -f docker-compose.yml -f docker-compose.ci.yml

# =============================================================================
# Demo reset — wipes all transient demo state and (optionally) brings
# the stack back up clean.
#
#   make demo-reset           stops everything, wipes volumes + .run/,
#                             and BRINGS THE STACK BACK UP fresh
#   make demo-reset NOUP=1    same, but stops short of starting again
# =============================================================================

.PHONY: demo-reset
demo-reset: container-check ## Stop everything, wipe demo state, and start fresh (NOUP=1 to skip the bring-up)
	@echo "[demo-reset] stopping host-side processes"
	-@$(MAKE) -s controlplane-down
	-@$(MAKE) -s workers-down
	-@$(MAKE) -s agent-down
	@echo "[demo-reset] killing OTA-spawned robot-app containers"
	-@$(CONTAINER_ENGINE) rm -f robot-app robot-app-new >/dev/null 2>&1 || true
	@echo "[demo-reset] tearing down sim + lab compose, wiping volumes"
	-@cd installer/docker-compose && $(COMPOSE) -p $(LAB_PROJECT) \
	  -f docker-compose.yml -f docker-compose.sim.yml down -v >/dev/null 2>&1 || true
	@echo "[demo-reset] clearing .run/ pid files and logs"
	@rm -rf .run/
	@if [ "$${NOUP:-0}" = "1" ]; then \
	  echo "[demo-reset] NOUP=1 — stopping after teardown"; \
	  exit 0; \
	fi
	@echo "[demo-reset] bringing the stack back up"
	@$(MAKE) -s sim-up
	@$(MAKE) -s agent-up
	@$(MAKE) -s workers-up
	@$(MAKE) -s controlplane-up
	@echo
	@echo "  demo reset complete. Re-run a demo:"
	@echo "    make ota-circle"
	@echo "    make collide"
	@echo "  GUI: http://localhost:14680/vnc.html?autoconnect=1&resize=scale"
	@echo "  Temporal UI: http://localhost:14080"

.PHONY: ci-up
ci-up: container-check ## Bring up an isolated CI/smoke cluster on alternate ports
	@echo "[$(CONTAINER_ENGINE)] bringing up CI stack on alt ports"
	cd installer/docker-compose && CONTAINER_SOCK=$(CONTAINER_SOCK) \
	  $(COMPOSE) -p $(CI_PROJECT) $(CI_FILES) up -d --wait

.PHONY: ci-down
ci-down: container-check ## Tear down the CI/smoke cluster (and wipe its volumes)
	cd installer/docker-compose && $(COMPOSE) -p $(CI_PROJECT) $(CI_FILES) down -v

.PHONY: ci-status
ci-status: ## Probe CI services on their alternate ports
	@echo "--- engine: $(CONTAINER_ENGINE) (compose: $(COMPOSE)) [ci] ---"
	@nc -z localhost 28080 2>/dev/null && echo "Temporal UI:    up   (:28080)" || echo "Temporal UI:    down (:28080)"
	@nc -z localhost 21883 2>/dev/null && echo "MQTT broker:    up   (:21883)" || echo "MQTT broker:    down (:21883)"
	@nc -z localhost 25432 2>/dev/null && echo "Postgres:       up   (:25432)" || echo "Postgres:       down (:25432)"
	@nc -z localhost 25050 2>/dev/null && echo "Registry:       up   (:25050)" || echo "Registry:       down (:25050)"
