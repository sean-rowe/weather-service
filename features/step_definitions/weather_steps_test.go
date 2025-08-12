package steps

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"

    "github.com/cucumber/godog"
    "github.com/gorilla/mux"
    "go.uber.org/zap"

    "github.com/yourname/weather-service/internal/adapters/primary/rest"
    "github.com/yourname/weather-service/internal/core/domain"
)

type testContext struct {
    server          *httptest.Server
    response        *http.Response
    responseBody    map[string]interface{}
    mockService     *mockWeatherService
    mockTemperature float64
    mockError       error
}

type mockWeatherService struct {
    temperature float64
    shouldFail  bool
    coords      domain.Coordinates
}

func (m *mockWeatherService) GetWeather(ctx context.Context, coords domain.Coordinates) (*domain.Weather, error) {
    m.coords = coords
    
    if m.shouldFail {
        return nil, &domain.WeatherError{
            Code:    "FORECAST_RETRIEVAL_ERROR",
            Message: "Service unavailable",
        }
    }

    if err := coords.Validate(); err != nil {
        return nil, &domain.WeatherError{
            Code:    "INVALID_COORDINATES",
            Message: err.Error(),
        }
    }

    var category domain.TemperatureCategory
    if m.temperature < 50 {
        category = domain.Cold
    } else if m.temperature > 85 {
        category = domain.Hot
    } else {
        category = domain.Moderate
    }

    return &domain.Weather{
        Coordinates: coords,
        Temperature: domain.Temperature{Value: m.temperature, Unit: domain.Fahrenheit},
        Forecast:    "Test Forecast",
        Category:    category,
    }, nil
}

func InitializeScenario(ctx *godog.ScenarioContext) {
    tc := &testContext{}

    ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
        tc.mockService = &mockWeatherService{temperature: 70}
        return context.WithValue(ctx, "testContext", tc), nil
    })

    ctx.Step(`^the weather service is running$`, tc.theWeatherServiceIsRunning)
    ctx.Step(`^I request weather for latitude ([\-\d.]+) and longitude ([\-\d.]+)$`, tc.iRequestWeatherForCoordinates)
    ctx.Step(`^I request weather without coordinates$`, tc.iRequestWeatherWithoutCoordinates)
    ctx.Step(`^I should receive a successful response$`, tc.iShouldReceiveSuccessfulResponse)
    ctx.Step(`^I should receive a bad request error$`, tc.iShouldReceiveBadRequestError)
    ctx.Step(`^I should receive a service unavailable error$`, tc.iShouldReceiveServiceUnavailableError)
    ctx.Step(`^the response should contain a forecast$`, tc.theResponseShouldContainForecast)
    ctx.Step(`^the response should contain a temperature category$`, tc.theResponseShouldContainCategory)
    ctx.Step(`^the temperature category should be "([^"]*)"$`, tc.theTemperatureCategoryShouldBe)
    ctx.Step(`^the error message should contain "([^"]*)"$`, tc.theErrorMessageShouldContain)
    ctx.Step(`^the temperature at coordinates is (\d+) degrees Fahrenheit$`, tc.theTemperatureIs)
    ctx.Step(`^the external weather service is unavailable$`, tc.theExternalServiceIsUnavailable)
}

func (tc *testContext) theWeatherServiceIsRunning() error {
    logger := zap.NewNop()
    handler := rest.NewWeatherHandler(tc.mockService, logger)
    
    router := mux.NewRouter()
    router.HandleFunc("/weather", handler.GetWeather).Methods("GET")
    
    tc.server = httptest.NewServer(router)
    return nil
}

func (tc *testContext) iRequestWeatherForCoordinates(lat, lon string) error {
    url := fmt.Sprintf("%s/weather?lat=%s&lon=%s", tc.server.URL, lat, lon)
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    
    tc.response = resp
    return json.NewDecoder(resp.Body).Decode(&tc.responseBody)
}

func (tc *testContext) iRequestWeatherWithoutCoordinates() error {
    url := fmt.Sprintf("%s/weather", tc.server.URL)
    resp, err := http.Get(url)
    if err != nil {
        return err
    }
    
    tc.response = resp
    return json.NewDecoder(resp.Body).Decode(&tc.responseBody)
}

func (tc *testContext) iShouldReceiveSuccessfulResponse() error {
    if tc.response.StatusCode != http.StatusOK {
        return fmt.Errorf("expected status 200, got %d", tc.response.StatusCode)
    }
    return nil
}

func (tc *testContext) iShouldReceiveBadRequestError() error {
    if tc.response.StatusCode != http.StatusBadRequest {
        return fmt.Errorf("expected status 400, got %d", tc.response.StatusCode)
    }
    return nil
}

func (tc *testContext) iShouldReceiveServiceUnavailableError() error {
    if tc.response.StatusCode != http.StatusServiceUnavailable {
        return fmt.Errorf("expected status 503, got %d", tc.response.StatusCode)
    }
    return nil
}

func (tc *testContext) theResponseShouldContainForecast() error {
    if _, ok := tc.responseBody["forecast"]; !ok {
        return fmt.Errorf("response does not contain forecast")
    }
    return nil
}

func (tc *testContext) theResponseShouldContainCategory() error {
    if _, ok := tc.responseBody["category"]; !ok {
        return fmt.Errorf("response does not contain category")
    }
    return nil
}

func (tc *testContext) theTemperatureCategoryShouldBe(expected string) error {
    category, ok := tc.responseBody["category"].(string)
    if !ok {
        return fmt.Errorf("category not found in response")
    }
    if category != expected {
        return fmt.Errorf("expected category %s, got %s", expected, category)
    }
    return nil
}

func (tc *testContext) theErrorMessageShouldContain(substring string) error {
    message, ok := tc.responseBody["message"].(string)
    if !ok {
        return fmt.Errorf("error message not found in response")
    }
    if !strings.Contains(strings.ToLower(message), strings.ToLower(substring)) {
        return fmt.Errorf("error message '%s' does not contain '%s'", message, substring)
    }
    return nil
}

func (tc *testContext) theTemperatureIs(temp int) error {
    tc.mockService.temperature = float64(temp)
    return nil
}

func (tc *testContext) theExternalServiceIsUnavailable() error {
    tc.mockService.shouldFail = true
    return nil
}