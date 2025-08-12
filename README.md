# Weather Service

A production-ready weather service built with Go that provides weather information using the National Weather Service API.

## Features

- **RESTful API** for retrieving weather information by coordinates
- **Temperature categorization** (hot, cold, moderate)
- **Clean Architecture** with Domain-Driven Design
- **SOLID principles** implementation
- **BDD testing** with Gherkin scenarios
- **Docker support** for containerization
- **Comprehensive error handling** and logging
- **Graceful shutdown** support

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

- Go 1.21 or higher
- Docker (optional)
- Make (optional)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd weather-service
```

2. Install dependencies:
```bash
make deps
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

The service will start on port 8080 by default.

### API Usage

Get weather information:
```bash
curl "http://localhost:8080/weather?lat=40.7128&lon=-74.0060"
```

Response:
```json
{
  "latitude": 40.7128,
  "longitude": -74.0060,
  "forecast": "Partly Cloudy",
  "temperature": 75,
  "unit": "F",
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

### Potential Improvements for Production
- Rate limiting
- Metrics collection (Prometheus)
- Distributed tracing
- Caching layer
- Circuit breaker for external API
- Authentication/Authorization
- API versioning
- OpenAPI documentation
- Database for audit logging
- Configuration management (Viper)
- Secrets management

## Development Workflow

1. **Code Quality**: Run `make check` before committing
2. **Testing**: Ensure all tests pass with `make test`
3. **Docker**: Test containerization with `make docker-run`
4. **BDD**: Validate behavior with `make test-bdd`

## Dependencies

- **gorilla/mux**: HTTP routing
- **zap**: Structured logging
- **testify**: Testing assertions
- **godog**: BDD testing framework

## License

[Your License Here]