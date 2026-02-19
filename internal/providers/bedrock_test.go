package providers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/omarluq/cc-relay/internal/providers"
)

// mockCredentialsProvider provides controllable AWS credentials for testing.
type mockCredentialsProvider struct {
	err   error
	creds aws.Credentials
}

func (m *mockCredentialsProvider) Retrieve(_ context.Context) (aws.Credentials, error) {
	if m.err != nil {
		return aws.Credentials{}, m.err
	}
	return m.creds, nil
}

func newMockCredentialsProvider(accessKey, secretKey string) *mockCredentialsProvider {
	return &mockCredentialsProvider{
		creds: aws.Credentials{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
		},
		err: nil,
	}
}

// assertBedrockPreservesBodyFields checks that body fields are preserved during
// request transformation.
func assertBedrockPreservesBodyFields(
	t *testing.T,
	result map[string]interface{},
	expectedVersion string,
) {
	t.Helper()

	const (
		expectedMaxTokens = float64(1024)
		expectedTemp      = 0.7
		expectedStream    = true
		expectedSystem    = "You are helpful"
	)

	if result["max_tokens"] != expectedMaxTokens {
		t.Errorf("Expected max_tokens=%v, got %v", expectedMaxTokens, result["max_tokens"])
	}
	if result["temperature"] != expectedTemp {
		t.Errorf("Expected temperature=%v, got %v", expectedTemp, result["temperature"])
	}
	if result["stream"] != expectedStream {
		t.Errorf("Expected stream=%v, got %v", expectedStream, result["stream"])
	}
	if result["system"] != expectedSystem {
		t.Errorf("Expected system=%v, got %v", expectedSystem, result["system"])
	}
	if result["messages"] == nil {
		t.Error("Expected messages to be set")
	}
	if result["anthropic_version"] != expectedVersion {
		t.Errorf(
			"Expected anthropic_version=%s, got %v",
			expectedVersion,
			result["anthropic_version"],
		)
	}
}

func TestNewBedrockProviderWithCredentials(t *testing.T) {
	t.Parallel()
	t.Run("creates provider with required config", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		assert.Equal(t, "test-bedrock", provider.Name())
		assert.Equal(t, "https://bedrock-runtime.us-east-1.amazonaws.com", provider.BaseURL())
		assert.Equal(t, providers.BedrockOwner, provider.Owner())
		assert.Equal(t, "us-east-1", provider.GetRegion())
	})

	t.Run("uses default models when none specified", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		models := provider.ListModels()
		assert.Len(t, models, len(providers.DefaultBedrockModels))
	})

	t.Run("uses custom models when specified", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       []string{"anthropic.claude-custom-v1:0"},
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		models := provider.ListModels()
		assert.Len(t, models, 1)
		assert.Equal(t, "anthropic.claude-custom-v1:0", models[0].ID)
	})
}

func TestNewBedrockProviderBaseURL(t *testing.T) {
	t.Parallel()
	t.Run("constructs correct base URL for different regions", func(t *testing.T) {
		t.Parallel()
		regions := []struct {
			region      string
			expectedURL string
		}{
			{"us-east-1", "https://bedrock-runtime.us-east-1.amazonaws.com"},
			{"us-west-2", "https://bedrock-runtime.us-west-2.amazonaws.com"},
			{"eu-west-1", "https://bedrock-runtime.eu-west-1.amazonaws.com"},
			{"ap-northeast-1", "https://bedrock-runtime.ap-northeast-1.amazonaws.com"},
		}

		for _, region := range regions {
			testCase := region
			t.Run(testCase.region, func(t *testing.T) {
				t.Parallel()
				cfg := &providers.BedrockConfig{
					ModelMapping: nil,
					Name:         "test-bedrock",
					Region:       testCase.region,
					Models:       nil,
				}
				creds := newMockCredentialsProvider("AKID", "SECRET")
				provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

				assert.Equal(t, testCase.expectedURL, provider.BaseURL())
			})
		}
	})
}

func TestBedrockProviderAuthenticateSigV4(t *testing.T) {
	t.Parallel()
	t.Run("adds SigV4 authorization header", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		body := []byte(`{"messages":[{"role":"user","content":"Hello"}],"max_tokens":100}`)
		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		err := provider.Authenticate(req, "") // key param ignored

		require.NoError(t, err)
		// Check that Authorization header was added with AWS4-HMAC-SHA256
		authHeader := req.Header.Get("Authorization")
		assert.Contains(t, authHeader, "AWS4-HMAC-SHA256")
		assert.Contains(t, authHeader, "Credential=AKIAIOSFODNN7EXAMPLE")
		assert.Contains(t, authHeader, "SignedHeaders=")
		assert.Contains(t, authHeader, "Signature=")
	})

	t.Run("adds X-Amz-Date header", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", bytes.NewReader([]byte(`{}`)))

		err := provider.Authenticate(req, "")

		require.NoError(t, err)
		assert.NotEmpty(t, req.Header.Get("X-Amz-Date"))
	})

	t.Run("preserves request body after signing", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		originalBody := []byte(`{"messages":[{"role":"user","content":"Test"}],"max_tokens":100}`)
		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", bytes.NewReader(originalBody))

		err := provider.Authenticate(req, "")

		require.NoError(t, err)

		// Body should still be readable
		bodyBytes, readErr := io.ReadAll(req.Body)
		require.NoError(t, readErr)
		assert.Equal(t, originalBody, bodyBytes)
	})
}

