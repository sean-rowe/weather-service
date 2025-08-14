// Package database provides PostgreSQL database operations for the weather service.
// It handles connection pooling, audit logging, and request analytics.
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// PostgresDB manages PostgreSQL database connections and operations.
type PostgresDB struct {
	db     *sql.DB
	logger *zap.Logger
}

// Config contains PostgreSQL connection configuration.
type Config struct {
	Host                  string
	Port                  int
	User                  string
	Password              string
	Database              string
	SSLMode               string
	MaxConnections        int
	MaxIdleConnections    int
	ConnectionMaxLifetime time.Duration
}

// NewPostgresDB creates a new PostgreSQL database connection with pooling.
//
// Parameters:
//   - cfg: Database configuration including connection settings
//   - logger: Zap logger for database operation logging
//
// Returns:
//   - *PostgresDB: Configured database connection
//   - error: Connection error, ping failure, or table creation error
func NewPostgresDB(cfg Config, logger *zap.Logger) (*PostgresDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)

	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnectionMaxLifetime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	pgDB := &PostgresDB{
		db:     db,
		logger: logger,
	}

	if err := pgDB.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return pgDB, nil
}

// createTables ensures database schema is up to date.
//
// Returns:
//   - error: Migration execution error if setup fails
func (p *PostgresDB) createTables() error {
	return RunMigrations(p.db, p.logger)
}


type AuditLog struct {
	CorrelationID string
	RequestID     string
	Method        string
	Path          string
	StatusCode    int
	DurationMs    int64
	UserAgent     string
	RemoteAddr    string
	ErrorMessage  *string
	Metadata      map[string]interface{}
}

// LogAudit records an audit entry for request tracking and compliance.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - log: Audit log entry with request details
//
// Returns:
//   - error: Database insertion error if logging fails
func (p *PostgresDB) LogAudit(ctx context.Context, log AuditLog) error {
	tracer := otel.Tracer("database")
	ctx, span := tracer.Start(ctx, "LogAudit")

	defer span.End()

	span.SetAttributes(
		attribute.String("correlation_id", log.CorrelationID),
		attribute.String("request_id", log.RequestID),
	)

	// Call the stored procedure
	query := `CALL sp_log_audit($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	start := time.Now()

	// Convert metadata to JSON
	var metadataJSON []byte
	if log.Metadata != nil {
		metadataJSON, _ = json.Marshal(log.Metadata)
	}

	_, err := p.db.ExecContext(ctx, query,
		log.CorrelationID,
		log.RequestID,
		log.Method,
		log.Path,
		log.StatusCode,
		log.DurationMs,
		log.UserAgent,
		log.RemoteAddr,
		log.ErrorMessage,
		metadataJSON,
	)

	duration := time.Since(start)

	if err != nil {
		p.logger.Error("failed to log audit",
			zap.Error(err),
			zap.String("correlation_id", log.CorrelationID),
			zap.Duration("duration", duration),
		)

		span.RecordError(err)

		return err
	}

	p.logger.Debug("audit logged",
		zap.String("correlation_id", log.CorrelationID),
		zap.Duration("duration", duration),
	)

	return nil
}

type WeatherRequest struct {
	RequestID       string
	Latitude        float64
	Longitude       float64
	Temperature     float64
	TemperatureUnit string
	Forecast        string
	Category        string
	ResponseTimeMs  int
	CacheHit        bool
}

// LogWeatherRequest records details about weather API requests for analytics.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - req: Weather request details including location and response time
//
// Returns:
//   - error: Database insertion error if logging fails
func (p *PostgresDB) LogWeatherRequest(ctx context.Context, req WeatherRequest) error {
	tracer := otel.Tracer("database")
	ctx, span := tracer.Start(ctx, "LogWeatherRequest")

	defer span.End()

	span.SetAttributes(
		attribute.String("request_id", req.RequestID),
		attribute.Float64("latitude", req.Latitude),
		attribute.Float64("longitude", req.Longitude),
	)

	// Call the stored procedure
	query := `CALL sp_log_weather_request($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	start := time.Now()
	_, err := p.db.ExecContext(ctx, query,
		req.RequestID,
		req.Latitude,
		req.Longitude,
		req.Temperature,
		req.TemperatureUnit,
		req.Forecast,
		req.Category,
		req.ResponseTimeMs,
		req.CacheHit,
	)

	duration := time.Since(start)

	if err != nil {
		p.logger.Error("failed to log weather request",
			zap.Error(err),
			zap.String("request_id", req.RequestID),
			zap.Duration("duration", duration),
		)

		span.RecordError(err)

		return err
	}

	return nil
}

// GetRequestStats retrieves aggregated statistics for monitoring and reporting.
//
// Parameters:
//   - ctx: Context for query cancellation
//   - since: Start time for statistics window
//
// Returns:
//   - map[string]interface{}: Statistics including total requests, response times, cache hit rate
//   - error: Query execution error or scan error
func (p *PostgresDB) GetRequestStats(ctx context.Context, since time.Time) (map[string]interface{}, error) {
	// Call the stored function
	query := `SELECT * FROM fn_get_request_stats($1)`

	var stats struct {
		TotalRequests   int64
		AvgResponseTime sql.NullFloat64
		MinResponseTime sql.NullInt64
		MaxResponseTime sql.NullInt64
		CacheHitRate    sql.NullFloat64
	}

	err := p.db.QueryRowContext(ctx, query, since).Scan(
		&stats.TotalRequests,
		&stats.AvgResponseTime,
		&stats.MinResponseTime,
		&stats.MaxResponseTime,
		&stats.CacheHitRate,
	)

	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"total_requests":    stats.TotalRequests,
		"avg_response_time": stats.AvgResponseTime.Float64,
		"min_response_time": stats.MinResponseTime.Int64,
		"max_response_time": stats.MaxResponseTime.Int64,
		"cache_hit_rate":    stats.CacheHitRate.Float64,
	}

	return result, nil
}

// Close closes the database connection pool.
//
// Returns:
//   - error: Connection close error
func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// Ping verifies the database connection is alive.
//
// Returns:
//   - error: Connection error if a database is unreachable
func (p *PostgresDB) Ping() error {
	return p.db.Ping()
}
