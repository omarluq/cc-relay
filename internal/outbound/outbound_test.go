package outbound_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/outbound"
)

const (
	socksProxy     = "socks5://127.0.0.1:1080"
	socksWithCreds = "socks5://alice:s3cret@127.0.0.1:1080" //nolint:gosec // test fixture, not a real credential
	httpProxy      = "http://proxy.internal:3128"
	secretValue    = "s3cret"

	msgUnsupportedScheme = "unsupported or missing scheme"
	userValue            = "alice"
	msgMissingHost       = "missing host"
	testTargetURL        = "https://api.anthropic.com/v1/messages"
	ftpWithCreds         = "ftp://alice:" + secretValue + "@proxy:21"
	noSchemeWithCreds    = "alice:" + secretValue + "@proxy:1080"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		global   string
		provider string
		want     string
	}{
		{name: "both unset", global: "", provider: "", want: ""},
		{name: "global only", global: socksProxy, provider: "", want: socksProxy},
		{name: "provider wins", global: socksProxy, provider: httpProxy, want: httpProxy},
		{name: "provider direct wins", global: socksProxy, provider: outbound.Direct, want: outbound.Direct},
		{name: "whitespace provider falls back", global: socksProxy, provider: "  ", want: socksProxy},
		{name: "values are trimmed", global: " " + socksProxy + " ", provider: "", want: socksProxy},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.want, outbound.Resolve(testCase.global, testCase.provider))
		})
	}
}

func TestValidateAccepts(t *testing.T) {
	t.Parallel()

	for _, setting := range []string{
		"",
		outbound.Direct,
		"DIRECT",
		" direct ",
		httpProxy,
		"https://proxy.internal:443",
		socksProxy,
		"socks5h://127.0.0.1:1080",
		"SOCKS5://127.0.0.1:1080",
		socksWithCreds,
		"http://[::1]:3128",
	} {
		t.Run(setting, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, outbound.Validate(setting))
		})
	}
}

func TestValidateRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setting string
		wantMsg string
	}{
		{name: "unsupported scheme", setting: ftpWithCreds, wantMsg: msgUnsupportedScheme},
		{name: "host and port only", setting: "127.0.0.1:1080", wantMsg: "not a valid URL"},
		{name: "hostname and port only", setting: "localhost:1080", wantMsg: msgUnsupportedScheme},
		{name: "userinfo without scheme", setting: noSchemeWithCreds, wantMsg: msgUnsupportedScheme},
		{name: "missing host", setting: "socks5://alice:" + secretValue + "@:1080", wantMsg: msgMissingHost},
		{name: "scheme only", setting: "socks5://", wantMsg: msgMissingHost},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := outbound.Validate(testCase.setting)
			require.Error(t, err)
			assert.True(t, errors.Is(err, outbound.ErrInvalidProxy))
			assert.Contains(t, err.Error(), testCase.wantMsg)
			assert.NotContains(t, err.Error(), secretValue, "error must not leak the password")
			assert.NotContains(t, err.Error(), userValue, "error must not leak the username")
		})
	}
}

func TestRedact(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setting string
		want    string
	}{
		{name: "empty", setting: "", want: ""},
		{name: "direct", setting: outbound.Direct, want: outbound.Direct},
		{name: "no credentials", setting: socksProxy, want: socksProxy},
		{name: "user and password", setting: socksWithCreds, want: "socks5://xxxxx@127.0.0.1:1080"},
		{name: "user only", setting: "http://token@proxy.internal:3128", want: "http://xxxxx@proxy.internal:3128"},
		{name: "invalid value", setting: "alice:" + secretValue + "@proxy:1080", want: "[invalid proxy URL]"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := outbound.Redact(testCase.setting)
			assert.Equal(t, testCase.want, got)
			assert.NotContains(t, got, secretValue)
		})
	}
}

func TestRegistryUnsetUsesDefaultTransport(t *testing.T) {
	t.Parallel()

	registry := outbound.NewRegistry()
	transport, err := registry.Transport("")
	require.NoError(t, err)
	assert.Same(t, http.DefaultTransport, transport)
	assert.Equal(t, 0, registry.Len(), "the default transport is not owned by the registry")
}

func TestRegistryDirectBypassesAllProxies(t *testing.T) {
	t.Parallel()

	registry := outbound.NewRegistry()
	transport, err := registry.Transport("Direct")
	require.NoError(t, err)

	httpTransport, isHTTP := transport.(*http.Transport)
	require.True(t, isHTTP)
	assert.Nil(t, httpTransport.Proxy, "direct must ignore HTTP(S)_PROXY from the environment")
	assert.NotSame(t, http.DefaultTransport, transport)
}

