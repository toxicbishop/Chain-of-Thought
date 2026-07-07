// Package auth provides Firebase ID-token validation for the CoT backend.
package auth

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Google's public JWKS endpoint for Firebase/Google ID tokens.
const googleJWKSURL = "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com"

// FirebaseClaims is the payload embedded inside every Firebase ID token.
type FirebaseClaims struct {
	Email         string         `json:"email"`
	EmailVerified bool           `json:"email_verified"`
	Name          string         `json:"name"`
	Picture       string         `json:"picture"`
	Firebase      map[string]any `json:"firebase"`
	jwt.RegisteredClaims
}

var (
	jwksOnce sync.Once
	jwks     keyfunc.Keyfunc
)

// firebaseProjectID returns the configured Firebase project ID.
func firebaseProjectID() string {
	return os.Getenv("FIREBASE_PROJECT_ID")
}

// initJWKS fetches Google's public JWKS for Firebase token validation.
func initJWKS() {
	jwksOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		k, err := keyfunc.NewDefaultCtx(ctx, []string{googleJWKSURL})
		if err != nil {
			log.Printf("[auth] failed to initialize JWKS from %s: %v", googleJWKSURL, err)
			return
		}

		jwks = k
		log.Printf("[auth] Firebase RS256 validation enabled via Google JWKS")
	})
}

// ValidateToken parses and validates a raw Firebase ID token string.
func ValidateToken(raw string) (*FirebaseClaims, error) {
	initJWKS()

	if jwks == nil {
		return nil, errors.New("auth: JWKS not initialized — cannot validate tokens")
	}

	token, err := jwt.ParseWithClaims(raw, &FirebaseClaims{}, jwks.Keyfunc,
		jwt.WithIssuer("https://securetoken.google.com/"+firebaseProjectID()),
		jwt.WithAudience(firebaseProjectID()),
	)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*FirebaseClaims)
	if !ok || !token.Valid {
		return nil, errors.New("auth: invalid token claims")
	}

	return claims, nil
}
