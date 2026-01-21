// Package auth provides authentication for cc-relay.
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarluq/cc-relay/internal/auth"
)

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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if string(tt.authType) != tt.want {
				t.Errorf("auth type = %q, want %q", tt.authType, tt.want)
			}
		})
	}
}

// TestAPIKeyAuthenticator_Validate tests API key authentication.
func TestAPIKeyAuthenticator_Validate(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewAPIKeyAuthenticator("test-api-key-12345")

	tests := []struct { //nolint:govet // test table struct alignment
		name       string
		apiKey     string
		wantValid  bool
		wantType   auth.Type
		wantErrMsg string
	}{
		{
			name:      "valid api key",
			apiKey:    "test-api-key-12345",
			wantValid: true,
			wantType:  auth.TypeAPIKey,
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
			if tt.apiKey != "" {
				req.Header.Set("x-api-key", tt.apiKey)
			}

			result := authenticator.Validate(req)

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", result.Type, tt.wantType)
			}

			if tt.wantErrMsg != "" && result.Error != tt.wantErrMsg {
				t.Errorf("Error = %q, want %q", result.Error, tt.wantErrMsg)
			}
		})
	}
}

// TestAPIKeyAuthenticator_Type verifies the type method.
func TestAPIKeyAuthenticator_Type(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewAPIKeyAuthenticator("test-key")

	if authenticator.Type() != auth.TypeAPIKey {
		t.Errorf("Type() = %q, want %q", authenticator.Type(), auth.TypeAPIKey)
	}
}

// TestBearerAuthenticator_Validate tests Bearer token authentication.
func TestBearerAuthenticator_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct { //nolint:govet // test table struct alignment
		name       string
		secret     string // empty means no validation
		authHeader string
		wantValid  bool
		wantType   auth.Type
		wantErrMsg string
	}{
		{
			name:       "valid bearer token with secret",
			secret:     "my-secret-token",
			authHeader: "Bearer my-secret-token",
			wantValid:  true,
			wantType:   auth.TypeBearer,
		},
		{
			name:       "invalid bearer token with secret",
			secret:     "my-secret-token",
			authHeader: "Bearer wrong-token",
			wantValid:  false,
			wantType:   auth.TypeBearer,
			wantErrMsg: "invalid bearer token",
		},
		{
			name:       "any bearer token without secret validation",
			secret:     "",
			authHeader: "Bearer any-token-works",
			wantValid:  true,
			wantType:   auth.TypeBearer,
		},
		{
			name:       "missing authorization header",
			secret:     "",
			authHeader: "",
			wantValid:  false,
			wantType:   auth.TypeBearer,
			wantErrMsg: "missing authorization header",
		},
		{
			name:       "authorization header without bearer prefix",
			secret:     "",
			authHeader: "Basic dXNlcjpwYXNz",
			wantValid:  false,
			wantType:   auth.TypeBearer,
			wantErrMsg: "invalid authorization scheme",
		},
		{
			name:       "bearer prefix only, no token",
			secret:     "",
			authHeader: "Bearer ",
			wantValid:  false,
			wantType:   auth.TypeBearer,
			wantErrMsg: "empty bearer token",
		},
		{
			name:       "bearer prefix case insensitive",
			secret:     "",
			authHeader: "bearer token-123",
			wantValid:  true,
			wantType:   auth.TypeBearer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			authenticator := auth.NewBearerAuthenticator(tt.secret)

			req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			result := authenticator.Validate(req)

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", result.Type, tt.wantType)
			}

			if tt.wantErrMsg != "" && result.Error != tt.wantErrMsg {
				t.Errorf("Error = %q, want %q", result.Error, tt.wantErrMsg)
			}
		})
	}
}

// TestBearerAuthenticator_Type verifies the type method.
func TestBearerAuthenticator_Type(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewBearerAuthenticator("")

	if authenticator.Type() != auth.TypeBearer {
		t.Errorf("Type() = %q, want %q", authenticator.Type(), auth.TypeBearer)
	}
}

// TestChainAuthenticator_Validate tests chained authentication.
func TestChainAuthenticator_Validate(t *testing.T) {
	t.Parallel()

	apiKeyAuth := auth.NewAPIKeyAuthenticator("secret-key")
	bearerAuth := auth.NewBearerAuthenticator("secret-token")

	// Chain: try bearer first, then api key
	chainAuth := auth.NewChainAuthenticator(bearerAuth, apiKeyAuth)

	tests := []struct { //nolint:govet // test table struct alignment
		name       string
		apiKey     string
		authHeader string
		wantValid  bool
		wantType   auth.Type
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer secret-token",
			wantValid:  true,
			wantType:   auth.TypeBearer,
		},
		{
			name:      "valid api key",
			apiKey:    "secret-key",
			wantValid: true,
			wantType:  auth.TypeAPIKey,
		},
		{
			name:       "both headers, bearer takes precedence",
			apiKey:     "secret-key",
			authHeader: "Bearer secret-token",
			wantValid:  true,
			wantType:   auth.TypeBearer,
		},
		{
			name:       "invalid bearer falls through to api key",
			apiKey:     "secret-key",
			authHeader: "Bearer wrong-token",
			wantValid:  true,
			wantType:   auth.TypeAPIKey,
		},
		{
			name:      "no credentials",
			wantValid: false,
			wantType:  auth.TypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
			if tt.apiKey != "" {
				req.Header.Set("x-api-key", tt.apiKey)
			}
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			result := chainAuth.Validate(req)

			if result.Valid != tt.wantValid {
				t.Errorf("Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", result.Type, tt.wantType)
			}
		})
	}
}

// TestChainAuthenticator_Type verifies the type method.
func TestChainAuthenticator_Type(t *testing.T) {
	t.Parallel()

	chainAuth := auth.NewChainAuthenticator()

	if chainAuth.Type() != auth.TypeNone {
		t.Errorf("Type() = %q, want %q", chainAuth.Type(), auth.TypeNone)
	}
}

// TestChainAuthenticator_EmptyChain tests the chain with no authenticators.
func TestChainAuthenticator_EmptyChain(t *testing.T) {
	t.Parallel()

	chainAuth := auth.NewChainAuthenticator() // No authenticators

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
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
