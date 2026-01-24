# Phase 6: Cloud Providers - Research

**Researched:** 2026-01-24
**Domain:** Cloud LLM Provider Integration (AWS Bedrock, Azure Foundry, Google Vertex AI)
**Confidence:** HIGH

## Summary

This phase adds three cloud provider integrations: AWS Bedrock (with SigV4 signing), Azure Foundry (with API key or Entra ID authentication), and Google Vertex AI (with OAuth token authentication). Each provider requires different authentication mechanisms and has subtle API format differences from direct Anthropic API.

All three providers use the Anthropic Messages API format but differ in:
1. **Authentication:** AWS uses SigV4 signing, Azure uses x-api-key or Bearer tokens, Vertex uses OAuth2 access tokens
2. **Endpoint structure:** Bedrock and Vertex embed the model ID in the URL path; Azure uses model in body
3. **anthropic_version:** Bedrock uses `bedrock-2023-05-31`, Vertex uses `vertex-2023-10-16`, Azure uses `2023-06-01`

**Primary recommendation:** Create three new provider types (bedrock, vertex, foundry) that extend BaseProvider with custom authentication logic. Use official AWS SDK v2, Google Cloud OAuth2, and Azure Identity libraries for credential handling.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/aws/aws-sdk-go-v2/config` | Latest | AWS credential loading | Official AWS SDK, handles all credential chains |
| `github.com/aws/aws-sdk-go-v2/aws/signer/v4` | Latest | SigV4 request signing | Official signer, handles complexity correctly |
| `golang.org/x/oauth2/google` | Latest | Google ADC and token source | Official Google OAuth library for Go |
| `github.com/Azure/azure-sdk-for-go/sdk/azidentity` | Latest | Azure credential chain | Official Azure SDK, handles Entra ID + API key |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/aws/aws-sdk-go-v2/credentials` | Latest | Static credential provider | When using explicit access key/secret |
| `github.com/Azure/azure-sdk-for-go/sdk/azcore` | Latest | Azure core types | Required by azidentity |
| `crypto/sha256` | stdlib | Payload hashing for SigV4 | Required for every signed request |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| AWS SDK SigV4 | Manual SigV4 implementation | SDK handles edge cases, escaping, and streaming; manual is error-prone |
| Google ADC | Manual service account parsing | ADC handles all credential types automatically |
| Azure DefaultAzureCredential | Manual token refresh | SDK handles token caching and refresh automatically |

**Installation:**
```bash
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/aws/signer/v4
go get golang.org/x/oauth2/google
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity
```

## Architecture Patterns

### Recommended Project Structure

```
internal/providers/
├── provider.go      # Provider interface (exists)
├── base.go          # BaseProvider (exists)
├── anthropic.go     # Anthropic provider (exists)
├── zai.go           # Z.AI provider (exists)
├── ollama.go        # Ollama provider (exists)
├── bedrock.go       # NEW: AWS Bedrock provider
├── vertex.go        # NEW: Google Vertex AI provider
└── foundry.go       # NEW: Azure Foundry provider
```

### Pattern 1: Custom Authentication Override

**What:** Cloud providers override the `Authenticate` method to use their respective authentication mechanisms.
**When to use:** When provider uses non-standard authentication (not x-api-key header).
**Example:**
```go
// Source: AWS SDK Go v2 SigV4 signing pattern
func (p *BedrockProvider) Authenticate(req *http.Request, key string) error {
    ctx := req.Context()

    // Get credentials from AWS SDK
    creds, err := p.awsConfig.Credentials.Retrieve(ctx)
    if err != nil {
        return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
    }

    // Compute payload hash
    bodyBytes, _ := io.ReadAll(req.Body)
    req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
    payloadHash := sha256Hex(bodyBytes)

    // Sign request with SigV4
    err = p.signer.SignHTTP(ctx, creds, req, payloadHash, "bedrock", p.region, time.Now())
    if err != nil {
        return fmt.Errorf("failed to sign request: %w", err)
    }

    return nil
}
```

### Pattern 2: Model-in-URL Path Transformation

