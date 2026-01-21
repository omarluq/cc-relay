package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

// BearerAuthenticator validates Authorization: Bearer token authentication.
// This is used by Claude Code subscription users.
type BearerAuthenticator struct {
	// secretHash is the pre-computed SHA-256 hash of the expected secret.
	// If empty (zero value), any bearer token is accepted.
	secretHash [32]byte
	// validateSecret indicates whether to validate the token against a secret.
	validateSecret bool
}

// NewBearerAuthenticator creates a new Bearer token authenticator.
// If secret is empty, any valid bearer token format is accepted.
// If secret is provided, tokens are validated against it using constant-time comparison.
func NewBearerAuthenticator(secret string) *BearerAuthenticator {
	auth := &BearerAuthenticator{}

	if secret != "" {
		auth.secretHash = sha256.Sum256([]byte(secret))
		auth.validateSecret = true
	}

	return auth
}

// Validate checks the Authorization header for a valid Bearer token.
// Returns a Result with Valid=true if authentication succeeds.
func (a *BearerAuthenticator) Validate(r *http.Request) Result {
	authHeader := r.Header.Get("Authorization")

	if authHeader == "" {
		return Result{
			Valid: false,
			Type:  TypeBearer,
			Error: "missing authorization header",
		}
	}

	// Check for "Bearer " prefix (case insensitive)
	if len(authHeader) < 7 || !strings.EqualFold(authHeader[:6], "bearer") {
		return Result{
			Valid: false,
			Type:  TypeBearer,
			Error: "invalid authorization scheme",
		}
	}

	// Extract token (everything after "Bearer ")
	token := strings.TrimSpace(authHeader[7:])

	if token == "" {
		return Result{
			Valid: false,
			Type:  TypeBearer,
			Error: "empty bearer token",
		}
	}

	// If secret validation is enabled, check the token
	if a.validateSecret {
		tokenHash := sha256.Sum256([]byte(token))

		// CRITICAL: Constant-time comparison prevents timing attacks
		if subtle.ConstantTimeCompare(tokenHash[:], a.secretHash[:]) != 1 {
			return Result{
				Valid: false,
				Type:  TypeBearer,
				Error: "invalid bearer token",
			}
		}
	}

	return Result{
		Valid: true,
		Type:  TypeBearer,
	}
}

// Type returns the authentication type (bearer).
func (a *BearerAuthenticator) Type() Type {
	return TypeBearer
}
