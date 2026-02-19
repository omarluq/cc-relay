package providers

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

// BuildEventStreamMessage re-exports ExportBuildEventStreamMessage for backward compatibility.
var BuildEventStreamMessage = ExportBuildEventStreamMessage
