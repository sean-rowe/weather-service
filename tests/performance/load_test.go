// +build performance

package performance

import (
    "fmt"
    "net/http"
    "os"
    "sync"
    "sync/atomic"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
)

type LoadTestConfig struct {
    BaseURL         string
    Duration        time.Duration
    RPS             int // Requests per second
    Concurrency     int
    WarmupDuration  time.Duration
}

type LoadTestResults struct {
    TotalRequests     int64
    SuccessfulRequests int64
    FailedRequests    int64
    TotalDuration     time.Duration
    MinLatency        time.Duration
    MaxLatency        time.Duration
    AvgLatency        time.Duration
    P50Latency        time.Duration
    P95Latency        time.Duration
    P99Latency        time.Duration
    ErrorRate         float64
    ActualRPS         float64
    StatusCodes       map[int]int64
}

type LoadTester struct {
    config    LoadTestConfig
    client    *http.Client
    results   *LoadTestResults
    latencies []time.Duration
    mu        sync.Mutex
    wg        sync.WaitGroup
}

func NewLoadTester(config LoadTestConfig) *LoadTester {
    return &LoadTester{
        config: config,
        client: &http.Client{
            Timeout: 10 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 100,
                IdleConnTimeout:     90 * time.Second,
            },
        },
        results: &LoadTestResults{
            StatusCodes: make(map[int]int64),
        },
        latencies: make([]time.Duration, 0),
    }
}

func (lt *LoadTester) Run() *LoadTestResults {
    fmt.Printf("Starting load test: %d RPS for %s with %d concurrent workers\n",
        lt.config.RPS, lt.config.Duration, lt.config.Concurrency)
    
    // Warmup phase
    if lt.config.WarmupDuration > 0 {
        fmt.Printf("Warming up for %s...\n", lt.config.WarmupDuration)
        lt.warmup()
    }
    
    // Reset results after warmup
    lt.results = &LoadTestResults{
        StatusCodes: make(map[int]int64),
    }
    lt.latencies = make([]time.Duration, 0)
    
    // Main test
    start := time.Now()
    stopChan := make(chan struct{})
    
    // Start workers
    for i := 0; i < lt.config.Concurrency; i++ {
        lt.wg.Add(1)
        go lt.worker(stopChan)
    }
    
    // Run for specified duration
    time.Sleep(lt.config.Duration)
    close(stopChan)
    
    // Wait for all workers to finish
    lt.wg.Wait()
    
    lt.results.TotalDuration = time.Since(start)
    lt.calculateStats()
    
    return lt.results
}

func (lt *LoadTester) warmup() {
    warmupStopChan := make(chan struct{})
    var warmupWg sync.WaitGroup
    
    for i := 0; i < lt.config.Concurrency/2; i++ {
        warmupWg.Add(1)
        go func() {
            defer warmupWg.Done()
            for {
                select {
                case <-warmupStopChan:
                    return
                default:
                    lt.makeRequest()
                    time.Sleep(time.Second / time.Duration(lt.config.RPS/lt.config.Concurrency))
                }
            }
        }()
    }
    
    time.Sleep(lt.config.WarmupDuration)
    close(warmupStopChan)
    warmupWg.Wait()
}

func (lt *LoadTester) worker(stopChan chan struct{}) {
    defer lt.wg.Done()
    
    ticker := time.NewTicker(time.Second * time.Duration(lt.config.Concurrency) / time.Duration(lt.config.RPS))
    defer ticker.Stop()
    
    for {
        select {
        case <-stopChan:
            return
        case <-ticker.C:
            lt.makeRequest()
        }
    }
}

