// Package providers provides shared transformation utilities for cloud providers.
package providers

import (
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ExtractModel extracts the model field from a JSON request body.
// Returns empty string if model field is not present.
func ExtractModel(body []byte) string {
	return gjson.GetBytes(body, "model").String()
}

// RemoveModelFromBody removes the model field from a JSON request body.
// Used by Bedrock/Vertex which put model in URL path, not body.
func RemoveModelFromBody(body []byte) ([]byte, error) {
	return sjson.DeleteBytes(body, "model")
}

// AddAnthropicVersion adds or updates the anthropic_version field in the request body.
// Bedrock uses "bedrock-2023-05-31", Vertex uses "vertex-2023-10-16".
func AddAnthropicVersion(body []byte, version string) ([]byte, error) {
	return sjson.SetBytes(body, "anthropic_version", version)
}

// TransformBodyForCloudProvider performs the standard transformation for cloud providers:
// 1. Extract model (for URL construction)
// 2. Remove model from body
// 3. Add anthropic_version to body
// Returns the modified body and the extracted model name.
func TransformBodyForCloudProvider(
	body []byte,
	anthropicVersion string,
) (newBody []byte, model string, err error) {
	model = ExtractModel(body)

	newBody, err = RemoveModelFromBody(body)
	if err != nil {
		return nil, "", err
	}

	newBody, err = AddAnthropicVersion(newBody, anthropicVersion)
	if err != nil {
		return nil, "", err
	}

	return newBody, model, nil
}
