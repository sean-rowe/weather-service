Feature: Weather API
  As a user of the weather service
  I want to retrieve weather information for specific coordinates
  So that I can make informed decisions based on current weather conditions

  Background:
    Given the weather service is running
    And the database is available
    And the external weather service is operational

  # Happy Path Scenarios
  Scenario: Successfully retrieve weather for valid US coordinates
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 200 status code
    And the response should contain:
      | field              | type    |
      | temperature        | number  |
      | temperatureUnit    | string  |
      | forecast           | string  |
      | category           | string  |
      | latitude           | number  |
      | longitude          | number  |
    And the temperature unit should be "F"
    And the category should be one of "hot", "cold", or "moderate"

  Scenario Outline: Weather categorization based on temperature
    Given the external weather service returns <temperature> degrees Fahrenheit
    When I request weather for latitude <lat> and longitude <lon>
    Then I should receive a 200 status code
    And the temperature category should be "<category>"

    Examples:
      | temperature | lat     | lon       | category |
      | 95          | 25.7617 | -80.1918  | hot      |
      | 85          | 33.4484 | -112.0740 | hot      |
      | 75          | 37.7749 | -122.4194 | moderate |
      | 65          | 47.6062 | -122.3321 | moderate |
      | 35          | 64.8378 | -147.7164 | cold     |
      | 25          | 44.9778 | -93.2650  | cold     |

  # Edge Cases - Boundary Coordinates
  Scenario Outline: Request weather for boundary coordinates
    When I request weather for latitude <lat> and longitude <lon>
    Then I should receive a <status> status code
    And the response <assertion>

    Examples:
      | lat  | lon   | status | assertion                        |
      | 90   | 0     | 200    | should contain weather data     |
      | -90  | 0     | 200    | should contain weather data     |
      | 0    | 180   | 200    | should contain weather data     |
      | 0    | -180  | 200    | should contain weather data     |
      | 91   | 0     | 400    | should contain "latitude"       |
      | -91  | 0     | 400    | should contain "latitude"       |
      | 0    | 181   | 400    | should contain "longitude"      |
      | 0    | -181  | 400    | should contain "longitude"      |

  # Error Scenarios - Invalid Inputs
  Scenario: Request weather with missing latitude parameter
    When I request weather with only longitude -74.0060
    Then I should receive a 400 status code
    And the error message should contain "latitude is required"

  Scenario: Request weather with missing longitude parameter
    When I request weather with only latitude 40.7128
    Then I should receive a 400 status code
    And the error message should contain "longitude is required"

  Scenario: Request weather with no parameters
    When I request weather without any parameters
    Then I should receive a 400 status code
    And the error message should contain "latitude is required"
    And the error message should contain "longitude is required"

  Scenario Outline: Request weather with invalid parameter types
    When I request weather with latitude "<lat>" and longitude "<lon>"
    Then I should receive a 400 status code
    And the error message should contain "invalid"

    Examples:
      | lat     | lon      |
      | abc     | -74.0060 |
      | 40.7128 | xyz      |
      | true    | false    |
      | null    | null     |
      | ""      | ""       |

  # External Service Failure Scenarios
  Scenario: External weather service returns 404
    Given the external weather service returns 404 for the requested coordinates
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 404 status code
    And the error message should contain "weather data not found"

  Scenario: External weather service times out
    Given the external weather service response time exceeds the timeout threshold
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 503 status code
    And the error message should contain "service unavailable"

  Scenario: External weather service returns invalid data
    Given the external weather service returns malformed JSON
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 502 status code
    And the error message should contain "invalid response from weather service"

  # Database Logging
  Scenario: Successful weather request is logged to database
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a 200 status code
    And a weather request should be logged in the database with:
      | field          | value    |
      | latitude       | 40.7128  |
      | longitude      | -74.0060 |
      | cache_hit      | false    |
    And an audit log should be created with status code 200

  Scenario: Failed weather request is logged to database
    When I request weather for latitude 91 and longitude 0
    Then I should receive a 400 status code
    And an audit log should be created with status code 400
    And the audit log should contain the error message