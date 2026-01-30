// Package auth provides authentication for cc-relay.
package auth_test

import (
	"errors"
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
		wantValid  bool
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

// Tests for mo.Result-based authentication methods

// TestAPIKeyAuthenticator_ValidateResult tests the mo.Result-returning method.
func TestAPIKeyAuthenticatorValidateResult(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewAPIKeyAuthenticator("test-api-key-12345")

	t.Run("valid key returns Ok", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		req.Header.Set("x-api-key", "test-api-key-12345")

		result := authenticator.ValidateResult(req)
		if result.IsError() {
			t.Errorf("Expected Ok, got Err: %v", result.Error())
		}
		authResult, _ := result.Get()
		if !authResult.Valid {
			t.Error("Expected Valid=true in Ok result")
		}
		if authResult.Type != auth.TypeAPIKey {
			t.Errorf("Expected Type=api_key, got %q", authResult.Type)
		}
	})

	t.Run("invalid key returns Err with ValidationError", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		req.Header.Set("x-api-key", "wrong-key")

		result := authenticator.ValidateResult(req)
		if result.IsOk() {
			t.Error("Expected Err, got Ok")
		}

		err := result.Error()
		if err == nil {
			t.Error("Expected error, got nil")
		}
		authErr := &auth.ValidationError{}
		ok := errors.As(err, &authErr)
		if !ok {
			t.Errorf("Expected *ValidationError, got %T", err)
		}
		if authErr.Type != auth.TypeAPIKey {
			t.Errorf("Expected Type=api_key, got %q", authErr.Type)
		}
		if authErr.Message != "invalid x-api-key" {
			t.Errorf("Expected message 'invalid x-api-key', got %q", authErr.Message)
		}
	})

	t.Run("missing key returns Err", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)

		result := authenticator.ValidateResult(req)
		if result.IsOk() {
			t.Error("Expected Err, got Ok")
		}
	})
}

// TestBearerAuthenticator_ValidateResult tests the mo.Result-returning method.
func TestBearerAuthenticatorValidateResult(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewBearerAuthenticator("my-secret-token")

	t.Run("valid bearer returns Ok", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		req.Header.Set("Authorization", "Bearer my-secret-token")

		result := authenticator.ValidateResult(req)
		if result.IsError() {
			t.Errorf("Expected Ok, got Err: %v", result.Error())
		}
		authResult, _ := result.Get()
		if !authResult.Valid {
			t.Error("Expected Valid=true in Ok result")
		}
		if authResult.Type != auth.TypeBearer {
			t.Errorf("Expected Type=bearer, got %q", authResult.Type)
		}
	})

	t.Run("invalid bearer returns Err with ValidationError", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		req.Header.Set("Authorization", "Bearer wrong-token")

		result := authenticator.ValidateResult(req)
		if result.IsOk() {
			t.Error("Expected Err, got Ok")
		}

		err := result.Error()
		authErr := &auth.ValidationError{}
		ok := errors.As(err, &authErr)
		if !ok {
			t.Errorf("Expected *ValidationError, got %T", err)
		}
		if authErr.Type != auth.TypeBearer {
			t.Errorf("Expected Type=bearer, got %q", authErr.Type)
		}
	})
}

// TestChainAuthenticator_ValidateResult tests the mo.Result-returning method.
func TestChainAuthenticatorValidateResult(t *testing.T) {
	t.Parallel()

	apiKeyAuth := auth.NewAPIKeyAuthenticator("secret-key")
	bearerAuth := auth.NewBearerAuthenticator("secret-token")
	chainAuth := auth.NewChainAuthenticator(bearerAuth, apiKeyAuth)

	t.Run("valid bearer returns Ok", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		req.Header.Set("Authorization", "Bearer secret-token")

		result := chainAuth.ValidateResult(req)
		if result.IsError() {
			t.Errorf("Expected Ok, got Err: %v", result.Error())
		}
		authResult, _ := result.Get()
		if authResult.Type != auth.TypeBearer {
			t.Errorf("Expected Type=bearer, got %q", authResult.Type)
		}
	})

	t.Run("valid api key returns Ok", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		req.Header.Set("x-api-key", "secret-key")

		result := chainAuth.ValidateResult(req)
		if result.IsError() {
			t.Errorf("Expected Ok, got Err: %v", result.Error())
		}
		authResult, _ := result.Get()
		if authResult.Type != auth.TypeAPIKey {
			t.Errorf("Expected Type=api_key, got %q", authResult.Type)
		}
	})

	t.Run("no credentials returns Err", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)

		result := chainAuth.ValidateResult(req)
		if result.IsOk() {
			t.Error("Expected Err, got Ok")
		}

		err := result.Error()
		authErr := &auth.ValidationError{}
		ok := errors.As(err, &authErr)
		if !ok {
			t.Errorf("Expected *ValidationError, got %T", err)
		}
		if authErr.Type != auth.TypeNone {
			t.Errorf("Expected Type=none, got %q", authErr.Type)
		}
	})
}

// TestValidationError tests the ValidationError type.
func TestValidationError(t *testing.T) {
	t.Parallel()

	t.Run("Error method returns message", func(t *testing.T) {
		t.Parallel()

		err := auth.NewValidationError(auth.TypeAPIKey, "test error message")
		if err.Error() != "test error message" {
			t.Errorf("Error() = %q, want %q", err.Error(), "test error message")
		}
	})

	t.Run("fields are set correctly", func(t *testing.T) {
		t.Parallel()

		err := auth.NewValidationError(auth.TypeBearer, "bearer error")
		if err.Type != auth.TypeBearer {
			t.Errorf("Type = %q, want %q", err.Type, auth.TypeBearer)
		}
		if err.Message != "bearer error" {
			t.Errorf("Message = %q, want %q", err.Message, "bearer error")
		}
	})
}

// TestValidateResult_RailwayPattern demonstrates Railway-Oriented Programming.
func TestValidateResultRailwayPattern(t *testing.T) {
	t.Parallel()

	apiKeyAuth := auth.NewAPIKeyAuthenticator("valid-key")

	t.Run("Map transforms successful result", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		req.Header.Set("x-api-key", "valid-key")

		// Use Map to transform the auth result
		transformed := apiKeyAuth.ValidateResult(req).Map(func(r auth.Result) (auth.Result, error) {
			// Add token to result (demonstrating transformation)
			r.Token = "transformed"
			return r, nil
		})

		if transformed.IsError() {
			t.Errorf("Expected Ok, got Err: %v", transformed.Error())
		}

		result, _ := transformed.Get()
		if result.Token != "transformed" {
			t.Errorf("Expected Token='transformed', got %q", result.Token)
		}
	})

	t.Run("OrElse provides default on failure", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodPost, "/v1/messages", http.NoBody)
		// No credentials - will fail

		defaultResult := auth.Result{Valid: false, Type: auth.TypeNone, Error: "default"}
		result := apiKeyAuth.ValidateResult(req).OrElse(defaultResult)

		if result.Error != "default" {
			t.Errorf("Expected Error='default', got %q", result.Error)
		}
	})
}
