package config_test

import (
	"strings"
	"testing"

	"github.com/omarluq/cc-relay/internal/config"
)

const (
	testSocksProxy   = "socks5://127.0.0.1:1080"
	testHTTPProxy    = "http://corp-proxy:3128"
	testProxySecret  = "s3cret"
	testDirectProxy  = "direct"
	testOllamaName   = "local"
	testZAIName      = "zai"
	testProviderPath = "provider[" + testProviderName + "].outbound_proxy"
)

func TestLoadOutboundProxyYAML(t *testing.T) {
	t.Parallel()

	yamlContent := `server:
  listen: "` + defaultListenAddr + `"
  outbound_proxy: "` + testSocksProxy + `"

providers:
  - name: "` + testProviderType + `"
    type: "` + testProviderType + `"
    enabled: true
    outbound_proxy: "` + testHTTPProxy + `"
    keys:
      - key: "sk-ant-test"
  - name: "` + testOllamaName + `"
    type: "ollama"
    enabled: true
    outbound_proxy: "` + testDirectProxy + `"
  - name: "` + testZAIName + `"
    type: "zai"
    enabled: true
    keys:
      - key: "zai-key"
`

	cfg, err := config.LoadFromReaderForTest(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("config.LoadFromReader failed: %v", err)
	}

	if cfg.Server.OutboundProxy != testSocksProxy {
		t.Errorf("server.outbound_proxy = %q, want %q", cfg.Server.OutboundProxy, testSocksProxy)
	}

	want := map[string]string{
		testProviderType: testHTTPProxy,
		testOllamaName:   testDirectProxy,
		testZAIName:      testSocksProxy,
	}
	for idx := range cfg.Providers {
		provider := &cfg.Providers[idx]
		got := provider.GetEffectiveOutboundProxy(cfg.Server.OutboundProxy)
		if got != want[provider.Name] {
			t.Errorf("provider %s effective outbound proxy = %q, want %q", provider.Name, got, want[provider.Name])
		}
	}
}

func TestLoadOutboundProxyTOML(t *testing.T) {
	t.Parallel()

	tomlContent := `[server]
listen = "` + defaultListenAddr + `"
outbound_proxy = "` + testSocksProxy + `"

[[providers]]
name = "` + testProviderType + `"
type = "` + testProviderType + `"
enabled = true
outbound_proxy = "` + testDirectProxy + `"

[[providers.keys]]
key = "sk-ant-test"
`

	cfg, err := config.LoadFromReaderWithFormatForTest(strings.NewReader(tomlContent), config.TestFormatTOML)
	if err != nil {
		t.Fatalf("config.LoadFromReaderWithFormat failed: %v", err)
	}

	if cfg.Server.OutboundProxy != testSocksProxy {
		t.Errorf("server.outbound_proxy = %q, want %q", cfg.Server.OutboundProxy, testSocksProxy)
	}
	if got := cfg.Providers[0].OutboundProxy; got != testDirectProxy {
		t.Errorf("providers[0].outbound_proxy = %q, want %q", got, testDirectProxy)
	}
}

func outboundProxyConfig(serverProxy, providerProxy string) *config.Config {
	cfg := configWithSingleProvider(defaultListenAddr)
	cfg.Server.OutboundProxy = serverProxy
	cfg.Providers[0].OutboundProxy = providerProxy
	return cfg
}

func TestValidateOutboundProxyAccepts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serverProxy   string
		providerProxy string
	}{
		{name: "unset", serverProxy: "", providerProxy: ""},
		{name: "global socks", serverProxy: testSocksProxy, providerProxy: ""},
		{name: "provider override", serverProxy: testSocksProxy, providerProxy: testHTTPProxy},
		{name: "provider direct", serverProxy: "", providerProxy: testDirectProxy},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if err := outboundProxyConfig(testCase.serverProxy, testCase.providerProxy).Validate(); err != nil {
				t.Fatalf("expected valid config, got: %v", err)
			}
		})
	}
}

func TestValidateOutboundProxyRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		serverProxy   string
		providerProxy string
		wantErrField  string
	}{
		{
			name:          "invalid global scheme",
			serverProxy:   "ftp://alice:" + testProxySecret + "@proxy:21",
			providerProxy: "",
			wantErrField:  "server.outbound_proxy",
		},
		{
			name:          "invalid provider value",
			serverProxy:   "",
			providerProxy: "alice:" + testProxySecret + "@proxy:1080",
			wantErrField:  testProviderPath,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := outboundProxyConfig(testCase.serverProxy, testCase.providerProxy).Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), testCase.wantErrField) {
				t.Errorf("expected error for %s, got: %v", testCase.wantErrField, err)
			}
			if strings.Contains(err.Error(), testProxySecret) || strings.Contains(err.Error(), "alice") {
				t.Errorf("validation error leaks proxy credentials: %v", err)
			}
		})
	}
}
