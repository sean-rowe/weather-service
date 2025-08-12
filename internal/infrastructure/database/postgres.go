package database

import (
    "context"
    "database/sql"
    "fmt"
    "time"

    _ "github.com/lib/pq"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.uber.org/zap"
)

type PostgresDB struct {
    db     *sql.DB
    logger *zap.Logger
}

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

func NewPostgresDB(cfg Config, logger *zap.Logger) (*PostgresDB, error) {
    dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)
    
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

func (p *PostgresDB) createTables() error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS audit_logs (
            id SERIAL PRIMARY KEY,
            correlation_id VARCHAR(36),
            request_id VARCHAR(36),
            timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            method VARCHAR(10),
            path VARCHAR(255),
            status_code INT,
            duration_ms BIGINT,
            user_agent TEXT,
            remote_addr VARCHAR(45),
            error_message TEXT,
            metadata JSONB
        )`,
        `CREATE INDEX IF NOT EXISTS idx_audit_logs_correlation_id ON audit_logs(correlation_id)`,
        `CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp)`,
        
        `CREATE TABLE IF NOT EXISTS weather_requests (
            id SERIAL PRIMARY KEY,
            request_id VARCHAR(36) UNIQUE,
            timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            latitude DECIMAL(10, 6),
            longitude DECIMAL(10, 6),
            temperature DECIMAL(5, 2),
            temperature_unit VARCHAR(1),
            forecast TEXT,
            category VARCHAR(20),
            response_time_ms INT,
            cache_hit BOOLEAN DEFAULT FALSE
        )`,
        `CREATE INDEX IF NOT EXISTS idx_weather_requests_timestamp ON weather_requests(timestamp)`,
        `CREATE INDEX IF NOT EXISTS idx_weather_requests_coordinates ON weather_requests(latitude, longitude)`,
    }
    
    for _, query := range queries {
        if _, err := p.db.Exec(query); err != nil {
            return fmt.Errorf("failed to execute query: %w", err)
        }
    }
    
    return nil
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

func (p *PostgresDB) LogAudit(ctx context.Context, log AuditLog) error {
    tracer := otel.Tracer("database")
    ctx, span := tracer.Start(ctx, "LogAudit")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("correlation_id", log.CorrelationID),
        attribute.String("request_id", log.RequestID),
    )
    
    query := `
        INSERT INTO audit_logs (
            correlation_id, request_id, method, path, status_code, 
            duration_ms, user_agent, remote_addr, error_message, metadata
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
    `
    
    start := time.Now()
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
        log.Metadata,
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

func (p *PostgresDB) LogWeatherRequest(ctx context.Context, req WeatherRequest) error {
    tracer := otel.Tracer("database")
    ctx, span := tracer.Start(ctx, "LogWeatherRequest")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("request_id", req.RequestID),
        attribute.Float64("latitude", req.Latitude),
        attribute.Float64("longitude", req.Longitude),
    )
    
    query := `
        INSERT INTO weather_requests (
            request_id, latitude, longitude, temperature, temperature_unit,
            forecast, category, response_time_ms, cache_hit
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `
    
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

func (p *PostgresDB) GetRequestStats(ctx context.Context, since time.Time) (map[string]interface{}, error) {
    query := `
        SELECT 
            COUNT(*) as total_requests,
            AVG(response_time_ms) as avg_response_time,
            MIN(response_time_ms) as min_response_time,
            MAX(response_time_ms) as max_response_time,
            SUM(CASE WHEN cache_hit THEN 1 ELSE 0 END)::float / COUNT(*)::float as cache_hit_rate
        FROM weather_requests
        WHERE timestamp >= $1
    `
    
    var stats struct {
        TotalRequests   int
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

func (p *PostgresDB) Close() error {
    return p.db.Close()
}

func (p *PostgresDB) Ping() error {
    return p.db.Ping()
}