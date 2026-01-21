package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
)

// APIKeyAuthenticator validates x-api-key header authentication.
// Uses constant-time comparison to prevent timing attacks.
type APIKeyAuthenticator struct {
	// expectedHash is the pre-computed SHA-256 hash of the expected key.
	expectedHash [32]byte
}

// NewAPIKeyAuthenticator creates a new API key authenticator.
// The expected key is hashed at creation time for secure comparison.
func NewAPIKeyAuthenticator(expectedKey string) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		expectedHash: sha256.Sum256([]byte(expectedKey)),
	}
}

// Validate checks the x-api-key header against the expected value.
// Uses constant-time comparison to prevent timing attacks.
func (a *APIKeyAuthenticator) Validate(r *http.Request) Result {
	providedKey := r.Header.Get("x-api-key")

	if providedKey == "" {
		return Result{
			Valid: false,
			Type:  TypeAPIKey,
			Error: "missing x-api-key header",
		}
	}

	providedHash := sha256.Sum256([]byte(providedKey))

	// CRITICAL: Constant-time comparison prevents timing attacks
	if subtle.ConstantTimeCompare(providedHash[:], a.expectedHash[:]) != 1 {
		return Result{
			Valid: false,
			Type:  TypeAPIKey,
			Error: "invalid x-api-key",
		}
	}

	return Result{
		Valid: true,
		Type:  TypeAPIKey,
		// Don't include the actual key in the result for security
	}
}

// Type returns the authentication type (api_key).
func (a *APIKeyAuthenticator) Type() Type {
	return TypeAPIKey
}
