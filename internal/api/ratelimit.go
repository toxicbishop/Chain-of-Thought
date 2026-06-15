package api

import (
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/go-redis/redis_rate/v10"

	"cot-backend/internal/auth"
)

// perUserRateLimit returns middleware that enforces per-user rate limiting using Redis.
// It reads the user ID from the JWT claims set by auth.Middleware, so it
// must be applied AFTER auth.Middleware in the middleware chain.
// If rdb is nil (e.g. cache disabled), it bypasses rate limiting.
func perUserRateLimit(rdb *redis.Client) func(http.Handler) http.Handler {
	var limiter *redis_rate.Limiter
	if rdb != nil {
		limiter = redis_rate.NewLimiter(rdb)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				// No claims means auth middleware didn't run or failed.
				next.ServeHTTP(w, r)
				return
			}

			if limiter == nil {
				// Rate limiting is disabled if no Redis client
				next.ServeHTTP(w, r)
				return
			}

			// Allow 5 requests per second
			res, err := limiter.Allow(r.Context(), "rate_limit:"+claims.Subject, redis_rate.PerSecond(5))
			if err != nil {
				// Fail open on Redis errors
				next.ServeHTTP(w, r)
				return
			}

			if res.Allowed == 0 {
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
