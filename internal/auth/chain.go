package auth

import (
	"net/http"

	"github.com/samber/lo"
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
