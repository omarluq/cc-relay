# Phase 6: Cloud Providers - Research

**Researched:** 2026-01-24
**Domain:** Cloud LLM Provider Integration (AWS Bedrock, Google Vertex AI, Azure Foundry)
**Confidence:** HIGH

## Summary

This research covers the API specifics, authentication methods, and request/response transformations required to integrate AWS Bedrock, Google Vertex AI, and Azure Foundry (Microsoft Foundry) as cloud providers in cc-relay.

All three cloud providers use Anthropic-compatible APIs with specific differences:
- **Bedrock/Vertex**: Model specified in URL path (not body), custom `anthropic_version` in body
- **Azure Foundry**: Standard Anthropic API format, model stays in body

The most significant implementation challenge is AWS Bedrock's streaming format, which uses AWS Event Stream rather than standard SSE and requires conversion.

**Primary recommendation:** Implement Azure Foundry first (minimal transformation), then Vertex AI (URL transformation + OAuth), then Bedrock (most complex with SigV4 and Event Stream conversion).

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/aws/aws-sdk-go-v2` | v1.25+ | AWS Bedrock SDK | Official AWS SDK for Go |
| `github.com/aws/aws-sdk-go-v2/service/bedrockruntime` | Latest | Bedrock Runtime API | Official Bedrock service package |
| `github.com/aws/aws-sdk-go-v2/aws/signer/v4` | Latest | SigV4 signing | Official AWS SigV4 implementation |
| `golang.org/x/oauth2/google` | Latest | Google OAuth | Official Google auth library |
| `cloud.google.com/go/compute/metadata` | Latest | GCE metadata | For service account in GCP |
| `github.com/Azure/azure-sdk-for-go/sdk/azidentity` | Latest | Azure Entra ID | Official Azure identity library |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/tidwall/gjson` | v1.17+ | JSON parsing | Extract model from request body |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| AWS SDK v2 | Manual SigV4 | More control but complex; SDK handles edge cases |
| Azure SDK | Manual HTTP + token fetch | Simpler but no token refresh |
| Google OAuth lib | Manual token refresh | SDK handles refresh automatically |

**Installation:**
```bash
# AWS Bedrock
go get github.com/aws/aws-sdk-go-v2
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/bedrockruntime

# Google Vertex AI
go get golang.org/x/oauth2/google
go get cloud.google.com/go/compute/metadata

# Azure Foundry
go get github.com/Azure/azure-sdk-for-go/sdk/azidentity
```

## Architecture Patterns

### Recommended Project Structure
```
internal/providers/
├── provider.go           # Interface definition (extended)
├── base.go               # BaseProvider implementation
├── anthropic.go          # Direct Anthropic (existing)
├── zai.go                # Z.AI (existing)
├── ollama.go             # Ollama (existing)
├── bedrock.go            # AWS Bedrock (new)
├── vertex.go             # Google Vertex AI (new)
├── azure.go              # Azure Foundry (new)
└── transform.go          # Shared transformation utilities
```

### Pattern 1: Extended Provider Interface

**What:** Add request/response transformation methods to Provider interface
**When to use:** Cloud providers that need URL or body modification

```go
// Source: Derived from existing provider.go structure
type Provider interface {
    // Existing methods
    Name() string
    BaseURL() string
    Owner() string
    Authenticate(req *http.Request, key string) error
    ForwardHeaders(originalHeaders http.Header) http.Header
    SupportsStreaming() bool
    SupportsTransparentAuth() bool
    ListModels() []Model
    GetModelMapping() map[string]string
    MapModel(model string) string

    // NEW: Cloud provider transformations
    // TransformRequest modifies request body and returns target URL
    // For non-cloud providers, returns body unchanged and base URL + endpoint
    TransformRequest(body []byte, isStreaming bool) (newBody []byte, targetURL string, err error)

    // RequiresURLTransform indicates if this provider needs model-in-URL
    RequiresURLTransform() bool
}
```

### Pattern 2: Cloud Provider Embedding

**What:** Cloud providers embed BaseProvider and add cloud-specific fields
**When to use:** All cloud provider implementations

```go
// Source: Pattern from existing anthropic.go
type BedrockProvider struct {
    BaseProvider
    region      string
    awsConfig   aws.Config
    signer      *v4.Signer
    authMethod  string // "sigv4" or "bearer_token"
    bearerToken string
}

type VertexProvider struct {
    BaseProvider
    projectID   string
    region      string
    tokenSource oauth2.TokenSource
}

type AzureProvider struct {
    BaseProvider
    resourceName string
    authMethod   string // "api_key" or "entra_id"
    credential   *azidentity.DefaultAzureCredential
}
```

### Pattern 3: Streaming Format Conversion (Bedrock only)

**What:** Convert AWS Event Stream to SSE
**When to use:** Only for AWS Bedrock streaming responses

