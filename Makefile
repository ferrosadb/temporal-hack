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

.PHONY: lab-up
lab-up: container-check ## Bring up local lab stack (Postgres + Temporal + MQTT + registry)
	@echo "[$(CONTAINER_ENGINE)] bringing up lab stack via '$(COMPOSE)'"
	cd installer/docker-compose && CONTAINER_SOCK=$(CONTAINER_SOCK) $(COMPOSE) up -d
	@echo "Lab stack starting. Run 'make lab-status' to check."

.PHONY: lab-down
lab-down: container-check ## Tear down local lab stack
	cd installer/docker-compose && $(COMPOSE) down

.PHONY: lab-status
lab-status: ## Probe lab services
	@echo "--- engine: $(CONTAINER_ENGINE) (compose: $(COMPOSE)) ---"
	@echo "--- Temporal frontend ---"
	@curl -sf http://localhost:8080 >/dev/null && echo "Temporal UI: up" || echo "Temporal UI: down"
	@echo "--- MQTT broker ---"
	@nc -z localhost 1883 2>/dev/null && echo "MQTT 1883: up" || echo "MQTT 1883: down"
	@echo "--- Postgres ---"
	@pg_isready -h localhost -p 5432 -U temporal >/dev/null 2>&1 && echo "Postgres: up" || echo "Postgres: down"
	@echo "--- Registry ---"
	@curl -sf http://localhost:5000/v2/ >/dev/null && echo "Registry: up" || echo "Registry: down"

.PHONY: lab-reset
lab-reset: container-check ## Wipe lab state and restart
	cd installer/docker-compose && $(COMPOSE) down -v
	$(MAKE) lab-up

.PHONY: sim-up
sim-up: container-check ## Bring up the lab stack + a Gazebo robot sim with bridge + agent
	@echo "[$(CONTAINER_ENGINE)] bringing up sim stack via '$(COMPOSE)'"
	cd installer/docker-compose && CONTAINER_SOCK=$(CONTAINER_SOCK) \
	  $(COMPOSE) -f docker-compose.yml -f docker-compose.sim.yml up -d --build

.PHONY: sim-down
sim-down: container-check ## Tear down the lab stack + sim
	cd installer/docker-compose && \
	  $(COMPOSE) -f docker-compose.yml -f docker-compose.sim.yml down

.PHONY: sim-logs
sim-logs: container-check ## Tail logs from sim + agent
	cd installer/docker-compose && \
	  $(COMPOSE) -f docker-compose.yml -f docker-compose.sim.yml logs -f sim agent
