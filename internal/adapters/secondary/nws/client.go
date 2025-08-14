// Package nws implements a client for the National Weather Service API.
// This package serves as a secondary adapter, translating domain requests
// into NWS API calls and converting responses back to domain objects.
package nws

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/domain"
	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// Client implements the WeatherClient interface for the National Weather Service API.
// It handles the two-step process required by NWS: first getting grid coordinates
// from lat/lon, then fetching the actual forecast from the grid endpoint.
type Client struct {
	// baseURL is the NWS API base endpoint
	baseURL string

	// httpClient handles HTTP communication with timeout and retry logic
	httpClient *http.Client

	// logger records API interactions and errors
	logger *zap.Logger
}

// NewClient creates a new NWS API client with the specified configuration.
//
// Parameters:
//   - baseURL: NWS API base URL (typically https://api.weather.gov)
//   - httpClient: HTTP client with timeout and retry configuration
//   - logger: Zap logger for API interaction logging
//
// Returns:
//   - *Client: Configured NWS API client
func NewClient(baseURL string, httpClient *http.Client, logger *zap.Logger) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		logger:     logger,
	}
}

// pointsResponse represents the NWS API response from the /points endpoint.
// This endpoint converts latitude/longitude coordinates to NWS grid coordinates.
type pointsResponse struct {
	Properties struct {
		Forecast string `json:"forecast"`
	} `json:"properties"`
}

// forecastResponse represents the NWS API response from the forecast endpoint.
type forecastResponse struct {
	Properties struct {
		Periods []forecastPeriod `json:"periods"`
	} `json:"properties"`
}

// forecastPeriod represents a single time period in the weather forecast.
type forecastPeriod struct {
	Name            string `json:"name"`
	Temperature     int    `json:"temperature"`
	TemperatureUnit string `json:"temperatureUnit"`
	ShortForecast   string `json:"shortForecast"`
}

// GetForecast retrieves weather forecast data from the NWS API.
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - coords: Geographic coordinates for the forecast location
//
// Return:
//   - *ports.WeatherData: Weather data including temperature and forecast text
//   - error: Returns error if coordinates are invalid, API is unavailable,
//     or no forecast data is available
func (c *Client) GetForecast(ctx context.Context, coords domain.Coordinates) (*ports.WeatherData, error) {
	forecastURL, err := c.getForecastURL(ctx, coords)

	if err != nil {
		return nil, fmt.Errorf("failed to get forecast URL: %w", err)
	}

	forecast, err := c.fetchForecast(ctx, forecastURL)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch forecast: %w", err)
	}

	if len(forecast.Properties.Periods) == 0 {
		return nil, fmt.Errorf("no forecast periods available")
	}

	todayPeriod := forecast.Properties.Periods[0]
	unit := domain.Fahrenheit

	if todayPeriod.TemperatureUnit == "C" {
		unit = domain.Celsius
	}

	return &ports.WeatherData{
		Temperature: float64(todayPeriod.Temperature),
		Unit:        unit,
		Forecast:    todayPeriod.ShortForecast,
	}, nil
}

// getForecastURL retrieves the forecast endpoint URL for the given coordinates.
//
// Parameters:
//   - ctx: Context for request cancellation
//   - coords: Geographic coordinates to convert to NWS grid
//
// Returns:
//   - string: URL for the forecast endpoint
//   - error: HTTP error, non-200 status, or missing forecast URL
func (c *Client) getForecastURL(ctx context.Context, coords domain.Coordinates) (string, error) {
	url := fmt.Sprintf("%s/points/%.4f,%.4f", c.baseURL, coords.Latitude, coords.Longitude)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)

	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "WeatherService/1.0")
	
	resp, err := c.httpClient.Do(req)

	if err != nil {
		return "", err
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("NWS API returned status %d", resp.StatusCode)
	}

	var points pointsResponse

	if err := json.NewDecoder(resp.Body).Decode(&points); err != nil {
		return "", err
	}

	if points.Properties.Forecast == "" {
		return "", fmt.Errorf("no forecast URL in response")
	}

	return points.Properties.Forecast, nil
}

// fetchForecast retrieves the actual forecast data from the NWS forecast endpoint.
//
// Parameters:
//   - ctx: Context for cancellation (auto-adds 10s timeout of none)
//   - forecastURL: NWS forecast endpoint URL
//
// Returns:
//   - *forecastResponse: Parsed forecast data with periods
//   - error: HTTP error, non-200 status, or JSON decode error
func (c *Client) fetchForecast(ctx context.Context, forecastURL string) (*forecastResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", forecastURL, nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "WeatherService/1.0")

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := c.httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()

		if err != nil {
			c.logger.Error("failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NWS API returned status %d", resp.StatusCode)
	}

	var forecast forecastResponse

	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return nil, err
	}

	return &forecast, nil
}
