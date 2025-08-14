// Package cache provides caching implementations for weather data.
// It includes both Redis-based distributed caching and in-memory caching
// with OpenTelemetry instrumentation for observability.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// RedisCache implements distributed caching using Redis.
// It provides persistent, scalable caching across multiple service instances
// with OpenTelemetry tracing for cache operations.
type RedisCache struct {
	client *redis.Client
	logger *zap.Logger
}

// Config holds Redis connection and performance settings.
// These settings control connection pooling, timeouts, and reliability.
type Config struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// NewRedisCache creates a new Redis-based cache service.
//
// Parameters:
//   - cfg: Redis connection configuration
//   - logger: Zap logger for cache operations
//
// Returns:
//   - ports.CacheService: Redis cache implementation
//   - error: Connection error if Redis is unavailable
func NewRedisCache(cfg Config, logger *zap.Logger) (ports.CacheService, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{
		client: rdb,
		logger: logger,
	}, nil
}

// Get retrieves a value from Redis cache.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - key: Cache key to retrieve
//
// Returns:
//   - []byte: Cached value if found
//   - error: ErrCacheMiss if not found, or Redis error
func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	tracer := otel.Tracer("cache")
	ctx, span := tracer.Start(ctx, "Cache.Get")

	defer span.End()

	span.SetAttributes(attribute.String("cache.key", key))
	start := time.Now()
	result, err := r.client.Get(ctx, key).Bytes()
	duration := time.Since(start)

	if errors.Is(err, redis.Nil) {
		span.SetAttributes(attribute.Bool("cache.hit", false))

		r.logger.Debug("cache miss",
			zap.String("key", key),
			zap.Duration("duration", duration))

		return nil, ErrCacheMiss
	}

	if err != nil {
		span.RecordError(err)

		r.logger.Error("cache get error",
			zap.String("key", key),
			zap.Error(err))

		return nil, err
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))

	r.logger.Debug("cache hit",
		zap.String("key", key),
		zap.Duration("duration", duration))

	return result, nil
}

// Set stores a value in Redis cache with TTL.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - key: Cache key
//   - value: Data to cache
//   - ttl: Time-to-live for the cache entry
//
// Returns:
//   - error: Redis set error if operation fails
func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	tracer := otel.Tracer("cache")
	ctx, span := tracer.Start(ctx, "Cache.Set")

	defer span.End()

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.Int("cache.value_size", len(value)),
		attribute.String("cache.ttl", ttl.String()),
	)

	start := time.Now()
	err := r.client.Set(ctx, key, value, ttl).Err()
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)

		r.logger.Error("cache set error",
			zap.String("key", key),
			zap.Error(err))

		return err
	}

	r.logger.Debug("cache set",
		zap.String("key", key),
		zap.Duration("duration", duration))

	return nil
}

// Delete removes a value from Redis cache.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - key: Cache key to delete
//
// Returns:
//   - error: Redis deletion error if operation fails
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	tracer := otel.Tracer("cache")
	ctx, span := tracer.Start(ctx, "Cache.Delete")

	defer span.End()

	span.SetAttributes(attribute.String("cache.key", key))
	start := time.Now()
	err := r.client.Del(ctx, key).Err()
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)

		r.logger.Error("cache delete error",
			zap.String("key", key),
			zap.Error(err))

		return err
	}

	r.logger.Debug("cache delete",
		zap.String("key", key),
		zap.Duration("duration", duration))

	return nil
}

// Clear flushes all values from the Redis database.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//
// Returns:
//   - error: Redis flush error if operation fails
func (r *RedisCache) Clear(ctx context.Context) error {
	tracer := otel.Tracer("cache")
	ctx, span := tracer.Start(ctx, "Cache.Clear")

	defer span.End()

	start := time.Now()
	err := r.client.FlushDB(ctx).Err()
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		r.logger.Error("cache clear error", zap.Error(err))

		return err
	}

	r.logger.Info("cache cleared", zap.Duration("duration", duration))

	return nil
}

// Close closes the Redis client connection.
//
// Returns:
//   - error: Connection close error
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// ErrCacheMiss indicates a cache key was not found.
var ErrCacheMiss = redis.Nil
