Feature: Health Monitoring and Version Information
  As a service operator
  I want health check and version endpoints
  So that I can monitor service status and track deployments

  Background:
    Given the weather service is running

  # Health Check Endpoint
  Scenario: Health check returns OK when service is healthy
    When I request GET /health
    Then I should receive a 200 status code
    And the response body should be "OK"
    And the response time should be less than 100ms

  Scenario: Health check is not rate limited
    Given the rate limit is 10 requests per second
    When I make 20 health check requests in 1 second
    Then all requests should receive a 200 status code
    And no requests should be rate limited

  Scenario: Health check is available during high load
    Given 1000 concurrent weather API requests are being processed
    When I request GET /health
    Then I should receive a 200 status code within 1 second

  Scenario: Health check does not require authentication
    Given authentication is enabled for API endpoints
    When I request GET /health without credentials
    Then I should receive a 200 status code

  Scenario: Health check works when external services are down
    Given the external weather service is unavailable
    And the database is unavailable
    And Redis is unavailable
    When I request GET /health
    Then I should receive a 200 status code
    And the response should indicate the service itself is running

  # Version Endpoint
  Scenario: Version endpoint returns build information
    When I request GET /version
    Then I should receive a 200 status code
    And the response should be valid JSON
    And the response should contain:
      | field       | description                          |
      | version     | Semantic version (e.g., "1.0.0")   |
      | buildTime   | ISO 8601 timestamp                  |
      | gitCommit   | Git commit hash                     |
      | gitBranch   | Git branch name                     |
      | goVersion   | Go runtime version                  |
      | os          | Operating system                    |
      | arch        | System architecture                 |

  Scenario: Version endpoint with development build
    Given the service is built without ldflags
    When I request GET /version
    Then I should receive a 200 status code
    And the version should be "1.0.0"
    And buildTime should be "unknown"
    And gitCommit should be "unknown"
    And gitBranch should be "unknown"

  Scenario: Version endpoint with production build
    Given the service is built with ldflags containing version info
    When I request GET /version
    Then I should receive a 200 status code
    And the version should match the build flag value
    And buildTime should be a valid timestamp
    And gitCommit should be a valid git hash
    And gitBranch should not be "unknown"

  Scenario: Version endpoint is not rate limited
    Given the rate limit is 10 requests per second
    When I make 20 version requests in 1 second
    Then all requests should receive a 200 status code

  # Extended Health Check with Dependencies
  Scenario: Extended health check includes dependency status
    When I request GET /health?detailed=true
    Then I should receive a 200 status code
    And the response should include:
      | component        | possible_status           |
      | service          | healthy                   |
      | database         | healthy, degraded, down   |
      | redis            | healthy, degraded, down   |
      | external_weather | healthy, degraded, down   |

  Scenario: Health check reflects database status
    Given the database is unavailable
    When I request GET /health?detailed=true
    Then I should receive a 200 status code
    And the service status should be "healthy"
    And the database status should be "down"

  Scenario: Health check reflects Redis status
    Given Redis is unavailable
    When I request GET /health?detailed=true
    Then I should receive a 200 status code
    And the service status should be "healthy"
    And the redis status should be "down"
    And the service should indicate it's using fallback memory cache

  Scenario: Health check reflects circuit breaker status
    Given the circuit breaker for external weather service is open
    When I request GET /health?detailed=true
    Then I should receive a 200 status code
    And the external_weather status should be "degraded"
    And the response should include circuit breaker state

  # Readiness and Liveness Probes (Kubernetes)
  Scenario: Readiness probe during startup
    Given the service is starting up
    When Kubernetes requests GET /ready
    Then I should receive a 503 status code
    When all components are initialized
    And Kubernetes requests GET /ready again
    Then I should receive a 200 status code

  Scenario: Liveness probe during normal operation
    Given the service has been running for 1 hour
    When Kubernetes requests GET /alive
    Then I should receive a 200 status code
    And the response time should be less than 1 second

  Scenario: Liveness probe during deadlock
    Given a goroutine deadlock has occurred
    When Kubernetes requests GET /alive
    Then the request should timeout
    And Kubernetes should restart the pod

  # Metrics Endpoint
  Scenario: Metrics endpoint exposes Prometheus metrics
    When I request GET /metrics
    Then I should receive a 200 status code
    And the response should be in Prometheus text format
    And the response should include:
      | metric_family                    | type      |
      | weather_requests_total           | counter   |
      | weather_request_duration_seconds | histogram |
      | cache_hits_total                | counter   |
      | cache_misses_total              | counter   |
      | circuit_breaker_state           | gauge     |
      | rate_limit_exceeded_total       | counter   |

  Scenario: Metrics reflect actual service usage
    Given I have made 10 weather requests
    And 7 were cache hits
    When I request GET /metrics
    Then weather_requests_total should equal 10
    And cache_hits_total should equal 7
    And cache_misses_total should equal 3