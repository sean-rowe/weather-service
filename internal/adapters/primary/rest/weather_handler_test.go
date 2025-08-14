// Package rest contains unit tests for REST API handlers.
package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/domain"
)

// MockWeatherService is a mock implementation of the WeatherService interface.
type MockWeatherService struct {
	mock.Mock
}

// GetWeather mocks the weather service GetWeather method.
//
// Parameters:
//   - ctx: Context for the request
//   - coords: Geographic coordinates
//
// Returns:
//   - *domain.Weather: Mocked weather data
//   - error: Mocked error if configured
func (m *MockWeatherService) GetWeather(ctx context.Context, coords domain.Coordinates) (*domain.Weather, error) {
	args := m.Called(ctx, coords)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*domain.Weather), args.Error(1)
}

// TestWeatherHandler_GetWeather tests the GetWeather handler with various scenarios.
func TestWeatherHandler_GetWeather(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		queryParams    string
		mockWeather    *domain.Weather
		mockError      error
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name:           "successful request",
			queryParams:    "?lat=40.7128&lon=-74.0060",
			expectedStatus: http.StatusOK,
			mockWeather: &domain.Weather{
				Coordinates: domain.Coordinates{Latitude: 40.7128, Longitude: -74.0060},
				Temperature: domain.Temperature{Value: 75, Unit: domain.Fahrenheit},
				Forecast:    "Partly Cloudy",
				Category:    domain.Moderate,
			},
			expectedBody: WeatherResponse{
				Latitude:    40.7128,
				Longitude:   -74.0060,
				Forecast:    "Partly Cloudy",
				Temperature: 75,
				TemperatureUnit: "F",
				Category:    "moderate",
			},
		},
		{
			name:           "missing latitude",
			queryParams:    "?lon=-74.0060",
			expectedStatus: http.StatusBadRequest,
			expectedBody: ErrorResponse{
				Error:   "MISSING_PARAMETERS",
				Message: "Both 'lat' and 'lon' query parameters are required",
			},
		},
		{
			name:           "missing longitude",
			queryParams:    "?lat=40.7128",
			expectedStatus: http.StatusBadRequest,
			expectedBody: ErrorResponse{
				Error:   "MISSING_PARAMETERS",
				Message: "Both 'lat' and 'lon' query parameters are required",
			},
		},
		{
			name:           "invalid latitude format",
			queryParams:    "?lat=invalid&lon=-74.0060",
			expectedStatus: http.StatusBadRequest,
			expectedBody: ErrorResponse{
				Error:   "INVALID_LATITUDE",
				Message: "Invalid latitude format",
			},
		},
		{
			name:           "invalid longitude format",
			queryParams:    "?lat=40.7128&lon=invalid",
			expectedStatus: http.StatusBadRequest,
			expectedBody: ErrorResponse{
				Error:   "INVALID_LONGITUDE",
				Message: "Invalid longitude format",
			},
		},
		{
			name:        "invalid coordinates error",
			queryParams: "?lat=91&lon=0",
			mockError: &domain.WeatherError{
				Code:    "INVALID_COORDINATES",
				Message: "The provided coordinates are invalid",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: ErrorResponse{
				Error:   "INVALID_COORDINATES",
				Message: "The provided coordinates are invalid",
			},
		},
		{
			name:        "service unavailable",
			queryParams: "?lat=40.7128&lon=-74.0060",
			mockError: &domain.WeatherError{
				Code:    "FORECAST_RETRIEVAL_ERROR",
				Message: "Failed to retrieve weather forecast",
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody: ErrorResponse{
				Error:   "FORECAST_RETRIEVAL_ERROR",
				Message: "Weather service is temporarily unavailable",
			},
		},
		{
			name:           "unexpected error",
			queryParams:    "?lat=40.7128&lon=-74.0060",
			mockError:      errors.New("unexpected error"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody: ErrorResponse{
				Error:   "INTERNAL_ERROR",
				Message: "An unexpected error occurred",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockWeatherService)
			handler := NewWeatherHandler(mockService, logger)

			if tt.mockWeather != nil || tt.mockError != nil {
				if tt.name == "invalid coordinates error" {
					mockService.On("GetWeather", mock.Anything, mock.MatchedBy(func(coords domain.Coordinates) bool {
						return coords.Latitude == 91
					})).Return(tt.mockWeather, tt.mockError)
				} else {
					coords := domain.Coordinates{
						Latitude:  40.7128,
						Longitude: -74.0060,
					}

					mockService.On("GetWeather", mock.Anything, coords).
						Return(tt.mockWeather, tt.mockError)
				}
			}

			req, _ := http.NewRequest("GET", "/weather"+tt.queryParams, nil)
			rr := httptest.NewRecorder()

			handler.GetWeather(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			var actualBody interface{}

			if tt.expectedStatus == http.StatusOK {
				var resp WeatherResponse

				err := json.Unmarshal(rr.Body.Bytes(), &resp)
				assert.NoError(t, err)
				actualBody = resp
			} else {
				var resp ErrorResponse

				err := json.Unmarshal(rr.Body.Bytes(), &resp)
				assert.NoError(t, err)
				actualBody = resp
			}

			assert.Equal(t, tt.expectedBody, actualBody)
			mockService.AssertExpectations(t)
		})
	}
}
