// Package auth provides authentication mechanisms for cc-relay.
// It supports multiple authentication methods including API keys and
// OAuth Bearer tokens used by Claude Code subscriptions.
package auth

import "net/http"

// Type represents the authentication method used.
type Type string

const (
	// TypeAPIKey represents x-api-key header authentication.
	TypeAPIKey Type = "api_key"
	// TypeBearer represents Authorization: Bearer token authentication.
	TypeBearer Type = "bearer"
	// TypeNone represents no authentication or failed auth with no valid type.
	TypeNone Type = "none"
)

// Result contains the outcome of an authentication attempt.
type Result struct {
	// Type indicates which authentication method was used (or attempted).
	Type Type
	// Error contains the error message if authentication failed.
	Error string
	// Token contains the extracted token/key value (for logging, not validation).
	Token string
	// Valid indicates whether authentication succeeded.
	Valid bool
}

// Authenticator defines the interface for authentication mechanisms.
type Authenticator interface {
	// Validate checks the request for valid credentials.
	// Returns a Result with Valid=true if authentication succeeds.
	Validate(r *http.Request) Result

	// Type returns the authentication type this authenticator handles.
	Type() Type
}
