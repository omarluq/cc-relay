package auth

import (
	"crypto/subtle"
	"net/http"

	"github.com/samber/mo"
)

// APIKeyAuthenticator validates x-api-key header authentication.
// Uses constant-time comparison to prevent timing attacks.
type APIKeyAuthenticator struct {
	// expectedKey is the configured API key.
	expectedKey string
}

// NewAPIKeyAuthenticator creates a new API key authenticator.
// The expected key is hashed at creation time for secure comparison.
//
// Security note: SHA-256 is appropriate for API key hashing because:
// - API keys are high-entropy secrets (32+ random characters), not passwords
// - SHA-256 provides sufficient pre-image resistance for high-entropy inputs
// - Passwords require slow hashes (bcrypt/argon2) due to limited entropy
// - Constant-time comparison prevents timing attacks (see Validate method).
func NewAPIKeyAuthenticator(expectedKey string) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		expectedKey: expectedKey,
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

	// CRITICAL: Constant-time comparison prevents timing attacks
	if subtle.ConstantTimeCompare([]byte(providedKey), []byte(a.expectedKey)) != 1 {
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

// ValidateResult validates the x-api-key header and returns mo.Result[Result].
// This is an alternative API that supports Railway-Oriented Programming patterns.
func (a *APIKeyAuthenticator) ValidateResult(r *http.Request) mo.Result[Result] {
	result := a.Validate(r)
	if result.Valid {
		return mo.Ok(result)
	}
	return mo.Err[Result](NewValidationError(result.Type, result.Error))
}
