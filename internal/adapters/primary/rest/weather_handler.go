// Package rest implements HTTP handlers for weather service endpoints.
// This package serves as the primary adapter, translating HTTP requests
// into domain operations and formatting responses for clients.
package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/domain"
	"github.com/sean-rowe/weather-service/internal/core/ports"
	"github.com/sean-rowe/weather-service/internal/middleware"
)

// WeatherHandler handles HTTP requests for weather-related operations.
// It acts as the primary adapter between HTTP transport and business logic,
// managing request parsing, validation, and response formatting.
type WeatherHandler struct {
	// service provides access to weather business operations
	service ports.WeatherService

	// logger records request processing events and errors
	logger *zap.Logger
}

// NewWeatherHandler creates a new HTTP handler for weather operations.
//
// Parameters:
//   - service: WeatherService interface for business logic operations
//   - logger: Zap logger for request logging and error tracking
//
// Returns:
//   - *WeatherHandler: Configured handler instance
func NewWeatherHandler(service ports.WeatherService, logger *zap.Logger) *WeatherHandler {
	return &WeatherHandler{
		service: service,
		logger:  logger,
	}
}

// WeatherResponse represents the JSON structure returned by weather endpoints.
// This DTO maps domain objects to a client-friendly format with consistent field naming.
type WeatherResponse struct {
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
	Forecast        string  `json:"forecast"`
	Temperature     float64 `json:"temperature"`
	TemperatureUnit string  `json:"temperatureUnit"`
	Category        string  `json:"category"`
}

// ErrorResponse represents a standardized error response structure.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// GetWeather handles GET requests for weather information.
//
// Parameters:
//   - w: HTTP response writer
//   - r: HTTP request containing 'lat' and 'lon' query parameters
//
// Response codes:
//   - 200: Success with WeatherResponse JSON
//   - 400: Invalid parameters (MISSING_PARAMETERS, INVALID_LATITUDE, INVALID_LONGITUDE)
//   - 503: Service unavailable (FORECAST_RETRIEVAL_ERROR)
//   - 500: Internal server error
func (h *WeatherHandler) GetWeather(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	if latStr == "" || lonStr == "" {
		h.respondWithError(
			w,
			http.StatusBadRequest,
			"MISSING_PARAMETERS",
			"Both 'lat' and 'lon' query parameters are required",
		)

		return
	}

	latitude, err := strconv.ParseFloat(latStr, 64)

	if err != nil {
		h.respondWithError(
			w,
			http.StatusBadRequest,
			"INVALID_LATITUDE",
			"Invalid latitude format",
		)

		return
	}

	longitude, err := strconv.ParseFloat(lonStr, 64)

	if err != nil {
		h.respondWithError(
			w,
			http.StatusBadRequest,
			"INVALID_LONGITUDE",
			"Invalid longitude format",
		)

		return
	}

	coords := domain.Coordinates{
		Latitude:  latitude,
		Longitude: longitude,
	}

	weather, err := h.service.GetWeather(r.Context(), coords)

	if err != nil {
		h.handleServiceError(w, r, err)
		return
	}

	response := WeatherResponse{
		Latitude:    weather.Coordinates.Latitude,
		Longitude:   weather.Coordinates.Longitude,
		Forecast:    weather.Forecast,
		Temperature: weather.Temperature.Value,
		TemperatureUnit: string(weather.Temperature.Unit),
		Category:    string(weather.Category),
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

// respondWithJSON sends a JSON response with the specified status code.
//
// Parameters:
//   - w: HTTP response writer
//   - status: HTTP status code to return
//   - payload: Data to encode as JSON response body
func (h *WeatherHandler) respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

// respondWithError sends a standardized error response.
//
// Parameters:
//   - w: HTTP response writer
//   - status: HTTP status code for the error
//   - code: Machine-readable error code
//   - message: Human-readable error message
func (h *WeatherHandler) respondWithError(w http.ResponseWriter, status int, code, message string) {
	response := ErrorResponse{
		Error:   code,
		Message: message,
	}

	h.respondWithJSON(w, status, response)
}

// handleServiceError maps domain errors to appropriate HTTP responses.
//
// Parameters:
//   - w: HTTP response writer
//   - r: HTTP request for context extraction
//   - err: Error from service layer to map to HTTP response
//
// Error mappings:
//   - WeatherError.INVALID_COORDINATES -> 400 Bad Request
//   - WeatherError.FORECAST_RETRIEVAL_ERROR -> 503 Service Unavailable
//   - Other errors -> 500 Internal Server Error
func (h *WeatherHandler) handleServiceError(w http.ResponseWriter, r *http.Request, err error) {
	var e *domain.WeatherError

	switch {
	case errors.As(err, &e):
		switch e.Code {
		case "INVALID_COORDINATES":
			h.respondWithError(w, http.StatusBadRequest, e.Code, e.Message)
		case "FORECAST_RETRIEVAL_ERROR":
			h.respondWithError(
				w,
				http.StatusServiceUnavailable,
				e.Code,
				"Weather service is temporarily unavailable",
			)
		default:
			h.respondWithError(
				w,
				http.StatusInternalServerError,
				"INTERNAL_ERROR",
				"An unexpected error occurred",
			)
		}
	default:
		h.logger.Error("unexpected error",
			zap.Error(err),
			zap.String("correlation_id", middleware.GetCorrelationID(r.Context())),
			zap.String("request_id", middleware.GetRequestID(r.Context())),
		)

		h.respondWithError(
			w,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"An unexpected error occurred",
		)
	}
}
