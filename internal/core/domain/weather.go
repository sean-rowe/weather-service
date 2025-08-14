// Package domain contains the core business entities and domain logic for the weather service.
// This package defines the fundamental types and business rules that are independent
// of external frameworks and infrastructure concerns.
package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Temperature represents a temperature measurement with its unit.
// It encapsulates both the numeric value and the unit of measurement
// to ensure type safety and prevent unit confusion.
type Temperature struct {
	// Value is the numeric temperature measurement
	Value float64

	// Unit specifies whether the temperature is in Celsius or Fahrenheit
	Unit TemperatureUnit
}

// TemperatureUnit defines the unit of temperature measurement.
// Only Celsius and Fahrenheit are supported as they are the most common units.
type TemperatureUnit string

const (
	// Celsius represents temperature in Celsius scale
	Celsius TemperatureUnit = "C"

	// Fahrenheit represents temperature in Fahrenheit scale
	Fahrenheit TemperatureUnit = "F"
)

// TemperatureCategory classifies temperature into human-readable categories.
type TemperatureCategory string

const (
	// Hot indicates temperatures that feel warm to hot
	Hot TemperatureCategory = "hot"

	// Cold indicates temperatures that feel cool to cold
	Cold TemperatureCategory = "cold"

	// Moderate indicates comfortable temperatures between hot and cold
	Moderate TemperatureCategory = "moderate"
)

// Coordinates represent a geographic location using latitude and longitude.
// This follows the standard geographic coordinate system used worldwide.
type Coordinates struct {
	// Latitude specifies the north-south position (-90 to 90 degrees)
	Latitude float64

	// Longitude specifies the east-west position (-180 to 180 degrees)
	Longitude float64
}

// Validate checks if the coordinates are within valid geographic bounds.
// Latitude must be between -90 and 90 degrees (south to north poles).
// Longitude must be between -180 and 180 degrees (international date line).
func (c Coordinates) Validate() error {
	if c.Latitude < -90 || c.Latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90, got %f", c.Latitude)
	}

	if c.Longitude < -180 || c.Longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180, got %f", c.Longitude)
	}

	return nil
}

// Weather represents a complete weather report for a specific location and time.
// This is the main aggregate root that combines location, temperature, forecast,
// and metadata about when the data was retrieved.
type Weather struct {
	// ID uniquely identifies this weather report
	ID uuid.UUID

	// Coordinates specify the geographic location
	Coordinates Coordinates

	// Temperature contains the current temperature measurement
	Temperature Temperature

	// Forecast provides a human-readable weather description
	Forecast string

	// Category classifies the temperature as hot, cold, or moderate
	Category TemperatureCategory

	// FetchedAt records when this weather data was retrieved
	FetchedAt time.Time
}

// WeatherError represents domain-specific errors that can occur during weather operations.
// It provides structured error information with error codes and optional underlying causes.
type WeatherError struct {
	// Code identifies the type of error for programmatic handling
	Code string

	// Message provides a human-readable error description
	Message string

	// Cause wraps an underlying error if applicable
	Cause error
}

// Error implements the error interface for WeatherError.
// It formats the error message to include the code, message, and underlying cause.
func (e WeatherError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}

	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
