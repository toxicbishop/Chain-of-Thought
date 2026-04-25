package api

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"cot-backend/internal/auth"
)

const (
	// rateLimit is the sustained requests per second allowed per user.
	rateLimit = 5
	// rateBurst is the maximum burst of requests allowed per user.
	rateBurst = 10
	// limiterCleanupInterval is how often stale limiter entries are purged.
	limiterCleanupInterval = 5 * time.Minute
	// limiterMaxIdle is how long a limiter can be idle before being removed.
	limiterMaxIdle = 10 * time.Minute
)

type userLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*userLimiter
}

func newRateLimiterStore() *rateLimiterStore {
	s := &rateLimiterStore{
		limiters: make(map[string]*userLimiter),
	}
	go s.cleanup()
	return s
}

func (s *rateLimiterStore) get(userID string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	ul, ok := s.limiters[userID]
	if !ok {
		ul = &userLimiter{
			limiter: rate.NewLimiter(rateLimit, rateBurst),
		}
		s.limiters[userID] = ul
	}
	ul.lastSeen = time.Now()
	return ul.limiter
}

func (s *rateLimiterStore) cleanup() {
	for {
		time.Sleep(limiterCleanupInterval)
		s.mu.Lock()
		for id, ul := range s.limiters {
			if time.Since(ul.lastSeen) > limiterMaxIdle {
				delete(s.limiters, id)
			}
		}
		s.mu.Unlock()
	}
}

// perUserRateLimit returns middleware that enforces per-user rate limiting.
// It reads the user ID from the JWT claims set by auth.Middleware, so it
// must be applied AFTER auth.Middleware in the middleware chain.
func perUserRateLimit(store *rateLimiterStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				// No claims means auth middleware didn't run or failed.
				// Let the request through — auth middleware will reject it.
				next.ServeHTTP(w, r)
				return
			}

			limiter := store.get(claims.Subject)
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"rate limit exceeded — try again shortly"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
