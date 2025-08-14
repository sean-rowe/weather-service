Feature: Database Operations
  As a service operator
  I want comprehensive database logging and analytics
  So that I can monitor usage, audit requests, and analyze patterns

  Background:
    Given the weather service is running
    And PostgreSQL database is available
    And database migrations have been applied

  # Audit Logging
  Scenario: Successful API request creates audit log
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then an audit log entry should be created with:
      | field          | validation                    |
      | correlation_id | should be a valid UUID        |
      | request_id     | should be a valid UUID        |
      | method         | should be "GET"               |
      | path           | should be "/api/v1/weather"   |
      | status_code    | should be 200                 |
      | duration_ms    | should be greater than 0      |
      | user_agent     | should contain client info    |
      | remote_addr    | should be a valid IP          |
      | error_message  | should be null                |

  Scenario: Failed API request includes error in audit log
    When I request weather for invalid latitude 91 and longitude 0
    Then an audit log entry should be created with:
      | field          | validation                          |
      | status_code    | should be 400                      |
      | error_message  | should contain "latitude must be"   |
      | duration_ms    | should be less than 100            |

  Scenario: Audit log captures request metadata
    Given I make a request with custom headers:
      | header         | value                    |
      | X-Request-ID   | custom-123              |
      | X-Correlation-ID | correlation-456        |
      | User-Agent     | CustomClient/1.0        |
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then the audit log metadata should contain:
      | key            | value                    |
      | request_id     | custom-123              |
      | correlation_id | correlation-456         |
      | user_agent     | CustomClient/1.0        |

  # Weather Request Logging
  Scenario: Successful weather request is logged
    When I request weather for latitude 40.7128 and longitude -74.0060
    And the external service returns temperature 72.5°F
    Then a weather_requests entry should be created with:
      | field            | value      |
      | latitude         | 40.7128    |
      | longitude        | -74.0060   |
      | temperature      | 72.5       |
      | temperature_unit | F          |
      | category         | moderate   |
      | cache_hit        | false      |

  Scenario: Cached weather request is logged with cache_hit
    Given weather for latitude 40.7128 and longitude -74.0060 is cached
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then a weather_requests entry should be created with:
      | field      | value    |
      | cache_hit  | true     |
      | response_time_ms | should be less than 10 |

  # Analytics Functions
  Scenario: Get request statistics for time period
    Given I have made the following requests in the last hour:
      | latitude | longitude | cache_hit | response_time_ms |
      | 40.7128  | -74.0060 | false     | 250             |
      | 40.7128  | -74.0060 | true      | 5               |
      | 37.7749  | -122.4194| false     | 300             |
      | 37.7749  | -122.4194| true      | 3               |
    When I query request statistics for the last hour
    Then the statistics should show:
      | metric           | value |
      | total_requests   | 4     |
      | avg_response_time| 139.5 |
      | min_response_time| 3     |
      | max_response_time| 300   |
      | cache_hit_rate   | 0.5   |

  Scenario: Get popular locations
    Given I have made requests for the following locations:
      | latitude | longitude | count |
      | 40.7128  | -74.0060 | 15    |
      | 37.7749  | -122.4194| 10    |
      | 41.8781  | -87.6298 | 5     |
    When I query popular locations for the last 24 hours
    Then the results should be ordered by request count descending
    And the top location should be 40.7128, -74.0060 with 15 requests

  Scenario: Get error summary
    Given the following errors occurred in the last hour:
      | error_type        | count |
      | invalid_latitude  | 5     |
      | invalid_longitude | 3     |
      | service_unavailable | 2   |
    When I query error summary for the last hour
    Then the summary should group errors by type
    And show counts for each error type

  # Database Migrations
  Scenario: Migrations run on service startup
    Given the database has no tables
    When the service starts
    Then the following tables should be created:
      | table_name       |
      | audit_logs       |
      | weather_requests |
    And the following stored procedures should exist:
      | procedure_name          |
      | sp_log_audit           |
      | sp_log_weather_request |
      | sp_cleanup_old_data    |
    And the following functions should exist:
      | function_name           |
      | fn_get_request_stats   |
      | fn_get_popular_locations |
      | fn_get_audit_logs      |
      | fn_get_error_summary   |

  Scenario: Migration version tracking
    When I check the migration version
    Then the current version should be 2
    And the migration should not be dirty
    And migration history should show all applied migrations

  Scenario: Rollback migration
    Given the current migration version is 2
    When I rollback the last migration
    Then the current version should be 1
    And stored procedures should be removed
    But tables should still exist

  # Data Retention
  Scenario: Old audit logs are cleaned up
    Given audit logs older than 30 days exist
    When the cleanup procedure runs
    Then audit logs older than 30 days should be deleted
    And recent audit logs should be retained

  Scenario: Old weather requests are cleaned up
    Given weather requests older than 7 days exist
    When the cleanup procedure runs
    Then weather requests older than 7 days should be deleted
    And recent weather requests should be retained

  # Connection Pooling
  Scenario: Database connection pool handles concurrent requests
    Given the connection pool is configured with:
      | setting            | value |
      | max_connections    | 25    |
      | max_idle          | 5     |
      | max_lifetime      | 1h    |
    When 50 concurrent requests are made
    Then all requests should complete successfully
    And no more than 25 database connections should be used
    And connection wait time should be logged for queued requests

  Scenario: Database connection failure handling
    Given the database becomes unavailable
    When I make a weather request
    Then the request should still succeed using cache or external service
    And a database error should be logged
    But the user should receive a valid response

  # Transaction Management
  Scenario: Audit log transaction rollback on failure
    Given a database constraint prevents audit log insertion
    When an API request is processed
    Then the audit log transaction should rollback
    And no partial data should be written
    But the API response should still be successful

  # Performance
  Scenario: Database queries use indexes efficiently
    Given 1 million audit logs exist in the database
    When I query logs for a specific correlation_id
    Then the query should complete in less than 100ms
    And the query plan should show index usage

  Scenario: Bulk insert optimization
    When 1000 weather requests are logged within 1 second
    Then batch insertion should be used
    And all records should be inserted successfully
    And the operation should complete in less than 500ms