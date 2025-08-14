// Package cache provides caching implementations for the weather service.
// It includes both in-memory and Redis-based caching solutions.
package cache

import (
	"context"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// MemoryCache provides an in-memory cache implementation using go-cache.
type MemoryCache struct {
	cache  *gocache.Cache
	logger *zap.Logger
}

// NewMemoryCache creates a new in-memory cache with specified TTL and cleanup intervals.
//
// Parameters:
//   - defaultTTL: Default time-to-live for cached items
//   - cleanupInterval: How often to clean up expired items
//   - logger: Zap logger for cache operations
//
// Returns:
//   - ports.CacheService: In-memory cache implementation
func NewMemoryCache(defaultTTL, cleanupInterval time.Duration, logger *zap.Logger) ports.CacheService {
	return &MemoryCache{
		cache:  gocache.New(defaultTTL, cleanupInterval),
		logger: logger,
	}
}

// Get retrieves a value from the cache by key.
//
// Parameters:
//   - ctx: Context for tracing
//   - key: Cache key to look up
//
// Returns:
//   - []byte: Cached value if found
//   - error: ErrCacheMiss if key is not found
func (m *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	tracer := otel.Tracer("cache")
	_, span := tracer.Start(ctx, "MemoryCache.Get")

	defer span.End()

	span.SetAttributes(attribute.String("cache.key", key))

	if value, found := m.cache.Get(key); found {
		span.SetAttributes(attribute.Bool("cache.hit", true))
		m.logger.Debug("memory cache hit", zap.String("key", key))

		return value.([]byte), nil
	}

	span.SetAttributes(attribute.Bool("cache.hit", false))
	m.logger.Debug("memory cache miss", zap.String("key", key))

	return nil, ErrCacheMiss
}

// Set stores a value in the cache with the specified TTL.
//
// Parameters:
//   - ctx: Context for tracing
//   - key: Cache key to store under
//   - value: Data to cache
//   - ttl: Time-to-live for this cache entry
//
// Returns:
//   - error: Always nil for in-memory cache
func (m *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	tracer := otel.Tracer("cache")
	_, span := tracer.Start(ctx, "MemoryCache.Set")

	defer span.End()

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.Int("cache.value_size", len(value)),
		attribute.String("cache.ttl", ttl.String()),
	)

	m.cache.Set(key, value, ttl)
	m.logger.Debug("memory cache set", zap.String("key", key))

	return nil
}

// Delete removes a value from the cache by key.
//
// Parameters:
//   - ctx: Context for tracing
//   - key: Cache key to delete
//
// Returns:
//   - error: Always nil for in-memory cache
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	tracer := otel.Tracer("cache")
	_, span := tracer.Start(ctx, "MemoryCache.Delete")

	defer span.End()

	span.SetAttributes(attribute.String("cache.key", key))
	m.cache.Delete(key)
	m.logger.Debug("memory cache delete", zap.String("key", key))

	return nil
}

// Clear removes all values from the cache.
//
// Parameters:
//   - ctx: Context for tracing
//
// Returns:
//   - error: Always nil for in-memory cache
func (m *MemoryCache) Clear(ctx context.Context) error {
	tracer := otel.Tracer("cache")
	_, span := tracer.Start(ctx, "MemoryCache.Clear")

	defer span.End()

	m.cache.Flush()
	m.logger.Info("memory cache cleared")

	return nil
}
