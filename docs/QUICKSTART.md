# Weather Service – Quick Start Guide

This repo contains a **Go-based microservice** that’s set up for real-world use and demonstrates:

* **Golang** microservice patterns
* **PostgreSQL** integration
* **Docker** and **Kubernetes** deployment
* **Prometheus / Grafana / Jaeger** monitoring
* **GCP**-friendly deployment
* **Circuit breaker** pattern for API resilience

## Prerequisites

* Go 1.21+
* Docker Desktop
* Make

## Quick Start

```bash
# Go to the project directory
cd weather-service

# Install dependencies
go mod tidy

# Spin up the stack (PostgreSQL, monitoring, app)
make run-all

# Test the API
curl 'http://localhost:8080/weather?lat=40.7128&lon=-74.0060'
```

## Accessing Services

| Service     | URL                                              | Credentials                              |
| ----------- | ------------------------------------------------ | ---------------------------------------- |
| Weather API | [http://localhost:8080](http://localhost:8080)   | —                                        |
| Prometheus  | [http://localhost:9090](http://localhost:9090)   | —                                        |
| Grafana     | [http://localhost:3000](http://localhost:3000)   | admin / \[GF\_SECURITY\_ADMIN\_PASSWORD] |
| Jaeger      | [http://localhost:16686](http://localhost:16686) | —                                        |
| PostgreSQL  | localhost:5432                                   | weather / \[DB\_PASSWORD]                |

## Key Endpoints

```bash
# Current weather
curl 'http://localhost:8080/weather?lat=40.7128&lon=-74.0060'

# Health check
curl http://localhost:8080/health

# Prometheus metrics
curl http://localhost:8080/metrics

# Circuit breaker status
curl http://localhost:8080/stats
```

## What’s Running

1. **Weather Service**

   * Clean, hexagonal architecture
   * OpenTelemetry tracing
   * Prometheus metrics
   * Circuit breaker for external calls
   * PostgreSQL audit logging

2. **PostgreSQL** – stores audit logs, request history, and analytics data

3. **Monitoring stack** – Prometheus, Grafana (preloaded dashboards), Jaeger, OpenTelemetry Collector

## Tests

```bash
make test             # Unit tests
make test-integration # Integration tests (stack required)
make load-test        # Performance tests
```

## Kubernetes Deployment (kind)

```bash
make k8s-setup
make k8s-deploy
kubectl port-forward service/weather-service 8080:80 -n weather-service
```

## Monitoring

**Grafana:** [http://localhost:3000](http://localhost:3000)
**Prometheus:** [http://localhost:9090](http://localhost:9090)
**Jaeger:** [http://localhost:16686](http://localhost:16686)

## Common Commands

```bash
make run-all      # Start everything
make stop-all     # Stop everything
make logs-app     # View app logs
make db-shell     # DB shell
make health-check # Check health of all services
```

## Project Layout

```
cmd/server/         # Entry point
internal/core/      # Business logic
internal/adapters/  # REST handlers, NWS client
internal/infra/     # DB, circuit breaker
internal/middleware # Observability
internal/obs/       # OpenTelemetry
k8s/                # Kubernetes manifests
monitoring/         # Prometheus & Grafana configs
tests/              # Integration & perf tests
docker-compose.yml  # Local stack config
```

## Troubleshooting

```bash
docker info        # Is Docker running?
lsof -i :8080      # Port in use?
make compose-down
make compose-up
docker-compose logs -f
```

## Features in Practice

* **Go microservice** with clean architecture and structured logging (Zap)
* **Cloud native**: Docker, Kubernetes, health probes, graceful shutdown
* **Observability**: tracing, metrics, logging, correlation IDs
* **Resilience**: circuit breaker, retries, timeouts
* **Database**: PostgreSQL with migrations, audit logging
* **Testing**: unit, integration, load, and BDD (Gherkin)

---
