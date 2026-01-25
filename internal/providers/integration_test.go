//go:build integration

package providers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"golang.org/x/oauth2"

	"github.com/omarluq/cc-relay/internal/providers"
)

// mockCredentialsProvider implements BedrockCredentialsProvider for testing.
type mockCredentialsProvider struct {
	creds aws.Credentials
}

func (m *mockCredentialsProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	return m.creds, nil
}

func newMockCredentialsProvider(accessKey, secretKey string) *mockCredentialsProvider {
	return &mockCredentialsProvider{
		creds: aws.Credentials{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
		},
	}
}

// mockTokenSource provides a controllable token source for Vertex AI testing.
type mockTokenSource struct {
	token *oauth2.Token
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	return m.token, nil
}

func newMockTokenSource(accessToken string) *mockTokenSource {
	return &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: accessToken,
			TokenType:   "Bearer",
		},
	}
}

// Integration tests for provider routing with mock backends.
// Run with: go test -v -tags=integration ./internal/providers/...
//
// These tests use httptest.Server to mock provider backends,
// verifying that providers correctly route requests and handle responses.

// mockAnthropicResponse returns a valid Anthropic Messages API response.
func mockAnthropicResponse() map[string]interface{} {
	return map[string]interface{}{
		"id":    "msg_01XFDUDYJgAACzvnptvVoYEL",
		"type":  "message",
		"role":  "assistant",
		"model": "claude-sonnet-4-5-20250514",
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": "Hello! This is a test response from the mock server.",
			},
		},
		"stop_reason": "end_turn",
		"usage": map[string]int{
			"input_tokens":  10,
			"output_tokens": 15,
		},
	}
}

// mockSSEResponse returns SSE events for a streaming response.
func mockSSEResponse() string {
	return `event: message_start
data: {"type":"message_start","message":{"id":"msg_01","type":"message","role":"assistant","model":"claude-sonnet-4-5-20250514","content":[],"stop_reason":null,"usage":{"input_tokens":10,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}

`
}

// TestZAIProvider_EndToEnd verifies Z.AI provider routing works end-to-end.
func TestZAIProvider_EndToEnd(t *testing.T) {
	t.Parallel()

	// Create mock server that mimics Z.AI's Anthropic-compatible endpoint
	var receivedRequest *http.Request
	var receivedBody []byte

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = r

		// Read body for verification
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		receivedBody = body

		// Return valid Anthropic-format response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockAnthropicResponse())
	}))
	defer mockServer.Close()

	// Create Z.AI provider pointing to mock server
	provider := providers.NewZAIProviderWithModels("test-zai", mockServer.URL, []string{"GLM-4.7"})

	// Build request with Anthropic Messages API format
	reqBody := map[string]interface{}{
		"model":      "GLM-4.7",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello, world!"},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", mockServer.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set original headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "max-tokens-3-5-sonnet-2024-07-15")

	// Test Authenticate - sets x-api-key header
	apiKey := "test-zai-api-key-123"
	err = provider.Authenticate(req, apiKey)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Verify x-api-key header was set
	if req.Header.Get("x-api-key") != apiKey {
		t.Errorf("Expected x-api-key=%s, got %s", apiKey, req.Header.Get("x-api-key"))
	}

	// Test ForwardHeaders - returns headers to forward to backend
	forwardedHeaders := provider.ForwardHeaders(req.Header)

	// Verify anthropic-* headers are forwarded
	if forwardedHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("Expected anthropic-version header to be forwarded")
	}
	if forwardedHeaders.Get("anthropic-beta") != "max-tokens-3-5-sonnet-2024-07-15" {
		t.Errorf("Expected anthropic-beta header to be forwarded")
	}
	if forwardedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type to be set")
	}

	// Actually send request to mock server
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request to mock server failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify request reached mock server
	if receivedRequest == nil {
		t.Fatal("Request did not reach mock server")
	}

	// Verify request method
	if receivedRequest.Method != "POST" {
		t.Errorf("Expected POST method, got %s", receivedRequest.Method)
	}

	// Verify x-api-key header reached server
	if receivedRequest.Header.Get("x-api-key") != apiKey {
		t.Errorf("Expected x-api-key header on server, got %s", receivedRequest.Header.Get("x-api-key"))
	}

	// Verify body was received
	if len(receivedBody) == 0 {
		t.Error("No body received by mock server")
	}

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["type"] != "message" {
		t.Errorf("Expected type=message, got %v", response["type"])
	}
}

