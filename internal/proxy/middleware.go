// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// AuthMiddleware creates middleware that validates x-api-key header.
// Uses constant-time comparison to prevent timing attacks.
func AuthMiddleware(expectedAPIKey string) func(http.Handler) http.Handler {
	// Pre-hash expected key at creation time (not per-request)
	expectedHash := sha256.Sum256([]byte(expectedAPIKey))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedKey := r.Header.Get("x-api-key")

			if providedKey == "" {
				WriteError(w, http.StatusUnauthorized, "authentication_error", "missing x-api-key header")
				return
			}

			providedHash := sha256.Sum256([]byte(providedKey))

			// CRITICAL: Constant-time comparison prevents timing attacks
			if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) != 1 {
				WriteError(w, http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
