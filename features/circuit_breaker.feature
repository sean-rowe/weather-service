Feature: Circuit Breaker
  As a service operator
  I want circuit breaker protection for external services
  So that the system can gracefully handle downstream failures

  Background:
    Given the weather service is running
    And circuit breaker is configured with:
      | setting              | value |
      | failure_threshold    | 5     |
      | success_threshold    | 3     |
      | timeout             | 30s   |
      | half_open_requests  | 3     |

  # Circuit Breaker States
  Scenario: Circuit breaker in closed state allows requests
    Given the circuit breaker is in closed state
    When I make 3 successful weather requests
    Then all requests should pass through to the external service
    And the circuit breaker should remain closed

  Scenario: Circuit breaker opens after consecutive failures
    Given the circuit breaker is in closed state
    And the external weather service is unavailable
    When I make 5 consecutive failed requests
    Then the circuit breaker should transition to open state
    And subsequent requests should fail immediately with 503
    And the error message should contain "circuit breaker is open"

  Scenario: Circuit breaker in open state rejects requests
    Given the circuit breaker is in open state
    When I make a weather request
    Then the request should fail immediately
    And the external service should not be called
    And I should receive a 503 status code
    And the response time should be less than 100ms

  Scenario: Circuit breaker transitions to half-open after timeout
    Given the circuit breaker is in open state
    And 30 seconds have passed since it opened
    When I make a weather request
    Then the circuit breaker should transition to half-open state
    And the request should be forwarded to the external service

  Scenario: Circuit breaker closes from half-open after successful requests
    Given the circuit breaker is in half-open state
    And the external weather service is available
    When I make 3 successful requests
    Then the circuit breaker should transition to closed state
    And normal request flow should resume

  Scenario: Circuit breaker reopens from half-open after failure
    Given the circuit breaker is in half-open state
    And the external weather service is unavailable
    When I make 1 failed request
    Then the circuit breaker should transition back to open state
    And the timeout period should reset

  # Failure Detection
  Scenario: HTTP 5xx errors trigger circuit breaker
    Given the circuit breaker is in closed state
    When the external service returns 5 consecutive 500 errors
    Then the circuit breaker should open
    And an error event should be logged

  Scenario: Timeouts trigger circuit breaker
    Given the circuit breaker is in closed state
    When 5 consecutive requests timeout
    Then the circuit breaker should open
    And timeout events should be logged

  Scenario: HTTP 4xx errors do not trigger circuit breaker
    Given the circuit breaker is in closed state
    When the external service returns 10 consecutive 404 errors
    Then the circuit breaker should remain closed
    And the errors should be passed to the client

  # Success Scenarios
  Scenario: Successful requests reset failure count
    Given the circuit breaker is in closed state
    And there have been 3 consecutive failures
    When a request succeeds
    Then the failure count should reset to 0
    And the circuit breaker should remain closed

  Scenario: Mixed success and failure below threshold
    Given the circuit breaker is in closed state
    When I make requests with pattern: fail, succeed, fail, succeed, fail
    Then the circuit breaker should remain closed
    And all requests should be forwarded to the external service

  # Multiple Circuit Breakers
  Scenario: Different endpoints have independent circuit breakers
    Given circuit breakers for "weather-api" and "geocoding-api"
    When "weather-api" circuit breaker opens due to failures
    Then "geocoding-api" circuit breaker should remain closed
    And requests to geocoding should continue normally

  # Metrics and Monitoring
  Scenario: Circuit breaker state changes are logged
    Given the circuit breaker is in closed state
    When it transitions through closed -> open -> half-open -> closed
    Then each state transition should be logged with:
      | field           | description                    |
      | timestamp       | Time of transition            |
      | previous_state  | State before transition       |
      | new_state       | State after transition        |
      | reason          | Cause of transition           |

  Scenario: Circuit breaker metrics are exposed
    When I query circuit breaker metrics
    Then I should see:
      | metric                      | description                        |
      | circuit_breaker_state       | Current state (0=closed, 1=half, 2=open) |
      | circuit_breaker_requests_total | Total requests through breaker  |
      | circuit_breaker_failures_total | Total failed requests           |
      | circuit_breaker_successes_total | Total successful requests      |
      | circuit_breaker_rejections_total | Requests rejected when open   |

  # Recovery Scenarios
  Scenario: Gradual recovery in half-open state
    Given the circuit breaker is in half-open state
    And it allows 3 concurrent requests
    When 5 requests are made simultaneously
    Then only 3 requests should be forwarded to the external service
    And 2 requests should be immediately rejected
    And if the 3 forwarded requests succeed, the circuit should close

  Scenario: Circuit breaker handles intermittent failures
    Given the external service has 30% failure rate
    When I make 100 requests over 1 minute
    Then the circuit breaker should remain closed
    And failed requests should be retried according to retry policy