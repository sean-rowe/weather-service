.PHONY: help build test run docker-build docker-run clean lint fmt vet \
        deps compose-up compose-down k8s-setup k8s-deploy k8s-delete \
        monitoring-up db-migrate test-integration test-performance \
        setup-local run-all stop-all

# Variables
BINARY_NAME=weather-service
DOCKER_IMAGE=weather-service:latest
GO=go
GOFLAGS=-v

# Version information
VERSION?=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS=-ldflags "-s -w \
	-X github.com/sean-rowe/weather-service/internal/version.Version=$(VERSION) \
	-X github.com/sean-rowe/weather-service/internal/version.BuildTime=$(BUILD_TIME) \
	-X github.com/sean-rowe/weather-service/internal/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/sean-rowe/weather-service/internal/version.GitBranch=$(GIT_BRANCH)"

# Default target - show help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development Commands
build: ## Build the application binary
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/server

run: ## Run the application locally (without Docker)
	DATABASE_ENABLED=false $(GO) run ./cmd/server

run-with-db: ## Run locally with database
	DB_HOST=localhost DB_PORT=5432 DB_USER=weather DB_PASSWORD="${DB_PASSWORD}" DB_NAME=weather_service \
	DATABASE_ENABLED=true $(GO) run ./cmd/server

test: ## Run unit tests
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

test-integration: ## Run integration tests
	$(GO) test -v -tags=integration ./tests/integration/...

test-performance: ## Run performance tests
	$(GO) test -v -tags=performance ./tests/performance/...

# Docker Commands
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE) .

docker-run: docker-build ## Run application in Docker
	docker run -p 8080:8080 --rm \
		-e DATABASE_ENABLED=false \
		$(DOCKER_IMAGE)

# Docker Compose Stack
compose-up: ## Start all services with docker-compose (PostgreSQL, monitoring, app)
	docker-compose up -d
	@echo "Services starting..."
	@echo "Weather Service: http://localhost:8080"
	@echo "Prometheus: http://localhost:9090"
	@echo "Grafana: http://localhost:3000 (admin/[use GF_SECURITY_ADMIN_PASSWORD])"
	@echo "Jaeger: http://localhost:16686"
	@echo "PostgreSQL: localhost:5432 (weather/[use DB_PASSWORD])"

compose-down: ## Stop all docker-compose services
	docker-compose down

compose-logs: ## Show logs from all services
	docker-compose logs -f

compose-restart: ## Restart all services
	docker-compose restart

# Kubernetes Commands
k8s-setup: ## Setup local Kubernetes with kind
	@./scripts/k8s-setup.sh

k8s-deploy: ## Deploy to Kubernetes
	kubectl apply -f k8s/base/

k8s-delete: ## Delete from Kubernetes
	kubectl delete -f k8s/base/

k8s-port-forward: ## Port forward to access service
	kubectl port-forward service/weather-service 8080:80

# Database Commands
migrate-up: ## Run all pending database migrations
	$(GO) run ./cmd/migrate -action=up

migrate-down: ## Rollback last database migration
	$(GO) run ./cmd/migrate -action=down

migrate-version: ## Migrate to specific version (use with VERSION=n)
	$(GO) run ./cmd/migrate -action=version -version=$(VERSION)

migrate-force: ## Force set migration version (use with VERSION=n)
	$(GO) run ./cmd/migrate -action=force -version=$(VERSION)

db-shell: ## Connect to PostgreSQL shell
	docker exec -it weather-postgres psql -U weather -d weather_service

# Monitoring Commands
monitoring-up: ## Start monitoring stack only
	docker-compose up -d prometheus grafana jaeger otel-collector

grafana-import: ## Import Grafana dashboards
	@echo "Dashboards are automatically imported from provisioning"

# Dependency Management
deps: ## Download and tidy dependencies
	$(GO) mod download
	$(GO) mod tidy

# Code Quality
fmt: ## Format code
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run

check: fmt vet test ## Run all checks

# Clean
clean: ## Clean build artifacts and test cache
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	$(GO) clean -testcache

# Setup Commands
setup-local: ## Complete local setup with all dependencies
	@echo "Setting up local development environment..."
	@./scripts/setup-local.sh

# All-in-one Commands
run-all: compose-up ## Start everything (PostgreSQL, monitoring, and service)
	@echo "All services are running!"
	@echo "Waiting for services to be ready..."
	@sleep 10
	@curl -s http://localhost:8080/health || echo "Service might still be starting..."

stop-all: compose-down ## Stop everything
	@echo "All services stopped"

# Development Workflow
dev: ## Run in development mode with hot reload (requires air)
	air

watch-tests: ## Run tests in watch mode
	$(GO) test -v ./... -watch

# Deployment
deploy-gcp: ## Deploy to Google Cloud Platform
	gcloud builds submit --config=cloudbuild.yaml

# Quick Start
quickstart: setup-local run-all ## Quick start - setup and run everything
	@echo "Weather Service is ready!"
	@echo "Try: curl 'http://localhost:8080/weather?lat=40.7128&lon=-74.0060'"

# Performance Testing
load-test: ## Run load test against local service
	@echo "Running load test..."
	$(GO) test -v -tags=performance -run TestLoadSmall ./tests/performance/

# Health Checks
health-check: ## Check health of all services
	@echo "Checking services..."
	@curl -s http://localhost:8080/health | jq '.' || echo "Weather Service: Not running"
	@curl -s http://localhost:9090/-/healthy || echo "Prometheus: Not running"
	@curl -s http://localhost:3000/api/health | jq '.' || echo "Grafana: Not running"

# Logs
logs-app: ## Show application logs
	docker-compose logs -f weather-service

logs-db: ## Show database logs
	docker-compose logs -f postgres

# Default
.DEFAULT_GOAL := help