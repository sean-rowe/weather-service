package nws

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "go.uber.org/zap"

    "github.com/yourname/weather-service/internal/core/domain"
    "github.com/yourname/weather-service/internal/core/ports"
)

type Client struct {
    baseURL    string
    httpClient *http.Client
    logger     *zap.Logger
}

func NewClient(baseURL string, httpClient *http.Client, logger *zap.Logger) *Client {
    return &Client{
        baseURL:    baseURL,
        httpClient: httpClient,
        logger:     logger,
    }
}

type pointsResponse struct {
    Properties struct {
        Forecast string `json:"forecast"`
    } `json:"properties"`
}

type forecastResponse struct {
    Properties struct {
        Periods []forecastPeriod `json:"periods"`
    } `json:"properties"`
}

type forecastPeriod struct {
    Name            string `json:"name"`
    Temperature     int    `json:"temperature"`
    TemperatureUnit string `json:"temperatureUnit"`
    ShortForecast   string `json:"shortForecast"`
}

func (c *Client) GetForecast(ctx context.Context, coords domain.Coordinates) (*ports.WeatherData, error) {
    forecastURL, err := c.getForecastURL(ctx, coords)
    if err != nil {
        return nil, fmt.Errorf("failed to get forecast URL: %w", err)
    }

    forecast, err := c.fetchForecast(ctx, forecastURL)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch forecast: %w", err)
    }

    if len(forecast.Properties.Periods) == 0 {
        return nil, fmt.Errorf("no forecast periods available")
    }

    todayPeriod := forecast.Properties.Periods[0]

    unit := domain.Fahrenheit
    if todayPeriod.TemperatureUnit == "C" {
        unit = domain.Celsius
    }

    return &ports.WeatherData{
        Temperature: float64(todayPeriod.Temperature),
        Unit:        unit,
        Forecast:    todayPeriod.ShortForecast,
    }, nil
}

func (c *Client) getForecastURL(ctx context.Context, coords domain.Coordinates) (string, error) {
    url := fmt.Sprintf("%s/points/%.4f,%.4f", c.baseURL, coords.Latitude, coords.Longitude)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", err
    }
    
    req.Header.Set("User-Agent", "WeatherService/1.0")
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("NWS API returned status %d", resp.StatusCode)
    }

    var points pointsResponse
    if err := json.NewDecoder(resp.Body).Decode(&points); err != nil {
        return "", err
    }

    if points.Properties.Forecast == "" {
        return "", fmt.Errorf("no forecast URL in response")
    }

    return points.Properties.Forecast, nil
}

func (c *Client) fetchForecast(ctx context.Context, forecastURL string) (*forecastResponse, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", forecastURL, nil)
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("User-Agent", "WeatherService/1.0")
    
    if _, hasDeadline := ctx.Deadline(); !hasDeadline {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
        defer cancel()
    }
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("NWS API returned status %d", resp.StatusCode)
    }

    var forecast forecastResponse
    if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
        return nil, err
    }

    return &forecast, nil
}