**What:** Bedrock and Vertex require model ID in the URL path, not request body.
**When to use:** Provider embeds model in endpoint URL.
**Example:**
```go
// Bedrock endpoint format
// https://bedrock-runtime.{region}.amazonaws.com/model/{model_id}/invoke

func (p *BedrockProvider) GetEndpoint(model string) string {
    return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke",
        p.region, url.PathEscape(model))
}

// Vertex endpoint format
// https://{region}-aiplatform.googleapis.com/v1/projects/{project}/locations/{region}/publishers/anthropic/models/{model}:streamRawPredict

func (p *VertexProvider) GetEndpoint(model string) string {
    return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/anthropic/models/%s:streamRawPredict",
        p.region, p.projectID, p.region, model)
}
```

### Pattern 3: Token Caching with Automatic Refresh

**What:** OAuth tokens expire (1 hour default); cache and refresh automatically.
**When to use:** Vertex AI OAuth authentication.
**Example:**
```go
// Source: golang.org/x/oauth2/google FindDefaultCredentials pattern
type VertexProvider struct {
    BaseProvider
    tokenSource oauth2.TokenSource
    projectID   string
    region      string
}

func (p *VertexProvider) Authenticate(req *http.Request, _ string) error {
    token, err := p.tokenSource.Token()
    if err != nil {
        return fmt.Errorf("failed to get OAuth token: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    return nil
}
```

### Pattern 4: Extended Config for Cloud Providers

**What:** ProviderConfig needs additional fields for cloud-specific settings.
**When to use:** Configuring cloud providers.
**Example:**
```go
// internal/config/config.go additions
type ProviderConfig struct {
    // Existing fields...

    // AWS Bedrock specific
    AWSRegion  string `yaml:"aws_region"`  // e.g., "us-west-2"
    AWSProfile string `yaml:"aws_profile"` // AWS config profile name

    // Google Vertex AI specific
    GCPProjectID string `yaml:"gcp_project_id"` // Google Cloud project
    GCPRegion    string `yaml:"gcp_region"`     // e.g., "us-east1" or "global"

    // Azure Foundry specific
    AzureResource string `yaml:"azure_resource"` // Azure resource name
}
```

### Anti-Patterns to Avoid

- **Manual SigV4 implementation:** Use AWS SDK; manual implementation is error-prone with URL escaping, date handling, and streaming.
- **Storing OAuth tokens in config:** Tokens expire; use token sources that auto-refresh.
- **Ignoring anthropic_version header:** Each provider requires a specific version string in requests.
- **Hardcoding endpoints:** Use configurable regions; each region has different endpoint URLs.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| AWS SigV4 signing | Manual signature calculation | `aws-sdk-go-v2/aws/signer/v4` | URL escaping, date format, header canonicalization are complex |
| Google OAuth tokens | Manual token refresh logic | `golang.org/x/oauth2/google.FindDefaultCredentials` | Handles ADC, token caching, refresh automatically |
| Azure credential chain | Manual credential loading | `azidentity.NewDefaultAzureCredential` | Handles environment, CLI, managed identity, workload identity |
| Payload hashing | Custom hash implementation | `crypto/sha256` | SigV4 requires exact SHA-256 hex encoding |
| Streaming with SigV4 | Custom streaming signer | AWS SDK `StreamSigner` | Streaming payloads require per-chunk signing |

**Key insight:** Cloud authentication is complex with many edge cases. Using official SDKs prevents security vulnerabilities and handles credential rotation automatically.

## Common Pitfalls

### Pitfall 1: SigV4 URL Escaping Mismatch

**What goes wrong:** SignatureDoesNotMatch errors from AWS.
**Why it happens:** Go's http.Client modifies URL paths; signed path differs from sent path.
**How to avoid:** Pre-escape URLs using `URL.Opaque` or `URL.RawPath` before signing.
**Warning signs:** Intermittent signature failures, especially with special characters in model IDs.

```go
// Correct: Set RawPath before signing
req.URL.RawPath = req.URL.EscapedPath()
```

### Pitfall 2: Credential Scope Region Mismatch

