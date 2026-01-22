package auth

import (
	"net/http"

	"github.com/samber/lo"
	"github.com/samber/mo"
)

// ChainAuthenticator tries multiple authenticators in order.
// The first authenticator to succeed is used. If all fail,
// the last error is returned.
type ChainAuthenticator struct {
	authenticators []Authenticator
}

// NewChainAuthenticator creates a chain of authenticators.
// Authenticators are tried in order; first success wins.
func NewChainAuthenticator(authenticators ...Authenticator) *ChainAuthenticator {
	return &ChainAuthenticator{
		authenticators: authenticators,
	}
}

// Validate tries each authenticator in order until one succeeds.
// Returns the first successful result, or the last failure if all fail.
func (c *ChainAuthenticator) Validate(r *http.Request) Result {
	// Handle empty chain case
	if len(c.authenticators) == 0 {
		return Result{
			Valid: false,
			Type:  TypeNone,
			Error: "no authentication configured",
		}
	}

	// Use lo.Reduce to find first valid result OR track last error.
	// Once a valid result is found, it's passed through unchanged.
	// If no valid result, the last error result is returned.
	result := lo.Reduce(c.authenticators, func(acc Result, auth Authenticator, _ int) Result {
		// Short-circuit: if we already have a valid result, skip remaining authenticators
		if acc.Valid {
			return acc
		}
		return auth.Validate(r)
	}, Result{Valid: false, Type: TypeNone})

	// If still not valid, normalize the error response
	if !result.Valid {
		return Result{
			Valid: false,
			Type:  TypeNone,
			Error: result.Error,
		}
	}

	return result
}

// Type returns TypeNone since this is a meta-authenticator.
func (c *ChainAuthenticator) Type() Type {
	return TypeNone
}

// ValidateResult tries each authenticator and returns the result as mo.Result[Result].
// This is an alternative API that supports Railway-Oriented Programming patterns.
// Returns mo.Ok(Result) if any authenticator succeeds.
// Returns mo.Err with the last error if all fail.
func (c *ChainAuthenticator) ValidateResult(r *http.Request) mo.Result[Result] {
	result := c.Validate(r)
	if result.Valid {
		return mo.Ok(result)
	}
	return mo.Err[Result](NewValidationError(result.Type, result.Error))
}

// ValidationError wraps authentication failure details.
type ValidationError struct {
	Type    Type
	Message string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new ValidationError with the given type and message.
func NewValidationError(authType Type, message string) *ValidationError {
	return &ValidationError{
		Type:    authType,
		Message: message,
	}
}
