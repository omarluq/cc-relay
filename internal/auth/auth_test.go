// Package auth provides authentication for cc-relay.
package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarluq/cc-relay/internal/auth"
)

// assertAuthResult is a test helper that validates an auth.Result.
func assertAuthResult(t *testing.T, result auth.Result, wantValid bool, wantType auth.Type, wantErrMsg string) {
	t.Helper()

	if result.Valid != wantValid {
		t.Errorf("Valid = %v, want %v", result.Valid, wantValid)
	}

	if result.Type != wantType {
		t.Errorf("Type = %q, want %q", result.Type, wantType)
	}

	if wantErrMsg != "" && result.Error != wantErrMsg {
		t.Errorf("Error = %q, want %q", result.Error, wantErrMsg)
	}
}

// TestAuthTypes verifies auth type constants are defined.
func TestAuthTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		authType auth.Type
		want     string
	}{
		{"api_key type", auth.TypeAPIKey, "api_key"},
		{"bearer type", auth.TypeBearer, "bearer"},
		{"none type", auth.TypeNone, "none"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if string(testCase.authType) != testCase.want {
				t.Errorf("auth type = %q, want %q", testCase.authType, testCase.want)
			}
		})
	}
}

// TestAPIKeyAuthenticator_Validate tests API key authentication.
func TestAPIKeyAuthenticatorValidate(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewAPIKeyAuthenticator("test-api-key-12345")

	tests := []struct {
		apiKey     string
		name       string
		wantErrMsg string
		wantType   auth.Type
		wantValid  bool
	}{
		{
			name:       "valid api key",
			apiKey:     "test-api-key-12345",
			wantValid:  true,
			wantType:   auth.TypeAPIKey,
			wantErrMsg: "",
		},
		{
			name:       "invalid api key",
			apiKey:     "wrong-key",
			wantValid:  false,
			wantType:   auth.TypeAPIKey,
			wantErrMsg: "invalid x-api-key",
		},
		{
			name:       "empty api key",
			apiKey:     "",
			wantValid:  false,
			wantType:   auth.TypeAPIKey,
			wantErrMsg: "missing x-api-key header",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/messages", http.NoBody)
			if testCase.apiKey != "" {
				req.Header.Set("x-api-key", testCase.apiKey)
			}

			result := authenticator.Validate(req)
			assertAuthResult(t, result, testCase.wantValid, testCase.wantType, testCase.wantErrMsg)
		})
	}
}

// TestAPIKeyAuthenticator_Type verifies the type method.
func TestAPIKeyAuthenticatorType(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewAPIKeyAuthenticator("test-key")

	if authenticator.Type() != auth.TypeAPIKey {
		t.Errorf("Type() = %q, want %q", authenticator.Type(), auth.TypeAPIKey)
	}
}

// TestBearerAuthenticator_Validate tests Bearer token authentication.
func TestBearerAuthenticatorValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		secret     string // empty means no validation
		authHeader string
		name       string
		wantErrMsg string
		wantType   auth.Type
		wantValid  bool
	}{
		{
			name: "valid bearer token with secret", secret: "my-secret-token",
			authHeader: "Bearer my-secret-token",
			wantValid:  true, wantType: auth.TypeBearer, wantErrMsg: "",
		},
		{
			name: "invalid bearer token with secret", secret: "my-secret-token",
			authHeader: "Bearer wrong-token",
			wantValid:  false, wantType: auth.TypeBearer, wantErrMsg: "invalid bearer token",
		},
		{
			name: "any bearer token without secret", secret: "",
			authHeader: "Bearer any-token-works",
			wantValid:  true, wantType: auth.TypeBearer, wantErrMsg: "",
		},
		{
			name: "missing authorization header", secret: "", authHeader: "",
			wantValid: false, wantType: auth.TypeBearer,
			wantErrMsg: "missing authorization header",
		},
		{
			name: "without bearer prefix", secret: "",
			authHeader: "Basic dXNlcjpwYXNz",
			wantValid:  false, wantType: auth.TypeBearer,
			wantErrMsg: "invalid authorization scheme",
		},
		{
			name: "bearer prefix only, no token", secret: "",
			authHeader: "Bearer ",
			wantValid:  false, wantType: auth.TypeBearer, wantErrMsg: "empty bearer token",
		},
		{
			name: "bearer prefix case insensitive", secret: "",
			authHeader: "bearer token-123",
			wantValid:  true, wantType: auth.TypeBearer, wantErrMsg: "",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			authenticator := auth.NewBearerAuthenticator(testCase.secret)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/messages", http.NoBody)
			if testCase.authHeader != "" {
				req.Header.Set("Authorization", testCase.authHeader)
			}

			result := authenticator.Validate(req)
			assertAuthResult(t, result, testCase.wantValid, testCase.wantType, testCase.wantErrMsg)
		})
	}
}