func (lt *LoadTester) makeRequest() {
    url := fmt.Sprintf("%s/weather?lat=40.7128&lon=-74.0060", lt.config.BaseURL)
    
    start := time.Now()
    resp, err := lt.client.Get(url)
    latency := time.Since(start)
    
    atomic.AddInt64(&lt.results.TotalRequests, 1)
    
    lt.mu.Lock()
    lt.latencies = append(lt.latencies, latency)
    lt.mu.Unlock()
    
    if err != nil {
        atomic.AddInt64(&lt.results.FailedRequests, 1)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusOK {
        atomic.AddInt64(&lt.results.SuccessfulRequests, 1)
    } else {
        atomic.AddInt64(&lt.results.FailedRequests, 1)
    }
    
    lt.mu.Lock()
    lt.results.StatusCodes[resp.StatusCode]++
    lt.mu.Unlock()
}

func (lt *LoadTester) calculateStats() {
    if len(lt.latencies) == 0 {
        return
    }
    
    // Sort latencies for percentile calculation
    sortedLatencies := make([]time.Duration, len(lt.latencies))
    copy(sortedLatencies, lt.latencies)
    
    // Simple bubble sort for demo (use sort.Slice in production)
    for i := 0; i < len(sortedLatencies); i++ {
        for j := i + 1; j < len(sortedLatencies); j++ {
            if sortedLatencies[i] > sortedLatencies[j] {
                sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
            }
        }
    }
    
    // Calculate min, max, avg
    lt.results.MinLatency = sortedLatencies[0]
    lt.results.MaxLatency = sortedLatencies[len(sortedLatencies)-1]
    
    var sum time.Duration
    for _, l := range sortedLatencies {
        sum += l
    }
    lt.results.AvgLatency = sum / time.Duration(len(sortedLatencies))
    
    // Calculate percentiles
    lt.results.P50Latency = sortedLatencies[len(sortedLatencies)*50/100]
    lt.results.P95Latency = sortedLatencies[len(sortedLatencies)*95/100]
    lt.results.P99Latency = sortedLatencies[len(sortedLatencies)*99/100]
    
    // Calculate error rate and actual RPS
    lt.results.ErrorRate = float64(lt.results.FailedRequests) / float64(lt.results.TotalRequests)
    lt.results.ActualRPS = float64(lt.results.TotalRequests) / lt.results.TotalDuration.Seconds()
}

func TestLoadSmall(t *testing.T) {
    config := LoadTestConfig{
        BaseURL:        getTestURL(),
        Duration:       30 * time.Second,
        RPS:            100,
        Concurrency:    10,
        WarmupDuration: 5 * time.Second,
    }
    
    tester := NewLoadTester(config)
    results := tester.Run()
    
    // Print results
    printResults(results)
    
    // Assertions
    assert.Less(t, results.ErrorRate, 0.01, "Error rate should be less than 1%")
    assert.Less(t, results.P95Latency, 500*time.Millisecond, "P95 latency should be less than 500ms")
    assert.Greater(t, results.ActualRPS, float64(config.RPS)*0.9, "Should achieve at least 90% of target RPS")
}

func TestLoadMedium(t *testing.T) {
    config := LoadTestConfig{
        BaseURL:        getTestURL(),
        Duration:       60 * time.Second,
        RPS:            500,
        Concurrency:    50,
        WarmupDuration: 10 * time.Second,
    }
    
    tester := NewLoadTester(config)
    results := tester.Run()
    
    printResults(results)
    
    assert.Less(t, results.ErrorRate, 0.02, "Error rate should be less than 2%")
    assert.Less(t, results.P95Latency, 1*time.Second, "P95 latency should be less than 1s")
}

func TestLoadSpike(t *testing.T) {
    // Test spike handling
    config := LoadTestConfig{
        BaseURL:        getTestURL(),
        Duration:       20 * time.Second,
        RPS:            1000,
        Concurrency:    100,
        WarmupDuration: 5 * time.Second,
    }
    
    tester := NewLoadTester(config)
    results := tester.Run()
    
    printResults(results)
    
    // During spike, we allow higher error rate but system should not crash
    assert.Less(t, results.ErrorRate, 0.1, "Error rate should be less than 10% during spike")
}

func TestLoadSustained(t *testing.T) {
    // Test sustained load for longer duration
    config := LoadTestConfig{
        BaseURL:        getTestURL(),
        Duration:       5 * time.Minute,
        RPS:            200,
        Concurrency:    20,
        WarmupDuration: 30 * time.Second,
    }
    
    tester := NewLoadTester(config)
    results := tester.Run()
    
    printResults(results)
    
    assert.Less(t, results.ErrorRate, 0.01, "Error rate should be less than 1% for sustained load")
    assert.Less(t, results.P99Latency, 2*time.Second, "P99 latency should be less than 2s")
}

func BenchmarkWeatherEndpoint(b *testing.B) {
    client := &http.Client{
        Timeout: 10 * time.Second,
    }
    
    url := fmt.Sprintf("%s/weather?lat=40.7128&lon=-74.0060", getTestURL())
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            resp, err := client.Get(url)
            if err != nil {
                b.Error(err)
                continue
            }
            resp.Body.Close()
        }
    })
}

func printResults(results *LoadTestResults) {
    fmt.Printf("\n=== Load Test Results ===\n")
    fmt.Printf("Total Requests:      %d\n", results.TotalRequests)
    fmt.Printf("Successful:          %d (%.2f%%)\n", 
        results.SuccessfulRequests, 
        float64(results.SuccessfulRequests)/float64(results.TotalRequests)*100)
    fmt.Printf("Failed:              %d (%.2f%%)\n", 
        results.FailedRequests, 
        results.ErrorRate*100)
    fmt.Printf("Duration:            %s\n", results.TotalDuration)
    fmt.Printf("Actual RPS:          %.2f\n", results.ActualRPS)
    fmt.Printf("\n=== Latency Stats ===\n")
    fmt.Printf("Min:                 %s\n", results.MinLatency)
    fmt.Printf("Max:                 %s\n", results.MaxLatency)
    fmt.Printf("Avg:                 %s\n", results.AvgLatency)
    fmt.Printf("P50:                 %s\n", results.P50Latency)
    fmt.Printf("P95:                 %s\n", results.P95Latency)
    fmt.Printf("P99:                 %s\n", results.P99Latency)
    fmt.Printf("\n=== Status Codes ===\n")
    for code, count := range results.StatusCodes {
        fmt.Printf("%d:                  %d\n", code, count)
    }
    fmt.Printf("========================\n\n")
}

func getTestURL() string {
    url := os.Getenv("TEST_URL")
    if url == "" {
        url = "http://localhost:8080"
    }
    return url
}