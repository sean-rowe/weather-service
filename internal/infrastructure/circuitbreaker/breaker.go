// Package circuitbreaker provides circuit breaker functionality for fault tolerance.
// It wraps Sony's GoBreaker library with observability instrumentation and
// management capabilities for protecting against cascading failures.
package circuitbreaker

import (
	"context"
	"time"

	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// CircuitBreakerWrapper wraps Sony's GoBreaker with additional functionality.
// It provides OpenTelemetry instrumentation, structured logging, and
// standardized error handling for circuit breaker operations.
//
//goland:noinspection GoNameStartsWithPackageName
type CircuitBreakerWrapper struct {
	breaker *gobreaker.CircuitBreaker
	logger  *zap.Logger
	name    string
}

// Config defines circuit breaker behavior and thresholds.
// It configures when the breaker opens, how long it stays open,
// and callback functions for state changes.
type Config struct {
	Name          string
	MaxRequests   uint32
	Interval      time.Duration
	Timeout       time.Duration
	ReadyToTrip   func(counts gobreaker.Counts) bool
	OnStateChange func(name string, from gobreaker.State, to gobreaker.State)
}

// NewCircuitBreaker creates a new circuit breaker with the specified configuration.
//
// Parameters:
//   - cfg: Circuit breaker configuration including thresholds and callbacks
//   - logger: Zap logger for state changes and operations
//
// Returns:
//   - *CircuitBreakerWrapper: Configured circuit breaker instance
func NewCircuitBreaker(cfg Config, logger *zap.Logger) *CircuitBreakerWrapper {
	settings := gobreaker.Settings{
		Name:        cfg.Name,
		MaxRequests: cfg.MaxRequests,
		Interval:    cfg.Interval,
		Timeout:     cfg.Timeout,
		ReadyToTrip: cfg.ReadyToTrip,
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info("circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))

			if cfg.OnStateChange != nil {
				cfg.OnStateChange(name, from, to)
			}
		},
	}

	if settings.ReadyToTrip == nil {
		settings.ReadyToTrip = func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)

			return counts.Requests >= 3 && failureRatio >= 0.5
		}
	}

	return &CircuitBreakerWrapper{
		breaker: gobreaker.NewCircuitBreaker(settings),
		logger:  logger,
		name:    cfg.Name,
	}
}

// Execute runs a function within the circuit breaker.
//
// Parameters:
//   - ctx: Context for tracing
//   - operation: Name of the operation for logging
//   - fn: Function to execute with circuit breaker protection
//
// Returns:
//   - error: Function error or gobreaker.ErrOpenState/ErrTooManyRequests
func (cb *CircuitBreakerWrapper) Execute(ctx context.Context, operation string, fn func() error) error {
	tracer := otel.Tracer("circuit-breaker")
	_, span := tracer.Start(ctx, "CircuitBreaker.Execute")

	defer span.End()

	span.SetAttributes(
		attribute.String("circuit_breaker.name", cb.name),
		attribute.String("circuit_breaker.operation", operation),
		attribute.String("circuit_breaker.state", cb.breaker.State().String()),
	)

	result, err := cb.breaker.Execute(func() (interface{}, error) {
		return nil, fn()
	})

	_ = result

	if err != nil {
		span.RecordError(err)

		cb.logger.Warn("circuit breaker execution failed",
			zap.String("name", cb.name),
			zap.String("operation", operation),
			zap.String("state", cb.breaker.State().String()),
			zap.Error(err))
	}

	span.SetAttributes(
		attribute.String("circuit_breaker.final_state", cb.breaker.State().String()),
		attribute.Bool("circuit_breaker.success", err == nil),
	)

	return err
}

// State returns the current circuit breaker state.
//
// Returns:
//   - gobreaker.State: Current state (Closed, Open, or HalfOpen)
func (cb *CircuitBreakerWrapper) State() gobreaker.State {
	return cb.breaker.State()
}

// Counts return the current circuit breaker statistics.
//
// Returns:
//   - gobreaker.Counts: Request counts and failure statistics
func (cb *CircuitBreakerWrapper) Counts() gobreaker.Counts {
	return cb.breaker.Counts()
}

// Manager manages multiple circuit breakers for different services.
type Manager struct {
	breakers map[string]*CircuitBreakerWrapper
	logger   *zap.Logger
}

// NewManager creates a new circuit breaker manager.
//
// Parameters:
//   - logger: Zap logger for circuit breaker operations
//
// Returns:
//   - *Manager: Circuit breaker manager instance
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreakerWrapper),
		logger:   logger,
	}
}

// GetBreaker retrieves or creates a circuit breaker by name.
//
// Parameters:
//   - name: Unique identifier for the circuit breaker
//   - cfg: Configuration for a new circuit breaker (ignored if already exists)
//
// Returns:
//   - *CircuitBreakerWrapper: Circuit breaker instance
func (m *Manager) GetBreaker(name string, cfg Config) *CircuitBreakerWrapper {
	if breaker, exists := m.breakers[name]; exists {
		return breaker
	}

	cfg.Name = name
	breaker := NewCircuitBreaker(cfg, m.logger)
	m.breakers[name] = breaker

	return breaker
}

// GetStats returns statistics for all managed circuit breakers.
//
// Returns:
//   - map[string]interface{}: Statistics keyed by breaker name
func (m *Manager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	for name, breaker := range m.breakers {
		counts := breaker.Counts()
		stats[name] = map[string]interface{}{
			"state":                 breaker.State().String(),
			"requests":              counts.Requests,
			"total_successes":       counts.TotalSuccesses,
			"total_failures":        counts.TotalFailures,
			"consecutive_successes": counts.ConsecutiveSuccesses,
			"consecutive_failures":  counts.ConsecutiveFailures,
		}
	}

	return stats
}
