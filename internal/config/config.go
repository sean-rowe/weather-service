// Package config provides centralized configuration management for the weather service.
// It loads configuration from environment variables with sensible defaults,
// supporting different deployment environments (development, staging, production).
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration settings for the weather service.
// It aggregates configuration for all service components including
// HTTP server, databases, external APIs, and observability tools.
type Config struct {
	Server        ServerConfig
	Redis         RedisConfig
	Database      DatabaseConfig
	Observability ObservabilityConfig
	External      ExternalConfig
	RateLimit     RateLimitConfig
}

// ServerConfig contains HTTP server settings and timeouts.
// These settings control how the service handles incoming requests.
type ServerConfig struct {
	Port         string
	MetricsPort  string
	Environment  string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// RedisConfig contains settings for Redis cache and rate limiting.
// Redis is used for distributed caching and rate limiting across instances.
type RedisConfig struct {
	Enabled      bool
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DatabaseConfig contains PostgreSQL database connection settings.
type DatabaseConfig struct {
	Enabled               bool
	Host                  string
	Port                  int
	User                  string
	Password              string
	Database              string
	SSLMode               string
	MaxConnections        int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
}

// ObservabilityConfig contains settings for distributed tracing and metrics.
type ObservabilityConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	SampleRate     float64
	JaegerHost     string
}

// ExternalConfig contains settings for external API integrations.
type ExternalConfig struct {
	NWSBaseURL  string
	HTTPTimeout time.Duration
}

// RateLimitConfig contains rate limiting settings.
type RateLimitConfig struct {
	RPS    int
	Window time.Duration
}

// Load reads configuration from environment variables and returns a Config instance.
//
// Returns:
//   - *Config: Configuration with values from environment or defaults
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         getEnv("PORT", "8080"),
			MetricsPort:  getEnv("METRICS_PORT", "9090"),
			Environment:  getEnv("ENVIRONMENT", "development"),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Redis: RedisConfig{
			Enabled:      getEnvAsBool("REDIS_ENABLED", true),
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvAsInt("REDIS_DB", 0),
			PoolSize:     10,
			MinIdleConns: 5,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		},
		Database: DatabaseConfig{
			Enabled:               getEnvAsBool("DATABASE_ENABLED", false),
			Host:                  getEnv("DB_HOST", "localhost"),
			Port:                  getEnvAsInt("DB_PORT", 5432),
			User:                  getEnv("DB_USER", "weather"),
			Password:              getEnv("DB_PASSWORD", ""),
			Database:              getEnv("DB_NAME", "weather_service"),
			SSLMode:               getEnv("DB_SSLMODE", "disable"),
			MaxConnections:        25,
			MaxIdleConnections:    5,
			ConnectionMaxLifetime: 5 * time.Minute,
		},
		Observability: ObservabilityConfig{
			ServiceName:    "weather-service",
			ServiceVersion: getEnv("VERSION", "1.0.0"),
			Environment:    getEnv("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			SampleRate:     0.1,
			JaegerHost:     getEnv("JAEGER_AGENT_HOST", "localhost"),
		},
		External: ExternalConfig{
			NWSBaseURL:  getEnv("NWS_BASE_URL", "https://api.weather.gov"),
			HTTPTimeout: 30 * time.Second,
		},
		RateLimit: RateLimitConfig{
			RPS:    getEnvAsInt("RATE_LIMIT_RPS", 100),
			Window: time.Minute,
		},
	}
}

// getEnv retrieves an environment variable value with a fallback default.
//
// Parameters:
//   - key: Environment variable name
//   - defaultValue: Value to use if variable is not set
//
// Returns:
//   - string: Environment value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

// getEnvAsInt retrieves an environment variable as an integer with a fallback default.
//
// Parameters:
//   - key: Environment variable name
//   - defaultValue: Value to use if variable is not set or invalid
//
// Returns:
//   - int: Parsed integer value or default
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}

	return defaultValue
}

// getEnvAsBool retrieves an environment variable as a boolean with a fallback default.
//
// Parameters:
//   - key: Environment variable name
//   - defaultValue: Value to use if variable is not set or invalid
//
// Returns:
//   - bool: Parsed boolean value or default
func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}

	return defaultValue
}
