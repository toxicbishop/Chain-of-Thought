// Package cache provides a Redis-backed caching layer for ReasoningTrace results.
//
// Cache keys use the format:   noetic:trace:<hex-sha256-of-query>
// Default TTL is 1 hour, configurable via REDIS_CACHE_TTL (seconds).
//
// The service degrades gracefully: when REDIS_URL is empty or Redis is
// unreachable, all cache operations become no-ops and the pipeline runs normally.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"cot-backend/internal/transformer"
)

const (
	keyPrefix  = "noetic:trace:"
	defaultTTL = time.Hour
)

// Service is the top-level cache object. Use NewService to construct one.
type Service struct {
	client  *redis.Client
	ttl     time.Duration
	enabled bool
}

// NewService creates a Service connected to the Redis instance at redisURL.
// redisURL should be a Redis URL, e.g. "redis://localhost:6379" or
// "redis://:password@host:6379/0".
//
// If redisURL is empty the service starts in disabled (no-op) mode.
func NewService(redisURL string) *Service {
	if redisURL == "" {
		log.Println("[cache] REDIS_URL not set — Redis cache disabled")
		return &Service{enabled: false}
	}

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("[cache] invalid REDIS_URL %q: %v — cache disabled", redisURL, err)
		return &Service{enabled: false}
	}

	client := redis.NewClient(opts)

	// Verify connectivity at startup.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[cache] Redis ping failed: %v — cache disabled", err)
		return &Service{enabled: false}
	}

	ttl := defaultTTL
	if s := os.Getenv("REDIS_CACHE_TTL"); s != "" {
		if secs, err := strconv.Atoi(s); err == nil && secs > 0 {
			ttl = time.Duration(secs) * time.Second
		}
	}

	log.Printf("[cache] Redis connected at %s (TTL=%s)", redisURL, ttl)
	return &Service{client: client, ttl: ttl, enabled: true}
}

// Enabled reports whether Redis is active.
func (s *Service) Enabled() bool { return s.enabled }

// GetTrace attempts to fetch a cached ReasoningTrace for the given query.
// Returns (trace, true) on a cache hit, or (zero, false) on a miss or error.
func (s *Service) GetTrace(ctx context.Context, query string) (transformer.ReasoningTrace, bool) {
	if !s.enabled {
		return transformer.ReasoningTrace{}, false
	}

	key := cacheKey(query)
	data, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return transformer.ReasoningTrace{}, false // cache miss
	}
	if err != nil {
		log.Printf("[cache] GET error key=%s: %v", key, err)
		return transformer.ReasoningTrace{}, false
	}

	var trace transformer.ReasoningTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		log.Printf("[cache] unmarshal error key=%s: %v", key, err)
		return transformer.ReasoningTrace{}, false
	}

	log.Printf("[cache] HIT query=%q key=%s", query, key)
	return trace, true
}

// SetTrace serialises trace and stores it in Redis under the query's cache key.
// Errors are logged but not returned — a failed write is non-fatal.
func (s *Service) SetTrace(ctx context.Context, query string, trace transformer.ReasoningTrace) {
	if !s.enabled {
		return
	}

	data, err := json.Marshal(trace)
	if err != nil {
		log.Printf("[cache] marshal error: %v", err)
		return
	}

	key := cacheKey(query)
	if err := s.client.Set(ctx, key, data, s.ttl).Err(); err != nil {
		log.Printf("[cache] SET error key=%s: %v", key, err)
		return
	}
	log.Printf("[cache] SET query=%q key=%s TTL=%s", query, key, s.ttl)
}

// Invalidate removes the cached trace for a specific query.
// Useful for forced cache-busting via an admin endpoint.
func (s *Service) Invalidate(ctx context.Context, query string) error {
	if !s.enabled {
		return nil
	}
	return s.client.Del(ctx, cacheKey(query)).Err()
}

// Close disconnects from Redis. Call during graceful shutdown.
func (s *Service) Close() error {
	if !s.enabled || s.client == nil {
		return nil
	}
	log.Println("[cache] closing Redis connection")
	return s.client.Close()
}

// cacheKey returns the Redis key for a given query string.
// Uses truncated SHA-256 to keep keys short and safe.
func cacheKey(query string) string {
	h := sha256.Sum256([]byte(query))
	return fmt.Sprintf("%s%x", keyPrefix, h)
}
