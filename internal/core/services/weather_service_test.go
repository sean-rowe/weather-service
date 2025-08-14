// Package services contain unit tests for the weather service.
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/domain"
	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// MockWeatherClient is a mock implementation of the WeatherClient interface.
type MockWeatherClient struct {
	mock.Mock
}

// GetForecast mocks the weather client GetForecast method.
//
// Parameters:
//   - ctx: Context for the request
//   - coords: Geographic coordinates
//
// Returns:
//   - *ports.WeatherData: Mocked weather data
//   - error: Mocked error if configured
func (m *MockWeatherClient) GetForecast(ctx context.Context, coords domain.Coordinates) (*ports.WeatherData, error) {
	args := m.Called(ctx, coords)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*ports.WeatherData), args.Error(1)
}

// MockCacheService is a mock implementation of the CacheService interface.
type MockCacheService struct {
	mock.Mock
}

// Get mocks the cache Get method.
//
// Parameters:
//   - ctx: Context for the request
//   - key: Cache key
//
// Returns:
//   - []byte: Mocked cached data
//   - error: Mocked error if configured
func (m *MockCacheService) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).([]byte), args.Error(1)
}

// Set mocks the cache Set method.
//
// Parameters:
//   - ctx: Context for the request
//   - key: Cache key
//   - value: Data to cache
//   - ttl: Time to live
//
// Returns:
//   - error: Mocked error if configured
func (m *MockCacheService) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

// Delete mocks the cache Delete method.
//
// Parameters:
//   - ctx: Context for the request
//   - key: Cache key to delete
//
// Returns:
//   - error: Mocked error if configured
func (m *MockCacheService) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

// Clear mocks the cache Clear method.
//
// Parameters:
//   - ctx: Context for the request
//
// Returns:
//   - error: Mocked error if configured
func (m *MockCacheService) Clear(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// TestWeatherService_GetWeather tests the GetWeather method with various scenarios.
func TestWeatherService_GetWeather(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name             string
		coords           domain.Coordinates
		mockData         *ports.WeatherData
		mockError        error
		expectedError    bool
		expectedCategory domain.TemperatureCategory
	}{
		{
			name:             "successful hot weather",
			coords:           domain.Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			expectedCategory: domain.Hot,
			mockData: &ports.WeatherData{
				Temperature: 95,
				Unit:        domain.Fahrenheit,
				Forecast:    "Sunny",
			},
		},
		{
			name:             "successful cold weather",
			coords:           domain.Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			expectedCategory: domain.Cold,
			mockData: &ports.WeatherData{
				Temperature: 40,
				Unit:        domain.Fahrenheit,
				Forecast:    "Cloudy",
			},
		},
		{
			name:             "successful moderate weather",
			coords:           domain.Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			expectedCategory: domain.Moderate,
			mockData: &ports.WeatherData{
				Temperature: 70,
				Unit:        domain.Fahrenheit,
				Forecast:    "Partly Cloudy",
			},
		},
		{
			name:             "celsius conversion",
			coords:           domain.Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			expectedCategory: domain.Hot,
			mockData: &ports.WeatherData{
				Temperature: 35,
				Unit:        domain.Celsius,
				Forecast:    "Hot",
			},
		},
		{
			name:          "invalid latitude",
			coords:        domain.Coordinates{Latitude: 91, Longitude: 0},
			expectedError: true,
		},
		{
			name:          "invalid longitude",
			coords:        domain.Coordinates{Latitude: 0, Longitude: 181},
			expectedError: true,
		},
		{
			name:          "client error",
			coords:        domain.Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			mockError:     errors.New("API error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockWeatherClient)
			mockCache := new(MockCacheService)
			service := NewWeatherService(mockClient, mockCache, nil, logger)

			// Mock cache miss to force API call
			mockCache.On("Get", mock.Anything, mock.Anything).Return(nil, errors.New("cache miss"))
			mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			if tt.mockData != nil || tt.mockError != nil {
				mockClient.On("GetForecast", mock.Anything, tt.coords).
					Return(tt.mockData, tt.mockError)
			}

			weather, err := service.GetWeather(context.Background(), tt.coords)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, weather)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, weather)
				assert.Equal(t, tt.expectedCategory, weather.Category)
				assert.Equal(t, tt.mockData.Forecast, weather.Forecast)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestWeatherService_CategorizeTemperature tests temperature categorization logic.
func TestWeatherService_CategorizeTemperature(t *testing.T) {
	logger := zap.NewNop()
	service := &weatherService{logger: logger}

	tests := []struct {
		name     string
		temp     domain.Temperature
		expected domain.TemperatureCategory
	}{
		{
			name:     "cold fahrenheit",
			temp:     domain.Temperature{Value: 45, Unit: domain.Fahrenheit},
			expected: domain.Cold,
		},
		{
			name:     "hot fahrenheit",
			temp:     domain.Temperature{Value: 90, Unit: domain.Fahrenheit},
			expected: domain.Hot,
		},
		{
			name:     "moderate fahrenheit",
			temp:     domain.Temperature{Value: 70, Unit: domain.Fahrenheit},
			expected: domain.Moderate,
		},
		{
			name:     "cold celsius",
			temp:     domain.Temperature{Value: 5, Unit: domain.Celsius},
			expected: domain.Cold,
		},
		{
			name:     "hot celsius",
			temp:     domain.Temperature{Value: 35, Unit: domain.Celsius},
			expected: domain.Hot,
		},
		{
			name:     "moderate celsius",
			temp:     domain.Temperature{Value: 20, Unit: domain.Celsius},
			expected: domain.Moderate,
		},
		{
			name:     "boundary cold",
			temp:     domain.Temperature{Value: 50, Unit: domain.Fahrenheit},
			expected: domain.Moderate,
		},
		{
			name:     "boundary hot",
			temp:     domain.Temperature{Value: 85, Unit: domain.Fahrenheit},
			expected: domain.Moderate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.categorizeTemperature(tt.temp)
			assert.Equal(t, tt.expected, result)
		})
	}
}
