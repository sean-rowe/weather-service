package services

import (
    "context"

    "go.uber.org/zap"

    "github.com/yourname/weather-service/internal/core/domain"
    "github.com/yourname/weather-service/internal/core/ports"
)

type weatherService struct {
    client ports.WeatherClient
    logger *zap.Logger
}

func NewWeatherService(client ports.WeatherClient, logger *zap.Logger) ports.WeatherService {
    return &weatherService{
        client: client,
        logger: logger,
    }
}

func (s *weatherService) GetWeather(ctx context.Context, coords domain.Coordinates) (*domain.Weather, error) {
    if err := coords.Validate(); err != nil {
        s.logger.Error("invalid coordinates", zap.Error(err))
        return nil, &domain.WeatherError{
            Code:    "INVALID_COORDINATES",
            Message: "The provided coordinates are invalid",
            Cause:   err,
        }
    }

    data, err := s.client.GetForecast(ctx, coords)
    if err != nil {
        s.logger.Error("failed to get forecast", 
            zap.Float64("latitude", coords.Latitude),
            zap.Float64("longitude", coords.Longitude),
            zap.Error(err))
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
        Coordinates: coords,
        Temperature: temperature,
        Forecast:    data.Forecast,
        Category:    category,
    }

    s.logger.Info("weather retrieved successfully",
        zap.Float64("latitude", coords.Latitude),
        zap.Float64("longitude", coords.Longitude),
        zap.String("category", string(category)))

    return weather, nil
}

func (s *weatherService) categorizeTemperature(temp domain.Temperature) domain.TemperatureCategory {
    fahrenheit := temp.Value
    if temp.Unit == domain.Celsius {
        fahrenheit = (temp.Value * 9 / 5) + 32
    }

    const (
        coldThreshold     = 50.0
        hotThreshold      = 85.0
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