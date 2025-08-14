// Package ratelimit Package rate limit provides distributed rate limiting using Redis.
// It implements sliding window rate limiting with atomic operations
// to ensure accurate counting across multiple service instances.
package ratelimit

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// RedisRateLimiter implements distributed rate limiting using Redis.
type RedisRateLimiter struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisRateLimiter creates a new Redis-based rate limiter.
//
// Parameters:
//   - client: Redis client for distributed state
//   - logger: Zap logger for rate limiting events
//
// Returns:
//   - ports.RateLimitService: Redis rate limiter implementation
func NewRedisRateLimiter(client *redis.Client, logger *zap.Logger) ports.RateLimitService {
	return &RedisRateLimiter{
		client: client,
		logger: logger,
	}
}

// Allow checks if a request is allowed under the rate limit.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - identifier: Client identifier (usually IP address)
//   - limit: Maximum requests allowed in window
//   - window: Time window for rate limiting
//
// Returns:
//   - bool: true if request is allowed, false if rate limit exceeded
//   - error: Redis error if operation fails
func (r *RedisRateLimiter) Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, error) {
	tracer := otel.Tracer("ratelimit")
	ctx, span := tracer.Start(ctx, "RateLimit.Allow")

	defer span.End()

	span.SetAttributes(
		attribute.String("ratelimit.identifier", identifier),
		attribute.Int("ratelimit.limit", limit),
		attribute.String("ratelimit.window", window.String()),
	)

	key := "ratelimit:" + identifier

	luaScript := `
        local key = KEYS[1]
        local limit = tonumber(ARGV[1])
        local window = tonumber(ARGV[2])
        local now = tonumber(ARGV[3])
        
        -- Remove expired entries
        redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
        
        -- Count current requests
        local current = redis.call('ZCARD', key)
        
        if current < limit then
            -- Add current request
            redis.call('ZADD', key, now, now)
            redis.call('EXPIRE', key, window)
            return 1
        else
            return 0
        end
    `

	now := time.Now().Unix()
	result, err := r.client.Eval(ctx, luaScript, []string{key}, limit, int(window.Seconds()), now).Result()

	if err != nil {
		span.RecordError(err)

		r.logger.Error("rate limit eval error",
			zap.String("identifier", identifier),
			zap.Error(err))

		return false, err
	}

	allowed := result.(int64) == 1
	span.SetAttributes(attribute.Bool("ratelimit.allowed", allowed))

	if !allowed {
		r.logger.Debug("rate limit exceeded",
			zap.String("identifier", identifier),
			zap.Int("limit", limit))
	}

	return allowed, nil
}

// Reset clears the rate limit history for an identifier.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - identifier: Client identifier to reset
//
// Returns:
//   - error: Redis deletion error if operation fails
func (r *RedisRateLimiter) Reset(ctx context.Context, identifier string) error {
	tracer := otel.Tracer("ratelimit")
	ctx, span := tracer.Start(ctx, "RateLimit.Reset")

	defer span.End()

	span.SetAttributes(attribute.String("ratelimit.identifier", identifier))

	key := "ratelimit:" + identifier
	err := r.client.Del(ctx, key).Err()

	if err != nil {
		span.RecordError(err)

		r.logger.Error("rate limit reset error",
			zap.String("identifier", identifier),
			zap.Error(err))

		return err
	}

	r.logger.Debug("rate limit reset", zap.String("identifier", identifier))
	return nil
}
