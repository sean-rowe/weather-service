package middleware

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/sean-rowe/weather-service/internal/core/ports"
)

// MemoryRateLimiter provides an in-memory rate-limiting implementation.
type MemoryRateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*clientInfo
	logger  *zap.Logger
}

// clientInfo tracks request timestamps for a single client.
type clientInfo struct {
	requests []time.Time
	mu       sync.Mutex
}

// NewMemoryRateLimiter creates a new in-memory rate limiter.
//
// Parameters:
//   - logger: Zap logger for rate limiter operations
//
// Returns:
//   - ports.RateLimitService: In-memory rate limiter implementation
func NewMemoryRateLimiter(logger *zap.Logger) ports.RateLimitService {
	rl := &MemoryRateLimiter{
		clients: make(map[string]*clientInfo),
		logger:  logger,
	}

	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given identifier is allowed under the rate limit.
//
// Parameters:
//   - ctx: Context for cancellation
//   - identifier: Client identifier (usually IP address)
//   - limit: Maximum requests allowed in window
//   - window: Time window for rate limiting
//
// Returns:
//   - bool: true if request is allowed, false if rate limit exceeded
//   - error: Always nil for in-memory implementation
func (rl *MemoryRateLimiter) Allow(ctx context.Context, identifier string, limit int, window time.Duration) (bool, error) {
	// Check if context is canceled
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	now := time.Now()

	rl.mu.RLock()

	client, exists := rl.clients[identifier]

	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()

		if client, exists = rl.clients[identifier]; !exists {
			client = &clientInfo{
				requests: make([]time.Time, 0, limit),
			}

			rl.clients[identifier] = client
		}

		rl.mu.Unlock()
	}

	client.mu.Lock()

	defer client.mu.Unlock()

	cutoff := now.Add(-window)
	validRequests := client.requests[:0]

	for _, req := range client.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}

	client.requests = validRequests

	if len(client.requests) >= limit {
		return false, nil
	}

	client.requests = append(client.requests, now)
	return true, nil
}

// Reset clears the rate limit history for a given identifier.
//
// Parameters:
//   - ctx: Context for cancellation
//   - identifier: Client identifier to reset
//
// Returns:
//   - error: Always nil for in-memory implementation
func (rl *MemoryRateLimiter) Reset(ctx context.Context, identifier string) error {
	// Check if context is canceled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if client, exists := rl.clients[identifier]; exists {
		client.mu.Lock()
		client.requests = client.requests[:0]
		client.mu.Unlock()
	}

	return nil
}

// Cleanup periodically removes idle clients from memory.
// Runs every 5 minutes to prevent memory leaks from inactive clients.
func (rl *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()

		for identifier, client := range rl.clients {
			client.mu.Lock()

			if len(client.requests) == 0 {
				delete(rl.clients, identifier)
			}

			client.mu.Unlock()
		}

		rl.mu.Unlock()
	}
}
