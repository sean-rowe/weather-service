# 🚀 Weather Service - Quick Start Guide

This is a **production-ready Go microservice** demonstrating expertise in:
- **Golang** microservices architecture
- **PostgreSQL** database integration
- **Docker** & **Kubernetes** deployment
- **Monitoring** with Prometheus/Grafana/Jaeger
- **GCP** deployment pipeline
- **Circuit breaker** pattern for resilience

## Prerequisites

- Go 1.21+
- Docker Desktop
- Make

## 🎯 Quick Start (2 minutes)

```bash
# 1. Navigate to the project
cd weather-service

# 2. Install dependencies
go mod tidy

# 3. Start everything (PostgreSQL, monitoring, service)
make run-all

# 4. Test the service
curl 'http://localhost:8080/weather?lat=40.7128&lon=-74.0060'
```

## 📊 Access Running Services

Once started, you can access:

| Service | URL | Credentials |
|---------|-----|-------------|
| **Weather API** | http://localhost:8080 | - |
| **Prometheus** | http://localhost:9090 | - |
| **Grafana** | http://localhost:3000 | admin/admin |
| **Jaeger** | http://localhost:16686 | - |
| **PostgreSQL** | localhost:5432 | weather/weather123 |

## 🔍 Key Endpoints

### Weather API
```bash
# Get weather
curl 'http://localhost:8080/weather?lat=40.7128&lon=-74.0060'

# Health check
curl http://localhost:8080/health

# Metrics (Prometheus format)
curl http://localhost:8080/metrics

# Circuit breaker stats
curl http://localhost:8080/stats
```

## 🏗️ Architecture Highlights

### What's Running

1. **Weather Service** - Main Go application with:
   - Clean hexagonal architecture
   - OpenTelemetry tracing
   - Prometheus metrics
   - Circuit breaker for external API calls
   - PostgreSQL audit logging

2. **PostgreSQL** - Database for:
   - Audit logs
   - Request tracking
   - Performance analytics

3. **Monitoring Stack**:
   - **Prometheus** - Metrics collection
   - **Grafana** - Dashboards (auto-provisioned)
   - **Jaeger** - Distributed tracing
   - **OpenTelemetry Collector** - Telemetry pipeline

## 🧪 Running Tests

```bash
# Unit tests
make test

# Integration tests (requires running stack)
make test-integration

# Performance tests
make load-test
```

## 🚢 Kubernetes Deployment

Deploy to local Kubernetes with kind:

```bash
# Setup local k8s cluster
make k8s-setup

# Deploy application
make k8s-deploy

# Access service
kubectl port-forward service/weather-service 8080:80 -n weather-service
```

## 📈 View Monitoring

### Grafana Dashboard
1. Open http://localhost:3000
2. Login with admin/admin
3. Go to Dashboards → Weather Service Monitoring

### Prometheus Metrics
1. Open http://localhost:9090
2. Try queries like:
   - `rate(http_requests_total[5m])`
   - `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))`

### Distributed Tracing
1. Open http://localhost:16686
2. Select "weather-service" from services
3. View request traces

## 🛠️ Common Commands

```bash
# Start everything
make run-all

# Stop everything
make stop-all

# View logs
make logs-app

# Connect to database
make db-shell

# Check all services health
make health-check
```

## 📁 Project Structure

```
weather-service/
├── cmd/server/           # Application entry point
├── internal/
│   ├── core/            # Business logic (hexagonal architecture)
│   ├── adapters/        # REST handlers, NWS client
│   ├── infrastructure/  # Database, circuit breaker
│   ├── middleware/      # Observability middleware
│   └── observability/   # OpenTelemetry setup
├── k8s/                 # Kubernetes manifests
├── monitoring/          # Prometheus, Grafana configs
├── tests/              # Integration & performance tests
└── docker-compose.yml   # Complete local stack
```

## 🔧 Troubleshooting

If services don't start:
```bash
# Check Docker is running
docker info

# Check port availability
lsof -i :8080
lsof -i :5432

# Restart everything
make compose-down
make compose-up

# View detailed logs
docker-compose logs -f
```

## 🎯 Features Demonstrated

✅ **Production-Ready Go Code**
- Clean architecture
- Comprehensive error handling
- Structured logging with Zap
- Context propagation

✅ **Cloud Native**
- Docker containerization
- Kubernetes deployment
- Health/readiness probes
- Graceful shutdown

✅ **Observability**
- Distributed tracing (OpenTelemetry)
- Metrics (Prometheus)
- Centralized logging
- Correlation IDs

✅ **Resilience**
- Circuit breaker pattern
- Connection pooling
- Timeout handling
- Retry logic

✅ **Database Integration**
- PostgreSQL with migrations
- Connection pooling
- Audit logging
- Query performance tracking

✅ **Testing**
- Unit tests with mocks
- Integration tests
- Performance/load tests
- BDD tests with Gherkin

## 🚀 Ready for Production

This service demonstrates all the skills needed for a **Senior Golang Engineer**:
- Microservices architecture
- Cloud platform deployment (GCP ready)
- Monitoring and observability
- Database integration
- Testing strategies
- Production debugging capabilities

---

**Need help?** Check the full README.md or run `make help` for all available commands.