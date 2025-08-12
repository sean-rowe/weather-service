package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gorilla/mux"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "go.uber.org/zap"

    "github.com/yourname/weather-service/internal/adapters/primary/rest"
    "github.com/yourname/weather-service/internal/adapters/secondary/nws"
    "github.com/yourname/weather-service/internal/core/domain"
    "github.com/yourname/weather-service/internal/core/ports"
    "github.com/yourname/weather-service/internal/core/services"
    "github.com/yourname/weather-service/internal/infrastructure/circuitbreaker"
    "github.com/yourname/weather-service/internal/infrastructure/database"
    "github.com/yourname/weather-service/internal/middleware"
    "github.com/yourname/weather-service/internal/observability"
)

func main() {
    // Initialize logger
    logger, err := zap.NewProduction()
    if err != nil {
        panic(fmt.Sprintf("failed to initialize logger: %v", err))
    }
    defer logger.Sync()

    // Configuration
    config := loadConfig()

    // Initialize observability (OpenTelemetry)
    ctx := context.Background()
    telemetryConfig := observability.Config{
        ServiceName:    "weather-service",
        ServiceVersion: getEnv("VERSION", "1.0.0"),
        Environment:    getEnv("ENVIRONMENT", "development"),
        OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
        SampleRate:     0.1,
    }

    telemetry, err := observability.InitTelemetry(ctx, telemetryConfig, logger)
    if err != nil {
        logger.Warn("failed to initialize telemetry, continuing without it", zap.Error(err))
    }
    defer func() {
        if telemetry != nil {
            shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            telemetry.Shutdown(shutdownCtx)
        }
    }()

    // Initialize database (PostgreSQL)
    var db *database.PostgresDB
    if config.DatabaseEnabled {
        dbConfig := database.Config{
            Host:                  getEnv("DB_HOST", "localhost"),
            Port:                  5432,
            User:                  getEnv("DB_USER", "weather"),
            Password:              getEnv("DB_PASSWORD", "weather123"),
            Database:              getEnv("DB_NAME", "weather_service"),
            SSLMode:               getEnv("DB_SSLMODE", "disable"),
            MaxConnections:        25,
            MaxIdleConnections:    5,
            ConnectionMaxLifetime: 5 * time.Minute,
        }

        db, err = database.NewPostgresDB(dbConfig, logger)
        if err != nil {
            logger.Warn("failed to connect to database, continuing without it", zap.Error(err))
        } else {
            defer db.Close()
        }
    }

    // Initialize circuit breaker manager
    cbManager := circuitbreaker.NewManager(logger)

    // Initialize HTTP client
    httpClient := &http.Client{
        Timeout: 30 * time.Second,
    }

    // Initialize weather client (NWS API)
    nwsBaseURL := getEnv("NWS_BASE_URL", "https://api.weather.gov")
    nwsClient := nws.NewClient(nwsBaseURL, httpClient, logger)
    
    // Wrap with circuit breaker
    weatherClient := &circuitBreakerWeatherClient{
        client: nwsClient,
        cb: cbManager.GetBreaker("nws-api", circuitbreaker.Config{
            MaxRequests:     3,
            Interval:        10 * time.Second,
            Timeout:         30 * time.Second,
            FailureRatio:    0.5,
            MinimumRequests: 3,
        }),
    }

    // Initialize domain service
    weatherService := services.NewWeatherService(weatherClient, logger)

    // Initialize HTTP handler
    weatherHandler := rest.NewWeatherHandler(weatherService, logger)

    // Setup router with middleware
    router := mux.NewRouter()

    // Apply middleware if telemetry is available
    if telemetry != nil {
        obsMiddleware := middleware.NewObservabilityMiddleware(telemetry, logger)
        router.Use(obsMiddleware.TracingMiddleware)
        router.Use(obsMiddleware.MetricsMiddleware)
        router.Use(obsMiddleware.LoggingMiddleware)
    }

    // Routes
    router.HandleFunc("/", rootHandler).Methods("GET")
    router.HandleFunc("/health", healthHandler).Methods("GET")
    router.HandleFunc("/health/live", livenessHandler).Methods("GET")
    router.HandleFunc("/health/ready", readinessHandler(db)).Methods("GET")
    router.HandleFunc("/weather", weatherHandler.GetWeather).Methods("GET")
    router.Handle("/metrics", promhttp.Handler()).Methods("GET")
    router.HandleFunc("/stats", statsHandler(cbManager, db)).Methods("GET")

    // Setup servers
    mainPort := getEnv("PORT", "8080")
    mainServer := &http.Server{
        Addr:         ":" + mainPort,
        Handler:      router,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
        IdleTimeout:  60 * time.Second,
    }

    // Start main server
    go func() {
        logger.Info("starting weather service", 
            zap.String("port", mainPort),
            zap.String("environment", getEnv("ENVIRONMENT", "development")))
        if err := mainServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            logger.Fatal("failed to start server", zap.Error(err))
        }
    }()

    // Wait for interrupt signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // Graceful shutdown
    logger.Info("shutting down servers...")
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := mainServer.Shutdown(shutdownCtx); err != nil {
        logger.Fatal("server forced to shutdown", zap.Error(err))
    }

    logger.Info("servers exited")
}

type config struct {
    DatabaseEnabled bool
}

func loadConfig() config {
    return config{
        DatabaseEnabled: getEnv("DATABASE_ENABLED", "true") == "true",
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{
        "service": "weather-service",
        "version": "1.0.0",
        "endpoints": {
            "GET /": "This page",
            "GET /health": "Health check",
            "GET /health/live": "Liveness probe",
            "GET /health/ready": "Readiness probe",
            "GET /weather?lat=LAT&lon=LON": "Get weather forecast",
            "GET /metrics": "Prometheus metrics",
            "GET /stats": "Service statistics"
        }
    }`))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"healthy","service":"weather-service","version":"1.0.0"}`))
}

func livenessHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status":"alive"}`))
}

func readinessHandler(db *database.PostgresDB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ready := true
        checks := map[string]bool{
            "server": true,
        }

        // Check database if enabled
        if db != nil {
            if err := db.Ping(); err != nil {
                ready = false
                checks["database"] = false
            } else {
                checks["database"] = true
            }
        }

        status := http.StatusOK
        if !ready {
            status = http.StatusServiceUnavailable
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(status)
        fmt.Fprintf(w, `{"ready":%t,"checks":%v}`, ready, checks)
    }
}

func statsHandler(cbManager *circuitbreaker.Manager, db *database.PostgresDB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stats := map[string]interface{}{
            "circuit_breakers": cbManager.GetStats(),
        }

        if db != nil {
            ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
            defer cancel()
            
            dbStats, err := db.GetRequestStats(ctx, time.Now().Add(-1*time.Hour))
            if err == nil {
                stats["database"] = dbStats
            }
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        
        // Simple JSON encoding
        fmt.Fprintf(w, `{"stats":%v}`, stats)
    }
}

// circuitBreakerWeatherClient wraps the weather client with circuit breaker
type circuitBreakerWeatherClient struct {
    client *nws.Client
    cb     *circuitbreaker.CircuitBreaker
}

func (c *circuitBreakerWeatherClient) GetForecast(ctx context.Context, coords domain.Coordinates) (*ports.WeatherData, error) {
    var result *ports.WeatherData
    err := c.cb.Execute(ctx, "get-forecast", func() error {
        var err error
        result, err = c.client.GetForecast(ctx, coords)
        return err
    })
    return result, err
}

