Feature: Weather Service Caching
  As a weather service operator
  I want to cache weather responses
  So that I can reduce load on external services and improve response times

  Background:
    Given the weather service is running
    And the cache is enabled
    And the cache TTL is set to 5 minutes

  Scenario: First request caches the response
    Given the cache is empty
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 200 status code
    And the response time should be recorded
    And the cache should contain an entry for coordinates "40.7128,-74.0060"
    And the database should log cache_hit as false

  Scenario: Subsequent request uses cached response
    Given weather data for latitude 40.7128 and longitude -74.0060 is cached
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 200 status code
    And the response should be identical to the cached data
    And the response time should be faster than the initial request
    And the database should log cache_hit as true
    And the external weather service should not be called

  Scenario: Cache expires after TTL
    Given weather data for latitude 40.7128 and longitude -74.0060 was cached 6 minutes ago
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 200 status code
    And the external weather service should be called
    And the cache should be updated with new data
    And the database should log cache_hit as false

  Scenario: Different coordinates have separate cache entries
    Given weather data for latitude 40.7128 and longitude -74.0060 is cached
    When I request weather for latitude 37.7749 and longitude -122.4194
    Then I should receive a 200 status code
    And the external weather service should be called
    And the cache should contain entries for both coordinate pairs
    And the database should log cache_hit as false

  Scenario: Cache key precision handles coordinate rounding
    Given weather data for latitude 40.7128 and longitude -74.0060 is cached
    When I request weather for latitude 40.71280 and longitude -74.00600
    Then I should receive a 200 status code
    And the cached response should be used
    And the database should log cache_hit as true

  Scenario: Failed requests are not cached
    Given the external weather service is unavailable
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 503 status code
    And the cache should not contain an entry for coordinates "40.7128,-74.0060"

  Scenario: Invalid requests bypass cache
    When I request weather for latitude 91 and longitude 0
    Then I should receive a 400 status code
    And the cache should not be accessed
    And the external weather service should not be called

  # Redis Cache Scenarios
  Scenario: Redis cache handles connection failure gracefully
    Given Redis is unavailable
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 200 status code
    And the system should fall back to memory cache
    And a warning should be logged about Redis unavailability

  Scenario: Redis cache reconnects after failure
    Given Redis becomes available after being down
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 200 status code
    And the system should use Redis cache
    And the cache entry should be stored in Redis

  # Memory Cache Scenarios
  Scenario: Memory cache respects size limits
    Given the memory cache is configured with a maximum of 100 entries
    When I request weather for 101 different coordinate pairs
    Then the oldest cache entry should be evicted
    And the cache should contain exactly 100 entries
    And all responses should be successful

  Scenario: Memory cache handles concurrent requests
    Given 10 concurrent requests are made for the same coordinates
    When all requests complete
    Then the external weather service should be called only once
    And all requests should receive the same response
    And the cache should contain one entry for the coordinates