// TestOllamaProvider_EndToEnd verifies Ollama provider routing works end-to-end.
func TestOllamaProvider_EndToEnd(t *testing.T) {
	t.Parallel()

	// Create mock server that mimics Ollama's Anthropic-compatible endpoint
	var receivedRequest *http.Request
	var receivedBody []byte

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequest = r

		// Read body for verification
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		receivedBody = body

		// Return valid Anthropic-format response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mockAnthropicResponse())
	}))
	defer mockServer.Close()

	// Create Ollama provider pointing to mock server
	provider := providers.NewOllamaProviderWithModels("test-ollama", mockServer.URL, []string{"qwen3:32b"})

	// Build request with Anthropic Messages API format
	reqBody := map[string]interface{}{
		"model":      "qwen3:32b",
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello from Ollama!"},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", mockServer.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set original headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	// Test Authenticate - Ollama accepts but ignores API keys
	apiKey := "ollama-dummy-key"
	err = provider.Authenticate(req, apiKey)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Verify x-api-key header was set (Ollama accepts but ignores it)
	if req.Header.Get("x-api-key") != apiKey {
		t.Errorf("Expected x-api-key=%s, got %s", apiKey, req.Header.Get("x-api-key"))
	}

	// Test ForwardHeaders
	forwardedHeaders := provider.ForwardHeaders(req.Header)

	// Verify anthropic-* headers are forwarded
	if forwardedHeaders.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("Expected anthropic-version header to be forwarded")
	}

	// Actually send request to mock server
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request to mock server failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify request reached mock server
	if receivedRequest == nil {
		t.Fatal("Request did not reach mock server")
	}

	// Verify body was received
	if len(receivedBody) == 0 {
		t.Error("No body received by mock server")
	}

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["type"] != "message" {
		t.Errorf("Expected type=message, got %v", response["type"])
	}
}

// TestOllamaProvider_StreamingResponse verifies Ollama handles SSE streaming correctly.
func TestOllamaProvider_StreamingResponse(t *testing.T) {
	t.Parallel()

	// Create mock server that returns SSE streaming response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify streaming request
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			if stream, ok := reqBody["stream"].(bool); !ok || !stream {
				t.Logf("Warning: stream field not set to true")
			}
		}

		// Return SSE streaming response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		// Write SSE events
		_, _ = w.Write([]byte(mockSSEResponse()))
	}))
	defer mockServer.Close()

	// Create Ollama provider
	provider := providers.NewOllamaProviderWithModels("test-ollama", mockServer.URL, []string{"qwen3:32b"})

	// Verify provider supports streaming
	if !provider.SupportsStreaming() {
		t.Error("Expected OllamaProvider to support streaming")
	}

	// Build streaming request
	reqBody := map[string]interface{}{
		"model":      "qwen3:32b",
		"max_tokens": 100,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "user", "content": "Count to 3"},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", mockServer.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	// Authenticate
	_ = provider.Authenticate(req, "dummy-key")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	// Verify streaming headers
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type=text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}

	// Read and verify SSE events
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Verify expected SSE events are present
	expectedEvents := []string{
		"event: message_start",
		"event: content_block_start",
		"event: content_block_delta",
		"event: content_block_stop",
		"event: message_delta",
		"event: message_stop",
	}

	for _, event := range expectedEvents {
		if !strings.Contains(bodyStr, event) {
			t.Errorf("Expected response to contain %q", event)
		}
	}
}

