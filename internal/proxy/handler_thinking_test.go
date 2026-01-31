package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/router"
)

const signatureJSONPath = "messages.0.content.0.signature"

func TestHandlerThinkingSignatureCacheHit(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	// Pre-populate cache with signature
	validSig := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz"
	thinkingText := "Let me think about this..."
	sigCache.Set(context.Background(), "claude-sonnet-4", thinkingText, validSig)
	time.Sleep(10 * time.Millisecond) // Wait for Ristretto

	backend, recorder := newRecordingBackend(t)

	provider := newTestProvider(backend.URL)
	handler := newHandlerWithSignatureCache(t, provider, sigCache)

	// Request with thinking block (no signature - should use cached)
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Let me think about this...", "signature": ""}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify cached signature was used in request to backend
	sig := gjson.GetBytes(recorder.Body(), signatureJSONPath).String()
	assert.Equal(t, validSig, sig, "should use cached signature")
}

func TestHandlerThinkingSignatureCacheMissClientSignature(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	backend, recorder := newRecordingBackend(t)

	provider := newTestProvider(backend.URL)
	handler := newHandlerWithSignatureCache(t, provider, sigCache)

	// Request with valid client signature
	clientSig := "client_signature_that_is_definitely_long_enough_for_validation"
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Some thinking...", "signature": "` + clientSig + `"}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify client signature was preserved
	sig := gjson.GetBytes(recorder.Body(), signatureJSONPath).String()
	assert.Equal(t, clientSig, sig, "should preserve valid client signature")
}

func TestHandlerThinkingSignatureUnsignedBlockDropped(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	backend, recorder := newRecordingBackend(t)

	provider := newTestProvider(backend.URL)
	handler := newHandlerWithSignatureCache(t, provider, sigCache)

	// Request with unsigned thinking block
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Some thinking...", "signature": ""},
				{"type": "text", "text": "Hello!"}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify thinking block was dropped
	content := gjson.GetBytes(recorder.Body(), "messages.0.content")
	assert.Equal(t, 1, len(content.Array()), "should have only 1 block (text)")
	assert.Equal(t, "text", content.Array()[0].Get("type").String())
}

func TestHandlerThinkingSignatureDropsEmptyAssistantMessage(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	backend, recorder := newRecordingBackend(t)

	provider := newTestProvider(backend.URL)
	handler := newHandlerWithSignatureCache(t, provider, sigCache)

	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "Hello"}]},
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Some thinking...", "signature": ""}
			]},
			{"role": "user", "content": [{"type": "text", "text": "Continue"}]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	messages := gjson.GetBytes(recorder.Body(), "messages").Array()
	assert.Len(t, messages, 2, "should drop assistant message with empty content")
	assert.Equal(t, "user", messages[0].Get("role").String())
	assert.Equal(t, "user", messages[1].Get("role").String())
}

func TestHandlerThinkingSignatureToolUseInheritance(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	backend, recorder := newRecordingBackend(t)

	provider := newTestProvider(backend.URL)
	handler := newHandlerWithSignatureCache(t, provider, sigCache)

	// Request with thinking block followed by tool_use
	thinkingSig := "thinking_signature_that_is_definitely_long_enough_for_validation"
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Analyzing...", "signature": "` + thinkingSig + `"},
				{"type": "tool_use", "id": "tool_1", "name": "search", "input": {}}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify tool_use does NOT have signature (API rejects tool_use with signature)
	toolBlock := gjson.GetBytes(recorder.Body(), "messages.0.content.1")
	assert.Equal(t, "tool_use", toolBlock.Get("type").String())
	assert.False(t, toolBlock.Get("signature").Exists(), "tool_use should not include signature")
}

func TestHandlerThinkingSignatureBlockReordering(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	backend, recorder := newRecordingBackend(t)

	provider := newTestProvider(backend.URL)
	handler := newHandlerWithSignatureCache(t, provider, sigCache)

	// Request with text before thinking (wrong order)
	sig := "valid_signature_that_is_definitely_long_enough_for_validation"
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "text", "text": "Hello"},
				{"type": "thinking", "thinking": "...", "signature": "` + sig + `"}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify blocks were reordered
	content := gjson.GetBytes(recorder.Body(), "messages.0.content")
	assert.Equal(t, 2, len(content.Array()))
	assert.Equal(t, "thinking", content.Array()[0].Get("type").String(), "thinking should be first")
	assert.Equal(t, "text", content.Array()[1].Get("type").String(), "text should be second")
}

