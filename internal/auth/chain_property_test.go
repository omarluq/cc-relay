package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
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

func TestChainAuthenticator_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Valid keys always authenticate
	properties.Property("valid keys authenticate", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true // Skip empty keys
			}

			chain := NewChainAuthenticator(NewAPIKeyAuthenticator(key))
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

			chain := NewChainAuthenticator(NewAPIKeyAuthenticator(validKey))
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
			chain := NewChainAuthenticator()
			req := createRequestWithAPIKey("any-key")

			result := chain.Validate(req)
			return !result.Valid && result.Type == TypeNone
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
			auth1 := NewAPIKeyAuthenticator(key)
			auth2 := NewAPIKeyAuthenticator("different-key")

			chain := NewChainAuthenticator(auth1, auth2)
			req := createRequestWithAPIKey(key)

			result := chain.Validate(req)
			return result.Valid && result.Type == TypeAPIKey
		},
		genNonEmptyAlpha,
	))

	// Property 5: ValidateResult returns Ok for valid authentication
	properties.Property("ValidateResult returns Ok for valid auth", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			chain := NewChainAuthenticator(NewAPIKeyAuthenticator(key))
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

			chain := NewChainAuthenticator(NewAPIKeyAuthenticator(validKey))
			req := createRequestWithAPIKey(providedKey)

			result := chain.ValidateResult(req)
			return result.IsError()
		},
		genMinLen5Alpha,
		genMinLen4Alpha, // Use different length to avoid dupOption
	))

	// Property 7: Type is always TypeNone for chain
	properties.Property("Type returns TypeNone", prop.ForAll(
		func(_ bool) bool {
			chain := NewChainAuthenticator()
			return chain.Type() == TypeNone
		},
		gen.Bool(),
	))

	properties.TestingRun(t)
}

func TestAPIKeyAuthenticator_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: Matching key always validates
	properties.Property("matching key validates", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			auth := NewAPIKeyAuthenticator(key)
			req := createRequestWithAPIKey(key)

			result := auth.Validate(req)
			return result.Valid && result.Type == TypeAPIKey
		},
		genNonEmptyAlpha,
	))

	// Property 2: Missing header fails
	properties.Property("missing header fails", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			auth := NewAPIKeyAuthenticator(key)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

			result := auth.Validate(req)
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

			auth := NewAPIKeyAuthenticator(key)
			req := createRequestWithAPIKey(provided)

			validateResult := auth.Validate(req)
			resultMonad := auth.ValidateResult(req)

			// Results should be consistent
			if validateResult.Valid {
				return resultMonad.IsOk()
			}
			return resultMonad.IsError()
		},
		genMinLen5Alpha,
		genMinLen6Alpha, // Different to avoid dupOption
	))

	// Property 4: Type returns TypeAPIKey
	properties.Property("Type returns TypeAPIKey", prop.ForAll(
		func(key string) bool {
			if key == "" {
				return true
			}

			auth := NewAPIKeyAuthenticator(key)
			return auth.Type() == TypeAPIKey
		},
		genNonEmptyAlpha,
	))

	properties.TestingRun(t)
}

func TestBearerAuthenticator_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property 1: With secret - matching token validates
	properties.Property("matching bearer token validates with secret", prop.ForAll(
		func(secret string) bool {
			if secret == "" {
				return true
			}

			auth := NewBearerAuthenticator(secret)
			req := createRequestWithBearerToken(secret)

			result := auth.Validate(req)
			return result.Valid && result.Type == TypeBearer
		},
		genNonEmptyAlpha,
	))

	// Property 2: Without secret - any token validates
	properties.Property("any token validates without secret", prop.ForAll(
		func(token string) bool {
			if token == "" {
				return true
			}

			auth := NewBearerAuthenticator("") // No secret = passthrough
			req := createRequestWithBearerToken(token)

			result := auth.Validate(req)
			return result.Valid && result.Type == TypeBearer
		},
		genNonEmptyAlpha,
	))

	// Property 3: Missing Authorization header fails
	properties.Property("missing Authorization fails", prop.ForAll(
		func(secret string) bool {
			auth := NewBearerAuthenticator(secret)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

			result := auth.Validate(req)
			return !result.Valid && result.Error == "missing authorization header"
		},
		genAnyAlpha,
	))

	// Property 4: Invalid scheme fails
	properties.Property("invalid scheme fails", prop.ForAll(
		func(secret string) bool {
			auth := NewBearerAuthenticator(secret)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic auth instead of Bearer

			result := auth.Validate(req)
			return !result.Valid && result.Error == "invalid authorization scheme"
		},
		genAnyAlpha,
	))

	// Property 5: Empty token after "Bearer " fails
	properties.Property("empty token fails", prop.ForAll(
		func(secret string) bool {
			auth := NewBearerAuthenticator(secret)
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.Header.Set("Authorization", "Bearer ")

			result := auth.Validate(req)
			return !result.Valid && result.Error == "empty bearer token"
		},
		genAnyAlpha,
	))

	// Property 6: Type returns TypeBearer
	properties.Property("Type returns TypeBearer", prop.ForAll(
		func(secret string) bool {
			auth := NewBearerAuthenticator(secret)
			return auth.Type() == TypeBearer
		},
		genAnyAlpha,
	))

	// Property 7: ValidateResult is consistent with Validate
	properties.Property("ValidateResult consistent with Validate", prop.ForAll(
		func(secret, token string) bool {
			if token == "" {
				return true
			}

			auth := NewBearerAuthenticator(secret)
			req := createRequestWithBearerToken(token)

			validateResult := auth.Validate(req)
			resultMonad := auth.ValidateResult(req)

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

func TestValidationError_Properties(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	// Property: Error() returns the message
	properties.Property("Error returns message", prop.ForAll(
		func(message string) bool {
			err := NewValidationError(TypeAPIKey, message)
			return err.Error() == message
		},
		genAnyAlpha,
	))

	// Property: Type is preserved
	properties.Property("Type is preserved", prop.ForAll(
		func(typeIdx int) bool {
			types := []Type{TypeAPIKey, TypeBearer, TypeNone}
			authType := types[typeIdx%len(types)]

			err := NewValidationError(authType, "test message")
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
