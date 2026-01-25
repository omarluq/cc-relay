// Package providers defines the interface for LLM backend providers.
package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	// VertexOwner is the owner identifier for Vertex AI provider.
	VertexOwner = "google"

	// VertexAnthropicVersion is the anthropic_version for Vertex AI requests.
	// Must be in request body (not header) for Vertex AI.
	VertexAnthropicVersion = "vertex-2023-10-16"

	// vertexScope is the OAuth scope required for Vertex AI.
	vertexScope = "https://www.googleapis.com/auth/cloud-platform"
)

// DefaultVertexModels are the default models available from Vertex AI.
// Model IDs use Vertex format: model-name@version.
var DefaultVertexModels = []string{
	"claude-sonnet-4-5@20250514",
	"claude-opus-4-5@20250514",
	"claude-haiku-3-5@20241022",
}

// VertexProvider implements the Provider interface for Google Vertex AI.
// Vertex AI requires:
// - Model in URL path (not body)
// - anthropic_version in request body (not header)
// - OAuth Bearer token authentication
//
//nolint:govet // Field alignment optimized for readability over memory
type VertexProvider struct {
	BaseProvider
	projectID   string
	region      string
	tokenSource oauth2.TokenSource
	tokenMu     sync.RWMutex
}

// VertexConfig holds Vertex AI-specific configuration.
type VertexConfig struct {
	ModelMapping map[string]string
	Name         string
	ProjectID    string // GCP project ID
	Region       string // GCP region (e.g., "us-central1")
	Models       []string
}

// NewVertexProvider creates a new Vertex AI provider instance.
// Uses Google Application Default Credentials for authentication.
// Token refresh is handled automatically by the TokenSource.
func NewVertexProvider(ctx context.Context, cfg *VertexConfig) (*VertexProvider, error) {
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("vertex: project_id is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("vertex: region is required")
	}

	models := cfg.Models
	if len(models) == 0 {
		models = DefaultVertexModels
	}

	// Construct base URL for Vertex AI
	// Format: https://{region}-aiplatform.googleapis.com
	baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com", cfg.Region)

	// Get Google credentials with cloud-platform scope
	creds, err := google.FindDefaultCredentials(ctx, vertexScope)
	if err != nil {
		return nil, fmt.Errorf("vertex: failed to find credentials: %w", err)
	}

	return &VertexProvider{
		BaseProvider: NewBaseProviderWithMapping(cfg.Name, baseURL, VertexOwner, models, cfg.ModelMapping),
		projectID:    cfg.ProjectID,
		region:       cfg.Region,
		tokenSource:  creds.TokenSource,
	}, nil
}

// NewVertexProviderWithTokenSource creates a Vertex provider with a custom token source.
// Useful for testing or when using explicit credentials.
func NewVertexProviderWithTokenSource(cfg *VertexConfig, tokenSource oauth2.TokenSource) *VertexProvider {
	models := cfg.Models
	if len(models) == 0 {
		models = DefaultVertexModels
	}

	baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com", cfg.Region)

	return &VertexProvider{
		BaseProvider: NewBaseProviderWithMapping(cfg.Name, baseURL, VertexOwner, models, cfg.ModelMapping),
		projectID:    cfg.ProjectID,
		region:       cfg.Region,
		tokenSource:  tokenSource,
	}
}

// Authenticate adds OAuth Bearer token to the request.
// The key parameter is ignored - we use the TokenSource instead.
func (p *VertexProvider) Authenticate(req *http.Request, _ string) error {
	p.tokenMu.RLock()
	ts := p.tokenSource
	p.tokenMu.RUnlock()

	if ts == nil {
		return fmt.Errorf("vertex: no token source configured")
	}

	token, err := ts.Token()
	if err != nil {
		return fmt.Errorf("vertex: failed to get token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	log.Ctx(req.Context()).Debug().
		Str("provider", p.name).
		Bool("token_valid", token.Valid()).
		Time("token_expiry", token.Expiry).
		Msg("added Vertex AI OAuth authentication")

	return nil
}

// ForwardHeaders returns headers to forward to Vertex AI.
// Note: anthropic_version goes in body for Vertex, not header.
func (p *VertexProvider) ForwardHeaders(originalHeaders http.Header) http.Header {
	headers := p.BaseProvider.ForwardHeaders(originalHeaders)

	// Remove anthropic-version from headers (it goes in body for Vertex)
	headers.Del("Anthropic-Version")

	return headers
}

// TransformRequest transforms the request for Vertex AI:
// 1. Extracts model from body
// 2. Removes model from body
// 3. Adds anthropic_version to body
// 4. Constructs URL with model in path.
func (p *VertexProvider) TransformRequest(
	body []byte,
	_ string, // endpoint unused - streaming detected from body
) (newBody []byte, targetURL string, err error) {
	// Detect streaming from request body before transformation
	isStreaming := IsStreamingRequest(body)

	// Use shared transformation utility
	newBody, model, err := TransformBodyForCloudProvider(body, VertexAnthropicVersion)
	if err != nil {
		return nil, "", fmt.Errorf("vertex: transform failed: %w", err)
	}

	// Map model name to Vertex format if needed
	model = p.MapModel(model)

	// Construct Vertex AI URL with model in path
	// Format: /v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:streamRawPredict
	//     or: /v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:rawPredict
	action := "rawPredict"
	if isStreaming {
		action = "streamRawPredict"
	}

	targetURL = fmt.Sprintf("%s/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:%s",
		p.baseURL,
		url.PathEscape(p.projectID),
		url.PathEscape(p.region),
		url.PathEscape(model),
		action)

	return newBody, targetURL, nil
}

// RequiresBodyTransform returns true for Vertex AI.
// Model is removed from body and added to URL path.
func (p *VertexProvider) RequiresBodyTransform() bool {
	return true
}

// RefreshToken forces a token refresh. Useful before long streaming requests.
// This is a proactive refresh to avoid token expiration mid-stream.
func (p *VertexProvider) RefreshToken(ctx context.Context) error {
	p.tokenMu.RLock()
	ts := p.tokenSource
	p.tokenMu.RUnlock()

	if ts == nil {
		return fmt.Errorf("vertex: no token source configured")
	}

	// TokenSource automatically handles refresh, just request a new token
	token, err := ts.Token()
	if err != nil {
		return fmt.Errorf("vertex: refresh failed: %w", err)
	}

	log.Ctx(ctx).Debug().
		Time("new_expiry", token.Expiry).
		Dur("valid_for", time.Until(token.Expiry)).
		Msg("refreshed Vertex AI OAuth token")

	return nil
}

// GetProjectID returns the configured GCP project ID.
func (p *VertexProvider) GetProjectID() string {
	return p.projectID
}

// GetRegion returns the configured GCP region.
func (p *VertexProvider) GetRegion() string {
	return p.region
}
