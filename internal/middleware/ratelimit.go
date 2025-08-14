// Package middleware provides HTTP middleware components for cross-cutting concerns.
// It includes rate limiting, observability, logging, and security middleware
// that can be applied to HTTP handlers.
package middleware

import (
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// RateLimitMiddleware provides HTTP rate-limiting functionality.
// It enforces request limits per client IP address to prevent abuse
// and protect the service from an excessive load.
type RateLimitMiddleware struct {
	rateLimiter ports.RateLimitService
	logger      *zap.Logger
	limit       int
	window      time.Duration
}

// NewRateLimitMiddleware creates new rate limiting middleware.
// It configures the middleware with the specified rate limit (requests per window)
// and uses the provided rate limiter service for enforcement.
func NewRateLimitMiddleware(rateLimiter ports.RateLimitService, limit int, window time.Duration, logger *zap.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		rateLimiter: rateLimiter,
		logger:      logger,
		limit:       limit,
		window:      window,
	}
}

// Middleware returns an HTTP handler that enforces rate limiting.
func (rl *RateLimitMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identifier := GetClientIP(r)
		allowed, err := rl.rateLimiter.Allow(r.Context(), identifier, rl.limit, rl.window)

		if err != nil {
			rl.logger.Error("rate limiter error",
				zap.String("client_ip", identifier),
				zap.Error(err),
			)

			next.ServeHTTP(w, r)
			return
		}

		if !allowed {
			rl.logger.Warn("rate limit exceeded",
				zap.String("client_ip", identifier),
				zap.Int("limit", rl.limit),
				zap.Duration("window", rl.window))

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
			w.Header().Set("X-RateLimit-Window", rl.window.String())
			w.WriteHeader(http.StatusTooManyRequests)

			if _, err := w.Write([]byte(`{"error":"RATE_LIMIT_EXCEEDED","message":"Too many requests"}`)); err != nil {
				rl.logger.Error("failed to write rate limit response", zap.Error(err))
			}

			return
		}

		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Window", rl.window.String())

		next.ServeHTTP(w, r)
	})
}
