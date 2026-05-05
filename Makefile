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

# =============================================================================
# Sim overlay — Gazebo + bridge + agent on top of the lab cluster
# =============================================================================

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
