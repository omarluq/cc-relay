//go:build integration

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
func setupTestProxy(t *testing.T) *httptest.Server {
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
	provider := providers.NewAnthropicProvider("test", "https://api.anthropic.com")

	// Setup routes
	handler, err := proxy.SetupRoutes(cfg, provider, providerKey, nil)
	if err != nil {
		t.Fatalf("Failed to setup routes: %v", err)
	}

	// Create test server
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server
}

func TestIntegrationNonStreamingRequest(t *testing.T) {
	t.Parallel()
	server := setupTestProxy(t)

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

func TestIntegrationStreamingRequest(t *testing.T) {
	t.Parallel()
	server := setupTestProxy(t)

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
func verifyStreamingBehavior(resp *http.Response) error {
	scanner := bufio.NewScanner(resp.Body)
	var (
		eventCount int
		lastEvent  time.Time
		flags      streamEventFlags
	)

	for scanner.Scan() {
		line := scanner.Text()
		if shouldSkipSSELine(line) {
			continue
		}
		eventType, ok := parseSSEEventType(line)
		if !ok {
			continue
		}
		flags.update(eventType)

		now := time.Now()
		if err := checkEventGap(lastEvent, now); err != nil {
			return err
		}
		lastEvent = now
		eventCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	if eventCount == 0 {
		return fmt.Errorf("no SSE events received")
	}
	if err := flags.validate(); err != nil {
		return err
	}
	return nil
}

type streamEventFlags struct {
	messageStart      bool
	contentBlockStart bool
	contentBlockDelta bool
	messageStop       bool
}

func (f *streamEventFlags) update(eventType string) {
	switch eventType {
	case "message_start":
		f.messageStart = true
	case "content_block_start":
		f.contentBlockStart = true
	case "content_block_delta":
		f.contentBlockDelta = true
	case "message_stop":
		f.messageStop = true
	}
}

func (f *streamEventFlags) validate() error {
	if f.messageStart && f.contentBlockStart && f.contentBlockDelta && f.messageStop {
		return nil
	}
	return fmt.Errorf("missing expected events: start=%v block_start=%v delta=%v stop=%v",
		f.messageStart, f.contentBlockStart, f.contentBlockDelta, f.messageStop)
}

func shouldSkipSSELine(line string) bool {
	return line == "" || strings.HasPrefix(line, ": ping")
}

func parseSSEEventType(line string) (string, bool) {
	if !strings.HasPrefix(line, "event: ") {
		return "", false
	}
	return strings.TrimPrefix(line, "event: "), true
}

func checkEventGap(last, now time.Time) error {
	if last.IsZero() {
		return nil
	}
	gap := now.Sub(last)
	if gap > 10*time.Second {
		return fmt.Errorf("large gap between events: %v (possible buffering)", gap)
	}
	return nil
}

func TestIntegrationToolUseIdPreservation(t *testing.T) {
	t.Parallel()
	server := setupTestProxy(t)

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

func TestIntegrationAuthenticationRejection(t *testing.T) {
	t.Parallel()
	server := setupTestProxy(t)

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

func TestIntegrationHeaderForwarding(t *testing.T) {
	t.Parallel()
	server := setupTestProxy(t)

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

func TestIntegrationErrorFormatCompliance(t *testing.T) {
	t.Parallel()
	for _, tt := range errorFormatCases() {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runErrorFormatCase(t, tt)
		})
	}
}

type errorFormatCase struct {
	name          string
	setupFunc     func(t *testing.T) *httptest.Server
	requestFunc   func(serverURL string) (*http.Request, error)
	wantStatus    int
	wantErrorType string
	timeout       time.Duration
}

func errorFormatCases() []errorFormatCase {
	return []errorFormatCase{
		{
			name: "401_missing_api_key",
			setupFunc: func(t *testing.T) *httptest.Server {
				return setupTestProxy(t)
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
				return req, nil
			},
			wantStatus:    http.StatusUnauthorized,
			wantErrorType: "authentication_error",
			timeout:       30 * time.Second,
		},
		{
			name: "400_invalid_json",
			setupFunc: func(t *testing.T) *httptest.Server {
				return setupTestProxy(t)
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
			timeout:       30 * time.Second,
		},
		{
			name: "502_upstream_failure",
			setupFunc: func(t *testing.T) *httptest.Server {
				providerKey := os.Getenv("ANTHROPIC_API_KEY")
				if providerKey == "" {
					t.Skip("ANTHROPIC_API_KEY not set")
				}

				cfg := &config.Config{
					Server: config.ServerConfig{
						Listen:    "127.0.0.1:0",
						APIKey:    testProxyAPIKey,
						TimeoutMS: 5000,
					},
				}

				provider := providers.NewAnthropicProvider("test", "http://localhost:1")

				handler, err := proxy.SetupRoutes(cfg, provider, providerKey, nil)
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
			timeout:       10 * time.Second,
		},
	}
}

func runErrorFormatCase(t *testing.T, tt errorFormatCase) {
	server := tt.setupFunc(t)

	req, err := tt.requestFunc(server.URL)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: tt.timeout}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("Failed to close response body: %v", closeErr)
		}
	}()

	bodyBytes, err := readResponseBody(resp)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if resp.StatusCode != tt.wantStatus {
		t.Fatalf("status = %d, want %d. Body: %s", resp.StatusCode, tt.wantStatus, string(bodyBytes))
	}

	errResp, err := parseErrorResponse(bodyBytes)
	if err != nil {
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
}

type errorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func parseErrorResponse(body []byte) (errorResponse, error) {
	var errResp errorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return errorResponse{}, err
	}
	return errResp, nil
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}

func TestIntegrationHealthEndpoint(t *testing.T) {
	t.Parallel()
	server := setupTestProxy(t)

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

func TestIntegrationConcurrentRequests(t *testing.T) {
	t.Parallel()
	server := setupTestProxy(t)

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
