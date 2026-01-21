package auth

import "net/http"

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
	var lastResult Result

	for _, auth := range c.authenticators {
		result := auth.Validate(r)
		if result.Valid {
			return result
		}

		lastResult = result
	}

	// If no authenticators or all failed, return failure
	if len(c.authenticators) == 0 {
		return Result{
			Valid: false,
			Type:  TypeNone,
			Error: "no authentication configured",
		}
	}

	// Return the last result but with TypeNone if no auth method worked
	return Result{
		Valid: false,
		Type:  TypeNone,
		Error: lastResult.Error,
	}
}

// Type returns TypeNone since this is a meta-authenticator.
func (c *ChainAuthenticator) Type() Type {
	return TypeNone
}