func TestBedrockProviderAuthenticateErrors(t *testing.T) {
	t.Parallel()
	t.Run("handles empty body", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", http.NoBody)

		err := provider.Authenticate(req, "")

		require.NoError(t, err)
		assert.Contains(t, req.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
	})

	t.Run("returns error when credentials fail", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := &mockCredentialsProvider{
			err:   errors.New("credential refresh failed"),
			creds: aws.Credentials{},
		}
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", http.NoBody)
		err := provider.Authenticate(req, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve credentials")
	})

	t.Run("returns error when no credentials configured", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		provider := providers.NewBedrockProviderWithCredentials(cfg, nil)

		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", http.NoBody)
		err := provider.Authenticate(req, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no credentials provider")
	})

	t.Run("ignores API key parameter", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", bytes.NewReader([]byte(`{}`)))
		err := provider.Authenticate(req, "ignored-api-key")

		require.NoError(t, err)
		// Should use SigV4, not x-api-key
		assert.Contains(t, req.Header.Get("Authorization"), "AWS4-HMAC-SHA256")
		assert.Empty(t, req.Header.Get("x-api-key"))
	})
}

func TestBedrockProviderForwardHeaders(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	t.Run("removes anthropic-version header", func(t *testing.T) {
		t.Parallel()
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Version", "2023-06-01")
		headers := provider.ForwardHeaders(origHeaders)

		// anthropic_version goes in body for Bedrock, not header
		assert.Empty(t, headers.Get("Anthropic-Version"))
	})

	t.Run("preserves other anthropic headers", func(t *testing.T) {
		t.Parallel()
		origHeaders := http.Header{}
		origHeaders.Set("Anthropic-Beta", "tools-2024-04-04")
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "tools-2024-04-04", headers.Get("Anthropic-Beta"))
	})

	t.Run("sets Content-Type", func(t *testing.T) {
		t.Parallel()
		origHeaders := http.Header{}
		headers := provider.ForwardHeaders(origHeaders)

		assert.Equal(t, "application/json", headers.Get("Content-Type"))
	})
}

func TestBedrockProviderTransformRequestBody(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	t.Run("removes model from body and adds anthropic_version", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"model":"anthropic.claude-sonnet-4-5-20250514-v1:0","messages":[]}`)

		newBody, _, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(newBody, &result)
		require.NoError(t, err)

		// Model should be removed
		_, hasModel := result["model"]
		assert.False(t, hasModel, "model should be removed from body")

		// anthropic_version should be added
		assert.Equal(t, providers.BedrockAnthropicVersion, result["anthropic_version"])
	})

	t.Run("preserves all other request body fields", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{
			"model": "anthropic.claude-sonnet-4-5-20250514-v1:0",
			"messages": [{"role": "user", "content": "Hello"}],
			"max_tokens": 1024,
			"temperature": 0.7,
			"stream": true,
			"system": "You are helpful"
		}`)

		newBody, _, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(newBody, &result)
		require.NoError(t, err)

		assertBedrockPreservesBodyFields(t, result, providers.BedrockAnthropicVersion)
	})
}

func TestBedrockProviderTransformRequestURL(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	t.Run("constructs correct URL with model in path", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"model":"anthropic.claude-sonnet-4-5-20250514-v1:0","messages":[]}`)

		_, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		// URL should have model in path (colon is valid in path segment)
		expected := "https://bedrock-runtime.us-east-1.amazonaws.com" +
			"/model/anthropic.claude-sonnet-4-5-20250514-v1:0/invoke-with-response-stream"
		assert.Equal(t, expected, targetURL)
	})

	t.Run("applies model mapping", func(t *testing.T) {
		t.Parallel()
		cfgWithMapping := &providers.BedrockConfig{
			ModelMapping: map[string]string{
				"claude-4": "anthropic.claude-sonnet-4-5-20250514-v1:0",
			},
			Name:   "test-bedrock",
			Region: "us-east-1",
			Models: nil,
		}
		providerWithMapping := providers.NewBedrockProviderWithCredentials(cfgWithMapping, creds)

		body := []byte(`{"model":"claude-4","messages":[]}`)
		_, targetURL, err := providerWithMapping.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		assert.Contains(t, targetURL, "anthropic.claude-sonnet-4-5-20250514-v1")
	})

	t.Run("handles special characters in model ID", func(t *testing.T) {
		t.Parallel()
		body := []byte(`{"model":"anthropic.model/version","messages":[]}`)

		_, targetURL, err := provider.TransformRequest(body, "/v1/messages")

		require.NoError(t, err)
		// Slash should be URL encoded as %2F
		assert.Contains(t, targetURL, "%2F")
	})
}

func TestBedrockProviderTransformResponse(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	t.Run("returns nil for non-event-stream response", func(t *testing.T) {
		t.Parallel()
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"result":"ok"}`))),
		}
		resp.Header.Set("Content-Type", "application/json")

		recorder := httptest.NewRecorder()

		err := provider.TransformResponse(resp, recorder)

		require.NoError(t, err)
		// Body should not have been consumed
		assert.Empty(t, recorder.Body.String())
	})

	t.Run("converts event stream to SSE", func(t *testing.T) {
		t.Parallel()

		msg := providers.ExportBuildEventStreamMessage(
			map[string]string{
				":event-type":   "message_start",
				":content-type": "application/json",
				":message-type": "event",
			},
			[]byte(`{"type":"message_start","message":{"id":"msg_123"}}`),
		)

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(msg)),
		}
		resp.Header.Set("Content-Type", providers.ContentTypeEventStream)

		recorder := httptest.NewRecorder()

		err := provider.TransformResponse(resp, recorder)

		require.NoError(t, err)

		// Check that SSE was produced
		body := recorder.Body.String()
		assert.Contains(t, body, "event: message_start")
		assert.Contains(t, body, "data: ")
	})
}

