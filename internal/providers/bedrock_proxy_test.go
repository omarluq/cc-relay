package providers_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"testing"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/outbound"
	"github.com/omarluq/cc-relay/internal/providers"
)

const testBedrockProxy = "socks5://127.0.0.1:1080"

// testCABundle returns a self-signed CA certificate in PEM form.
func testCABundle(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cc-relay test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// TestBedrockAWSConfigWithProxyAndCABundle covers AWS_CA_BUNDLE (custom CA
// bundle) together with an outbound proxy: the SDK client must accept the
// bundle and still route credential requests through the proxy.
func TestBedrockAWSConfigWithProxyAndCABundle(t *testing.T) {
	t.Parallel()

	transport, err := outbound.NewRegistry().Transport(testBedrockProxy)
	require.NoError(t, err)

	cfg := testBedrockConfig(func(c *providers.BedrockConfig) { c.Transport = transport })
	awsCfg, err := providers.LoadBedrockAWSConfigForTest(t.Context(), cfg,
		config.WithCustomCABundle(bytes.NewReader(testCABundle(t))))
	require.NoError(t, err)

	client, isBuildable := awsCfg.HTTPClient.(*awshttp.BuildableClient)
	require.True(t, isBuildable, "SDK client must stay a BuildableClient so SDK options apply")

	sdkTransport := client.GetTransport()
	require.NotNil(t, sdkTransport.TLSClientConfig)
	require.NotNil(t, sdkTransport.TLSClientConfig.RootCAs, "custom CA bundle must be applied")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://sts.amazonaws.com/", http.NoBody)
	require.NoError(t, err)
	proxyURL, err := sdkTransport.Proxy(req)
	require.NoError(t, err)
	require.NotNil(t, proxyURL)
	assert.Equal(t, testBedrockProxy, proxyURL.String())

	imdsReq, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		"http://169.254.169.254/latest/api/token", http.NoBody)
	require.NoError(t, err)
	imdsProxy, err := sdkTransport.Proxy(imdsReq)
	require.NoError(t, err)
	assert.Nil(t, imdsProxy, "instance metadata must not go through the proxy")
}

func TestBedrockAWSConfigDirectDisablesSDKProxy(t *testing.T) {
	t.Parallel()

	transport, err := outbound.NewRegistry().Transport(outbound.Direct)
	require.NoError(t, err)

	cfg := testBedrockConfig(func(c *providers.BedrockConfig) { c.Transport = transport })
	awsCfg, err := providers.LoadBedrockAWSConfigForTest(t.Context(), cfg)
	require.NoError(t, err)

	client, isBuildable := awsCfg.HTTPClient.(*awshttp.BuildableClient)
	require.True(t, isBuildable)
	assert.Nil(t, client.GetTransport().Proxy, "direct must ignore environment proxies in the SDK client too")
}