func TestHandlerThinkingSignatureModelGroupSharing(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	// Cache signature with claude-sonnet-4
	validSig := "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz"
	thinkingText := "Shared thinking text"
	sigCache.Set(context.Background(), "claude-sonnet-4", thinkingText, validSig)
	time.Sleep(10 * time.Millisecond) // Wait for Ristretto

	backend, recorder := newRecordingBackend(t)

	provider := newTestProvider(backend.URL)
	handler := newHandlerWithSignatureCache(t, provider, sigCache)

	// Request with different model but same group
	body := `{
		"model": "claude-3-opus",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "Shared thinking text", "signature": ""}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify shared signature was used
	sig := gjson.GetBytes(recorder.Body(), signatureJSONPath).String()
	assert.Equal(t, validSig, sig, "should use signature from same model group")
}

func TestHandlerThinkingSignatureCrossProviderRouting(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := newTestSignatureCache(t)
	defer cleanup()

	// Track which providers were called
	var provider1Calls int
	var provider2Calls int

	// Create backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		provider1Calls++
		// Return response with signature (use printable chars, not null bytes)
		resp := map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{
					"type":      "thinking",
					"thinking":  "Provider 1 thinking",
					"signature": "sig1_abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz",
				},
			},
		}
		w.Header().Set(contentTypeHeader, jsonContentType)
		json.NewEncoder(w).Encode(resp)
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		provider2Calls++
		w.Header().Set(contentTypeHeader, jsonContentType)
		w.Write([]byte(`{"content": []}`))
	}))
	defer backend2.Close()

	// Create providers
	provider1 := newNamedProvider("provider1", backend1.URL)
	provider2 := newNamedProvider("provider2", backend2.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }},
		{Provider: provider2, IsHealthy: func() bool { return true }},
	}

	// Create round-robin router
	mockRouter := &roundRobinMock{providers: providerInfos, index: 0}

	handler, err := NewHandler(&HandlerOptions{
		Provider:       provider1,
		ProviderInfos:  providerInfos,
		ProviderRouter: mockRouter,
		APIKey:         "test-key",
		ProviderPools:  map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:   map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:   config.DebugOptions{},
		SignatureCache: sigCache,
	})
	require.NoError(t, err)

	// First request - should go to first provider
	body := `{"model": "claude-sonnet-4", "messages": [{"role": "user", "content": "Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.GreaterOrEqual(t, provider1Calls+provider2Calls, 1, "at least one provider should be called")
}

// roundRobinMock is a simple round-robin router for testing.
type roundRobinMock struct {
	providers []router.ProviderInfo
	index     int
}

func (r *roundRobinMock) Select(
	_ context.Context, candidates []router.ProviderInfo,
) (router.ProviderInfo, error) {
	if len(candidates) == 0 {
		return router.ProviderInfo{}, router.ErrNoProviders
	}
	result := candidates[r.index%len(candidates)]
	r.index++
	return result, nil
}

func (r *roundRobinMock) Name() string {
	return "round-robin"
}

func TestHandlerNoSignatureCachePassesThrough(t *testing.T) {
	t.Parallel()

	backend, recorder := newRecordingBackend(t)

	// Create handler without signature cache
	provider := newTestProvider(backend.URL)
	handler, err := NewHandler(&HandlerOptions{
		Provider:     provider,
		APIKey:       "test-key",
		DebugOptions: config.DebugOptions{},
		// nil signature cache
	})
	require.NoError(t, err)

	// Request with thinking block
	sig := "valid_signature_that_is_definitely_long_enough_for_validation"
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "` + sig + `"}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(contentTypeHeader, jsonContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Body should be passed through unchanged (no processing)
	receivedSig := gjson.GetBytes(recorder.Body(), signatureJSONPath).String()
	assert.Equal(t, sig, receivedSig, "signature should pass through unchanged")
}
