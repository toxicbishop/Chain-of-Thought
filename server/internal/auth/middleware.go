package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is an unexported type to avoid key collisions in context values.
type contextKey string

// ClaimsKey is the context key under which validated JWT claims are stored.
const ClaimsKey contextKey = "jwt_claims"

// Middleware returns an http.Handler that enforces JWT Bearer authentication.
// On success it stores the validated *Claims in the request context.
// On failure it responds with 401 JSON and does not call next.
//
// Usage (with gorilla/mux subrouter):
//
//	protected := mx.PathPrefix("/api").Subrouter()
//	protected.Use(auth.Middleware)
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"missing or malformed Authorization header"}`))
			return
		}

		raw := strings.TrimPrefix(header, "Bearer ")
		claims, err := ValidateToken(raw)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid or expired token"}`))
			return
		}

		// Inject claims into context so handlers can read them.
		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromContext retrieves the Firebase JWT claims stored by Middleware.
// Returns (nil, false) if the request did not pass through Middleware.
func ClaimsFromContext(ctx context.Context) (*FirebaseClaims, bool) {
	c, ok := ctx.Value(ClaimsKey).(*FirebaseClaims)
	return c, ok
}
