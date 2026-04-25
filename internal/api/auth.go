package api

import (
	"encoding/json"
	"net/http"

	"cot-backend/internal/auth"
)

// GET /auth/me
//
// Returns the authenticated Firebase user's identity from the JWT claims.
// Requires a valid Firebase ID token as Bearer token.
func (r *Router) me(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	claims, ok := auth.ClaimsFromContext(req.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"not authenticated"}`))
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"user_id":    claims.Subject,
		"email":      claims.Email,
		"issued_at":  claims.RegisteredClaims.IssuedAt,
		"expires_at": claims.RegisteredClaims.ExpiresAt,
		"aud":        claims.RegisteredClaims.Audience,
	})
}