**What goes wrong:** AWS returns "Credential should be scoped to a valid Region".
**Why it happens:** Signing region doesn't match endpoint region.
**How to avoid:** Ensure `signer.SignHTTP(..., region)` matches the endpoint URL region.
**Warning signs:** Works in us-east-1, fails in other regions.

### Pitfall 3: OAuth Token Expiry Not Handled

**What goes wrong:** First hour works, then 401 Unauthorized errors.
**Why it happens:** OAuth tokens expire after 1 hour by default.
**How to avoid:** Use `oauth2.TokenSource` which auto-refreshes; don't cache tokens manually.
**Warning signs:** Works initially, fails after ~1 hour.

### Pitfall 4: Missing anthropic_version in Request Body

**What goes wrong:** Provider returns validation errors.
**Why it happens:** Bedrock and Vertex expect anthropic_version in body, not header.
**How to avoid:** Transform request body to add anthropic_version field.
**Warning signs:** "anthropic_version is required" errors.

```go
// Bedrock: Add to request body
body["anthropic_version"] = "bedrock-2023-05-31"

// Vertex: Add to request body
body["anthropic_version"] = "vertex-2023-10-16"
```

### Pitfall 5: Model ID Format Differences

**What goes wrong:** Model not found errors from cloud providers.
**Why it happens:** Cloud providers use different model ID formats than direct Anthropic API.
**How to avoid:** Use model_mapping in config or document required formats.
**Warning signs:** "Model ID not valid" or "Resource not found" errors.

```yaml
# Bedrock model IDs include version suffix
model_mapping:
  "claude-sonnet-4-5": "global.anthropic.claude-sonnet-4-5-20250929-v1:0"

# Vertex model IDs use @ for version
model_mapping:
  "claude-sonnet-4-5": "claude-sonnet-4-5@20250929"
```

### Pitfall 6: Azure Deployment Name vs Model ID

**What goes wrong:** Azure returns 404 for model requests.
**Why it happens:** Azure uses deployment names, not model IDs.
**How to avoid:** Configure deployment names in model_mapping.
**Warning signs:** "Deployment not found" errors; works in portal but not API.

## Code Examples

### AWS Bedrock Provider

```go
// Source: AWS SDK Go v2 documentation patterns
package providers

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "net/http"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
    "github.com/aws/aws-sdk-go-v2/config"
)

const (
    BedrockOwner = "bedrock"
    BedrockAnthropicVersion = "bedrock-2023-05-31"
)

type BedrockProvider struct {
    BaseProvider
    awsConfig aws.Config
    signer    *v4.Signer
    region    string
}

func NewBedrockProvider(name, region string, awsCfg aws.Config) *BedrockProvider {
    baseURL := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)
    return &BedrockProvider{
        BaseProvider: NewBaseProvider(name, baseURL, BedrockOwner, nil),
        awsConfig:    awsCfg,
        signer:       v4.NewSigner(),
        region:       region,
    }
}

func (p *BedrockProvider) Authenticate(req *http.Request, _ string) error {
    ctx := req.Context()

    creds, err := p.awsConfig.Credentials.Retrieve(ctx)
    if err != nil {
        return fmt.Errorf("failed to retrieve AWS credentials: %w", err)
    }

    // Read and hash payload
    var bodyBytes []byte
    if req.Body != nil {
        bodyBytes, _ = io.ReadAll(req.Body)
        req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
    }
    payloadHash := sha256Hex(bodyBytes)

    // Pre-escape URL path
    req.URL.RawPath = req.URL.EscapedPath()

    return p.signer.SignHTTP(ctx, creds, req, payloadHash, "bedrock", p.region, time.Now())
}

func (p *BedrockProvider) SupportsTransparentAuth() bool {
    return false // Uses AWS credentials, not client tokens
}

func sha256Hex(data []byte) string {
    h := sha256.Sum256(data)
    return hex.EncodeToString(h[:])
}
```

### Google Vertex AI Provider

