package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/providers"
)

const (
	anthropicVersion              = "2023-06-01"
	anthropicBaseURL              = "https://api.anthropic.com"
	anthropicVersion2024          = "2024-01-01"
	anthropicBetaExtendedThinking = "extended-thinking-2024-01-01"
	messagesPath                  = "/v1/messages"
	apiKeyHeader                  = "x-api-key"
)

type recordingBackend struct {
	body []byte
	mu   sync.Mutex
}

func (r *recordingBackend) Body() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.body == nil {
		return nil
	}
	return append([]byte(nil), r.body...)
}

func newRecordingBackend(t *testing.T) (*httptest.Server, *recordingBackend) {
	t.Helper()

	recorder := &recordingBackend{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		recorder.mu.Lock()
		recorder.body = body
		recorder.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content": []}`))
	}))
	t.Cleanup(server.Close)
	return server, recorder
}

func newBackendServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newJSONBackend(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if body != "" {
			_, _ = w.Write([]byte(body))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newMessagesRequest(body io.Reader) *http.Request {
	req := httptest.NewRequest("POST", messagesPath, body)
	req.Header.Set("anthropic-version", anthropicVersion)
	return req
}

type headerPair struct {
	key   string
	value string
}

func newMessagesRequestWithHeaders(body string, headers ...headerPair) *http.Request {
	req := newMessagesRequest(bytes.NewReader([]byte(body)))
	for _, header := range headers {
		req.Header.Set(header.key, header.value)
	}
	return req
}

func serveRequest(t *testing.T, handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func newTestProvider(url string) providers.Provider {
	return providers.NewAnthropicProvider("test", url)
}

func newNamedProvider(name, url string) providers.Provider {
	return providers.NewAnthropicProvider(name, url)
}

func newTestSignatureCache(t *testing.T) (sigCache *SignatureCache, cleanup func()) {
	t.Helper()
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e4,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}
	c, err := cache.New(context.Background(), &cfg)
	require.NoError(t, err)

	sigCache = NewSignatureCache(c)
	cleanup = func() { c.Close() }
	return sigCache, cleanup
}

func newHandlerWithSignatureCache(
	t *testing.T,
	provider providers.Provider,
	sigCache *SignatureCache,
) *Handler {
	t.Helper()
	handler, err := NewHandler(&HandlerOptions{
		Provider:       provider,
		APIKey:         "test-key",
		DebugOptions:   config.DebugOptions{},
		SignatureCache: sigCache,
	})
	require.NoError(t, err)
	return handler
}
