// Package ports define the interfaces that connect the core domain with external systems.
// These interfaces follow the Dependency Inversion Principle, allowing the domain layer
// to remain independent of infrastructure concerns while defining contracts for external services.
package ports

import (
	"context"
	"time"

	"github.com/sean-rowe/weather-service/internal/core/domain"
)

// WeatherService defines the primary port for weather operations.
// This interface represents the core business use cases that the application supports.
type WeatherService interface {
	// GetWeather retrieves weather information for the specified coordinates.
	// It returns a complete Weather domain object or an error if the operation fails.
	GetWeather(ctx context.Context, coords domain.Coordinates) (*domain.Weather, error)
}

// WeatherClient defines the secondary port for external weather data providers.
// This interface abstracts the communication with external weather APIs,
// allowing different implementations (NWS, OpenWeather, etc.) to be used interchangeably.
type WeatherClient interface {
	// GetForecast retrieves raw weather data from an external provider.
	// It returns basic weather information that needs to be transformed into domain objects.
	GetForecast(ctx context.Context, coords domain.Coordinates) (*WeatherData, error)
}

// WeatherData represents raw weather information from external providers.
// This is a data transfer object that bridges external APIs and our domain model.
type WeatherData struct {
	// Temperature is the current temperature value
	Temperature float64

	// Unit specifies whether the temperature is in Celsius or Fahrenheit
	Unit domain.TemperatureUnit

	// Forecast contains the weather description from the provider
	Forecast string
}

// CacheService defines the interface for caching weather data.
// This abstraction allows switching between different cache implementations
// (Redis, in-memory, file-based) without affecting business logic.
type CacheService interface {
	// Get retrieves a cached value by key, returning an error if not found
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with the specified TTL
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a cached value by key
	Delete(ctx context.Context, key string) error

	// Clear removes all cached values
	Clear(ctx context.Context) error
}

// RateLimitService defines the interface for rate-limiting functionality.
// This abstraction enables different rate-limiting strategies and storage backends.
type RateLimitService interface {
	// Allow checks if a request should be allowed based on the rate limit.
	// Returns true if allowed, false if the rate limit is exceeded.
	Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, error)

	// Reset clears the rate limit counter for the specified identifier
	Reset(ctx context.Context, identifier string) error
}

// DatabaseRepository defines the interface for persistent data operations.
// This abstraction enables audit logging and request analytics while keeping
// the business logic independent of specific database technologies.
type DatabaseRepository interface {
	// LogAudit records an audit entry for request tracking and compliance
	LogAudit(ctx context.Context, log AuditLog) error

	// LogWeatherRequest records details about weather API requests for analytics
	LogWeatherRequest(ctx context.Context, req WeatherRequest) error

	// GetRequestStats retrieves aggregated statistics for monitoring and reporting
	GetRequestStats(ctx context.Context, since time.Time) (map[string]interface{}, error)
}

// AuditLog represents a complete audit trail entry for a request.
// This structure captures all necessary information for security monitoring,
// compliance reporting, and troubleshooting.
type AuditLog struct {
	// CorrelationID links related operations across service boundaries
	CorrelationID string

	// RequestID uniquely identifies this specific request
	RequestID string

	// Method is the HTTP method used (GET, POST, etc.)
	Method string

	// Path is the requested URL path
	Path string

	// StatusCode is the HTTP response status code
	StatusCode int

	// DurationMs is the request processing time in milliseconds
	DurationMs int64

	// UserAgent contains the client's user agent string
	UserAgent string

	// RemoteAddr is the client's IP address
	RemoteAddr string

	// ErrorMessage contains error details if the request failed
	ErrorMessage *string

	// Metadata stores additional contextual information
	Metadata map[string]interface{}
}

// WeatherRequest represents a processed weather request for analytics.
// This structure captures business-relevant metrics about weather API usage
// for monitoring service performance and user behavior.
type WeatherRequest struct {
	// RequestID uniquely identifies this weather request
	RequestID string

	// Latitude of the requested location
	Latitude float64

	// Longitude of the requested location
	Longitude float64

	// Temperature value that was returned
	Temperature float64

	// TemperatureUnit specifies Celsius or Fahrenheit
	TemperatureUnit string

	// Forecast is the weather description that was returned
	Forecast string

	// Category is the temperature category (hot/cold/moderate)
	Category string

	// ResponseTimeMs tracks how long the request took to process
	ResponseTimeMs int

	// CacheHit indicates whether the response came from a cache
	CacheHit bool
}
