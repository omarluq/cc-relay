package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/omarluq/cc-relay/internal/auth"
)

// Reusable generator functions to avoid gocritic dupOption warnings.
var (
	genNonEmptyAlpha = gen.AlphaString().SuchThat(func(s string) bool { return s != "" })
	genMinLen5Alpha  = gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 5 })
	genMinLen6Alpha  = gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 6 }) // Different from 5
	genMinLen4Alpha  = gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 4 }) // Different from 5
	genAnyAlpha      = gen.AlphaString()
)

// Property-based tests for ChainAuthenticator

func TestChainAuthenticatorCoreProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Valid keys always authenticate
	properties.Property("valid keys authenticate", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true // Skip empty keys
			}

			chain := auth.NewChainAuthenticator(auth.NewAPIKeyAuthenticator(key))
			req := createRequestWithAPIKey(key)

			result := chain.Validate(req)
			return result.Valid
		},
		genNonEmptyAlpha,
	))

	// Property 2: Invalid keys always fail
	properties.Property("invalid keys fail", prop.ForAll(
		func(validKey, providedKey string) bool {
			// Skip if keys happen to match
			if validKey == providedKey || validKey == "" || providedKey == "" {
				return true
			}

			chain := auth.NewChainAuthenticator(auth.NewAPIKeyAuthenticator(validKey))
			req := createRequestWithAPIKey(providedKey)

			result := chain.Validate(req)
			return !result.Valid
		},
		genMinLen5Alpha,
		genMinLen6Alpha, // Use different length to avoid dupOption
	))

	// Property 3: Empty chain returns invalid
	properties.Property("empty chain returns invalid", prop.ForAll(
		func(_ bool) bool {
			chain := auth.NewChainAuthenticator()
			req := createRequestWithAPIKey("any-key")

			result := chain.Validate(req)
			return !result.Valid && result.Type == auth.TypeNone
		},
		gen.Bool(),
	))

	// Property 4: First valid authenticator wins
	properties.Property("first valid authenticator wins", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			// Chain with the same key in two authenticators
			auth1 := auth.NewAPIKeyAuthenticator(key)
			auth2 := auth.NewAPIKeyAuthenticator("different-key")

			chain := auth.NewChainAuthenticator(auth1, auth2)
			req := createRequestWithAPIKey(key)

			result := chain.Validate(req)
			return result.Valid && result.Type == auth.TypeAPIKey
		},
		genNonEmptyAlpha,
	))

	properties.TestingRun(t)
}

func TestChainAuthenticatorResultProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 5: ValidateResult returns Ok for valid authentication
	properties.Property("ValidateResult returns Ok for valid auth", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			chain := auth.NewChainAuthenticator(auth.NewAPIKeyAuthenticator(key))
			req := createRequestWithAPIKey(key)

			result := chain.ValidateResult(req)
			return result.IsOk()
		},
		genNonEmptyAlpha,
	))

	// Property 6: ValidateResult returns Err for invalid authentication
	properties.Property("ValidateResult returns Err for invalid auth", prop.ForAll(
		func(validKey, providedKey string) bool {
			if validKey == providedKey || validKey == "" || providedKey == "" {
				return true
			}

			chain := auth.NewChainAuthenticator(auth.NewAPIKeyAuthenticator(validKey))
			req := createRequestWithAPIKey(providedKey)

			result := chain.ValidateResult(req)
			return result.IsError()
		},
		genMinLen5Alpha,
		genMinLen4Alpha, // Use different length to avoid dupOption
	))

	// Property 7: Type is always auth.TypeNone for chain
	properties.Property("Type returns auth.TypeNone", prop.ForAll(
		func(_ bool) bool {
			chain := auth.NewChainAuthenticator()
			return chain.Type() == auth.TypeNone
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

func TestAPIKeyAuthenticatorProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Matching key always validates
	properties.Property("matching key validates", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			authenticator := auth.NewAPIKeyAuthenticator(key)
			req := createRequestWithAPIKey(key)

			result := authenticator.Validate(req)
			return result.Valid && result.Type == auth.TypeAPIKey
		},
		genNonEmptyAlpha,
	))

	// Property 2: Missing header fails
	properties.Property("missing header fails", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			authenticator := auth.NewAPIKeyAuthenticator(key)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

			result := authenticator.Validate(req)
			return !result.Valid && result.Error == "missing x-api-key header"
		},
		genNonEmptyAlpha,
	))

	// Property 3: ValidateResult is consistent with Validate
	properties.Property("ValidateResult consistent with Validate", prop.ForAll(
		func(key, provided string) bool {
			if key == "" {
				return true
			}

			authenticator := auth.NewAPIKeyAuthenticator(key)
			req := createRequestWithAPIKey(provided)

			validateResult := authenticator.Validate(req)
			resultMonad := authenticator.ValidateResult(req)

			// Results should be consistent
			if validateResult.Valid {
				return resultMonad.IsOk()
			}
			return resultMonad.IsError()
		},
		genMinLen5Alpha,
		genMinLen6Alpha, // Different to avoid dupOption
	))

	// Property 4: Type returns auth.TypeAPIKey
	properties.Property("Type returns auth.TypeAPIKey", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			authenticator := auth.NewAPIKeyAuthenticator(key)
			return authenticator.Type() == auth.TypeAPIKey
		},
		genNonEmptyAlpha,
	))

	properties.TestingRun(t)
}

func TestBearerAuthenticatorValidationProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: With secret - matching token validates
	properties.Property("matching bearer token validates with secret", prop.ForAll(
		func(secret string) bool {
			if secret == "" {
				return true
			}

			authenticator := auth.NewBearerAuthenticator(secret)
			req := createRequestWithBearerToken(secret)

			result := authenticator.Validate(req)
			return result.Valid && result.Type == auth.TypeBearer
		},
		genNonEmptyAlpha,
	))

	// Property 2: Without secret - any token validates
	properties.Property("any token validates without secret", prop.ForAll(
		func(token string) bool {
			if token == "" {
				return true
			}

			authenticator := auth.NewBearerAuthenticator("") // No secret = passthrough
			req := createRequestWithBearerToken(token)

			result := authenticator.Validate(req)
			return result.Valid && result.Type == auth.TypeBearer
		},
		genNonEmptyAlpha,
	))

	// Property 3: Missing Authorization header fails
	properties.Property("missing Authorization fails", prop.ForAll(
		func(secret string) bool {
			authenticator := auth.NewBearerAuthenticator(secret)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

			result := authenticator.Validate(req)
			return !result.Valid && result.Error == "missing authorization header"
		},
		genAnyAlpha,
	))

	// Property 4: Invalid scheme fails
	properties.Property("invalid scheme fails", prop.ForAll(
		func(secret string) bool {
			authenticator := auth.NewBearerAuthenticator(secret)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic auth instead of Bearer

			result := authenticator.Validate(req)
			return !result.Valid && result.Error == "invalid authorization scheme"
		},
		genAnyAlpha,
	))

	properties.TestingRun(t)
}

func TestBearerAuthenticatorResultProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 5: Empty token after "Bearer " fails
	properties.Property("empty token fails", prop.ForAll(
		func(secret string) bool {
			authenticator := auth.NewBearerAuthenticator(secret)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.Header.Set("Authorization", "Bearer ")

			result := authenticator.Validate(req)
			return !result.Valid && result.Error == "empty bearer token"
		},
		genAnyAlpha,
	))

	// Property 6: Type returns auth.TypeBearer
	properties.Property("Type returns auth.TypeBearer", prop.ForAll(
		func(secret string) bool {
			bearer := auth.NewBearerAuthenticator(secret)
			return bearer.Type() == auth.TypeBearer
		},
		genAnyAlpha,
	))

	// Property 7: ValidateResult is consistent with Validate
	properties.Property("ValidateResult consistent with Validate", prop.ForAll(
		func(secret, token string) bool {
			if token == "" {
				return true
			}

			bearer := auth.NewBearerAuthenticator(secret)
			req := createRequestWithBearerToken(token)

			validateResult := bearer.Validate(req)
			resultMonad := bearer.ValidateResult(req)

			if validateResult.Valid {
				return resultMonad.IsOk()
			}
			return resultMonad.IsError()
		},
		genAnyAlpha,
		genNonEmptyAlpha,
	))

	properties.TestingRun(t)
}

func TestValidationErrorProperties(t *testing.T) {
	t.Parallel()
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Error() returns the message
	properties.Property("Error returns message", prop.ForAll(
		func(message string) bool {
			err := auth.NewValidationError(auth.TypeAPIKey, message)
			return err.Error() == message
		},
		genAnyAlpha,
	))

	// Property: Type is preserved
	properties.Property("Type is preserved", prop.ForAll(
		func(typeIdx int) bool {
			types := []auth.Type{auth.TypeAPIKey, auth.TypeBearer, auth.TypeNone}
			authType := types[typeIdx%len(types)]

			err := auth.NewValidationError(authType, "test message")
			return err.Type == authType
		},
		gen.IntRange(0, 2),
	))

	properties.TestingRun(t)
}

// Helper functions for creating test requests

func createRequestWithAPIKey(key string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("x-api-key", key)
	return req
}

func createRequestWithBearerToken(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}