```go
// Source: golang.org/x/oauth2/google documentation
package providers

import (
    "context"
    "fmt"
    "net/http"

    "golang.org/x/oauth2"
    "golang.org/x/oauth2/google"
)

const (
    VertexOwner = "vertex"
    VertexAnthropicVersion = "vertex-2023-10-16"
    VertexScope = "https://www.googleapis.com/auth/cloud-platform"
)

type VertexProvider struct {
    BaseProvider
    tokenSource oauth2.TokenSource
    projectID   string
    region      string
}

func NewVertexProvider(ctx context.Context, name, projectID, region string) (*VertexProvider, error) {
    creds, err := google.FindDefaultCredentials(ctx, VertexScope)
    if err != nil {
        return nil, fmt.Errorf("failed to find GCP credentials: %w", err)
    }

    baseURL := fmt.Sprintf("https://%s-aiplatform.googleapis.com", region)
    return &VertexProvider{
        BaseProvider: NewBaseProvider(name, baseURL, VertexOwner, nil),
        tokenSource:  creds.TokenSource,
        projectID:    projectID,
        region:       region,
    }, nil
}

func (p *VertexProvider) Authenticate(req *http.Request, _ string) error {
    token, err := p.tokenSource.Token()
    if err != nil {
        return fmt.Errorf("failed to get OAuth token: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    return nil
}

func (p *VertexProvider) SupportsTransparentAuth() bool {
    return false // Uses GCP credentials, not client tokens
}
```

### Azure Foundry Provider

```go
// Source: Azure SDK for Go azidentity documentation
package providers

import (
    "context"
    "fmt"
    "net/http"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
    "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
    FoundryOwner = "foundry"
    FoundryAnthropicVersion = "2023-06-01"
    AzureCognitiveScope = "https://cognitiveservices.azure.com/.default"
)

type FoundryProvider struct {
    BaseProvider
    credential *azidentity.DefaultAzureCredential
    resource   string
    useAPIKey  bool
    apiKey     string
}

func NewFoundryProvider(name, resource string) (*FoundryProvider, error) {
    cred, err := azidentity.NewDefaultAzureCredential(nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create Azure credential: %w", err)
    }

    baseURL := fmt.Sprintf("https://%s.services.ai.azure.com/anthropic", resource)
    return &FoundryProvider{
        BaseProvider: NewBaseProvider(name, baseURL, FoundryOwner, nil),
        credential:   cred,
        resource:     resource,
    }, nil
}

func NewFoundryProviderWithAPIKey(name, resource, apiKey string) *FoundryProvider {
    baseURL := fmt.Sprintf("https://%s.services.ai.azure.com/anthropic", resource)
    return &FoundryProvider{
        BaseProvider: NewBaseProvider(name, baseURL, FoundryOwner, nil),
        resource:     resource,
        useAPIKey:    true,
        apiKey:       apiKey,
    }
}

func (p *FoundryProvider) Authenticate(req *http.Request, key string) error {
    // Prefer configured API key, then passed key, then Entra ID
    if p.useAPIKey && p.apiKey != "" {
        req.Header.Set("x-api-key", p.apiKey)
        return nil
    }

    if key != "" {
        req.Header.Set("x-api-key", key)
        return nil
    }

    // Fall back to Entra ID
    ctx := req.Context()
    token, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
        Scopes: []string{AzureCognitiveScope},
    })
    if err != nil {
        return fmt.Errorf("failed to get Azure token: %w", err)
    }

    req.Header.Set("Authorization", "Bearer "+token.Token)
    return nil
}

func (p *FoundryProvider) SupportsTransparentAuth() bool {
    return false // Uses Azure credentials, not client tokens
}
```

### Configuration Examples

