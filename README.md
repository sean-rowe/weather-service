# Weather Service

Production-ready Go microservice demonstrating hexagonal architecture, GKE deployment, and comprehensive monitoring. Fetches real-time weather data from the National Weather Service API with caching, rate limiting, and circuit breaker patterns.

## Features

### Core Functionality
- **RESTful API** for retrieving weather information by coordinates
- **Temperature categorization** (hot, cold, moderate)
- **Redis caching** for improved performance
- **Rate limiting** to prevent abuse

### Architecture & Design
- **Hexagonal architecture** with clean separation of concerns
- **Domain-Driven Design** principles
- **SOLID principles** implementation
- **Circuit breaker pattern** for fault tolerance

### Infrastructure
- **Docker** multi-stage builds for minimal images
- **Kubernetes** manifests for GKE deployment
- **Google Cloud Platform** integration (GKE, Cloud Build, Artifact Registry)
- **PostgreSQL** for audit logging and analytics

### Observability
- **OpenTelemetry** distributed tracing
- **Prometheus** metrics collection
- **Grafana** dashboards for visualization
- **Structured logging** with Zap
- **Health checks** (liveness and readiness probes)

### Testing
- **Unit tests** with mocks and table-driven tests
- **Integration tests** with real components
- **BDD tests** with Cucumber/Godog
- **Performance tests** for load testing

## Architecture

This service follows hexagonal architecture (ports and adapters) with clear separation of concerns:

```
.
├── cmd/server/          # Application entry point
├── internal/
│   ├── core/           # Business logic (domain)
│   │   ├── domain/     # Domain entities and value objects
│   │   ├── ports/      # Interface definitions
│   │   └── services/   # Domain services
│   └── adapters/       # External integrations
│       ├── primary/    # REST API handlers
│       └── secondary/  # NWS API client
├── features/           # BDD test scenarios
└── docker/            # Docker configuration
```

## Getting Started

### Prerequisites

- Go 1.23 or higher
- Docker and Docker Compose
- Kubernetes (kubectl configured)
- Google Cloud SDK (for GCP deployment)
- Make

### Installation

1. Clone the repository:
```bash
git clone https://github.com/sean-rowe/weather-service.git
cd weather-service
```

2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

### Running the Service

#### Local Development
```bash
make run
```

#### Using Docker
```bash
make docker-run
```

#### Using Docker Compose
```bash
make compose-up
```

The service will start on:
- Weather API: http://localhost:8080/api/v1/weather
- Prometheus metrics: http://localhost:8080/metrics
- Health check: http://localhost:8080/health

### API Usage

Get weather information:
```bash
curl "http://localhost:8080/api/v1/weather?lat=40.7128&lon=-74.0060"
```

Response:
```json
{
  "latitude": 40.7128,
  "longitude": -74.0060,
  "forecast": "Partly Cloudy",
  "temperature": 75,
  "temperatureUnit": "F",
  "category": "moderate"
}
```

### Testing

Run unit tests:
```bash
make test
```

Run BDD tests:
```bash
make test-bdd
```

Run all checks (format, vet, lint, test):
```bash
make check
```

## Deployment

### Google Kubernetes Engine (GKE)

1. Set up GCP project:
```bash
gcloud config set project YOUR_PROJECT_ID
```

2. Create GKE cluster:
```bash
gcloud container clusters create weather-service-cluster \
  --zone us-central1-a \
  --num-nodes 3
```

3. Deploy to GKE:
```bash
kubectl apply -f k8s/base/
```

4. Get service endpoint:
```bash
kubectl get service weather-service-lb
```

### Using Cloud Build

Deploy directly from source:
```bash
gcloud builds submit --config cloudbuild.yaml .
```

## API Documentation

### Endpoints

#### GET /health
Health check endpoint.

**Response:**
- `200 OK`: Service is healthy

#### GET /weather
Get weather information for specific coordinates.

**Query Parameters:**
- `lat` (required): Latitude (-90 to 90)
- `lon` (required): Longitude (-180 to 180)

**Response:**
- `200 OK`: Weather information retrieved successfully
- `400 Bad Request`: Invalid parameters
- `503 Service Unavailable`: External service error

**Error Response Format:**
```json
{
  "error": "ERROR_CODE",
  "message": "Human-readable error message"
}
```

## Design Decisions

### Temperature Categorization
- **Cold**: Below 50°F
- **Hot**: Above 85°F  
- **Moderate**: Between 50°F and 85°F

### Architecture Choices

1. **Hexagonal Architecture**: Ensures the business logic is isolated from external concerns, making the code more testable and maintainable.

2. **Interface Segregation**: Small, focused interfaces following the Interface Segregation Principle.

3. **Dependency Inversion**: Core domain depends on abstractions, not concrete implementations.

4. **Error Handling**: Custom error types with proper context and error codes.

5. **Logging**: Structured logging with zap for production readiness.

### Testing Strategy

- **Unit Tests**: Test business logic in isolation using mocks
- **BDD Tests**: Validate behavior from user perspective
- **Table-Driven Tests**: Comprehensive test coverage with multiple scenarios

## Production Considerations

### What's Included
- Structured logging
- Graceful shutdown
- Health checks
- Docker support
- Error handling
- Input validation
- HTTP timeouts

### What's Ready for Production
- ✅ Rate limiting with Redis
- ✅ Prometheus metrics collection
- ✅ OpenTelemetry distributed tracing
- ✅ Redis caching layer
- ✅ Circuit breaker with Sony GoBreaker
- ✅ PostgreSQL audit logging
- ✅ Configuration management
- ✅ Health checks and readiness probes
- ✅ Graceful shutdown
- ✅ GKE deployment ready

### Future Enhancements
- Authentication/Authorization (OAuth2/JWT)
- API versioning
- OpenAPI/Swagger documentation
- Secrets management (Google Secret Manager)
- GraphQL endpoint
- WebSocket support for real-time updates

## Development Workflow

1. **Code Quality**: Run `make check` before committing
2. **Testing**: Ensure all tests pass with `make test`
3. **Docker**: Test containerization with `make docker-run`
4. **BDD**: Validate behavior with `make test-bdd`

## Key Dependencies

- **gorilla/mux**: HTTP routing and middleware
- **go.uber.org/zap**: High-performance structured logging
- **lib/pq**: PostgreSQL driver
- **go-redis/redis**: Redis client for caching
- **sony/gobreaker**: Circuit breaker implementation
- **opentelemetry**: Distributed tracing and metrics
- **prometheus/client_golang**: Metrics exposition
- **testify**: Testing assertions and mocks
- **godog**: BDD testing with Cucumber

## License

MIT License - See [LICENSE](LICENSE) file for details