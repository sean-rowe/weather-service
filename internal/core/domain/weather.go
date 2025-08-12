package domain

import (
    "errors"
    "fmt"
)

type Temperature struct {
    Value float64
    Unit  TemperatureUnit
}

type TemperatureUnit string

const (
    Celsius    TemperatureUnit = "C"
    Fahrenheit TemperatureUnit = "F"
)

type TemperatureCategory string

const (
    Hot      TemperatureCategory = "hot"
    Cold     TemperatureCategory = "cold"
    Moderate TemperatureCategory = "moderate"
)

type Coordinates struct {
    Latitude  float64
    Longitude float64
}

func (c Coordinates) Validate() error {
    if c.Latitude < -90 || c.Latitude > 90 {
        return fmt.Errorf("latitude must be between -90 and 90, got %f", c.Latitude)
    }
    if c.Longitude < -180 || c.Longitude > 180 {
        return fmt.Errorf("longitude must be between -180 and 180, got %f", c.Longitude)
    }
    return nil
}

type Weather struct {
    Coordinates  Coordinates
    Temperature  Temperature
    Forecast     string
    Category     TemperatureCategory
}

type WeatherError struct {
    Code    string
    Message string
    Cause   error
}

func (e WeatherError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

var (
    ErrInvalidCoordinates = errors.New("invalid coordinates")
    ErrWeatherNotFound    = errors.New("weather data not found")
    ErrExternalService    = errors.New("external service error")
)