// TestProvider_ModelMapping verifies provider.ListModels() returns configured models.
func TestProvider_ModelMapping(t *testing.T) {
	t.Parallel()

	t.Run("ZAI returns default models when none configured", func(t *testing.T) {
		t.Parallel()

		provider := providers.NewZAIProvider("test-zai", "")
		models := provider.ListModels()

		// Z.AI should have default models
		if len(models) == 0 {
			t.Error("Expected Z.AI to have default models")
		}

		// Check for expected default models
		foundGLM47 := false
		for _, m := range models {
			if m.ID == "GLM-4.7" {
				foundGLM47 = true
				break
			}
		}
		if !foundGLM47 {
			t.Error("Expected GLM-4.7 in default Z.AI models")
		}
	})

	t.Run("ZAI returns configured models", func(t *testing.T) {
		t.Parallel()

		configuredModels := []string{"custom-model-1", "custom-model-2"}
		provider := providers.NewZAIProviderWithModels("test-zai", "", configuredModels)
		models := provider.ListModels()

		if len(models) != 2 {
			t.Errorf("Expected 2 models, got %d", len(models))
		}

		if models[0].ID != "custom-model-1" {
			t.Errorf("Expected first model ID=custom-model-1, got %s", models[0].ID)
		}
		if models[1].ID != "custom-model-2" {
			t.Errorf("Expected second model ID=custom-model-2, got %s", models[1].ID)
		}

		// Verify owner is zhipu
		for _, m := range models {
			if m.OwnedBy != "zhipu" {
				t.Errorf("Expected model owned_by=zhipu, got %s", m.OwnedBy)
			}
		}
	})

	t.Run("Ollama returns empty models when none configured", func(t *testing.T) {
		t.Parallel()

		provider := providers.NewOllamaProvider("test-ollama", "")
		models := provider.ListModels()

		// Ollama should have no default models (models are user-installed)
		if len(models) != 0 {
			t.Errorf("Expected Ollama to have 0 default models, got %d", len(models))
		}
	})

	t.Run("Ollama returns configured models", func(t *testing.T) {
		t.Parallel()

		configuredModels := []string{"llama3.2:3b", "qwen3:32b", "codestral:latest"}
		provider := providers.NewOllamaProviderWithModels("test-ollama", "", configuredModels)
		models := provider.ListModels()

		if len(models) != 3 {
			t.Errorf("Expected 3 models, got %d", len(models))
		}

		// Verify owner is ollama
		for _, m := range models {
			if m.OwnedBy != "ollama" {
				t.Errorf("Expected model owned_by=ollama, got %s", m.OwnedBy)
			}
		}
	})
}

// TestProvider_HealthCheck_Integration verifies health check can reach endpoint.
func TestProvider_HealthCheck_Integration(t *testing.T) {
	t.Parallel()

	t.Run("healthy server returns 200", func(t *testing.T) {
		t.Parallel()

		// Create mock server that returns 200
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer mockServer.Close()

		// Create provider pointing to mock server
		provider := providers.NewOllamaProviderWithModels("test-ollama", mockServer.URL, nil)

		// Verify provider base URL is set correctly
		if provider.BaseURL() != mockServer.URL {
			t.Errorf("Expected BaseURL=%s, got %s", mockServer.URL, provider.BaseURL())
		}

		// Make health check request manually
		resp, err := http.Get(mockServer.URL)
		if err != nil {
			t.Fatalf("Health check request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("unhealthy server returns 500", func(t *testing.T) {
		t.Parallel()

		// Create mock server that returns 500
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"status":"error"}`))
		}))
		defer mockServer.Close()

		// Create provider pointing to mock server
		provider := providers.NewZAIProviderWithModels("test-zai", mockServer.URL, nil)

		// Make health check request manually
		resp, err := http.Get(provider.BaseURL())
		if err != nil {
			t.Fatalf("Health check request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should detect 500 status
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("rate limited server returns 429", func(t *testing.T) {
		t.Parallel()

		// Create mock server that returns 429
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"Too many requests"}}`))
		}))
		defer mockServer.Close()

		// Create provider pointing to mock server
		provider := providers.NewOllamaProviderWithModels("test-ollama", mockServer.URL, nil)

		// Make request manually
		resp, err := http.Get(provider.BaseURL())
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should detect 429 status
		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("Expected status 429, got %d", resp.StatusCode)
		}

		// Verify Retry-After header
		if resp.Header.Get("Retry-After") != "60" {
			t.Errorf("Expected Retry-After=60, got %s", resp.Header.Get("Retry-After"))
		}
	})
}

