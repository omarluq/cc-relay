package providers_test

import (
	"net/http"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

const (
	anthropicVersionDate = "2023-06-01"
	jsonContentType      = "application/json"
	featureHeaderValue   = "feature-1"
)

// assertProviderNameAndURL is a shared test helper that verifies a provider's
// name and base URL are set correctly.
func assertProviderNameAndURL(
	t *testing.T,
	provider providers.Provider,
	expectedName string,
	expectedBaseURL string,
) {
	t.Helper()

	if provider.Name() != expectedName {
		t.Errorf("Expected name=%s, got %s", expectedName, provider.Name())
	}

	if provider.BaseURL() != expectedBaseURL {
		t.Errorf(
			"Expected baseURL=%s, got %s",
			expectedBaseURL, provider.BaseURL(),
		)
	}
}

// assertAuthenticateSetsKey is a shared test helper that verifies authentication
// sets the x-api-key header correctly.
func assertAuthenticateSetsKey(
	t *testing.T,
	provider providers.Provider,
	req *http.Request,
) {
	t.Helper()

	// Using "test-auth-key" to avoid gosec G101 (hardcoded credentials)
	testAuthKey := "test-auth-key-for-testing-only"

	err := provider.Authenticate(req, testAuthKey)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	gotKey := req.Header.Get("x-api-key")
	if gotKey != testAuthKey {
		t.Errorf("Expected x-api-key=%s, got %s", testAuthKey, gotKey)
	}
}

// assertForwardHeaders is a shared test helper that verifies forward headers
// behavior for any provider that follows the Anthropic header forwarding pattern.
func assertForwardHeaders(t *testing.T, provider providers.Provider) {
	t.Helper()

	// Create original headers with mix of anthropic-* and other headers
	originalHeaders := http.Header{
		"anthropic-version":                         []string{anthropicVersionDate},
		"anthropic-dangerous-direct-browser-access": []string{"true"},
		"Authorization":                             []string{"Bearer token"},
		"User-Agent":                                []string{"test-agent"},
		"X-Custom-Header":                           []string{"custom-value"},
	}

	forwardedHeaders := provider.ForwardHeaders(originalHeaders)

	// Verify anthropic-* headers are forwarded
	if forwardedHeaders.Get("anthropic-version") != anthropicVersionDate {
		t.Errorf("Expected anthropic-version header to be forwarded")
	}

	if forwardedHeaders.Get("anthropic-dangerous-direct-browser-access") != "true" {
		t.Errorf(
			"Expected anthropic-dangerous-direct-browser-access header to be forwarded",
		)
	}

	// Verify Content-Type is set
	if forwardedHeaders.Get("Content-Type") != jsonContentType {
		t.Errorf(
			"Expected Content-Type=%s, got %s",
			jsonContentType, forwardedHeaders.Get("Content-Type"),
		)
	}

	// Verify non-anthropic headers are NOT forwarded
	if forwardedHeaders.Get("Authorization") != "" {
		t.Error("Expected Authorization header to not be forwarded")
	}

	if forwardedHeaders.Get("User-Agent") != "" {
		t.Error("Expected User-Agent header to not be forwarded")
	}

	if forwardedHeaders.Get("X-Custom-Header") != "" {
		t.Error("Expected X-Custom-Header to not be forwarded")
	}
}

// providerConstructor is a function type for creating a provider with name and baseURL.
type providerConstructor func(name, baseURL string) providers.Provider

// providerTestCase holds test data for testing provider construction.
type providerTestCase struct {
	name         string
	providerName string
	baseURL      string
	wantBaseURL  string
}

// assertNewProvider is a shared test helper that verifies provider construction
// with different base URL configurations.
func assertNewProvider(
	t *testing.T,
	newProvider providerConstructor,
	tests []providerTestCase,
) {
	t.Helper()

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			provider := newProvider(testCase.providerName, testCase.baseURL)

			assertProviderNameAndURL(
				t, provider,
				testCase.providerName, testCase.wantBaseURL,
			)
		})
	}
}

// Helper function for TestForwardHeadersEdgeCases tests.
// Asserts that a provider correctly handles various edge cases when forwarding headers.
func assertForwardHeadersEdgeCases(t *testing.T, provider providers.Provider) {
	t.Helper()

	tests := []struct {
		originalHeaders http.Header
		checkFunc       func(*testing.T, http.Header)
		name            string
	}{
		{
			name:            "empty headers",
			originalHeaders: http.Header{},
			checkFunc: func(t *testing.T, h http.Header) {
				t.Helper()
				if h.Get("Content-Type") != jsonContentType {
					t.Error(
						"Expected Content-Type to be set even with empty original headers",
					)
				}
			},
		},
		{
			name: "multiple anthropic headers",
			originalHeaders: http.Header{
				"anthropic-version": []string{anthropicVersionDate},
				"anthropic-beta":    []string{featureHeaderValue, "feature-2"},
			},
			checkFunc: func(t *testing.T, h http.Header) {
				t.Helper()
				if h.Get("anthropic-version") != anthropicVersionDate {
					t.Error("Expected anthropic-version to be forwarded")
				}
				beta := h["Anthropic-Beta"]
				if len(beta) != 2 || beta[0] != featureHeaderValue || beta[1] != "feature-2" {
					t.Errorf("Expected anthropic-beta to have both values, got %v", beta)
				}
			},
		},
		{
			name: "short header name starting with 'a'",
			originalHeaders: http.Header{
				"accept": []string{jsonContentType},
			},
			checkFunc: func(t *testing.T, h http.Header) {
				t.Helper()
				if h.Get("accept") != "" {
					t.Error("Expected short header starting with 'a' to not be forwarded")
				}
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			forwardedHeaders := provider.ForwardHeaders(testCase.originalHeaders)
			testCase.checkFunc(t, forwardedHeaders)
		})
	}
}

// Helper function for TestListModelsWithConfiguredModels tests.
// Asserts that models are correctly listed with proper metadata.
func assertListModelsWithConfiguredModels(
	t *testing.T,
	result []providers.Model,
	expectedOwner string,
	expectedProvider string,
) {
	t.Helper()

	if len(result) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(result))
	}

	// First model
	if result[0].Object != "model" {
		t.Errorf("Expected object=model, got %s", result[0].Object)
	}
	if result[0].OwnedBy != expectedOwner {
		t.Errorf("Expected owned_by=%s, got %s", expectedOwner, result[0].OwnedBy)
	}
	if result[0].Provider != expectedProvider {
		t.Errorf("Expected provider=%s, got %s", expectedProvider, result[0].Provider)
	}
	if result[0].Created == 0 {
		t.Error("Expected created timestamp to be set")
	}
}
