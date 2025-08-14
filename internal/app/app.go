// Package app provides application-level coordination and dependency injection.
// It orchestrates the initialization of all service components, manages their lifecycles,
// and provides a clean application structure following dependency inversion principles.
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/adapters/primary/rest"
	"github.com/sean-rowe/weather-service/internal/adapters/secondary/nws"
	"github.com/sean-rowe/weather-service/internal/config"
	"github.com/sean-rowe/weather-service/internal/core/ports"
	"github.com/sean-rowe/weather-service/internal/core/services"
	"github.com/sean-rowe/weather-service/internal/infrastructure/cache"
	"github.com/sean-rowe/weather-service/internal/infrastructure/circuitbreaker"
	"github.com/sean-rowe/weather-service/internal/infrastructure/database"
	"github.com/sean-rowe/weather-service/internal/infrastructure/ratelimit"
	"github.com/sean-rowe/weather-service/internal/middleware"
	"github.com/sean-rowe/weather-service/internal/observability"
	"github.com/sean-rowe/weather-service/internal/version"
)

// Server represents the HTTP server instance.
type Server struct {
	server *http.Server
	logger *zap.Logger
}

// App manages the application lifecycle and dependencies.
type App struct {
	cfg       *config.Config
	server    *Server
	logger    *zap.Logger
	telemetry *observability.Telemetry
	db        *database.PostgresDB
}

// New creates a new application instance.
//
// Returns:
//   - *App: Configured application instance
//   - error: Logger initialization error
func New() (*App, error) {
	logger, err := zap.NewProduction()

	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	cfg := config.Load()

	return &App{
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Start initializes and starts all application components.
//
// Parameters:
//   - ctx: Context for initialization
//
// Returns:
//   - error: Server start error
func (a *App) Start(ctx context.Context) error {
	if err := a.initTelemetry(ctx); err != nil {
		a.logger.Warn("failed to initialize telemetry, continuing without it", zap.Error(err))
	}

	cacheService, rateLimitService := a.initRedisServices(ctx)

	if err := a.initDatabase(); err != nil {
		a.logger.Warn("failed to connect to database, continuing without it", zap.Error(err))
	}

	weatherClient := a.initWeatherClient()
	// Create database adapter if database is available
	var dbRepo ports.DatabaseRepository
	if a.db != nil {
		dbRepo = NewDatabaseAdapter(a.db)
	}
	
	weatherService := services.NewWeatherService(weatherClient, cacheService, dbRepo, a.logger)
	weatherHandler := rest.NewWeatherHandler(weatherService, a.logger)

	rateLimitMiddleware := middleware.NewRateLimitMiddleware(
		rateLimitService,
		a.cfg.RateLimit.RPS,
		a.cfg.RateLimit.Window,
		a.logger,
	)

	router := a.setupRouter(
		weatherHandler,
		rateLimitMiddleware,
		a.telemetry,
	)

	a.server = &Server{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%s", a.cfg.Server.Port),
			Handler: router,
		},
		logger: a.logger,
	}

	go func() {
		a.logger.Info("starting HTTP server", zap.String("port", a.cfg.Server.Port))

		if err := a.server.server.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				a.logger.Fatal("failed to start server", zap.Error(err))
			}
		}
	}()

	return nil
}

// Stop gracefully shuts down all application components.
func (a *App) Stop() {
	a.logger.Info("shutting down application...")

	if a.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := a.server.server.Shutdown(shutdownCtx); err != nil {
			a.logger.Error("failed to shutdown server gracefully", zap.Error(err))
		}
	}

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Error("failed to close database connection", zap.Error(err))
		}
	}

	if a.telemetry != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.telemetry.Shutdown(shutdownCtx); err != nil {
			a.logger.Error("failed to shutdown telemetry", zap.Error(err))
		}
	}

	if err := a.logger.Sync(); err != nil {
		// Sync can fail on some platforms, ignore the error
		_ = err
	}
}

// WaitForShutdown blocks until the server receives a shutdown signal.
func (a *App) WaitForShutdown() {
	// Wait for the interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	a.logger.Info("shutdown signal received")
}

// initTelemetry initializes OpenTelemetry providers.
//
// Parameters:
//   - ctx: Context for initialization
//
// Returns:
//   - error: Telemetry initialization error
func (a *App) initTelemetry(ctx context.Context) error {
	telemetryConfig := observability.Config{
		ServiceName:    a.cfg.Observability.ServiceName,
		ServiceVersion: a.cfg.Observability.ServiceVersion,
		Environment:    a.cfg.Observability.Environment,
		OTLPEndpoint:   a.cfg.Observability.OTLPEndpoint,
		SampleRate:     a.cfg.Observability.SampleRate,
	}

	var err error
	a.telemetry, err = observability.InitTelemetry(ctx, telemetryConfig, a.logger)

	return err
}

