// Package app provides application initialization and dependency injection.
package app

import (
	"context"

	"github.com/sean-rowe/weather-service/internal/adapters/secondary/nws"
	"github.com/sean-rowe/weather-service/internal/core/domain"
	"github.com/sean-rowe/weather-service/internal/core/ports"
	"github.com/sean-rowe/weather-service/internal/infrastructure/circuitbreaker"
)

// CircuitBreakerWeatherClient wraps a weather client with circuit breaker protection
// to provide fault tolerance for external API calls.
type CircuitBreakerWeatherClient struct {
	client *nws.Client
	cb     *circuitbreaker.CircuitBreakerWrapper
}

// GetForecast retrieves weather forecast data with circuit breaker protection.
// It wraps the underlying weather client call to handle failures gracefully.
func (c *CircuitBreakerWeatherClient) GetForecast(ctx context.Context, coords domain.Coordinates) (*ports.WeatherData, error) {
	var result *ports.WeatherData

	err := c.cb.Execute(ctx, "get-forecast", func() error {
		var err error
		result, err = c.client.GetForecast(ctx, coords)

		return err
	})

	return result, err
}
