package providers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/omarluq/cc-relay/internal/providers"
)

// stubTransport is a distinct RoundTripper used to check transport plumbing.
type stubTransport struct{}

func (*stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, http.ErrNotSupported
}

func TestTransportForDefaultsToDefaultTransport(t *testing.T) {
	t.Parallel()

	provider := providers.NewAnthropicProvider("anthropic", "https://api.anthropic.com", nil, nil)
	assert.Nil(t, provider.OutboundTransport())
	assert.Same(t, http.DefaultTransport, providers.TransportFor(provider))
}

func TestTransportForUsesProviderTransport(t *testing.T) {
	t.Parallel()

	transport := &stubTransport{}
	provider := providers.NewZAIProvider("zai", "", nil, nil)
	provider.SetOutboundTransport(transport)
	assert.Same(t, transport, providers.TransportFor(provider))
}

func TestBedrockConfigTransportIsUsed(t *testing.T) {
	t.Parallel()

	transport := &stubTransport{}
	cfg := testBedrockConfig(func(c *providers.BedrockConfig) { c.Transport = transport })
	provider := providers.NewBedrockProviderWithCredentials(cfg, newMockCredentialsProvider("AKID", "SECRET"))
	assert.Same(t, transport, providers.TransportFor(provider))
}

func TestVertexConfigTransportIsUsed(t *testing.T) {
	t.Parallel()

	transport := &stubTransport{}
	cfg := newTestVertexConfig()
	cfg.Transport = transport
	provider := providers.NewVertexProviderWithTokenSource(cfg, newMockTokenSource("token"))
	assert.Same(t, transport, providers.TransportFor(provider))
}