// TestProvider_SupportsTransparentAuth verifies transparent auth support.
func TestProvider_SupportsTransparentAuth(t *testing.T) {
	t.Parallel()

	t.Run("ZAI does not support transparent auth", func(t *testing.T) {
		t.Parallel()

		provider := providers.NewZAIProvider("test-zai", "")
		if provider.SupportsTransparentAuth() {
			t.Error("Expected Z.AI to not support transparent auth")
		}
	})

	t.Run("Ollama does not support transparent auth", func(t *testing.T) {
		t.Parallel()

		provider := providers.NewOllamaProvider("test-ollama", "")
		if provider.SupportsTransparentAuth() {
			t.Error("Expected Ollama to not support transparent auth")
		}
	})
}

// TestProvider_BaseURL verifies base URL handling.
func TestProvider_BaseURL(t *testing.T) {
	t.Parallel()

	t.Run("ZAI uses default URL when empty", func(t *testing.T) {
		t.Parallel()

		provider := providers.NewZAIProvider("test", "")
		if provider.BaseURL() != providers.DefaultZAIBaseURL {
			t.Errorf("Expected default URL %s, got %s", providers.DefaultZAIBaseURL, provider.BaseURL())
		}
	})

	t.Run("ZAI uses custom URL when provided", func(t *testing.T) {
		t.Parallel()

		customURL := "https://custom.zhipu.com/api"
		provider := providers.NewZAIProvider("test", customURL)
		if provider.BaseURL() != customURL {
			t.Errorf("Expected custom URL %s, got %s", customURL, provider.BaseURL())
		}
	})

	t.Run("Ollama uses default URL when empty", func(t *testing.T) {
		t.Parallel()

		provider := providers.NewOllamaProvider("test", "")
		if provider.BaseURL() != providers.DefaultOllamaBaseURL {
			t.Errorf("Expected default URL %s, got %s", providers.DefaultOllamaBaseURL, provider.BaseURL())
		}
	})

	t.Run("Ollama uses custom URL when provided", func(t *testing.T) {
		t.Parallel()

		customURL := "http://192.168.1.100:11434"
		provider := providers.NewOllamaProvider("test", customURL)
		if provider.BaseURL() != customURL {
			t.Errorf("Expected custom URL %s, got %s", customURL, provider.BaseURL())
		}
	})
}

// TestAzureProvider_Integration tests Azure provider URL construction and body handling.
func TestAzureProvider_Integration(t *testing.T) {
	t.Parallel()

	t.Run("TransformRequest constructs correct URL with api-version", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			DeploymentID: "claude-deployment",
			APIVersion:   "2025-01-01",
			Models:       []string{"claude-sonnet-4-5"},
		}
		provider, err := providers.NewAzureProvider(cfg)
		if err != nil {
			t.Fatalf("NewAzureProvider failed: %v", err)
		}

		body := []byte(`{"model":"claude-sonnet-4-5-20250514","max_tokens":100}`)
		newBody, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}

		// Verify URL contains resource name
		if !strings.Contains(targetURL, "my-resource.services.ai.azure.com") {
			t.Errorf("Expected URL to contain resource name, got %s", targetURL)
		}

		// Verify URL contains API version
		if !strings.Contains(targetURL, "api-version=2025-01-01") {
			t.Errorf("Expected URL to contain api-version=2025-01-01, got %s", targetURL)
		}

		// Azure keeps body unchanged (same as direct Anthropic)
		if !bytes.Contains(newBody, []byte(`"model"`)) {
			t.Errorf("Expected body to contain model")
		}
	})

	t.Run("Authenticate sets x-api-key header", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			DeploymentID: "claude-deployment",
		}
		provider, err := providers.NewAzureProvider(cfg)
		if err != nil {
			t.Fatalf("NewAzureProvider failed: %v", err)
		}

		req := httptest.NewRequest("POST", "https://example.com", nil)
		err = provider.Authenticate(req, "azure-api-key-123")

		if err != nil {
			t.Fatalf("Authenticate failed: %v", err)
		}

		// Azure uses x-api-key header (Anthropic-compatible)
		if req.Header.Get("x-api-key") != "azure-api-key-123" {
			t.Errorf("Expected x-api-key header, got %s", req.Header.Get("x-api-key"))
		}
	})

	t.Run("RequiresBodyTransform returns false", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
		}
		provider, err := providers.NewAzureProvider(cfg)
		if err != nil {
			t.Fatalf("NewAzureProvider failed: %v", err)
		}

		// Azure uses standard Anthropic body format - no transformation needed
		if provider.RequiresBodyTransform() {
			t.Error("Expected Azure provider to not require body transform")
		}
	})
}