```go
// Bedrock returns application/vnd.amazon.eventstream
// Must convert to text/event-stream for Claude Code compatibility
func (p *BedrockProvider) ConvertStreamToSSE(eventStream io.Reader, w http.ResponseWriter) error {
    // Set SSE headers
    SetSSEHeaders(w.Header())

    for event := range p.parseEventStream(eventStream) {
        // Convert each event to SSE format
        fmt.Fprintf(w, "event: %s\n", event.Type)
        fmt.Fprintf(w, "data: %s\n\n", event.Data)
        if f, ok := w.(http.Flusher); ok {
            f.Flush()
        }
    }
    return nil
}
```

### Anti-Patterns to Avoid
- **Hardcoding cloud credentials:** Always use SDK credential chains or environment variables
- **Ignoring token expiration:** OAuth/Entra tokens expire; use SDK token sources that auto-refresh
- **Blocking on stream conversion:** Process Event Stream chunks as they arrive, don't buffer entire response
- **Assuming SSE format for Bedrock:** Bedrock uses Event Stream, not SSE

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| AWS SigV4 signing | Custom HMAC-SHA256 implementation | `aws-sdk-go-v2/aws/signer/v4` | SigV4 has many edge cases (presigned URLs, session tokens, canonical request format) |
| Google OAuth token | Manual token fetch/refresh | `golang.org/x/oauth2/google` | Token refresh, caching, and error handling is complex |
| Azure Entra ID tokens | Manual MSAL implementation | `azidentity.DefaultAzureCredential` | Handles workload identity, managed identity, CLI auth |
| AWS Event Stream parsing | Custom binary parser | AWS SDK event stream decoder | Binary format with checksums, error handling |

**Key insight:** Cloud authentication is deceptively complex. Token refresh, credential rotation, and error handling have many edge cases that SDKs handle correctly.

## Common Pitfalls

### Pitfall 1: Model ID Format Mismatch

**What goes wrong:** Sending Anthropic model IDs to Bedrock/Vertex without transformation
**Why it happens:** Different providers use different model ID formats
**How to avoid:** Always map model IDs before constructing URLs

```go
// Mapping examples
anthropic:  "claude-sonnet-4-5-20250514"
bedrock:    "anthropic.claude-sonnet-4-5-20250514-v1:0"
vertex:     "claude-sonnet-4-5@20250514"
azure:      "claude-sonnet-4-5" (deployment name)
```

**Warning signs:** 404 errors, "model not found" responses

### Pitfall 2: anthropic_version Placement

**What goes wrong:** Putting anthropic_version in wrong location
**Why it happens:** Different from direct Anthropic API (header vs body)
**How to avoid:**

| Provider | anthropic_version Location | Value |
|----------|---------------------------|-------|
| Anthropic Direct | Header | `2023-06-01` |
| Bedrock | Request Body | `bedrock-2023-05-31` |
| Vertex AI | Request Body | `vertex-2023-10-16` |
| Azure Foundry | Header | `2023-06-01` |

**Warning signs:** 400 Bad Request, validation errors

### Pitfall 3: Bedrock Streaming Format

**What goes wrong:** Treating Bedrock stream as SSE
**Why it happens:** Other providers use SSE, natural assumption
**How to avoid:** Check Content-Type header and use appropriate parser

```go
// Bedrock returns:
Content-Type: application/vnd.amazon.eventstream

// Not:
Content-Type: text/event-stream
```

**Warning signs:** Garbled streaming output, JSON parse errors

### Pitfall 4: OAuth Token Expiration

**What goes wrong:** Stale tokens during long streaming requests
**Why it happens:** Tokens expire (typically 1 hour), streaming can exceed this
**How to avoid:** Use SDK TokenSource that auto-refreshes, or refresh before streaming

**Warning signs:** 401 errors mid-stream

### Pitfall 5: Missing Rate Limit Headers

**What goes wrong:** Expecting Anthropic rate limit headers from Azure Foundry
**Why it happens:** Azure doesn't pass through these headers
**How to avoid:** Use Azure monitoring for rate limits, or implement client-side tracking

**Warning signs:** Rate limiting without warning, no header data

## Code Examples

Verified patterns from official sources:

### AWS Bedrock SigV4 Signing
```go
// Source: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/signer/v4
import (
    "context"
    "crypto/sha256"
    "encoding/hex"
    "net/http"
    "time"

    "github.com/aws/aws-sdk-go-v2/aws"
    v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
    "github.com/aws/aws-sdk-go-v2/config"
)

func signBedrockRequest(ctx context.Context, req *http.Request, body []byte, region string) error {
    cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
    if err != nil {
        return err
    }

    creds, err := cfg.Credentials.Retrieve(ctx)
    if err != nil {
        return err
    }

    // Compute payload hash
    hash := sha256.Sum256(body)
    payloadHash := hex.EncodeToString(hash[:])

    // Sign the request
    signer := v4.NewSigner()
    return signer.SignHTTP(ctx, creds, req, payloadHash, "bedrock", region, time.Now())
}
```

