// Package services implement the core business logic for weather operations.
// This layer orchestrates the interaction between domain objects and external services,
// implementing the primary ports defined in the port package.
package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/domain"
	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// weatherService implements the WeatherService interface and provides
// the core business logic for weather operations including caching,
// external API integration, and data transformation.
type weatherService struct {
	// client handles communication with external weather APIs
	client ports.WeatherClient

	// cache provides temporary storage for weather data to reduce API calls
	cache ports.CacheService

	// db provides database operations for logging and analytics
	db ports.DatabaseRepository

	// logger records operational events and errors
	logger *zap.Logger

	// cacheTTL defines how long weather data remains valid in cache
	cacheTTL time.Duration
}

// NewWeatherService creates a new instance of the weather service.
//
// Parameters:
//   - client: WeatherClient interface for fetching weather data from external APIs
//   - cache: CacheService interface for caching weather data
//   - db: DatabaseService interface for logging and analytics (can be nil)
//   - logger: Zap logger for recording operational events
//
// Returns:
//   - ports.WeatherService: Implementation of the WeatherService interface
func NewWeatherService(client ports.WeatherClient, cache ports.CacheService, db ports.DatabaseRepository, logger *zap.Logger) ports.WeatherService {
	return &weatherService{
		client:   client,
		cache:    cache,
		db:       db,
		logger:   logger,
		cacheTTL: 5 * time.Minute, // Cache weather data for 5 minutes
	}
}

// GetWeather retrieves weather information for the specified coordinates.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - coords: Geographic coordinates (latitude and longitude)
//
// Returns:
//   - *domain.Weather: Weather data including temperature, forecast, and category
//   - error: WeatherError with code INVALID_COORDINATES if coordinates are invalid,
//     FORECAST_RETRIEVAL_ERROR if external API fails, or other errors
func (s *weatherService) GetWeather(ctx context.Context, coords domain.Coordinates) (*domain.Weather, error) {
	if err := coords.Validate(); err != nil {
		s.logger.Error("invalid coordinates", zap.Error(err))

		return nil, &domain.WeatherError{
			Code:    "INVALID_COORDINATES",
			Message: "The provided coordinates are invalid",
			Cause:   err,
		}
	}

	// Generate cache key
	cacheKey := s.generateCacheKey(coords)

	// Try to get from the cache first
	cacheHit := false
	startTime := time.Now()
	
	if cached, err := s.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		s.logger.Debug("weather data retrieved from cache",
			zap.Float64("latitude", coords.Latitude),
			zap.Float64("longitude", coords.Longitude),
		)
		
		cacheHit = true
		
		// Log to database if available
		if s.db != nil {
			s.logWeatherRequest(ctx, coords, cached, time.Since(startTime), cacheHit)
		}

		return cached, nil
	}

	// Cache miss - fetch from external API
	data, err := s.client.GetForecast(ctx, coords)

	if err != nil {
		s.logger.Error("failed to get forecast",
			zap.Float64("latitude", coords.Latitude),
			zap.Float64("longitude", coords.Longitude),
			zap.Error(err),
		)

		return nil, &domain.WeatherError{
			Code:    "FORECAST_RETRIEVAL_ERROR",
			Message: "Failed to retrieve weather forecast",
			Cause:   err,
		}
	}

	temperature := domain.Temperature{
		Value: data.Temperature,
		Unit:  data.Unit,
	}

	category := s.categorizeTemperature(temperature)

	weather := &domain.Weather{
		ID:          uuid.New(),
		Coordinates: coords,
		Temperature: temperature,
		Forecast:    data.Forecast,
		Category:    category,
		FetchedAt:   time.Now(),
	}

	// Cache the result
	if err := s.setToCache(ctx, cacheKey, weather); err != nil {
		s.logger.Warn("failed to cache weather data", zap.Error(err))
		// Don't fail the request if caching fails
	}

	s.logger.Info("weather retrieved successfully",
		zap.Float64("latitude", coords.Latitude),
		zap.Float64("longitude", coords.Longitude),
		zap.String("category", string(category)),
	)

	// Log to database if available
	if s.db != nil {
		s.logWeatherRequest(ctx, coords, weather, time.Since(startTime), cacheHit)
	}

	return weather, nil
}

