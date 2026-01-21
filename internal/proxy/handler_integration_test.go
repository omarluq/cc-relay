//go:build integration
// +build integration

package proxy_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
)

// Integration tests require ANTHROPIC_API_KEY environment variable
// Run with: go test -tags=integration -v ./internal/proxy/...

const (
	testProxyAPIKey = "test-proxy-key"
	testModel       = "claude-sonnet-4-5-20250929"
)

// setupTestProxy creates a test HTTP server with the proxy configured.
// If customBackendURL is empty, uses default Anthropic API.
//
//nolint:unparam // customBackendURL parameter intentional for future test flexibility
func setupTestProxy(t *testing.T, customBackendURL string) *httptest.Server {
	t.Helper()

	// Get provider API key from environment
	providerKey := os.Getenv("ANTHROPIC_API_KEY")
	if providerKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping integration test")
	}

	// Create config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen:    "127.0.0.1:0",
			APIKey:    testProxyAPIKey,
			TimeoutMS: 60000,
		},
		Logging: config.LoggingConfig{
			Level:  "debug",
			Format: "text",
		},
	}

	// Create provider
	backendURL := "https://api.anthropic.com"
	if customBackendURL != "" {
		backendURL = customBackendURL
	}

	provider := providers.NewAnthropicProvider("test", backendURL)

	// Setup routes
	handler, err := proxy.SetupRoutes(cfg, provider, providerKey)
	if err != nil {
		t.Fatalf("Failed to setup routes: %v", err)
	}

	// Create test server
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

func TestIntegration_NonStreamingRequest(t *testing.T) {
	server := setupTestProxy(t, "")

	// Create request body
	reqBody := map[string]interface{}{
		"model":      testModel,
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'integration test passed' and nothing else."},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Send request
	req, err := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", testProxyAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	defer resp.Body.Close()

	// Verify response
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response["type"] != "message" {
		t.Errorf("Expected type=message, got %v", response["type"])
	}

	if response["model"] == nil {
		t.Error("Response missing model field")
	}

	if content, ok := response["content"].([]interface{}); !ok || len(content) == 0 {
		t.Error("Response missing or empty content array")
	}
}

func TestIntegration_StreamingRequest(t *testing.T) {
	server := setupTestProxy(t, "")

	// Create streaming request
	reqBody := map[string]interface{}{
		"model":      testModel,
		"max_tokens": 100,
		"stream":     true,
		"messages": []map[string]string{
			{"role": "user", "content": "Count from 1 to 5 slowly."},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Send request
	req, err := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", testProxyAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	defer resp.Body.Close()

	// Verify streaming headers
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type=text/event-stream, got %q", resp.Header.Get("Content-Type"))
	}

	// Verify events arrive incrementally
	if err := verifyStreamingBehavior(resp); err != nil {
		t.Errorf("Streaming behavior verification failed: %v", err)
	}
}

// verifyStreamingBehavior checks that SSE events arrive incrementally (not buffered).
//
//nolint:gocognit // Integration test - complexity unavoidable
func verifyStreamingBehavior(resp *http.Response) error {
	scanner := bufio.NewScanner(resp.Body)

	var eventCount int

	var lastEventTime time.Time

	var sawMessageStart, sawContentBlockStart, sawDelta, sawMessageStop bool

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and ping events
		if line == "" || strings.HasPrefix(line, ": ping") {
			continue
		}

		// Parse SSE event
		if strings.HasPrefix(line, "event: ") {
			eventType := strings.TrimPrefix(line, "event: ")

			// Track event sequence
			switch eventType {
			case "message_start":
				sawMessageStart = true
			case "content_block_start":
				sawContentBlockStart = true
			case "content_block_delta":
				sawDelta = true
			case "message_stop":
				sawMessageStop = true
			}

			now := time.Now()
			if !lastEventTime.IsZero() {
				// Events should arrive quickly (within 10s of each other)
				// This is a conservative check - buffering would cause longer delays
				if now.Sub(lastEventTime) > 10*time.Second {
					return fmt.Errorf("large gap between events: %v (possible buffering)", now.Sub(lastEventTime))
				}
			}

			lastEventTime = now
			eventCount++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	// Verify we got events
	if eventCount == 0 {
		return fmt.Errorf("no SSE events received")
	}

	// Verify event sequence
	if !sawMessageStart {
		return fmt.Errorf("missing message_start event")
	}

	if !sawContentBlockStart {
		return fmt.Errorf("missing content_block_start event")
	}

	if !sawDelta {
		return fmt.Errorf("missing content_block_delta event")
	}

	if !sawMessageStop {
		return fmt.Errorf("missing message_stop event")
	}

	return nil
}

func TestIntegration_ToolUseIdPreservation(t *testing.T) {
	server := setupTestProxy(t, "")

	// First request: Ask Claude to use a tool
	reqBody1 := map[string]interface{}{
		"model":      testModel,
		"max_tokens": 500,
		"tools": []map[string]interface{}{
			{
				"name":        "get_weather",
				"description": "Get the weather for a location",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]string{"type": "string"},
					},
					"required": []string{"location"},
				},
			},
		},
		"messages": []map[string]string{
			{"role": "user", "content": "What's the weather in San Francisco?"},
		},
	}

	bodyBytes1, _ := json.Marshal(reqBody1)

	req1, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("x-api-key", testProxyAPIKey)
	req1.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}

	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	defer resp1.Body.Close()

	// Parse response to get tool_use_id
	var response1 map[string]interface{}
	if err := json.NewDecoder(resp1.Body).Decode(&response1); err != nil {
		t.Fatalf("Failed to decode first response: %v", err)
	}

	// Extract tool_use_id from response
	content, ok := response1["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Skip("Response didn't include tool use (Claude didn't call tool)")
	}

	var toolUseID string

	for _, block := range content {
		blockMap, ok := block.(map[string]interface{})
		if !ok {
			continue
		}

		if blockMap["type"] == "tool_use" {
			toolUseID, _ = blockMap["id"].(string)
			break
		}
	}

	if toolUseID == "" {
		t.Skip("Response didn't include tool_use_id (Claude didn't call tool)")
	}

	// Second request: Provide tool result with tool_use_id
	reqBody2 := map[string]interface{}{
		"model":      testModel,
		"max_tokens": 500,
		"tools": []map[string]interface{}{
			{
				"name":        "get_weather",
				"description": "Get the weather for a location",
				"input_schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]string{"type": "string"},
					},
					"required": []string{"location"},
				},
			},
		},
		"messages": []interface{}{
			map[string]string{
				"role":    "user",
				"content": "What's the weather in San Francisco?",
			},
			response1, // Include previous assistant message
			map[string]interface{}{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": toolUseID,
						"content":     "Sunny, 72°F",
					},
				},
			},
		},
	}

	bodyBytes2, _ := json.Marshal(reqBody2)

	req2, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("x-api-key", testProxyAPIKey)
	req2.Header.Set("anthropic-version", "2023-06-01")

	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	defer resp2.Body.Close()

	// If we got here without errors, tool_use_id was preserved
	// (Anthropic API would return 400 if tool_use_id was modified/missing)
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Errorf("Second request failed with status %d: %s", resp2.StatusCode, string(body))
	}

	t.Logf("Successfully preserved tool_use_id: %s", toolUseID)
}

