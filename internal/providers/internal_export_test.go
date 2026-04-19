package providers

import "net/http"

// internal_export_test.go exports internal fields and functions for use by
// black-box tests in the providers_test package.

// AzureAPIVersion returns the Azure provider's API version for testing.
func (p *AzureProvider) AzureAPIVersion() string {
	return p.apiVersion
}

// AzureAuthMethod returns the Azure provider's auth method for testing.
func (p *AzureProvider) AzureAuthMethod() string {
	return p.authMethod
}

// AzureResourceName returns the Azure provider's resource name for testing.
func (p *AzureProvider) AzureResourceName() string {
	return p.resourceName
}

// AzureDeploymentID returns the Azure provider's deployment ID for testing.
func (p *AzureProvider) AzureDeploymentID() string {
	return p.deploymentID
}

// NewBaseProviderForTest creates a BaseProvider for testing.
func NewBaseProviderForTest(baseURL string, modelMapping map[string]string) *BaseProvider {
	return &BaseProvider{
		modelMapping: modelMapping,
		name:         "",
		baseURL:      baseURL,
		owner:        "",
		models:       nil,
	}
}

// ExportSkipNonStringHeader exports skipNonStringHeader for testing.
func ExportSkipNonStringHeader(
	data []byte, offset int, headerType byte, name string,
) (value *string, nextOffset int, err error) {
	return skipNonStringHeader(data, offset, headerType, name)
}

// ExportAdvanceOffset exports advanceOffset for testing.
func ExportAdvanceOffset(
	data []byte, offset int, length int, name string,
) (value *string, nextOffset int, err error) {
	return advanceOffset(data, offset, length, name)
}

// ExportWriteSSEEvent exports writeSSEEvent for testing.
func ExportWriteSSEEvent(w http.ResponseWriter, f http.Flusher, msg *EventStreamMessage) (bool, error) {
	return writeSSEEvent(w, f, msg)
}

// ExportWriteExceptionEvent exports writeExceptionEvent for testing.
func ExportWriteExceptionEvent(
	w http.ResponseWriter, f http.Flusher, exceptionType string, payload []byte,
) (bool, error) {
	return writeExceptionEvent(w, f, exceptionType, payload)
}
