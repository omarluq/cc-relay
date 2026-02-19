package proxy_test

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

	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/router"

	"github.com/omarluq/cc-relay/internal/proxy"
)

const signatureJSONPath = "messages.0.content.0.signature"

// testSignatureCacheScenario tests that a pre-cached signature is used in a request
// to the backend when the request thinking block has an empty signature.
func testSignatureCacheScenario(
	t *testing.T,
	cacheModel, requestModel, thinkingText, expectedSig, assertionMsg string,
) {
	t.Helper()

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	sigCache.Set(context.Background(), cacheModel, thinkingText, expectedSig)
	time.Sleep(10 * time.Millisecond) // Wait for Ristretto

	backend, recorder := proxy.NewRecordingBackend(t)

	provider := proxy.NewTestProvider(backend.URL)
	handler := proxy.NewHandlerWithSignatureCache(t, provider, sigCache)

	body := `{
		"model": "` + requestModel + `",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "` + thinkingText + `", "signature": ""}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
	responseRec := httptest.NewRecorder()

	handler.ServeHTTP(responseRec, req)

	assert.Equal(t, http.StatusOK, responseRec.Code)

	sig := gjson.GetBytes(recorder.Body(), signatureJSONPath).String()
	assert.Equal(t, expectedSig, sig, assertionMsg)
}

func TestHandlerThinkingSignatureCacheHit(t *testing.T) {
	t.Parallel()

	testSignatureCacheScenario(
		t,
		"claude-sonnet-4",            // cacheModel
		"claude-sonnet-4",            // requestModel
		"Let me think about this...", // thinkingText
		"abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz", // expectedSig
		"should use cached signature",                                    // assertionMsg
	)
}

func TestHandlerThinkingSignatureCacheMissClientSignature(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	backend, recorder := proxy.NewRecordingBackend(t)

	provider := proxy.NewTestProvider(backend.URL)
	handler := proxy.NewHandlerWithSignatureCache(t, provider, sigCache)

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
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Verify client signature was preserved
	sig := gjson.GetBytes(recorder.Body(), signatureJSONPath).String()
	assert.Equal(t, clientSig, sig, "should preserve valid client signature")
}

func TestHandlerThinkingSignatureUnsignedBlockDropped(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	backend, recorder := proxy.NewRecordingBackend(t)

	provider := proxy.NewTestProvider(backend.URL)
	handler := proxy.NewHandlerWithSignatureCache(t, provider, sigCache)

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
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
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

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	backend, recorder := proxy.NewRecordingBackend(t)

	provider := proxy.NewTestProvider(backend.URL)
	handler := proxy.NewHandlerWithSignatureCache(t, provider, sigCache)

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
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	messages := gjson.GetBytes(recorder.Body(), "messages").Array()
	assert.Len(t, messages, 3, "should keep assistant message with placeholder to prevent consecutive user messages")
	assert.Equal(t, "user", messages[0].Get("role").String())
	assert.Equal(t, "assistant", messages[1].Get("role").String())
	// Thinking block replaced with empty text placeholder
	content := messages[1].Get("content").Array()
	assert.Len(t, content, 1)
	assert.Equal(t, "text", content[0].Get("type").String())
	assert.Equal(t, "", content[0].Get("text").String())
	assert.Equal(t, "user", messages[2].Get("role").String())
}

func TestHandlerThinkingSignatureToolUseInheritance(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	backend, recorder := proxy.NewRecordingBackend(t)

	provider := proxy.NewTestProvider(backend.URL)
	handler := proxy.NewHandlerWithSignatureCache(t, provider, sigCache)

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
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
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

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	backend, recorder := proxy.NewRecordingBackend(t)

	provider := proxy.NewTestProvider(backend.URL)
	handler := proxy.NewHandlerWithSignatureCache(t, provider, sigCache)

	// Request with text before thinking (wrong order)
	sig := proxy.ValidTestSignature
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
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
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

	testSignatureCacheScenario(
		t,
		"claude-sonnet-4",      // cacheModel
		"claude-3-opus",        // requestModel (different group)
		"Shared thinking text", // thinkingText
		"abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz",
		"should use signature from same model group",
	)
}

func TestHandlerThinkingSignatureCrossProviderRouting(t *testing.T) {
	t.Parallel()

	sigCache, cleanup := proxy.NewTestSignatureCache(t)
	defer cleanup()

	// Track which providers were called
	var provider1Calls int
	var provider2Calls int

	// Create backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		provider1Calls++
		// Return response with signature (use printable chars, not null bytes)
		resp := map[string]any{
			"content": []any{
				map[string]any{
					"type":      "thinking",
					"thinking":  "Provider 1 thinking",
					"signature": "sig1_abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz",
				},
			},
		}
		writer.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		if err := json.NewEncoder(writer).Encode(resp); err != nil {
			return
		}
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		provider2Calls++
		w.Header().Set(proxy.ContentTypeHeader, proxy.JSONContentType)
		if _, err := w.Write([]byte(`{"content": []}`)); err != nil {
			return
		}
	}))
	defer backend2.Close()

	// Create providers
	provider1 := proxy.NewNamedProvider("provider1", backend1.URL)
	provider2 := proxy.NewNamedProvider("provider2", backend2.URL)

	providerInfos := []router.ProviderInfo{
		{Provider: provider1, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
		{Provider: provider2, IsHealthy: func() bool { return true }, Weight: 0, Priority: 0},
	}

	// Create round-robin router
	mockRouter := &roundRobinMock{providers: providerInfos, index: 0}

	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider1,
		ProviderInfos:     providerInfos,
		ProviderRouter:    mockRouter,
		APIKey:            "test-key",
		ProviderPools:     map[string]*keypool.KeyPool{"provider1": nil, "provider2": nil},
		ProviderKeys:      map[string]string{"provider1": "key1", "provider2": "key2"},
		DebugOptions:      proxy.TestDebugOptions(),
		SignatureCache:    sigCache,
		ProviderInfosFunc: nil,
		Pool:              nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	// First request - should go to first provider
	body := `{"model": "claude-sonnet-4", "messages": [{"role": "user", "content": "Hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
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

	backend, recorder := proxy.NewRecordingBackend(t)

	// Create handler without signature cache
	provider := proxy.NewTestProvider(backend.URL)
	handler, err := proxy.NewHandler(&proxy.HandlerOptions{
		Provider:          provider,
		ProviderRouter:    nil,
		ProviderPools:     nil,
		ProviderInfosFunc: nil,
		Pool:              nil,
		ProviderKeys:      nil,
		GetProviderPools:  nil,
		GetProviderKeys:   nil,
		RoutingConfig:     nil,
		HealthTracker:     nil,
		SignatureCache:    nil,
		APIKey:            "test-key",
		ProviderInfos:     nil,
		DebugOptions:      proxy.TestDebugOptions(),
		RoutingDebug:      false,
	})
	require.NoError(t, err)

	// Request with thinking block
	sig := proxy.ValidTestSignature
	body := `{
		"model": "claude-sonnet-4",
		"messages": [
			{"role": "assistant", "content": [
				{"type": "thinking", "thinking": "...", "signature": "` + sig + `"}
			]}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewBufferString(body))
	req.Header.Set(proxy.ContentTypeHeader, proxy.JSONContentType)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	// Body should be passed through unchanged (no processing)
	receivedSig := gjson.GetBytes(recorder.Body(), signatureJSONPath).String()
	assert.Equal(t, sig, receivedSig, "signature should pass through unchanged")
}