func TestBedrockProviderRequiresBodyTransform(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	assert.True(t, provider.RequiresBodyTransform())
}

func TestBedrockProviderSupportsStreaming(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	assert.True(t, provider.SupportsStreaming())
}

func TestBedrockProviderStreamingContentType(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	// Bedrock uses Event Stream format
	assert.Equal(t, providers.ContentTypeEventStream, provider.StreamingContentType())
}

func TestBedrockProviderModelMapping(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: map[string]string{
			"claude-4":     "anthropic.claude-sonnet-4-5-20250514-v1:0",
			"claude-opus":  "anthropic.claude-opus-4-5-20250514-v1:0",
			"claude-haiku": "anthropic.claude-haiku-3-5-20241022-v1:0",
		},
		Name:   "test-bedrock",
		Region: "us-east-1",
		Models: nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	t.Run("maps known model", func(t *testing.T) {
		t.Parallel()
		mapped := provider.MapModel("claude-4")
		assert.Equal(t, "anthropic.claude-sonnet-4-5-20250514-v1:0", mapped)
	})

	t.Run("returns original for unknown model", func(t *testing.T) {
		t.Parallel()
		mapped := provider.MapModel("unknown-model")
		assert.Equal(t, "unknown-model", mapped)
	})
}

func TestBedrockProviderInterfaceCompliance(t *testing.T) {
	t.Parallel()
	// Compile-time check that BedrockProvider implements Provider
	var _ providers.Provider = (*providers.BedrockProvider)(nil)
}

func TestBedrockProviderOwner(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	assert.Equal(t, "aws", provider.Owner())
}

func TestBedrockProviderSupportsTransparentAuth(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "us-east-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	// Bedrock uses SigV4, not client API keys
	assert.False(t, provider.SupportsTransparentAuth())
}

func TestBedrockProviderSigningDetails(t *testing.T) {
	t.Parallel()
	t.Run("signature covers request body", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		body1 := []byte(`{"messages":[{"role":"user","content":"Body 1"}]}`)
		body2 := []byte(`{"messages":[{"role":"user","content":"Body 2"}]}`)

		req1 := httptest.NewRequest(http.MethodPost, "/model/test/invoke", bytes.NewReader(body1))
		req2 := httptest.NewRequest(http.MethodPost, "/model/test/invoke", bytes.NewReader(body2))

		err1 := provider.Authenticate(req1, "")
		err2 := provider.Authenticate(req2, "")

		require.NoError(t, err1)
		require.NoError(t, err2)

		// Different bodies should produce different signatures
		sig1 := req1.Header.Get("Authorization")
		sig2 := req2.Header.Get("Authorization")
		assert.NotEqual(t, sig1, sig2)
	})

	t.Run("signed headers include host and content-type", func(t *testing.T) {
		t.Parallel()
		cfg := &providers.BedrockConfig{
			ModelMapping: nil,
			Name:         "test-bedrock",
			Region:       "us-east-1",
			Models:       nil,
		}
		creds := newMockCredentialsProvider("AKID", "SECRET")
		provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

		req := httptest.NewRequest(http.MethodPost, "/model/test/invoke", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")

		err := provider.Authenticate(req, "")

		require.NoError(t, err)

		authHeader := req.Header.Get("Authorization")
		// SignedHeaders should include host
		assert.Contains(t, authHeader, "SignedHeaders=")
		assert.Contains(t, authHeader, "host")
	})
}

func TestBedrockProviderGetRegion(t *testing.T) {
	t.Parallel()
	cfg := &providers.BedrockConfig{
		ModelMapping: nil,
		Name:         "test-bedrock",
		Region:       "eu-central-1",
		Models:       nil,
	}
	creds := newMockCredentialsProvider("AKID", "SECRET")
	provider := providers.NewBedrockProviderWithCredentials(cfg, creds)

	assert.Equal(t, "eu-central-1", provider.GetRegion())
}
