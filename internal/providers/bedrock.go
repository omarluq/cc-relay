// Package providers implements AWS Bedrock provider for Claude models.
package providers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/rs/zerolog/log"
)

const (
	// BedrockOwner is the owner identifier for AWS Bedrock provider.
	BedrockOwner = "aws"

	// BedrockAnthropicVersion is the anthropic_version for Bedrock requests.
	// Must be in request body (not header) for Bedrock.
	BedrockAnthropicVersion = "bedrock-2023-05-31"

	// bedrockService is the AWS service name for signing.
	bedrockService = "bedrock"

	// ContentTypeEventStream is the Content-Type for Bedrock streaming responses.
	ContentTypeEventStream = "application/vnd.amazon.eventstream"
)

// DefaultBedrockModels are the default Claude models available on Bedrock.
// Model IDs use Bedrock format: anthropic.model-name-version.
var DefaultBedrockModels = []string{
	"anthropic.claude-sonnet-4-5-20250514-v1:0",
	"anthropic.claude-opus-4-5-20250514-v1:0",
	"anthropic.claude-haiku-3-5-20241022-v1:0",
}

// BedrockCredentialsProvider abstracts AWS credential retrieval for testing.
type BedrockCredentialsProvider interface {
	Retrieve(ctx context.Context) (aws.Credentials, error)
}

// BedrockProvider implements the Provider interface for AWS Bedrock.
// Bedrock requires:
// - Model in URL path (not body)
// - anthropic_version in request body (not header)
// - AWS SigV4 authentication
// - Event Stream response format (needs conversion to SSE).
type BedrockProvider struct {
	credentials BedrockCredentialsProvider
	signer      *v4.Signer
	region      string
	BaseProvider
}

// BedrockConfig holds Bedrock-specific configuration.
type BedrockConfig struct {
	ModelMapping map[string]string
	Name         string
	Region       string // AWS region (e.g., "us-east-1")
	Models       []string
}

// NewBedrockProvider creates a new Bedrock provider instance.
// Uses AWS SDK default credential chain for authentication.
func NewBedrockProvider(ctx context.Context, cfg *BedrockConfig) (*BedrockProvider, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("bedrock: region is required")
	}

	models := cfg.Models
	if len(models) == 0 {
		models = DefaultBedrockModels
	}

	// Load AWS config with default credential chain
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("bedrock: failed to load AWS config: %w", err)
	}

	// Construct base URL for Bedrock
	// Format: https://bedrock-runtime.{region}.amazonaws.com
	baseURL := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", cfg.Region)

	return &BedrockProvider{
		BaseProvider: NewBaseProviderWithMapping(
			cfg.Name,
			baseURL,
			BedrockOwner,
			models,
			cfg.ModelMapping,
		),
		region:      cfg.Region,
		credentials: awsCfg.Credentials,
		signer:      v4.NewSigner(),
	}, nil
}

// NewBedrockProviderWithCredentials creates a Bedrock provider with explicit credentials.
// Useful for testing or when using non-default credential providers.
func NewBedrockProviderWithCredentials(
	cfg *BedrockConfig,
	credentials BedrockCredentialsProvider,
) *BedrockProvider {
	models := cfg.Models
	if len(models) == 0 {
		models = DefaultBedrockModels
	}

	baseURL := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", cfg.Region)

	return &BedrockProvider{
		BaseProvider: NewBaseProviderWithMapping(
			cfg.Name,
			baseURL,
			BedrockOwner,
			models,
			cfg.ModelMapping,
		),
		region:      cfg.Region,
		credentials: credentials,
		signer:      v4.NewSigner(),
	}
}