func TestRegistryProxyTransportClonesDefaults(t *testing.T) {
	t.Parallel()

	registry := outbound.NewRegistry()
	transport, err := registry.Transport(socksWithCreds)
	require.NoError(t, err)

	httpTransport, isHTTP := transport.(*http.Transport)
	require.True(t, isHTTP)
	require.NotNil(t, httpTransport.Proxy)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testTargetURL, http.NoBody)
	require.NoError(t, err)
	proxyURL, err := httpTransport.Proxy(req)
	require.NoError(t, err)
	assert.Equal(t, socksWithCreds, proxyURL.String())

	defaults, isHTTP := http.DefaultTransport.(*http.Transport)
	require.True(t, isHTTP)
	assert.Equal(t, defaults.ForceAttemptHTTP2, httpTransport.ForceAttemptHTTP2)
	assert.Equal(t, defaults.MaxIdleConns, httpTransport.MaxIdleConns)
	assert.Equal(t, defaults.IdleConnTimeout, httpTransport.IdleConnTimeout)
	assert.Equal(t, defaults.TLSHandshakeTimeout, httpTransport.TLSHandshakeTimeout)
}

func TestRegistrySharesTransportsPerSetting(t *testing.T) {
	t.Parallel()

	registry := outbound.NewRegistry()

	first, err := registry.Transport(socksProxy)
	require.NoError(t, err)
	second, err := registry.Transport(" " + socksProxy + " ")
	require.NoError(t, err)
	assert.Same(t, first, second, "same setting must share one transport")

	directA, err := registry.Transport("direct")
	require.NoError(t, err)
	directB, err := registry.Transport("DIRECT")
	require.NoError(t, err)
	assert.Same(t, directA, directB)

	withCreds, err := registry.Transport(socksWithCreds)
	require.NoError(t, err)
	assert.NotSame(t, first, withCreds, "different credentials must not share a transport")

	assert.Equal(t, 3, registry.Len())
}

func TestRegistryRetainReleasesUnusedTransports(t *testing.T) {
	t.Parallel()

	registry := outbound.NewRegistry()
	oldTransport, err := registry.Transport(socksWithCreds)
	require.NoError(t, err)
	kept, err := registry.Transport(httpProxy)
	require.NoError(t, err)

	// Simulate a reload where the SOCKS credentials changed.
	rotated := strings.Replace(socksWithCreds, secretValue, "n3w", 1)
	newTransport, err := registry.Transport(rotated)
	require.NoError(t, err)
	registry.Retain([]string{httpProxy, rotated, ""})

	assert.Equal(t, 2, registry.Len())

	again, err := registry.Transport(httpProxy)
	require.NoError(t, err)
	assert.Same(t, kept, again)

	againRotated, err := registry.Transport(rotated)
	require.NoError(t, err)
	assert.Same(t, newTransport, againRotated)

	recreated, err := registry.Transport(socksWithCreds)
	require.NoError(t, err)
	assert.NotSame(t, oldTransport, recreated, "released transport must not be reused")
}

func TestRegistryRejectsInvalidSetting(t *testing.T) {
	t.Parallel()

	registry := outbound.NewRegistry()
	_, err := registry.Transport(ftpWithCreds)
	require.ErrorIs(t, err, outbound.ErrInvalidProxy)
	assert.NotContains(t, err.Error(), secretValue)
	assert.Equal(t, 0, registry.Len())
}

func TestIsLocalHost(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"localhost":            true,
		"LOCALHOST":            true,
		"ollama.localhost":     true,
		"127.0.0.1":            true,
		"::1":                  true,
		"169.254.169.254":      true, // AWS IMDS
		"169.254.170.2":        true, // ECS task credentials
		"fe80::1":              true,
		"api.anthropic.com":    false,
		"10.0.0.5":             false,
		"192.168.1.10":         false,
		"localhost.example.io": false,
	}

	for host, want := range tests {
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, want, outbound.IsLocalHost(host))
		})
	}
}

func TestProxyTransportSkipsLocalDestinations(t *testing.T) {
	t.Parallel()

	registry := outbound.NewRegistry()
	transport, err := registry.Transport(socksProxy)
	require.NoError(t, err)
	httpTransport, isHTTP := transport.(*http.Transport)
	require.True(t, isHTTP)

	for _, target := range []string{
		"http://169.254.169.254/latest/api/token",
		"http://localhost:11434/v1/messages",
		"http://127.0.0.1:8080/",
	} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
		require.NoError(t, err)
		proxyURL, err := httpTransport.Proxy(req)
		require.NoError(t, err)
		assert.Nil(t, proxyURL, "%s must connect directly", target)
	}
}