func TestIntegration_AuthenticationRejection(t *testing.T) {
	server := setupTestProxy(t, "")

	// Create request without API key
	reqBody := map[string]interface{}{
		"model":      testModel,
		"max_tokens": 10,
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	// Intentionally NOT setting x-api-key header

	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	defer resp.Body.Close()

	// Verify 401 response
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}

	// Verify error format matches Anthropic
	var errResp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if errResp.Type != "error" {
		t.Errorf("type = %q, want \"error\"", errResp.Type)
	}

	if errResp.Error.Type != "authentication_error" {
		t.Errorf("error.type = %q, want \"authentication_error\"", errResp.Error.Type)
	}

	if errResp.Error.Message == "" {
		t.Error("error.message is empty")
	}
}

func TestIntegration_HeaderForwarding(t *testing.T) {
	server := setupTestProxy(t, "")

	// Create request with anthropic headers
	reqBody := map[string]interface{}{
		"model":      testModel,
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": "Hi"},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", testProxyAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	defer resp.Body.Close()

	// If headers weren't forwarded correctly, the request would fail
	// Success indicates headers were forwarded properly
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to verify it's valid
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["type"] != "message" {
		t.Errorf("Expected type=message, got %v", response["type"])
	}
}

//nolint:gocognit // Integration test - table-driven test requires setup complexity
func TestIntegration_ErrorFormatCompliance(t *testing.T) {
	tests := []struct {
		setupFunc     func(t *testing.T) *httptest.Server
		requestFunc   func(serverURL string) (*http.Request, error)
		name          string
		wantErrorType string
		wantStatus    int
	}{
		{
			name: "401_missing_api_key",
			setupFunc: func(t *testing.T) *httptest.Server {
				return setupTestProxy(t, "")
			},
			requestFunc: func(serverURL string) (*http.Request, error) {
				reqBody := map[string]interface{}{
					"model":      testModel,
					"max_tokens": 10,
					"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
				}
				bodyBytes, _ := json.Marshal(reqBody)
				req, err := http.NewRequest("POST", serverURL+"/v1/messages", bytes.NewReader(bodyBytes))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/json")
				// No x-api-key header
				return req, nil
			},
			wantStatus:    http.StatusUnauthorized,
			wantErrorType: "authentication_error",
		},
		{
			name: "400_invalid_json",
			setupFunc: func(t *testing.T) *httptest.Server {
				return setupTestProxy(t, "")
			},
			requestFunc: func(serverURL string) (*http.Request, error) {
				req, err := http.NewRequest("POST", serverURL+"/v1/messages", strings.NewReader("not valid json"))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("x-api-key", testProxyAPIKey)
				req.Header.Set("anthropic-version", "2023-06-01")
				return req, nil
			},
			wantStatus:    http.StatusBadRequest,
			wantErrorType: "invalid_request_error",
		},
		{
			name: "502_upstream_failure",
			setupFunc: func(t *testing.T) *httptest.Server {
				// Use invalid backend URL to trigger 502
				providerKey := os.Getenv("ANTHROPIC_API_KEY")
				if providerKey == "" {
					t.Skip("ANTHROPIC_API_KEY not set")
				}

				cfg := &config.Config{
					Server: config.ServerConfig{
						Listen:    "127.0.0.1:0",
						APIKey:    testProxyAPIKey,
						TimeoutMS: 5000, // Short timeout
					},
				}

				// Create provider with unreachable backend
				provider := providers.NewAnthropicProvider("test", "http://localhost:1")

				handler, err := proxy.SetupRoutes(cfg, provider, providerKey)
				if err != nil {
					t.Fatalf("Failed to setup routes: %v", err)
				}

				server := httptest.NewServer(handler)
				t.Cleanup(server.Close)
				return server
			},
			requestFunc: func(serverURL string) (*http.Request, error) {
				reqBody := map[string]interface{}{
					"model":      testModel,
					"max_tokens": 10,
					"messages":   []map[string]string{{"role": "user", "content": "Hi"}},
				}
				bodyBytes, _ := json.Marshal(reqBody)
				req, err := http.NewRequest("POST", serverURL+"/v1/messages", bytes.NewReader(bodyBytes))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("x-api-key", testProxyAPIKey)
				req.Header.Set("anthropic-version", "2023-06-01")
				return req, nil
			},
			wantStatus:    http.StatusBadGateway,
			wantErrorType: "api_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupFunc(t)

			req, err := tt.requestFunc(server.URL)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			// Set short timeout for upstream failure test
			timeout := 30 * time.Second
			if tt.name == "502_upstream_failure" {
				timeout = 10 * time.Second
			}

			client := &http.Client{Timeout: timeout}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}

			defer resp.Body.Close()

			// Verify status code
			if resp.StatusCode != tt.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("status = %d, want %d. Body: %s", resp.StatusCode, tt.wantStatus, string(body))
			}

			// Verify error format
			var errResp struct {
				Type  string `json:"type"`
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
				t.Fatalf("Failed to parse error response: %v. Body: %s", err, string(bodyBytes))
			}

			if errResp.Type != "error" {
				t.Errorf("type = %q, want \"error\"", errResp.Type)
			}

			if errResp.Error.Type != tt.wantErrorType {
				t.Errorf("error.type = %q, want %q", errResp.Error.Type, tt.wantErrorType)
			}

			if errResp.Error.Message == "" {
				t.Error("error.message is empty")
			}
		})
	}
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	server := setupTestProxy(t, "")

	// Create health check request
	req, err := http.NewRequest("GET", server.URL+"/health", http.NoBody)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	defer resp.Body.Close()

	// Verify status
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	expectedBody := `{"status":"ok"}`
	if strings.TrimSpace(string(body)) != expectedBody {
		t.Errorf("body = %q, want %q", string(body), expectedBody)
	}
	// Health endpoint should not require authentication
	// (already verified by not setting x-api-key)
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	server := setupTestProxy(t, "")

	// Test that proxy can handle concurrent requests
	const numRequests = 5
	errChan := make(chan error, numRequests)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for i := 0; i < numRequests; i++ {
		go func(requestNum int) {
			reqBody := map[string]interface{}{
				"model":      testModel,
				"max_tokens": 20,
				"messages": []map[string]string{
					{"role": "user", "content": fmt.Sprintf("Say 'request %d'", requestNum)},
				},
			}

			bodyBytes, err := json.Marshal(reqBody)
			if err != nil {
				errChan <- fmt.Errorf("request %d: marshal failed: %w", requestNum, err)
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", server.URL+"/v1/messages", bytes.NewReader(bodyBytes))
			if err != nil {
				errChan <- fmt.Errorf("request %d: create request failed: %w", requestNum, err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-api-key", testProxyAPIKey)
			req.Header.Set("anthropic-version", "2023-06-01")

			client := &http.Client{Timeout: 30 * time.Second}

			resp, err := client.Do(req)
			if err != nil {
				errChan <- fmt.Errorf("request %d: do failed: %w", requestNum, err)
				return
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				errChan <- fmt.Errorf("request %d: status %d: %s", requestNum, resp.StatusCode, string(body))

				return
			}

			errChan <- nil
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		if err := <-errChan; err != nil {
			t.Error(err)
		}
	}
}