// TestBearerAuthenticator_Type verifies the type method.
func TestBearerAuthenticatorType(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewBearerAuthenticator("")

	if authenticator.Type() != auth.TypeBearer {
		t.Errorf("Type() = %q, want %q", authenticator.Type(), auth.TypeBearer)
	}
}

// TestChainAuthenticator_Validate tests chained authentication.
func TestChainAuthenticatorValidate(t *testing.T) {
	t.Parallel()

	apiKeyAuth := auth.NewAPIKeyAuthenticator("secret-key")
	bearerAuth := auth.NewBearerAuthenticator("secret-token")

	// Chain: try bearer first, then api key
	chainAuth := auth.NewChainAuthenticator(bearerAuth, apiKeyAuth)

	tests := []struct {
		apiKey     string
		authHeader string
		name       string
		wantType   auth.Type
		wantErrMsg string
		wantValid  bool
	}{
		{
			name:       "valid bearer token",
			apiKey:     "",
			authHeader: "Bearer secret-token",
			wantValid:  true,
			wantType:   auth.TypeBearer,
			wantErrMsg: "",
		},
		{
			name:       "valid api key",
			apiKey:     "secret-key",
			authHeader: "",
			wantValid:  true,
			wantType:   auth.TypeAPIKey,
			wantErrMsg: "",
		},
		{
			name:       "both headers, bearer takes precedence",
			apiKey:     "secret-key",
			authHeader: "Bearer secret-token",
			wantValid:  true,
			wantType:   auth.TypeBearer,
			wantErrMsg: "",
		},
		{
			name:       "invalid bearer falls through to api key",
			apiKey:     "secret-key",
			authHeader: "Bearer wrong-token",
			wantValid:  true,
			wantType:   auth.TypeAPIKey,
			wantErrMsg: "",
		},
		{
			name:       "no credentials",
			apiKey:     "",
			authHeader: "",
			wantValid:  false,
			wantType:   auth.TypeNone,
			wantErrMsg: "no authentication configured",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/messages", http.NoBody)
			if testCase.apiKey != "" {
				req.Header.Set("x-api-key", testCase.apiKey)
			}
			if testCase.authHeader != "" {
				req.Header.Set("Authorization", testCase.authHeader)
			}

			result := chainAuth.Validate(req)

			if result.Valid != testCase.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, testCase.wantValid)
			}

			if result.Type != testCase.wantType {
				t.Errorf("Type = %q, want %q", result.Type, testCase.wantType)
			}
		})
	}
}

// TestChainAuthenticator_Type verifies the type method.
func TestChainAuthenticatorType(t *testing.T) {
	t.Parallel()

	chainAuth := auth.NewChainAuthenticator()

	if chainAuth.Type() != auth.TypeNone {
		t.Errorf("Type() = %q, want %q", chainAuth.Type(), auth.TypeNone)
	}
}

// TestChainAuthenticator_EmptyChain tests the chain with no authenticators.
func TestChainAuthenticatorEmptyChain(t *testing.T) {
	t.Parallel()

	chainAuth := auth.NewChainAuthenticator() // No authenticators

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/messages", http.NoBody)
	result := chainAuth.Validate(req)

	if result.Valid {
		t.Error("Expected Valid=false for empty chain")
	}

	if result.Type != auth.TypeNone {
		t.Errorf("Expected Type=none, got %q", result.Type)
	}

	if result.Error != "no authentication configured" {
		t.Errorf("Expected error 'no authentication configured', got %q", result.Error)
	}
}
