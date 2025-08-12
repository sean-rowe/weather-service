package ports

import (
    "context"

    "github.com/yourname/weather-service/internal/core/domain"
)

type WeatherService interface {
    GetWeather(ctx context.Context, coords domain.Coordinates) (*domain.Weather, error)
}

type WeatherClient interface {
    GetForecast(ctx context.Context, coords domain.Coordinates) (*WeatherData, error)
}

type WeatherData struct {
    Temperature float64
    Unit        domain.TemperatureUnit
    Forecast    string
}