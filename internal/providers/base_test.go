package providers_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omarluq/cc-relay/internal/providers"
)

func TestBaseProviderTransformRequest(t *testing.T) {
	t.Parallel()

	baseURL := "https://api.example.com"
	provider := providers.NewBaseProviderForTest(baseURL, nil)

	body := []byte(`{"test": "data"}`)
	endpoint := "/v1/messages"

	newBody, targetURL, err := provider.TransformRequest(body, endpoint)
	if err != nil {
		t.Fatalf("TransformRequest() unexpected error: %v", err)
	}

	if targetURL != baseURL+endpoint {
		t.Errorf("TransformRequest() targetURL = %s, want %s", targetURL, baseURL+endpoint)
	}

	if !bytes.Equal(newBody, body) {
		t.Errorf("TransformRequest() body changed, expected unchanged")
	}
}

func TestBaseProviderTransformResponse(t *testing.T) {
	t.Parallel()

	provider := providers.NewBaseProviderForTest("", nil)

	resp := &http.Response{StatusCode: 200}
	w := httptest.NewRecorder()

	err := provider.TransformResponse(resp, w)
	if err != nil {
		t.Fatalf("TransformResponse() unexpected error: %v", err)
	}
}

func TestBaseProviderRequiresBodyTransform(t *testing.T) {
	t.Parallel()

	provider := providers.NewBaseProviderForTest("", nil)

	if provider.RequiresBodyTransform() {
		t.Error("RequiresBodyTransform() = true, want false for base provider")
	}
}