```yaml
# AWS Bedrock provider
- name: "bedrock"
  type: "bedrock"
  enabled: true
  aws_region: "us-west-2"
  aws_profile: "" # Uses default credential chain if empty

  models:
    - "global.anthropic.claude-sonnet-4-5-20250929-v1:0"
    - "global.anthropic.claude-opus-4-5-20251101-v1:0"

  model_mapping:
    "claude-sonnet-4-5": "global.anthropic.claude-sonnet-4-5-20250929-v1:0"
    "claude-opus-4-5": "global.anthropic.claude-opus-4-5-20251101-v1:0"

# Google Vertex AI provider
- name: "vertex"
  type: "vertex"
  enabled: true
  gcp_project_id: "my-gcp-project"
  gcp_region: "global" # or "us-east1" for regional endpoint

  models:
    - "claude-sonnet-4-5@20250929"
    - "claude-opus-4-5@20251101"

  model_mapping:
    "claude-sonnet-4-5": "claude-sonnet-4-5@20250929"
    "claude-opus-4-5": "claude-opus-4-5@20251101"

# Azure Foundry provider
- name: "foundry"
  type: "foundry"
  enabled: true
  azure_resource: "my-azure-resource"

  keys:
    - key: "${AZURE_FOUNDRY_API_KEY}" # Optional, falls back to Entra ID

  models:
    - "claude-sonnet-4-5"
    - "claude-opus-4-5"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Regional Bedrock endpoints only | Global endpoints with dynamic routing | Dec 2025 | Use `global.` prefix for maximum availability |
| Service account JSON files | Application Default Credentials (ADC) | Stable | Workload Identity preferred for production |
| Azure API key only | API key or Entra ID | Nov 2025 | DefaultAzureCredential handles both |
| Manual token refresh | OAuth2 TokenSource auto-refresh | Stable | Never cache tokens manually |

**Deprecated/outdated:**
- AWS SDK v1: Use aws-sdk-go-v2, not aws-sdk-go
- Manual AWS credential loading: Use config.LoadDefaultConfig
- Azure ADAL library: Replaced by azidentity SDK

## Open Questions

1. **Streaming with SigV4**
   - What we know: Standard SigV4 signs the full payload hash
   - What's unclear: How streaming requests work with Bedrock's invoke-with-response-stream
   - Recommendation: Test streaming; may need StreamSigner for SSE responses

2. **Vertex AI Provisioned Throughput**
   - What we know: Regional endpoints support provisioned throughput
   - What's unclear: Configuration requirements for reserved capacity
   - Recommendation: Document as advanced config; start with pay-as-you-go

3. **Azure Foundry Content Filters**
   - What we know: Azure requires manual content filter configuration
   - What's unclear: Whether missing filters cause request failures
   - Recommendation: Document Azure portal setup requirements

4. **Inference Profile ARNs for Bedrock**
   - What we know: Users can create inference profile ARNs for routing
   - What's unclear: Exact format and configuration in cc-relay
   - Recommendation: Support ARNs as model IDs; document format

## Sources

### Primary (HIGH confidence)
- [AWS SDK Go v2 SigV4 Signer](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/signer/v4) - SignHTTP method, parameters
- [AWS SigV4 Troubleshooting](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_sigv-troubleshooting.html) - Common errors and fixes
- [Google OAuth2 for Go](https://pkg.go.dev/golang.org/x/oauth2/google) - FindDefaultCredentials, TokenSource
- [Azure azidentity Package](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity) - DefaultAzureCredential
- [Claude on Amazon Bedrock](https://platform.claude.com/docs/en/api/claude-on-amazon-bedrock) - API format, model IDs
- [Claude on Vertex AI](https://platform.claude.com/docs/en/api/claude-on-vertex-ai) - API format, anthropic_version
- [Azure Foundry Claude](https://learn.microsoft.com/en-us/azure/ai-foundry/foundry-models/how-to/use-foundry-models-claude) - Endpoint format, auth headers

### Secondary (MEDIUM confidence)
- [AWS SDK Go v2 Config](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/config) - LoadDefaultConfig patterns
- [Vertex AI Authentication](https://docs.cloud.google.com/vertex-ai/docs/authentication) - ADC setup
- [Azure Identity Overview](https://learn.microsoft.com/en-us/azure/developer/go/sdk/authentication/authentication-overview) - Credential chain

### Tertiary (LOW confidence)
- Web search results for streaming and edge cases - Community patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official SDK documentation verified
- Architecture: HIGH - Patterns from existing Phase 5 BaseProvider + SDK docs
- Pitfalls: HIGH - Verified against official troubleshooting guides
- Code examples: MEDIUM - Based on SDK documentation, needs runtime validation

**Research date:** 2026-01-24
**Valid until:** 30 days (SDK versions may update; cloud provider APIs are stable)
