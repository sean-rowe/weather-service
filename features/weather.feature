Feature: Weather Service
  As a user of the weather service
  I want to get weather information for a location
  So that I can know the current weather conditions

  Scenario: Get weather for valid coordinates
    Given the weather service is running
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a successful response
    And the response should contain a forecast
    And the response should contain a temperature category

  Scenario: Get hot weather categorization
    Given the weather service is running
    And the temperature at coordinates is 90 degrees Fahrenheit
    When I request weather for latitude 25.7617 and longitude -80.1918
    Then the temperature category should be "hot"

  Scenario: Get cold weather categorization
    Given the weather service is running
    And the temperature at coordinates is 40 degrees Fahrenheit
    When I request weather for latitude 64.8378 and longitude -147.7164
    Then the temperature category should be "cold"

  Scenario: Get moderate weather categorization
    Given the weather service is running
    And the temperature at coordinates is 70 degrees Fahrenheit
    When I request weather for latitude 37.7749 and longitude -122.4194
    Then the temperature category should be "moderate"

  Scenario: Invalid latitude
    Given the weather service is running
    When I request weather for latitude 91 and longitude 0
    Then I should receive a bad request error
    And the error message should contain "latitude"

  Scenario: Invalid longitude
    Given the weather service is running
    When I request weather for latitude 0 and longitude 181
    Then I should receive a bad request error
    And the error message should contain "longitude"

  Scenario: Missing parameters
    Given the weather service is running
    When I request weather without coordinates
    Then I should receive a bad request error
    And the error message should contain "required"

  Scenario: External service failure
    Given the weather service is running
    And the external weather service is unavailable
    When I request weather for latitude 40.7128 and longitude -74.0060
    Then I should receive a service unavailable error