### Google Vertex AI OAuth
```go
// Source: https://cloud.google.com/vertex-ai/generative-ai/docs/partner-models/claude/use-claude
import (
    "context"
    "net/http"

    "golang.org/x/oauth2/google"
)

func authenticateVertex(ctx context.Context, req *http.Request) error {
    creds, err := google.FindDefaultCredentials(ctx,
        "https://www.googleapis.com/auth/cloud-platform")
    if err != nil {
        return err
    }

    token, err := creds.TokenSource.Token()
    if err != nil {
        return err
    }

    req.Header.Set("Authorization", "Bearer "+token.AccessToken)
    return nil
}
```

### Azure Foundry Entra ID
```go
// Source: https://learn.microsoft.com/en-us/azure/ai-foundry/foundry-models/how-to/use-foundry-models-claude
import (
    "context"
    "net/http"

    "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
    "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

func authenticateAzure(ctx context.Context, req *http.Request) error {
    cred, err := azidentity.NewDefaultAzureCredential(nil)
    if err != nil {
        return err
    }

    token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
        Scopes: []string{"https://cognitiveservices.azure.com/.default"},
    })
    if err != nil {
        return err
    }

    req.Header.Set("Authorization", "Bearer "+token.Token)
    return nil
}
```

### Request Body Transformation (Bedrock/Vertex)
```go
// Source: Derived from Anthropic docs
import (
    "encoding/json"

    "github.com/tidwall/gjson"
    "github.com/tidwall/sjson"
)

func transformForBedrock(body []byte) ([]byte, string, error) {
    // Extract model from request
    model := gjson.GetBytes(body, "model").String()

    // Remove model from body
    newBody, _ := sjson.DeleteBytes(body, "model")

    // Add anthropic_version
    newBody, _ = sjson.SetBytes(newBody, "anthropic_version", "bedrock-2023-05-31")

    // Construct URL with model in path
    // Note: Model should already be mapped to Bedrock format
    url := fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke",
        region, url.PathEscape(model))

    return newBody, url, nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Bedrock regional only | Global endpoints available | Claude Sonnet 4.5 (2025) | Better availability, no price premium |
| Vertex regional only | Global endpoints available | Claude Sonnet 4.5 (2025) | Better availability, no price premium |
| Azure API Foundry | Microsoft Foundry rebrand | Late 2025 | Same API, new branding |
| Bedrock Text Completions API | Messages API only | 2024 | Older API deprecated |

**Deprecated/outdated:**
- Bedrock Text Completions API: Use Messages API instead
- Claude 3.7 Sonnet on Vertex: Deprecated October 28, 2025, shutdown May 11, 2026
- Claude 3.5 Haiku: Deprecated December 19, 2025

## Open Questions

Things that couldn't be fully resolved:

1. **Bedrock Event Stream → SSE Conversion**
   - What we know: Bedrock returns `application/vnd.amazon.eventstream`
   - What's unclear: Exact mapping of Event Stream event types to Anthropic SSE events
   - Recommendation: Test with live Bedrock API to verify conversion logic

2. **Long-Running Stream Token Refresh**
   - What we know: OAuth/Entra tokens typically expire in 1 hour
   - What's unclear: How to handle token refresh mid-stream for requests > 1 hour
   - Recommendation: Refresh token before streaming starts; for 10-minute timeout this is not an issue

3. **Azure Rate Limit Tracking**
   - What we know: Azure doesn't return standard Anthropic rate limit headers
   - What's unclear: Best approach for client-side rate limit estimation
   - Recommendation: Implement optional client-side rate tracking

## Sources

### Primary (HIGH confidence)
- [Anthropic - Claude on Amazon Bedrock](https://platform.claude.com/docs/en/api/claude-on-amazon-bedrock)
- [Anthropic - Claude on Vertex AI](https://platform.claude.com/docs/en/api/claude-on-vertex-ai)
- [Anthropic - Claude in Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry)
- [AWS - InvokeModelWithResponseStream API](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_InvokeModelWithResponseStream.html)
- [AWS SDK Go v2 - SigV4 Package](https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/aws/signer/v4)
- [Google Cloud - Use Claude on Vertex AI](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/partner-models/claude/use-claude)
- [Microsoft Learn - Deploy Claude in Foundry](https://learn.microsoft.com/en-us/azure/ai-foundry/foundry-models/how-to/use-foundry-models-claude)

### Secondary (MEDIUM confidence)
- [AWS Go SDK Bedrock Examples](https://docs.aws.amazon.com/code-library/latest/ug/go_2_bedrock-runtime_code_examples.html)
- [AWS SigV4 Signing Examples Repository](https://github.com/aws-samples/sigv4-signing-examples)

### Tertiary (LOW confidence)
- General web search results for integration patterns (marked for validation during implementation)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Official SDK documentation verified
- Architecture: HIGH - Based on existing codebase patterns
- Pitfalls: MEDIUM - Based on documentation; some need live testing to confirm

**Research date:** 2026-01-24
**Valid until:** 2026-02-24 (30 days - stable APIs with infrequent changes)