// TestVertexProvider_Integration tests Vertex AI provider URL construction.
func TestVertexProvider_Integration(t *testing.T) {
	t.Parallel()

	// Mock token source for testing (no real GCP access needed)
	mockTokenSrc := newMockTokenSource("test-access-token")

	t.Run("TransformRequest constructs correct URL with project and model", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.VertexConfig{
			Name:      "test-vertex",
			ProjectID: "my-gcp-project",
			Region:    "us-central1",
			Models:    []string{"claude-sonnet-4-5"},
		}
		provider := providers.NewVertexProviderWithTokenSource(cfg, mockTokenSrc)

		// stream: true in body triggers streamRawPredict endpoint
		body := []byte(`{"model":"claude-sonnet-4-5-20250514","max_tokens":100,"stream":true}`)
		newBody, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}

		// Verify URL contains project ID, region, and model
		if !strings.Contains(targetURL, "us-central1-aiplatform.googleapis.com") {
			t.Errorf("Expected URL to contain region, got %s", targetURL)
		}
		if !strings.Contains(targetURL, "my-gcp-project") {
			t.Errorf("Expected URL to contain project ID, got %s", targetURL)
		}
		if !strings.Contains(targetURL, "claude-sonnet-4-5-20250514") {
			t.Errorf("Expected URL to contain model, got %s", targetURL)
		}
		if !strings.Contains(targetURL, ":streamRawPredict") {
			t.Errorf("Expected URL to contain :streamRawPredict, got %s", targetURL)
		}

		// Verify body has anthropic_version
		if !bytes.Contains(newBody, []byte(`"anthropic_version"`)) {
			t.Errorf("Expected body to contain anthropic_version")
		}
	})

	t.Run("RequiresBodyTransform returns true", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.VertexConfig{
			Name:      "test-vertex",
			ProjectID: "project",
			Region:    "region",
		}
		provider := providers.NewVertexProviderWithTokenSource(cfg, mockTokenSrc)

		if !provider.RequiresBodyTransform() {
			t.Error("Expected Vertex provider to require body transform")
		}
	})

	t.Run("StreamingContentType returns SSE", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.VertexConfig{
			Name:      "test-vertex",
			ProjectID: "project",
			Region:    "region",
		}
		provider := providers.NewVertexProviderWithTokenSource(cfg, mockTokenSrc)

		if provider.StreamingContentType() != providers.ContentTypeSSE {
			t.Errorf("Expected Vertex to use SSE, got %s", provider.StreamingContentType())
		}
	})
}