// logWeatherRequest logs weather request details to the database.
func (s *weatherService) logWeatherRequest(ctx context.Context, coords domain.Coordinates, weather *domain.Weather, responseTime time.Duration, cacheHit bool) {
	if s.db == nil {
		return
	}

	req := ports.WeatherRequest{
		RequestID:       weather.ID.String(),
		Latitude:        coords.Latitude,
		Longitude:       coords.Longitude,
		Temperature:     weather.Temperature.Value,
		TemperatureUnit: string(weather.Temperature.Unit),
		Forecast:        weather.Forecast,
		Category:        string(weather.Category),
		ResponseTimeMs:  int(responseTime.Milliseconds()),
		CacheHit:        cacheHit,
	}

	if err := s.db.LogWeatherRequest(ctx, req); err != nil {
		s.logger.Warn("failed to log weather request to database", zap.Error(err))
		// Don't fail the request if logging fails
	}
}

// generateCacheKey creates a unique cache key for the given coordinates.
//
// Parameters:
//   - coords: Geographic coordinates to generate key for
//
// Returns:
//   - string: MD5 hash of rounded coordinates for use as a cache key
func (s *weatherService) generateCacheKey(coords domain.Coordinates) string {
	// Round coordinates to reduce cache misses for nearby locations
	lat := fmt.Sprintf("%.2f", coords.Latitude)
	lon := fmt.Sprintf("%.2f", coords.Longitude)
	data := fmt.Sprintf("weather:%s:%s", lat, lon)

	return fmt.Sprintf("%x", md5.Sum([]byte(data)))
}

// getFromCache attempts to retrieve weather data from the cache.
//
// Parameters:
//   - ctx: Context for cancellation
//   - key: Cache key to look up
//
// Returns:
//   - *domain.Weather: Cached weather data if found and valid
//   - error: Cache miss error or JSON unmarshal error
func (s *weatherService) getFromCache(ctx context.Context, key string) (*domain.Weather, error) {
	data, err := s.cache.Get(ctx, key)

	if err != nil {
		return nil, err
	}

	var weather domain.Weather

	if err := json.Unmarshal(data, &weather); err != nil {
		return nil, err
	}

	return &weather, nil
}

// setToCache stores weather data in the cache with the configured TTL.
//
// Parameters:
//   - ctx: Context for cancellation
//   - key: Cache key to store data under
//   - weather: Weather data to cache
//
// Returns:
//   - error: JSON marshal error or cache storage error (non-fatal)
func (s *weatherService) setToCache(ctx context.Context, key string, weather *domain.Weather) error {
	data, err := json.Marshal(weather)

	if err != nil {
		return err
	}

	return s.cache.Set(ctx, key, data, s.cacheTTL)
}

// categorizeTemperature classifies a temperature reading into hot, cold, or moderate categories.
//
// Parameters:
//   - temp: Temperature value with unit (Celsius or Fahrenheit)
//
// Returns:
//   - domain.TemperatureCategory: Cold (<50°F), Hot (>85°F), or Moderate (50-85°F)
func (s *weatherService) categorizeTemperature(temp domain.Temperature) domain.TemperatureCategory {
	fahrenheit := temp.Value

	if temp.Unit == domain.Celsius {
		fahrenheit = (temp.Value * 9 / 5) + 32
	}

	const (
		coldThreshold = 50.0
		hotThreshold  = 85.0
	)

	switch {
	case fahrenheit < coldThreshold:
		return domain.Cold
	case fahrenheit > hotThreshold:
		return domain.Hot
	default:
		return domain.Moderate
	}
}
