SHELL := /usr/bin/env bash

CLOUD_PKG := ./cloud/...
AGENT_PKG := ./agent/...

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
  # Rootless podman: $XDG_RUNTIME_DIR/podman/podman.sock. Rootful: /run/podman/podman.sock.
  CONTAINER_SOCK := $(if $(XDG_RUNTIME_DIR),$(XDG_RUNTIME_DIR)/podman/podman.sock,/run/podman/podman.sock)
else
  COMPOSE := /bin/false
  CONTAINER_SOCK :=
endif

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
proto: ## Regenerate protobuf bindings
	@command -v protoc >/dev/null || { echo "protoc not installed"; exit 1; }
	protoc \
		--go_out=. --go_opt=module=github.com/example/temporal-hack \
		--go-grpc_out=. --go-grpc_opt=module=github.com/example/temporal-hack \
		--python_out=bridge \
		--grpc_python_out=bridge \
		proto/*.proto

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
sim-up: container-check ## Bring up the lab stack + a Gazebo robot sim with bridge + agent
	@echo "[$(CONTAINER_ENGINE)] bringing up sim stack via '$(COMPOSE)'"
	cd installer/docker-compose && CONTAINER_SOCK=$(CONTAINER_SOCK) \
	  $(COMPOSE) -p $(LAB_PROJECT) -f docker-compose.yml -f docker-compose.sim.yml up -d --build

.PHONY: sim-down
sim-down: container-check ## Tear down the lab stack + sim
	cd installer/docker-compose && \
	  $(COMPOSE) -p $(LAB_PROJECT) -f docker-compose.yml -f docker-compose.sim.yml down

.PHONY: sim-logs
sim-logs: container-check ## Tail logs from sim + agent
	cd installer/docker-compose && \
	  $(COMPOSE) -p $(LAB_PROJECT) -f docker-compose.yml -f docker-compose.sim.yml logs -f sim agent

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
