package rest

import (
    "encoding/json"
    "net/http"
    "strconv"

    "go.uber.org/zap"

    "github.com/yourname/weather-service/internal/core/domain"
    "github.com/yourname/weather-service/internal/core/ports"
)

type WeatherHandler struct {
    service ports.WeatherService
    logger  *zap.Logger
}

func NewWeatherHandler(service ports.WeatherService, logger *zap.Logger) *WeatherHandler {
    return &WeatherHandler{
        service: service,
        logger:  logger,
    }
}

type WeatherResponse struct {
    Latitude     float64 `json:"latitude"`
    Longitude    float64 `json:"longitude"`
    Forecast     string  `json:"forecast"`
    Temperature  float64 `json:"temperature"`
    Unit         string  `json:"unit"`
    Category     string  `json:"category"`
}

type ErrorResponse struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}

func (h *WeatherHandler) GetWeather(w http.ResponseWriter, r *http.Request) {
    latStr := r.URL.Query().Get("lat")
    lonStr := r.URL.Query().Get("lon")

    if latStr == "" || lonStr == "" {
        h.respondWithError(w, http.StatusBadRequest, "MISSING_PARAMETERS", 
            "Both 'lat' and 'lon' query parameters are required")
        return
    }

    latitude, err := strconv.ParseFloat(latStr, 64)
    if err != nil {
        h.respondWithError(w, http.StatusBadRequest, "INVALID_LATITUDE", 
            "Invalid latitude format")
        return
    }

    longitude, err := strconv.ParseFloat(lonStr, 64)
    if err != nil {
        h.respondWithError(w, http.StatusBadRequest, "INVALID_LONGITUDE", 
            "Invalid longitude format")
        return
    }

    coords := domain.Coordinates{
        Latitude:  latitude,
        Longitude: longitude,
    }

    weather, err := h.service.GetWeather(r.Context(), coords)
    if err != nil {
        h.handleServiceError(w, err)
        return
    }

    response := WeatherResponse{
        Latitude:    weather.Coordinates.Latitude,
        Longitude:   weather.Coordinates.Longitude,
        Forecast:    weather.Forecast,
        Temperature: weather.Temperature.Value,
        Unit:        string(weather.Temperature.Unit),
        Category:    string(weather.Category),
    }

    h.respondWithJSON(w, http.StatusOK, response)
}

func (h *WeatherHandler) respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    
    if err := json.NewEncoder(w).Encode(payload); err != nil {
        h.logger.Error("failed to encode response", zap.Error(err))
    }
}

func (h *WeatherHandler) respondWithError(w http.ResponseWriter, status int, code, message string) {
    response := ErrorResponse{
        Error:   code,
        Message: message,
    }
    h.respondWithJSON(w, status, response)
}

func (h *WeatherHandler) handleServiceError(w http.ResponseWriter, err error) {
    switch e := err.(type) {
    case *domain.WeatherError:
        switch e.Code {
        case "INVALID_COORDINATES":
            h.respondWithError(w, http.StatusBadRequest, e.Code, e.Message)
        case "FORECAST_RETRIEVAL_ERROR":
            h.respondWithError(w, http.StatusServiceUnavailable, e.Code, 
                "Weather service is temporarily unavailable")
        default:
            h.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", 
                "An unexpected error occurred")
        }
    default:
        h.logger.Error("unexpected error", zap.Error(err))
        h.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", 
            "An unexpected error occurred")
    }
}