// Authenticate adds AWS SigV4 authentication to the request.
// The key parameter is ignored - we use the credentials provider instead.
// IMPORTANT: This must be called AFTER the request body is set, as SigV4
// requires hashing the body.
func (p *BedrockProvider) Authenticate(req *http.Request, _ string) error {
	if p.credentials == nil {
		return fmt.Errorf("bedrock: no credentials provider configured")
	}

	ctx := req.Context()

	// Get credentials
	creds, err := p.credentials.Retrieve(ctx)
	if err != nil {
		return fmt.Errorf("bedrock: failed to retrieve credentials: %w", err)
	}

	// Read and buffer the body for hashing
	var bodyReader io.ReadSeeker
	var payloadHash string

	if req.Body != nil {
		bodyBytes, readErr := io.ReadAll(req.Body)
		if readErr != nil {
			return fmt.Errorf("bedrock: failed to read request body: %w", readErr)
		}
		// Replace body for later use
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		bodyReader = bytes.NewReader(bodyBytes)

		// Compute SHA256 hash of body
		hash := sha256.Sum256(bodyBytes)
		payloadHash = hex.EncodeToString(hash[:])
	} else {
		bodyReader = bytes.NewReader([]byte{})
		// Empty body hash
		hash := sha256.Sum256([]byte{})
		payloadHash = hex.EncodeToString(hash[:])
	}

	// Sign the request
	err = p.signer.SignHTTP(
		ctx,
		creds,
		req,
		payloadHash,
		bedrockService,
		p.region,
		time.Now(),
		func(options *v4.SignerOptions) {
			options.DisableURIPathEscaping = true
		},
	)
	if err != nil {
		return fmt.Errorf("bedrock: failed to sign request: %w", err)
	}

	// Reset body reader for the actual request
	if _, seekErr := bodyReader.Seek(0, io.SeekStart); seekErr != nil {
		return fmt.Errorf("bedrock: failed to reset body: %w", seekErr)
	}
	req.Body = io.NopCloser(bodyReader)

	log.Ctx(ctx).Debug().
		Str("provider", p.name).
		Str("region", p.region).
		Msg("added Bedrock SigV4 authentication")

	return nil
}

// ForwardHeaders returns headers to forward to Bedrock.
// Note: anthropic_version goes in body for Bedrock, not header.
func (p *BedrockProvider) ForwardHeaders(originalHeaders http.Header) http.Header {
	headers := p.BaseProvider.ForwardHeaders(originalHeaders)

	// Remove anthropic-version from headers (it goes in body for Bedrock)
	headers.Del("Anthropic-Version")

	return headers
}

// TransformRequest transforms the request for Bedrock:
// 1. Extracts model from body
// 2. Removes model from body
// 3. Adds anthropic_version to body
// 4. Constructs URL with model in path.
func (p *BedrockProvider) TransformRequest(
	body []byte,
	_ string,
) (newBody []byte, targetURL string, err error) {
	// Use shared transformation utility
	newBody, model, err := TransformBodyForCloudProvider(body, BedrockAnthropicVersion)
	if err != nil {
		return nil, "", fmt.Errorf("bedrock: transform failed: %w", err)
	}

	// Map model name to Bedrock format if needed
	model = p.MapModel(model)

	// Construct Bedrock URL with model in path
	// Format: /model/{model}/invoke-with-response-stream
	// The model ID needs URL encoding because it contains special characters (colons, etc.)
	targetURL = fmt.Sprintf(
		"%s/model/%s/invoke-with-response-stream",
		p.baseURL,
		url.PathEscape(model),
	)

	return newBody, targetURL, nil
}

// TransformResponse handles Bedrock's Event Stream response format.
// Converts Event Stream to SSE for Claude Code compatibility.
func (p *BedrockProvider) TransformResponse(resp *http.Response, w http.ResponseWriter) error {
	// Check if this is an Event Stream response
	if resp.Header.Get("Content-Type") != ContentTypeEventStream {
		// Not a streaming response, let standard proxy handle it
		return nil
	}

	// Convert Event Stream to SSE
	_, err := EventStreamToSSE(resp, w)
	if err != nil {
		return fmt.Errorf("bedrock: event stream conversion failed: %w", err)
	}

	return nil
}

// RequiresBodyTransform returns true for Bedrock.
// Model is removed from body and added to URL path.
func (p *BedrockProvider) RequiresBodyTransform() bool {
	return true
}

// StreamingContentType returns the Event Stream content type used by Bedrock.
func (p *BedrockProvider) StreamingContentType() string {
	return ContentTypeEventStream
}

// GetRegion returns the configured AWS region.
func (p *BedrockProvider) GetRegion() string {
	return p.region
}
