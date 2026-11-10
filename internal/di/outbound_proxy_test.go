package di_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/di"
	"github.com/omarluq/cc-relay/internal/providers"
)

const (
	testOutboundSocks = "socks5://alice:s3cret@127.0.0.1:1080"
	testOutboundHTTP  = "http://corp-proxy:3128"
	testZAIBaseURL    = "https://api.zai.example.com"
)

func outboundTestConfig(globalProxy, anthropicProxy, zaiProxy string) *config.Config {
	cfg := di.MustTestConfig()
	cfg.Server.OutboundProxy = globalProxy
	cfg.Providers = []config.ProviderConfig{
		di.MustTestProviderConfig("anthropic", "anthropic", anthropicBaseURL,
			[]config.KeyConfig{di.MustTestKeyConfig(testKey1)}),
		di.MustTestProviderConfig("zai", "zai", testZAIBaseURL,
			[]config.KeyConfig{di.MustTestKeyConfig("zai-key-1")}),
	}
	cfg.Providers[0].OutboundProxy = anthropicProxy
	cfg.Providers[1].OutboundProxy = zaiProxy
	return &cfg
}

// proxyFor returns the proxy URL the provider's transport would use, or "" for none.
func proxyFor(t *testing.T, svc *di.ProviderMapService, name string) (transport http.RoundTripper, proxy string) {
	t.Helper()

	prov, ok := svc.GetProvider(name)
	require.True(t, ok, "provider %s not found", name)

	transport = providers.TransportFor(prov)
	httpTransport, ok := transport.(*http.Transport)
	require.True(t, ok)
	if httpTransport.Proxy == nil {
		return transport, ""
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://api.anthropic.com", http.NoBody)
	require.NoError(t, err)
	proxyURL, err := httpTransport.Proxy(req)
	require.NoError(t, err)
	if proxyURL == nil {
		return transport, ""
	}
	return transport, proxyURL.String()
}

func TestProviderMapAppliesOutboundProxy(t *testing.T) {
	t.Parallel()

	cfg := outboundTestConfig(testOutboundSocks, "", "direct")
	svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceWithConfig(cfg))
	require.NoError(t, svc.RebuildFrom(cfg))

	_, anthropicProxy := proxyFor(t, svc, "anthropic")
	assert.Equal(t, testOutboundSocks, anthropicProxy, "provider without override inherits the global proxy")

	zaiTransport, zaiProxy := proxyFor(t, svc, "zai")
	assert.Empty(t, zaiProxy, "direct must bypass the global proxy")
	assert.NotSame(t, http.DefaultTransport, zaiTransport, "direct must not fall back to environment proxies")

	assert.Equal(t, 2, svc.OutboundTransportCount())
}

func TestProviderMapWithoutOutboundProxyKeepsDefaultTransport(t *testing.T) {
	t.Parallel()

	cfg := outboundTestConfig("", "", "")
	svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceWithConfig(cfg))
	require.NoError(t, svc.RebuildFrom(cfg))

	for _, name := range []string{"anthropic", "zai"} {
		prov, ok := svc.GetProvider(name)
		require.True(t, ok)
		assert.Same(t, http.DefaultTransport, providers.TransportFor(prov),
			"unset outbound_proxy must keep environment-based proxy behavior")
	}
	assert.Equal(t, 0, svc.OutboundTransportCount())
}

func TestProviderMapReloadSwapsOutboundProxy(t *testing.T) {
	t.Parallel()

	cfgA := outboundTestConfig(testOutboundSocks, "", "")
	svc := di.NewProviderMapServiceWithConfigService(di.NewConfigServiceWithConfig(cfgA))
	require.NoError(t, svc.RebuildFrom(cfgA))

	oldTransport, oldProxy := proxyFor(t, svc, "anthropic")
	require.Equal(t, testOutboundSocks, oldProxy)
	assert.Equal(t, 1, svc.OutboundTransportCount(), "providers with the same proxy share one transport")

	// Reload: the global proxy is removed and anthropic gets its own HTTP proxy.
	cfgB := outboundTestConfig("", testOutboundHTTP, "")
	require.NoError(t, svc.RebuildFrom(cfgB))

	newTransport, newProxy := proxyFor(t, svc, "anthropic")
	assert.Equal(t, testOutboundHTTP, newProxy)
	assert.NotSame(t, oldTransport, newTransport)

	zai, ok := svc.GetProvider("zai")
	require.True(t, ok)
	assert.Same(t, http.DefaultTransport, providers.TransportFor(zai))

	assert.Equal(t, 1, svc.OutboundTransportCount(), "the unused SOCKS transport must be released")
}