// TestBedrockProvider_Integration tests Bedrock provider URL and body transformation.
func TestBedrockProvider_Integration(t *testing.T) {
	t.Parallel()

	// Mock credentials for testing (no real AWS access needed)
	mockCreds := newMockCredentialsProvider("test-access-key", "test-secret-key")

	t.Run("TransformRequest constructs correct URL with model", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.BedrockConfig{
			Name:   "test-bedrock",
			Region: "us-west-2",
			Models: []string{"claude-sonnet-4-5"},
		}
		provider := providers.NewBedrockProviderWithCredentials(cfg, mockCreds)

		body := []byte(`{"model":"claude-sonnet-4-5-20250514","max_tokens":100}`)
		newBody, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		if err != nil {
			t.Fatalf("TransformRequest failed: %v", err)
		}

		// Verify URL contains region
		if !strings.Contains(targetURL, "bedrock-runtime.us-west-2.amazonaws.com") {
			t.Errorf("Expected URL to contain region, got %s", targetURL)
		}

		// Verify URL contains model and invoke path
		if !strings.Contains(targetURL, "claude-sonnet-4-5-20250514") {
			t.Errorf("Expected URL to contain model, got %s", targetURL)
		}
		if !strings.Contains(targetURL, "invoke-with-response-stream") {
			t.Errorf("Expected URL to contain invoke path, got %s", targetURL)
		}

		// Verify body has anthropic_version
		if !bytes.Contains(newBody, []byte(`"anthropic_version"`)) {
			t.Errorf("Expected body to contain anthropic_version")
		}
	})

	t.Run("RequiresBodyTransform returns true", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.BedrockConfig{
			Name:   "test-bedrock",
			Region: "us-east-1",
		}
		provider := providers.NewBedrockProviderWithCredentials(cfg, mockCreds)

		if !provider.RequiresBodyTransform() {
			t.Error("Expected Bedrock provider to require body transform")
		}
	})

	t.Run("StreamingContentType returns Event Stream", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.BedrockConfig{
			Name:   "test-bedrock",
			Region: "us-east-1",
		}
		provider := providers.NewBedrockProviderWithCredentials(cfg, mockCreds)

		if provider.StreamingContentType() != providers.ContentTypeEventStream {
			t.Errorf("Expected Bedrock to use Event Stream, got %s", provider.StreamingContentType())
		}
	})
}

// TestCloudProviders_ModelMapping tests model mapping for all cloud providers.
func TestCloudProviders_ModelMapping(t *testing.T) {
	t.Parallel()

	t.Run("Azure maps models correctly", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.AzureConfig{
			Name:         "test-azure",
			ResourceName: "my-resource",
			ModelMapping: map[string]string{
				"claude-sonnet-4-5": "my-azure-claude",
			},
		}
		provider, err := providers.NewAzureProvider(cfg)
		if err != nil {
			t.Fatalf("NewAzureProvider failed: %v", err)
		}

		mapped := provider.MapModel("claude-sonnet-4-5")
		if mapped != "my-azure-claude" {
			t.Errorf("Expected mapped model my-azure-claude, got %s", mapped)
		}

		// Unmapped model returns as-is
		unmapped := provider.MapModel("unknown-model")
		if unmapped != "unknown-model" {
			t.Errorf("Expected unmapped model to be returned as-is, got %s", unmapped)
		}
	})

	t.Run("Vertex maps models correctly", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.VertexConfig{
			Name:      "test-vertex",
			ProjectID: "project",
			Region:    "region",
			ModelMapping: map[string]string{
				"claude-sonnet-4-5": "my-vertex-model",
			},
		}
		mockTokenSrc := newMockTokenSource("test-token")
		provider := providers.NewVertexProviderWithTokenSource(cfg, mockTokenSrc)

		mapped := provider.MapModel("claude-sonnet-4-5")
		if mapped != "my-vertex-model" {
			t.Errorf("Expected mapped model my-vertex-model, got %s", mapped)
		}
	})

	t.Run("Bedrock maps models correctly", func(t *testing.T) {
		t.Parallel()

		cfg := &providers.BedrockConfig{
			Name:   "test-bedrock",
			Region: "us-east-1",
			ModelMapping: map[string]string{
				"claude-sonnet-4-5": "anthropic.claude-3-sonnet-20240229-v1:0",
			},
		}
		mockCreds := newMockCredentialsProvider("test-access-key", "test-secret-key")
		provider := providers.NewBedrockProviderWithCredentials(cfg, mockCreds)

		mapped := provider.MapModel("claude-sonnet-4-5")
		if mapped != "anthropic.claude-3-sonnet-20240229-v1:0" {
			t.Errorf("Expected mapped model, got %s", mapped)
		}
	})
}
