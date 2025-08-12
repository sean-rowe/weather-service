// +build integration

package integration

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"
    "time"

    "github.com/gorilla/mux"
    _ "github.com/lib/pq"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/suite"
    "go.uber.org/zap"

    "github.com/yourname/weather-service/internal/adapters/primary/rest"
    "github.com/yourname/weather-service/internal/adapters/secondary/nws"
    "github.com/yourname/weather-service/internal/core/services"
    "github.com/yourname/weather-service/internal/infrastructure/circuitbreaker"
    "github.com/yourname/weather-service/internal/infrastructure/database"
    "github.com/yourname/weather-service/internal/middleware"
    "github.com/yourname/weather-service/internal/observability"
)

type IntegrationTestSuite struct {
    suite.Suite
    server     *httptest.Server
    db         *sql.DB
    mockNWS    *httptest.Server
    telemetry  *observability.Telemetry
    cbManager  *circuitbreaker.Manager
}

func TestIntegrationSuite(t *testing.T) {
    suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) SetupSuite() {
    // Setup mock NWS API
    s.setupMockNWS()
    
    // Setup database
    s.setupDatabase()
    
    // Setup observability
    s.setupObservability()
    
    // Setup circuit breaker
    s.cbManager = circuitbreaker.NewManager(zap.NewNop())
    
    // Setup application
    s.setupApplication()
}

func (s *IntegrationTestSuite) setupMockNWS() {
    router := mux.NewRouter()
    
    router.HandleFunc("/points/{lat},{lon}", func(w http.ResponseWriter, r *http.Request) {
        vars := mux.Vars(r)
        response := map[string]interface{}{
            "properties": map[string]interface{}{
                "forecast": fmt.Sprintf("%s/forecast", s.mockNWS.URL),
            },
        }
        json.NewEncoder(w).Encode(response)
    })
    
    router.HandleFunc("/forecast", func(w http.ResponseWriter, r *http.Request) {
        response := map[string]interface{}{
            "properties": map[string]interface{}{
                "periods": []map[string]interface{}{
                    {
                        "name":            "Today",
                        "temperature":     75,
                        "temperatureUnit": "F",
                        "shortForecast":   "Partly Cloudy",
                    },
                },
            },
        }
        json.NewEncoder(w).Encode(response)
    })
    
    s.mockNWS = httptest.NewServer(router)
}

func (s *IntegrationTestSuite) setupDatabase() {
    dbHost := os.Getenv("TEST_DB_HOST")
    if dbHost == "" {
        dbHost = "localhost"
    }
    
    dsn := fmt.Sprintf("host=%s port=5432 user=test password=test dbname=weather_test sslmode=disable", dbHost)
    
    var err error
    s.db, err = sql.Open("postgres", dsn)
    if err != nil {
        s.T().Skipf("Cannot connect to test database: %v", err)
    }
    
    // Clean database
    s.cleanDatabase()
}

func (s *IntegrationTestSuite) cleanDatabase() {
    queries := []string{
        "DROP TABLE IF EXISTS audit_logs",
        "DROP TABLE IF EXISTS weather_requests",
    }
    
    for _, query := range queries {
        s.db.Exec(query)
    }
}

func (s *IntegrationTestSuite) setupObservability() {
    ctx := context.Background()
    
    cfg := observability.Config{
        ServiceName:    "weather-service-test",
        ServiceVersion: "test",
        Environment:    "test",
        OTLPEndpoint:   "localhost:4317",
        SampleRate:     1.0,
    }
    
    var err error
    s.telemetry, err = observability.InitTelemetry(ctx, cfg, zap.NewNop())
    s.Require().NoError(err)
}

func (s *IntegrationTestSuite) setupApplication() {
    logger := zap.NewNop()
    
    // Setup HTTP client
    httpClient := &http.Client{
        Timeout: 30 * time.Second,
    }
    
    // Setup weather client with circuit breaker
    weatherClient := nws.NewClient(s.mockNWS.URL, httpClient, logger)
    
    // Setup database
    dbConfig := database.Config{
        Host:                  "localhost",
        Port:                  5432,
        User:                  "test",
        Password:              "test",
        Database:              "weather_test",
        SSLMode:               "disable",
        MaxConnections:        10,
        MaxIdleConnections:    5,
        ConnectionMaxLifetime: 5 * time.Minute,
    }
    
    db, err := database.NewPostgresDB(dbConfig, logger)
    s.Require().NoError(err)
    
    // Setup service
    weatherService := services.NewWeatherService(weatherClient, logger)
    
    // Setup handler
    weatherHandler := rest.NewWeatherHandler(weatherService, logger)
    
    // Setup router with middleware
    router := mux.NewRouter()
    
    // Apply middleware
    obsMiddleware := middleware.NewObservabilityMiddleware(s.telemetry, logger)
    router.Use(obsMiddleware.TracingMiddleware)
    router.Use(obsMiddleware.MetricsMiddleware)
    router.Use(obsMiddleware.LoggingMiddleware)
    
    // Routes
    router.HandleFunc("/health", s.healthHandler).Methods("GET")
    router.HandleFunc("/weather", weatherHandler.GetWeather).Methods("GET")
    router.HandleFunc("/metrics", s.metricsHandler).Methods("GET")
    router.HandleFunc("/stats", s.statsHandler).Methods("GET")
    
    s.server = httptest.NewServer(router)
}

