Feature: Rate Limiting
  As a service operator
  I want to implement rate limiting
  So that I can prevent abuse and ensure fair usage of the API

  Background:
    Given the weather service is running
    And rate limiting is enabled
    And the rate limit is set to 10 requests per second per IP

  Scenario: Requests within rate limit are allowed
    Given a client with IP address "192.168.1.100"
    When the client makes 5 requests in 1 second
    Then all 5 requests should receive a 200 status code
    And no rate limit headers should indicate exhaustion

  Scenario: Requests exceeding rate limit are rejected
    Given a client with IP address "192.168.1.100"
    When the client makes 15 requests in 1 second
    Then the first 10 requests should receive a 200 status code
    And the remaining 5 requests should receive a 429 status code
    And the error message should contain "Too many requests"

  Scenario: Rate limit headers are included in responses
    Given a client with IP address "192.168.1.100"
    When the client makes a request
    Then the response should include the following headers:
      | header                  | description                           |
      | X-RateLimit-Limit      | The rate limit ceiling for the client |
      | X-RateLimit-Remaining  | Number of requests left in window     |
      | X-RateLimit-Reset      | Time when the rate limit resets       |

  Scenario: Rate limit resets after time window
    Given a client with IP address "192.168.1.100"
    And the client has exhausted their rate limit
    When 1 second passes
    Then the client can make new requests successfully
    And the rate limit counter should be reset

  Scenario: Different clients have independent rate limits
    Given client A with IP address "192.168.1.100"
    And client B with IP address "192.168.1.101"
    When client A makes 10 requests in 1 second
    And client B makes 10 requests in 1 second
    Then all requests from both clients should receive a 200 status code

  Scenario: Rate limit applies to all API endpoints
    Given a client with IP address "192.168.1.100"
    When the client makes 10 requests to different API endpoints
    Then the rate limit should apply across all endpoints
    And the 11th request should receive a 429 status code

  Scenario: Rate limit with X-Forwarded-For header
    Given a request with X-Forwarded-For header "203.0.113.0, 198.51.100.0"
    When the request is processed
    Then the rate limit should be applied to IP "203.0.113.0"
    And not to the proxy IP "198.51.100.0"

  Scenario: Rate limit with X-Real-IP header
    Given a request with X-Real-IP header "203.0.113.0"
    When the request is processed
    Then the rate limit should be applied to IP "203.0.113.0"

  # Redis Rate Limiter Scenarios
  Scenario: Redis rate limiter handles connection failure
    Given Redis is unavailable
    When a client makes requests
    Then the system should fall back to memory-based rate limiting
    And a warning should be logged about Redis unavailability
    And rate limiting should still be enforced

  Scenario: Redis rate limiter shares state across instances
    Given multiple service instances are running
    And they share the same Redis instance
    When a client makes requests distributed across instances
    Then the rate limit should be enforced globally
    And the total allowed requests should not exceed the limit

  # Memory Rate Limiter Scenarios
  Scenario: Memory rate limiter cleanup
    Given the memory rate limiter is active
    And 1000 different IP addresses have made requests
    When 1 hour passes without requests from 500 of those IPs
    Then the rate limiter should clean up stale entries
    And memory usage should decrease

  Scenario: Rate limit burst handling
    Given a client with IP address "192.168.1.100"
    And burst size is set to 20
    When the client makes 20 requests instantly
    Then the first 20 requests should be allowed
    And the 21st request should be rate limited

  # Error Scenarios
  Scenario: Rate limited response format
    Given a client has exceeded the rate limit
    When the client makes a request
    Then the response should have:
      | field        | value                    |
      | status_code  | 429                      |
      | content_type | application/json         |
      | body.error   | RATE_LIMIT_EXCEEDED      |
      | body.message | Too many requests        |

  Scenario: Rate limit does not apply to health check
    Given a client with IP address "192.168.1.100"
    And the client has exhausted their rate limit for API endpoints
    When the client requests /health
    Then the request should receive a 200 status code
    And the health check should not be rate limited