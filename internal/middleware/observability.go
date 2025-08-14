// Package middleware provides HTTP middleware components for the weather service.
// It includes observability, rate limiting, and other cross-cutting concerns.
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

	"github.com/sean-rowe/weather-service/internal/observability"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// CorrelationIDKey is the context key for correlation ID.
	CorrelationIDKey contextKey = "correlation-id"

	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "request-id"
)

// ObservabilityMiddleware provides HTTP middleware for distributed tracing, metrics, and logging.
type ObservabilityMiddleware struct {
	telemetry *observability.Telemetry
	logger    *zap.Logger
}

// NewObservabilityMiddleware creates a new observability middleware instance.
func NewObservabilityMiddleware(telemetry *observability.Telemetry, logger *zap.Logger) *ObservabilityMiddleware {
	return &ObservabilityMiddleware{
		telemetry: telemetry,
		logger:    logger,
	}
}

// TracingMiddleware adds distributed tracing to HTTP requests using OpenTelemetry.
func (m *ObservabilityMiddleware) TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
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

		correlationID := r.Header.Get("X-Correlation-ID")

		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
		requestID := uuid.New().String()
		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		span.SetAttributes(
			attribute.String("correlation_id", correlationID),
			attribute.String("request_id", requestID),
		)

		w.Header().Set("X-Correlation-ID", correlationID)
		w.Header().Set("X-Request-ID", requestID)

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r.WithContext(ctx))
		span.SetAttributes(attribute.Int("http.status_code", wrapped.statusCode))

		if wrapped.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	})
}

// MetricsMiddleware records HTTP request metrics including latency and status codes.
func (m *ObservabilityMiddleware) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

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

// LoggingMiddleware adds structured logging to HTTP requests.
func (m *ObservabilityMiddleware) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		correlationID := GetCorrelationID(r.Context())
		requestID := GetRequestID(r.Context())

		logger := m.logger.With(
			zap.String("correlation_id", correlationID),
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)

		logger.Info("request started")

		wrapped := &responseWriterWithSize{
			responseWriter: responseWriter{ResponseWriter: w, statusCode: http.StatusOK},
			bytesWritten:   0,
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		logger.Info("request completed",
			zap.Int("status_code", wrapped.statusCode),
			zap.Int64("bytes_written", wrapped.bytesWritten),
			zap.Duration("duration", duration),
			zap.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before writing it.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// responseWriterWithSize extends responseWriter to also track bytes written.
type responseWriterWithSize struct {
	responseWriter
	bytesWritten int64
}

// Write writes data and tracks the number of bytes written.
func (rw *responseWriterWithSize) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)

	return n, err
}

// GetCorrelationID retrieves the correlation ID from the context.
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}

	return ""
}

// GetRequestID retrieves the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}

	return ""
}
