package app

import (
	"context"
	"time"

	"github.com/sean-rowe/weather-service/internal/core/ports"
	"github.com/sean-rowe/weather-service/internal/infrastructure/database"
)

// DatabaseAdapter adapts the PostgresDB implementation to the ports.DatabaseRepository interface
type DatabaseAdapter struct {
	db *database.PostgresDB
}

// NewDatabaseAdapter creates a new database adapter
func NewDatabaseAdapter(db *database.PostgresDB) *DatabaseAdapter {
	return &DatabaseAdapter{db: db}
}

// LogAudit implements ports.DatabaseRepository
func (d *DatabaseAdapter) LogAudit(ctx context.Context, log ports.AuditLog) error {
	// Convert ports.AuditLog to database.AuditLog
	dbLog := database.AuditLog{
		CorrelationID: log.CorrelationID,
		RequestID:     log.RequestID,
		Method:        log.Method,
		Path:          log.Path,
		StatusCode:    log.StatusCode,
		DurationMs:    log.DurationMs,
		UserAgent:     log.UserAgent,
		RemoteAddr:    log.RemoteAddr,
		ErrorMessage:  log.ErrorMessage,
		Metadata:      log.Metadata,
	}
	
	return d.db.LogAudit(ctx, dbLog)
}

// LogWeatherRequest implements ports.DatabaseRepository
func (d *DatabaseAdapter) LogWeatherRequest(ctx context.Context, req ports.WeatherRequest) error {
	// Convert ports.WeatherRequest to database.WeatherRequest
	dbReq := database.WeatherRequest{
		RequestID:       req.RequestID,
		Latitude:        req.Latitude,
		Longitude:       req.Longitude,
		Temperature:     req.Temperature,
		TemperatureUnit: req.TemperatureUnit,
		Forecast:        req.Forecast,
		Category:        req.Category,
		ResponseTimeMs:  req.ResponseTimeMs,
		CacheHit:        req.CacheHit,
	}
	
	return d.db.LogWeatherRequest(ctx, dbReq)
}

// GetRequestStats implements ports.DatabaseRepository
func (d *DatabaseAdapter) GetRequestStats(ctx context.Context, since time.Time) (map[string]interface{}, error) {
	return d.db.GetRequestStats(ctx, since)
}