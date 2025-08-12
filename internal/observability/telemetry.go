package observability

import (
    "context"
    "fmt"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/propagation"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
    "go.opentelemetry.io/otel/trace"
    "go.uber.org/zap"
)

type Telemetry struct {
    TracerProvider *sdktrace.TracerProvider
    MeterProvider  *sdkmetric.MeterProvider
    Tracer         trace.Tracer
    Meter          metric.Meter
    logger         *zap.Logger
    
    // Metrics
    RequestCounter   metric.Int64Counter
    RequestDuration  metric.Float64Histogram
    ErrorCounter     metric.Int64Counter
    DBQueryDuration  metric.Float64Histogram
    CacheHitCounter  metric.Int64Counter
    CacheMissCounter metric.Int64Counter
}

type Config struct {
    ServiceName    string
    ServiceVersion string
    Environment    string
    OTLPEndpoint   string
    SampleRate     float64
}

func InitTelemetry(ctx context.Context, cfg Config, logger *zap.Logger) (*Telemetry, error) {
    res, err := resource.Merge(
        resource.Default(),
        resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(cfg.ServiceName),
            semconv.ServiceVersion(cfg.ServiceVersion),
            attribute.String("environment", cfg.Environment),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create resource: %w", err)
    }

    // Initialize trace provider
    tracerProvider, err := initTracerProvider(ctx, cfg, res)
    if err != nil {
        return nil, fmt.Errorf("failed to init tracer provider: %w", err)
    }

    // Initialize meter provider
    meterProvider, err := initMeterProvider(res)
    if err != nil {
        return nil, fmt.Errorf("failed to init meter provider: %w", err)
    }

    // Set global providers
    otel.SetTracerProvider(tracerProvider)
    otel.SetMeterProvider(meterProvider)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    meter := meterProvider.Meter(cfg.ServiceName)
    
    // Initialize metrics
    requestCounter, err := meter.Int64Counter(
        "http_requests_total",
        metric.WithDescription("Total number of HTTP requests"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }

    requestDuration, err := meter.Float64Histogram(
        "http_request_duration_seconds",
        metric.WithDescription("HTTP request duration in seconds"),
        metric.WithUnit("s"),
    )
    if err != nil {
        return nil, err
    }

    errorCounter, err := meter.Int64Counter(
        "errors_total",
        metric.WithDescription("Total number of errors"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }

    dbQueryDuration, err := meter.Float64Histogram(
        "db_query_duration_seconds",
        metric.WithDescription("Database query duration in seconds"),
        metric.WithUnit("s"),
    )
    if err != nil {
        return nil, err
    }

    cacheHitCounter, err := meter.Int64Counter(
        "cache_hits_total",
        metric.WithDescription("Total number of cache hits"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }

    cacheMissCounter, err := meter.Int64Counter(
        "cache_misses_total",
        metric.WithDescription("Total number of cache misses"),
        metric.WithUnit("1"),
    )
    if err != nil {
        return nil, err
    }

    return &Telemetry{
        TracerProvider:   tracerProvider,
        MeterProvider:    meterProvider,
        Tracer:           tracerProvider.Tracer(cfg.ServiceName),
        Meter:            meter,
        logger:           logger,
        RequestCounter:   requestCounter,
        RequestDuration:  requestDuration,
        ErrorCounter:     errorCounter,
        DBQueryDuration:  dbQueryDuration,
        CacheHitCounter:  cacheHitCounter,
        CacheMissCounter: cacheMissCounter,
    }, nil
}

func initTracerProvider(ctx context.Context, cfg Config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
    exporter, err := otlptrace.New(
        ctx,
        otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
            otlptracegrpc.WithInsecure(),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create trace exporter: %w", err)
    }

    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(cfg.SampleRate)),
    )

    return tp, nil
}

func initMeterProvider(res *resource.Resource) (*sdkmetric.MeterProvider, error) {
    exporter, err := prometheus.New()
    if err != nil {
        return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
    }

    mp := sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(exporter),
        sdkmetric.WithResource(res),
    )

    return mp, nil
}

func (t *Telemetry) RecordRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
    attrs := []attribute.KeyValue{
        attribute.String("method", method),
        attribute.String("path", path),
        attribute.Int("status_code", statusCode),
    }

    t.RequestCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
    t.RequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

    if statusCode >= 400 {
        t.ErrorCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
    }
}

func (t *Telemetry) RecordDBQuery(ctx context.Context, operation string, duration time.Duration, err error) {
    attrs := []attribute.KeyValue{
        attribute.String("operation", operation),
        attribute.Bool("error", err != nil),
    }

    t.DBQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
    
    if err != nil {
        t.ErrorCounter.Add(ctx, 1, metric.WithAttributes(
            attribute.String("type", "database"),
            attribute.String("operation", operation),
        ))
    }
}

func (t *Telemetry) RecordCacheHit(ctx context.Context, key string) {
    t.CacheHitCounter.Add(ctx, 1, metric.WithAttributes(
        attribute.String("key", key),
    ))
}

func (t *Telemetry) RecordCacheMiss(ctx context.Context, key string) {
    t.CacheMissCounter.Add(ctx, 1, metric.WithAttributes(
        attribute.String("key", key),
    ))
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
    if err := t.TracerProvider.Shutdown(ctx); err != nil {
        return fmt.Errorf("failed to shutdown tracer provider: %w", err)
    }
    
    if err := t.MeterProvider.Shutdown(ctx); err != nil {
        return fmt.Errorf("failed to shutdown meter provider: %w", err)
    }
    
    return nil
}