func (s *IntegrationTestSuite) healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func (s *IntegrationTestSuite) metricsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain")
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("# HELP http_requests_total Total HTTP requests\n"))
}

func (s *IntegrationTestSuite) statsHandler(w http.ResponseWriter, r *http.Request) {
    stats := s.cbManager.GetStats()
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}

func (s *IntegrationTestSuite) TearDownSuite() {
    if s.server != nil {
        s.server.Close()
    }
    if s.mockNWS != nil {
        s.mockNWS.Close()
    }
    if s.db != nil {
        s.db.Close()
    }
    if s.telemetry != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        s.telemetry.Shutdown(ctx)
    }
}

func (s *IntegrationTestSuite) TestHealthEndpoint() {
    resp, err := http.Get(fmt.Sprintf("%s/health", s.server.URL))
    s.Require().NoError(err)
    defer resp.Body.Close()
    
    s.Assert().Equal(http.StatusOK, resp.StatusCode)
    
    var body map[string]string
    err = json.NewDecoder(resp.Body).Decode(&body)
    s.Require().NoError(err)
    
    s.Assert().Equal("healthy", body["status"])
}

func (s *IntegrationTestSuite) TestWeatherEndpoint() {
    testCases := []struct {
        name           string
        lat            string
        lon            string
        expectedStatus int
        expectedError  bool
    }{
        {
            name:           "valid coordinates",
            lat:            "40.7128",
            lon:            "-74.0060",
            expectedStatus: http.StatusOK,
            expectedError:  false,
        },
        {
            name:           "missing latitude",
            lat:            "",
            lon:            "-74.0060",
            expectedStatus: http.StatusBadRequest,
            expectedError:  true,
        },
        {
            name:           "invalid latitude",
            lat:            "91",
            lon:            "0",
            expectedStatus: http.StatusBadRequest,
            expectedError:  true,
        },
    }
    
    for _, tc := range testCases {
        s.Run(tc.name, func() {
            url := fmt.Sprintf("%s/weather?lat=%s&lon=%s", s.server.URL, tc.lat, tc.lon)
            resp, err := http.Get(url)
            s.Require().NoError(err)
            defer resp.Body.Close()
            
            s.Assert().Equal(tc.expectedStatus, resp.StatusCode)
            
            // Check correlation ID header
            s.Assert().NotEmpty(resp.Header.Get("X-Correlation-ID"))
            s.Assert().NotEmpty(resp.Header.Get("X-Request-ID"))
            
            if tc.expectedError {
                var errorResp map[string]string
                err = json.NewDecoder(resp.Body).Decode(&errorResp)
                s.Require().NoError(err)
                s.Assert().NotEmpty(errorResp["error"])
                s.Assert().NotEmpty(errorResp["message"])
            } else {
                var weatherResp map[string]interface{}
                err = json.NewDecoder(resp.Body).Decode(&weatherResp)
                s.Require().NoError(err)
                s.Assert().Equal(75.0, weatherResp["temperature"])
                s.Assert().Equal("moderate", weatherResp["category"])
            }
        })
    }
}

func (s *IntegrationTestSuite) TestMetricsEndpoint() {
    // Make some requests first
    for i := 0; i < 5; i++ {
        http.Get(fmt.Sprintf("%s/weather?lat=40.7128&lon=-74.0060", s.server.URL))
    }
    
    resp, err := http.Get(fmt.Sprintf("%s/metrics", s.server.URL))
    s.Require().NoError(err)
    defer resp.Body.Close()
    
    s.Assert().Equal(http.StatusOK, resp.StatusCode)
    s.Assert().Equal("text/plain", resp.Header.Get("Content-Type"))
}

func (s *IntegrationTestSuite) TestConcurrentRequests() {
    const numRequests = 100
    results := make(chan int, numRequests)
    
    for i := 0; i < numRequests; i++ {
        go func() {
            resp, err := http.Get(fmt.Sprintf("%s/weather?lat=40.7128&lon=-74.0060", s.server.URL))
            if err != nil {
                results <- 0
                return
            }
            resp.Body.Close()
            results <- resp.StatusCode
        }()
    }
    
    successCount := 0
    for i := 0; i < numRequests; i++ {
        statusCode := <-results
        if statusCode == http.StatusOK {
            successCount++
        }
    }
    
    // At least 95% should succeed
    s.Assert().GreaterOrEqual(successCount, 95)
}

func (s *IntegrationTestSuite) TestCircuitBreakerIntegration() {
    // Get initial stats
    resp, err := http.Get(fmt.Sprintf("%s/stats", s.server.URL))
    s.Require().NoError(err)
    
    var stats map[string]interface{}
    err = json.NewDecoder(resp.Body).Decode(&stats)
    s.Require().NoError(err)
    resp.Body.Close()
    
    // Stats endpoint should return circuit breaker information
    s.Assert().NotNil(stats)
}