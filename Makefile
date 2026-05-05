SHELL := /usr/bin/env bash

CLOUD_PKG := ./cloud/...
AGENT_PKG := ./agent/...

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
lab-up: ## Bring up local lab stack (Postgres + Temporal + MQTT + registry)
	cd installer/docker-compose && docker compose up -d
	@echo "Lab stack starting. Run 'make lab-status' to check."

.PHONY: lab-down
lab-down: ## Tear down local lab stack
	cd installer/docker-compose && docker compose down

.PHONY: lab-status
lab-status: ## Probe lab services
	@echo "--- Temporal frontend ---"
	@curl -sf http://localhost:8080 >/dev/null && echo "Temporal UI: up" || echo "Temporal UI: down"
	@echo "--- MQTT broker ---"
	@nc -z localhost 1883 2>/dev/null && echo "MQTT 1883: up" || echo "MQTT 1883: down"
	@echo "--- Postgres ---"
	@pg_isready -h localhost -p 5432 -U temporal >/dev/null 2>&1 && echo "Postgres: up" || echo "Postgres: down"
	@echo "--- Registry ---"
	@curl -sf http://localhost:5000/v2/ >/dev/null && echo "Registry: up" || echo "Registry: down"

.PHONY: lab-reset
lab-reset: ## Wipe lab state and restart
	cd installer/docker-compose && docker compose down -v
	$(MAKE) lab-up

.PHONY: sim-up
sim-up: ## Bring up the lab stack + a Gazebo robot sim with bridge + agent
	cd installer/docker-compose && \
	  docker compose -f docker-compose.yml -f docker-compose.sim.yml up -d --build

.PHONY: sim-down
sim-down: ## Tear down the lab stack + sim
	cd installer/docker-compose && \
	  docker compose -f docker-compose.yml -f docker-compose.sim.yml down

.PHONY: sim-logs
sim-logs: ## Tail logs from sim + agent
	cd installer/docker-compose && \
	  docker compose -f docker-compose.yml -f docker-compose.sim.yml logs -f sim agent