// initRedisServices initializes Redis-based or memory-based cache and rate limiting.
//
// Parameters:
//   - ctx: Context for Redis connection testing
//
// Returns:
//   - ports.CacheService: Cache implementation (Redis or memory)
//   - ports.RateLimitService: Rate limiter implementation (Redis or memory)
func (a *App) initRedisServices(ctx context.Context) (ports.CacheService, ports.RateLimitService) {
	if !a.cfg.Redis.Enabled {
		a.logger.Info("Redis disabled, using memory-based services")

		memCache := cache.NewMemoryCache(5*time.Minute, 10*time.Minute, a.logger)
		memRateLimit := middleware.NewMemoryRateLimiter(a.logger)

		return memCache, memRateLimit
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:         a.cfg.Redis.Addr,
		Password:     a.cfg.Redis.Password,
		DB:           a.cfg.Redis.DB,
		PoolSize:     a.cfg.Redis.PoolSize,
		MinIdleConns: a.cfg.Redis.MinIdleConns,
		MaxRetries:   a.cfg.Redis.MaxRetries,
		DialTimeout:  a.cfg.Redis.DialTimeout,
		ReadTimeout:  a.cfg.Redis.ReadTimeout,
		WriteTimeout: a.cfg.Redis.WriteTimeout,
	})

	if err := redisClient.Ping(ctx).Err(); err != nil {
		a.logger.Warn("Redis connection failed, falling back to memory-based services", zap.Error(err))

		memCache := cache.NewMemoryCache(5*time.Minute, 10*time.Minute, a.logger)
		memRateLimit := middleware.NewMemoryRateLimiter(a.logger)

		return memCache, memRateLimit
	}

	a.logger.Info("Redis connected successfully")

	redisCfg := cache.Config{
		Addr:         a.cfg.Redis.Addr,
		Password:     a.cfg.Redis.Password,
		DB:           a.cfg.Redis.DB,
		PoolSize:     a.cfg.Redis.PoolSize,
		MinIdleConns: a.cfg.Redis.MinIdleConns,
		MaxRetries:   a.cfg.Redis.MaxRetries,
		DialTimeout:  a.cfg.Redis.DialTimeout,
		ReadTimeout:  a.cfg.Redis.ReadTimeout,
		WriteTimeout: a.cfg.Redis.WriteTimeout,
	}

	cacheService, _ := cache.NewRedisCache(redisCfg, a.logger)
	rateLimitService := ratelimit.NewRedisRateLimiter(redisClient, a.logger)

	return cacheService, rateLimitService
}

// initDatabase initializes PostgreSQL database connection.
//
// Returns:
//   - error: Database connection or initialization error
func (a *App) initDatabase() error {
	if !a.cfg.Database.Enabled {
		return nil
	}

	dbConfig := database.Config{
		Host:                  a.cfg.Database.Host,
		Port:                  a.cfg.Database.Port,
		User:                  a.cfg.Database.User,
		Password:              a.cfg.Database.Password,
		Database:              a.cfg.Database.Database,
		SSLMode:               a.cfg.Database.SSLMode,
		MaxConnections:        a.cfg.Database.MaxConnections,
		MaxIdleConnections:    a.cfg.Database.MaxIdleConnections,
		ConnectionMaxLifetime: a.cfg.Database.ConnectionMaxLifetime,
	}

	var err error
	a.db, err = database.NewPostgresDB(dbConfig, a.logger)

	return err
}

// setupRouter creates and configures the HTTP router with all middleware.
//
// Parameters:
//   - weatherHandler: Handler for weather endpoints
//   - rateLimitMiddleware: Rate-limiting middleware instance
//   - telemetry: Telemetry instance for observability
//
// Returns:
//   - http.Handler: Configured router with all routes and middleware
func (a *App) setupRouter(
	weatherHandler *rest.WeatherHandler,
	rateLimitMiddleware *middleware.RateLimitMiddleware,
	telemetry *observability.Telemetry,
) http.Handler {
	router := mux.NewRouter()

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}).Methods("GET")

	// Version endpoint
	router.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		versionInfo := version.Get()

		if err := json.NewEncoder(w).Encode(versionInfo); err != nil {
			a.logger.Error("failed to encode version info", zap.Error(err))
		}
	}).Methods("GET")

	// Apply observability middleware if telemetry is available
	if telemetry != nil {
		obsMiddleware := middleware.NewObservabilityMiddleware(telemetry, a.logger)
		router.Use(obsMiddleware.TracingMiddleware)
		router.Use(obsMiddleware.MetricsMiddleware)
	}

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Apply rate limiting to API routes
	if rateLimitMiddleware != nil {
		api.Use(rateLimitMiddleware.Middleware)
	}

	// Weather endpoints
	api.HandleFunc("/weather", weatherHandler.GetWeather).Methods("GET")

	return router
}

// initWeatherClient creates a weather client with circuit breaker protection.
//
// Returns:
//   - ports.WeatherClient: NWS client wrapped with circuit breaker
func (a *App) initWeatherClient() ports.WeatherClient {
	httpClient := &http.Client{
		Timeout: a.cfg.External.HTTPTimeout,
	}

	nwsClient := nws.NewClient(a.cfg.External.NWSBaseURL, httpClient, a.logger)
	cbManager := circuitbreaker.NewManager(a.logger)

	weatherClient := &CircuitBreakerWeatherClient{
		client: nwsClient,
		cb: cbManager.GetBreaker("nws-api", circuitbreaker.Config{
			MaxRequests: 3,
			Interval:    10 * time.Second,
			Timeout:     30 * time.Second,
		}),
	}

	return weatherClient
}
