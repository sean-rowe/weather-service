package middleware

import (
    "context"
    "net/http"
    "time"

    "github.com/google/uuid"
    "github.com/gorilla/mux"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/trace"
    "go.uber.org/zap"

    "github.com/yourname/weather-service/internal/observability"
)

type contextKey string

const (
    CorrelationIDKey contextKey = "correlation-id"
    RequestIDKey     contextKey = "request-id"
)

type ObservabilityMiddleware struct {
    telemetry *observability.Telemetry
    logger    *zap.Logger
}

func NewObservabilityMiddleware(telemetry *observability.Telemetry, logger *zap.Logger) *ObservabilityMiddleware {
    return &ObservabilityMiddleware{
        telemetry: telemetry,
        logger:    logger,
    }
}

func (m *ObservabilityMiddleware) TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract trace context from headers
        ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
        
        // Start span
        tracer := m.telemetry.Tracer
        ctx, span := tracer.Start(ctx, r.Method+" "+r.URL.Path,
            trace.WithAttributes(
                attribute.String("http.method", r.Method),
                attribute.String("http.url", r.URL.String()),
                attribute.String("http.scheme", r.URL.Scheme),
                attribute.String("http.host", r.Host),
                attribute.String("http.user_agent", r.UserAgent()),
                attribute.String("http.remote_addr", r.RemoteAddr),
            ),
        )
        defer span.End()

        // Add correlation ID
        correlationID := r.Header.Get("X-Correlation-ID")
        if correlationID == "" {
            correlationID = uuid.New().String()
        }
        ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
        
        // Add request ID
        requestID := uuid.New().String()
        ctx = context.WithValue(ctx, RequestIDKey, requestID)
        
        // Add IDs to span
        span.SetAttributes(
            attribute.String("correlation_id", correlationID),
            attribute.String("request_id", requestID),
        )
        
        // Add IDs to response headers
        w.Header().Set("X-Correlation-ID", correlationID)
        w.Header().Set("X-Request-ID", requestID)
        
        // Wrap response writer to capture status code
        wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // Continue with request
        next.ServeHTTP(wrapped, r.WithContext(ctx))
        
        // Set span status based on HTTP status code
        span.SetAttributes(attribute.Int("http.status_code", wrapped.statusCode))
        if wrapped.statusCode >= 400 {
            span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
        } else {
            span.SetStatus(codes.Ok, "")
        }
    })
}

func (m *ObservabilityMiddleware) MetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Wrap response writer to capture status code
        wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
        
        // Continue with request
        next.ServeHTTP(wrapped, r)
        
        // Record metrics
        duration := time.Since(start)
        route := mux.CurrentRoute(r)
        path := r.URL.Path
        if route != nil {
            if template, err := route.GetPathTemplate(); err == nil {
                path = template
            }
        }
        
        m.telemetry.RecordRequest(r.Context(), r.Method, path, wrapped.statusCode, duration)
    })
}

func (m *ObservabilityMiddleware) LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Get correlation and request IDs from context
        correlationID, _ := r.Context().Value(CorrelationIDKey).(string)
        requestID, _ := r.Context().Value(RequestIDKey).(string)
        
        // Create logger with context
        logger := m.logger.With(
            zap.String("correlation_id", correlationID),
            zap.String("request_id", requestID),
            zap.String("method", r.Method),
            zap.String("path", r.URL.Path),
            zap.String("remote_addr", r.RemoteAddr),
            zap.String("user_agent", r.UserAgent()),
        )
        
        // Log request
        logger.Info("request started")
        
        // Wrap response writer to capture status code and bytes written
        wrapped := &responseWriterWithSize{
            responseWriter: responseWriter{ResponseWriter: w, statusCode: http.StatusOK},
            bytesWritten:   0,
        }
        
        // Continue with request
        next.ServeHTTP(wrapped, r)
        
        // Log response
        duration := time.Since(start)
        logger.Info("request completed",
            zap.Int("status_code", wrapped.statusCode),
            zap.Int64("bytes_written", wrapped.bytesWritten),
            zap.Duration("duration", duration),
            zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
        )
    })
}

type responseWriter struct {
    http.ResponseWriter
    statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
    rw.statusCode = code
    rw.ResponseWriter.WriteHeader(code)
}

type responseWriterWithSize struct {
    responseWriter
    bytesWritten int64
}

func (rw *responseWriterWithSize) Write(b []byte) (int, error) {
    n, err := rw.ResponseWriter.Write(b)
    rw.bytesWritten += int64(n)
    return n, err
}

func GetCorrelationID(ctx context.Context) string {
    if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
        return id
    }
    return ""
}

func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value(RequestIDKey).(string); ok {
        return id
    }
    return ""
}