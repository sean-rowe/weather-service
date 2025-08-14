Feature: Observability and Telemetry
  As a service operator
  I want comprehensive observability features
  So that I can monitor, trace, and debug the service effectively

  Background:
    Given the weather service is running
    And OpenTelemetry is configured
    And the OTLP collector endpoint is available

  # Distributed Tracing
  Scenario: Request generates trace span
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then a trace should be created with:
      | span_name              | attributes                          |
      | HTTP GET /api/v1/weather | http.method, http.route, http.status_code |
      | WeatherService.GetWeather | weather.latitude, weather.longitude |
      | Cache.Get              | cache.key, cache.hit                |
      | NWSClient.GetWeather   | external.service, external.url      |
      | Database.LogAudit      | db.operation, db.statement          |

  Scenario: Trace context propagation
    Given I send a request with trace headers:
      | header                | value                              |
      | traceparent          | 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01 |
      | tracestate           | rojo=00f067aa0ba902b7              |
    When the request is processed
    Then the trace should be continued with the provided context
    And child spans should have the same trace ID

  Scenario: Error spans include exception details
    Given the external weather service is unavailable
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then the trace should contain an error span with:
      | attribute           | value                    |
      | error              | true                     |
      | exception.type     | connection error         |
      | exception.message  | contains error details   |
      | exception.stacktrace | contains stack trace   |

  # Metrics Collection
  Scenario: HTTP request metrics are collected
    When I make 10 weather requests
    Then the following metrics should be updated:
      | metric                                | type      | labels                           |
      | http_server_duration                  | histogram | method, route, status_code       |
      | http_server_active_requests          | gauge     | method, route                    |
      | http_server_request_size_bytes       | histogram | method, route                    |
      | http_server_response_size_bytes      | histogram | method, route                    |

  Scenario: Business metrics are collected
    When I make weather requests with various outcomes
    Then the following metrics should be recorded:
      | metric                         | type      | description                      |
      | weather_requests_total         | counter   | Total weather requests           |
      | weather_cache_hits_total       | counter   | Cache hit count                  |
      | weather_cache_misses_total     | counter   | Cache miss count                 |
      | weather_external_calls_total   | counter   | External API calls               |
      | weather_response_time_seconds  | histogram | Response time distribution       |

  Scenario: Circuit breaker metrics
    Given the circuit breaker transitions through states
    Then the following metrics should be updated:
      | metric                           | type    | values                    |
      | circuit_breaker_state           | gauge   | 0=closed, 1=half, 2=open  |
      | circuit_breaker_transitions_total | counter | by from_state, to_state  |

  Scenario: Database metrics
    When database operations are performed
    Then the following metrics should be collected:
      | metric                        | type      | labels              |
      | db_connection_pool_size      | gauge     | state (idle, used)  |
      | db_query_duration_seconds    | histogram | operation, table    |
      | db_errors_total              | counter   | operation, error    |

  # Logging Integration
  Scenario: Logs include trace context
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then log entries should include:
      | field            | description                    |
      | trace_id         | OpenTelemetry trace ID        |
      | span_id          | Current span ID               |
      | correlation_id   | Request correlation ID        |
      | severity         | Log level (INFO, WARN, ERROR) |

  Scenario: Structured logging with context
    When an error occurs during request processing
    Then the error log should include:
      | field           | example_value              |
      | timestamp       | 2024-01-15T10:30:00Z      |
      | level           | ERROR                      |
      | message         | Failed to fetch weather    |
      | trace_id        | 4bf92f3577b34da6a3ce929d0e0e4736 |
      | error           | connection timeout         |
      | latency_ms      | 5000                      |
      | service         | weather-service           |
      | version         | 1.0.0                     |

  # Performance Profiling
  Scenario: CPU profiling endpoint
    When I request GET /debug/pprof/profile?seconds=30
    Then I should receive a CPU profile
    And the profile should be in pprof format
    And it should cover 30 seconds of execution

  Scenario: Memory profiling endpoint
    When I request GET /debug/pprof/heap
    Then I should receive a heap profile
    And it should show memory allocations
    And goroutine information

  Scenario: Trace sampling
    Given the trace sampling rate is set to 0.1
    When I make 100 requests
    Then approximately 10 traces should be exported
    And all error traces should be included regardless of sampling

  # Custom Metrics and Attributes
  Scenario: Weather category distribution metric
    When I make requests resulting in different weather categories
    Then a metric should track category distribution:
      | category | count |
      | hot      | 25    |
      | moderate | 60    |
      | cold     | 15    |

  Scenario: Geographic distribution tracking
    When requests are made from different regions
    Then metrics should include geographic labels:
      | label          | example_value |
      | country        | US           |
      | state          | NY           |
      | city           | New York     |

  # Alerting Integration
  Scenario: High error rate triggers metric threshold
    When the error rate exceeds 5% over 5 minutes
    Then the error_rate metric should reflect this
    And it should be queryable by Prometheus
    And available for alerting rules

  Scenario: Slow response time tracking
    When p99 latency exceeds 1 second
    Then the latency histogram should capture this
    And percentile metrics should be available:
      | percentile | metric_name                    |
      | p50        | http_server_duration_p50       |
      | p95        | http_server_duration_p95       |
      | p99        | http_server_duration_p99       |

  # Service Mesh Integration
  Scenario: Envoy sidecar metrics
    Given the service runs with an Envoy sidecar
    When requests pass through the sidecar
    Then both application and Envoy metrics should be available
    And trace context should be preserved

  # Debugging Features
  Scenario: Debug mode increases log verbosity
    Given the service is started with DEBUG=true
    When I make a request
    Then debug level logs should be emitted including:
      | log_type                 |
      | cache key computation    |
      | circuit breaker decisions |
      | rate limit calculations  |
      | database query details   |

  Scenario: Request ID tracking
    When I make a request with X-Request-ID header
    Then this ID should appear in:
      | location           |
      | all log entries    |
      | trace attributes   |
      | error responses    |
      | audit logs         |