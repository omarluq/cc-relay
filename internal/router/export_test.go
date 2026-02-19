package router

import (
	"net/http"

	"github.com/omarluq/cc-relay/internal/providers"
)

// testProvider is a minimal implementation of providers.Provider for testing.
type testProvider struct {
	name string
}

func (t *testProvider) Name() string                              { return t.name }
func (t *testProvider) BaseURL() string                           { return "http://test" }
func (t *testProvider) Owner() string                             { return "test" }
func (t *testProvider) Authenticate(_ *http.Request, _ string) error { return nil }
func (t *testProvider) ForwardHeaders(_ http.Header) http.Header  { return http.Header{} }
func (t *testProvider) SupportsStreaming() bool                   { return true }
func (t *testProvider) SupportsTransparentAuth() bool             { return false }
func (t *testProvider) ListModels() []providers.Model              { return nil }
func (t *testProvider) GetModelMapping() map[string]string        { return nil }
func (t *testProvider) MapModel(model string) string              { return model }
func (t *testProvider) TransformRequest(body []byte, endpoint string) (newBody []byte, targetURL string, err error) {
	return body, "http://test" + endpoint, nil
}
func (t *testProvider) TransformResponse(_ *http.Response, _ http.ResponseWriter) error { return nil }
func (t *testProvider) RequiresBodyTransform() bool                                     { return false }
func (t *testProvider) StreamingContentType() string {
	return "text/event-stream"
}

// NewTestProvider creates a test provider with the given name.
func NewTestProvider(name string) providers.Provider {
	return &testProvider{name: name}
}

// Export functions for testing in external test packages

// SortByPriority is the exported version of sortByPriority for testing.
func SortByPriority(providerInfos []ProviderInfo) []ProviderInfo {
	return sortByPriority(providerInfos)
}

// GetEffectiveWeight is the exported version of getEffectiveWeight for testing.
func GetEffectiveWeight(p ProviderInfo) int {
	return getEffectiveWeight(p)
}

// StringSliceEqual is the exported version of stringSliceEqual for testing.
func StringSliceEqual(a, b []string) bool {
	return stringSliceEqual(a, b)
}
