package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
)

// countingTransport records how many requests went through it.
type countingTransport struct {
	next  http.RoundTripper
	calls atomic.Int32
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return c.next.RoundTrip(req)
}

func TestProviderProxyUsesProviderOutboundTransport(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(writer, `{"ok":true}`); err != nil {
			return
		}
	}))
	t.Cleanup(upstream.Close)

	transport := &countingTransport{next: http.DefaultTransport, calls: atomic.Int32{}}
	provider := providers.NewAnthropicProvider(testValueGeneric, upstream.URL, nil, nil)
	provider.SetOutboundTransport(transport)

	providerProxy, err := proxy.NewProviderProxy(provider, "test-key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/messages",
		strings.NewReader(`{"model":"claude"}`))
	recorder := httptest.NewRecorder()
	providerProxy.Proxy.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, int32(1), transport.calls.Load(), "request must go through the provider's outbound transport")
}

func TestProviderProxyDefaultsToDefaultTransport(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider(testValueGeneric, "https://api.anthropic.com", nil, nil)

	providerProxy, err := proxy.NewProviderProxy(provider, "test-key", nil, proxy.TestDebugOptions(), nil)
	require.NoError(t, err)
	assert.Same(t, http.DefaultTransport, providerProxy.Proxy.Transport)
}
