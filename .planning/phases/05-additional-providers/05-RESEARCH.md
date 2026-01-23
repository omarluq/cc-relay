# Phase 5: Additional Providers - Research

**Researched:** 2026-01-23
**Domain:** LLM Provider Integration (Z.AI and Ollama)
**Confidence:** HIGH

## Summary

This phase adds Z.AI (Zhipu AI) and Ollama providers to cc-relay. Research reveals a key insight: both providers are now Anthropic Messages API compatible, making implementation straightforward using the existing `BaseProvider` pattern.

**Z.AI Status:** Provider code exists at `internal/providers/zai.go` and is wired into the DI container. The provider is fully functional - this phase primarily validates the integration is complete.

**Ollama Status:** No provider code exists yet. However, Ollama v0.14+ now supports native Anthropic Messages API compatibility at `/v1/messages`, meaning we can implement Ollama using the same `BaseProvider` pattern as Z.AI with minimal differences.

**Primary recommendation:** Create `OllamaProvider` embedding `BaseProvider` with Ollama-specific defaults (localhost:11434, no authentication required) and feature detection methods for unsupported capabilities (prompt caching, PDF support, extended thinking budget enforcement).

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `BaseProvider` | Internal | Anthropic-compatible provider base | Already implemented, battle-tested |
| `samber/do` | v2 | Dependency injection | Existing DI pattern in codebase |
| `samber/lo` | Latest | Functional utilities | Already used in BaseProvider |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `net/http` | stdlib | HTTP client for health checks | Ollama model discovery |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Native Anthropic Messages API | OpenAI-compatible API | OpenAI format would require request transformation; Anthropic format is direct passthrough |
| BaseProvider embedding | Separate implementation | Would duplicate code; embedding reuses proven patterns |

## Architecture Patterns

### Recommended Project Structure

```
internal/providers/
├── provider.go      # Provider interface (exists)
├── base.go          # BaseProvider for Anthropic-compatible (exists)
├── anthropic.go     # Anthropic provider (exists)
├── zai.go           # Z.AI provider (exists)
└── ollama.go        # NEW: Ollama provider
```

### Pattern 1: BaseProvider Embedding

**What:** Ollama and Z.AI embed `BaseProvider` for shared Anthropic-compatible functionality.
**When to use:** When provider uses Anthropic Messages API format.
**Example:**
```go
// Source: internal/providers/zai.go (existing pattern)
type OllamaProvider struct {
    BaseProvider
}

func NewOllamaProvider(name, baseURL string) *OllamaProvider {
    if baseURL == "" {
        baseURL = DefaultOllamaBaseURL // "http://localhost:11434"
    }
    return &OllamaProvider{
        BaseProvider: NewBaseProvider(name, baseURL, OllamaOwner, nil),
    }
}
```

### Pattern 2: Feature Detection Methods

**What:** Provider methods indicate unsupported features so proxy can handle gracefully.
**When to use:** When provider has capability gaps vs Anthropic.
**Example:**
```go
// Ollama-specific feature detection
func (p *OllamaProvider) SupportsPromptCaching() bool {
    return false
}

func (p *OllamaProvider) SupportsPDFInput() bool {
    return false
}

func (p *OllamaProvider) SupportsExtendedThinkingBudget() bool {
    return false // accepts budget_tokens but doesn't enforce
}
```

### Pattern 3: DI Container Registration

**What:** Providers registered via type switch in `NewProviderMap`.
**When to use:** Adding new provider type.
**Example:**
```go
// Source: cmd/cc-relay/di/providers.go (existing pattern)
switch p.Type {
case "anthropic":
    prov = providers.NewAnthropicProviderWithModels(p.Name, p.BaseURL, p.Models)
case "zai":
    prov = providers.NewZAIProviderWithModels(p.Name, p.BaseURL, p.Models)
case "ollama":  // NEW
    prov = providers.NewOllamaProviderWithModels(p.Name, p.BaseURL, p.Models)
default:
    continue
}
```

### Anti-Patterns to Avoid

- **Request/Response Transformation:** Don't build request transformers; Ollama's Anthropic endpoint handles this natively.
- **Dynamic Model Discovery at Startup:** Don't query `/api/tags` at startup; use configured models or empty list (user configures).
- **Authentication Validation:** Ollama accepts but ignores API keys; don't validate.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Anthropic API compatibility | Request/response transformers | Ollama's `/v1/messages` endpoint | Ollama handles transformation natively since v0.14 |
| Model list endpoint | Custom model discovery | Configured models list | Ollama models are user-installed; static config is clearer |
| Streaming support | Custom SSE handling | BaseProvider.SupportsStreaming() | Streaming works identically to Anthropic |

**Key insight:** Ollama v0.14+ provides native Anthropic Messages API compatibility. The `/v1/messages` endpoint speaks Anthropic format directly, including streaming events, tool calling, and extended thinking.

## Common Pitfalls

### Pitfall 1: Assuming Request Transformation Needed

**What goes wrong:** Developer builds OpenAI-to-Anthropic request transformers for Ollama.
**Why it happens:** Older Ollama versions only had OpenAI-compatible endpoint; new Anthropic endpoint not well known.
**How to avoid:** Use `/v1/messages` endpoint (Anthropic format), not `/v1/chat/completions` (OpenAI format).
**Warning signs:** Building message format converters, role mappings, tool call transformers.

### Pitfall 2: Forgetting Feature Limitations

**What goes wrong:** Proxy assumes all Anthropic features work, leading to silent failures or confusing errors.
**Why it happens:** Ollama's Anthropic compatibility is partial.
**How to avoid:** Document and detect unsupported features:
- No prompt caching
- No PDF input support
- No token counting endpoint
- Images must be base64 (no URLs)
- Extended thinking `budget_tokens` accepted but not enforced

**Warning signs:** Proxy sends prompt caching headers to Ollama, no errors but no caching.

### Pitfall 3: Localhost vs Network Access

**What goes wrong:** Ollama unreachable when cc-relay runs in container but Ollama on host.
**Why it happens:** `localhost:11434` doesn't resolve from container to host.
**How to avoid:** Support configurable base_url; document Docker networking requirements.
**Warning signs:** Connection refused errors only in containerized deployments.

### Pitfall 4: Model Name Confusion

**What goes wrong:** User expects `claude-sonnet-4-5` to work; Ollama needs `qwen3:32b`.
**Why it happens:** Ollama uses its own model naming; mapping needed.
**How to avoid:** Use `model_mapping` config like other providers; document clearly.
**Warning signs:** Model not found errors.

## Code Examples

### Ollama Provider Implementation

```go
// Source: Pattern from internal/providers/zai.go
package providers

const (
    DefaultOllamaBaseURL = "http://localhost:11434"
    OllamaOwner          = "ollama"
)

type OllamaProvider struct {
    BaseProvider
}

func NewOllamaProvider(name, baseURL string) *OllamaProvider {
    return NewOllamaProviderWithModels(name, baseURL, nil)
}

func NewOllamaProviderWithModels(name, baseURL string, models []string) *OllamaProvider {
    if baseURL == "" {
        baseURL = DefaultOllamaBaseURL
    }
    return &OllamaProvider{
        BaseProvider: NewBaseProvider(name, baseURL, OllamaOwner, models),
    }
}

// Authenticate is a no-op for Ollama (accepts but ignores API key)
func (p *OllamaProvider) Authenticate(req *http.Request, key string) error {
    // Ollama accepts x-api-key but doesn't validate it
    // We still set it for consistency with BaseProvider pattern
    req.Header.Set("x-api-key", key)
    return nil
}
```

### DI Container Registration

```go
// Source: cmd/cc-relay/di/providers.go NewProviderMap function
case "ollama":
    prov = providers.NewOllamaProviderWithModels(p.Name, p.BaseURL, p.Models)
```

### Example Configuration

```yaml
# Source: example.yaml pattern
- name: "ollama"
  type: "ollama"
  enabled: true
  base_url: "http://localhost:11434"

  models:
    - "qwen3:32b"
    - "qwen3:8b"
    - "codestral:latest"

  model_mapping:
    "claude-sonnet-4-5-20250929": "qwen3:32b"
    "claude-sonnet-4-5": "qwen3:32b"
    "claude-haiku-4-5-20251001": "qwen3:8b"
    "claude-haiku-4-5": "qwen3:8b"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| OpenAI-compatible endpoint only | Native Anthropic Messages API | Ollama v0.14 (Jan 2026) | No transformation needed; direct passthrough |
| External shim (ollama-anthropic-shim) | Built-in Anthropic endpoint | Ollama v0.14 | Simplifies integration significantly |

**Deprecated/outdated:**
- `ollama-anthropic-shim` GitHub project: No longer needed; Ollama has native support.
- OpenAI-format transformation: Use `/v1/messages` not `/v1/chat/completions`.

## Z.AI Provider Verification

Z.AI provider (`internal/providers/zai.go`) is already implemented and wired:

**Verified complete:**
- Provider code exists with BaseProvider embedding
- DI container handles `type: "zai"` in NewProviderMap
- Tests exist at `internal/providers/zai_test.go`
- Example config in `example.yaml` with model mapping

**What this phase should verify:**
1. End-to-end test: Configure Z.AI, send request, verify response
2. Model mapping works: Request for `claude-sonnet-4-5` routes to `GLM-4.7`
3. Health check integration: Circuit breaker tracks Z.AI failures

## Ollama Feature Support Matrix

| Feature | Anthropic | Z.AI | Ollama | Notes |
|---------|-----------|------|--------|-------|
| `/v1/messages` endpoint | Yes | Yes | Yes | All use same format |
| Streaming (SSE) | Yes | Yes | Yes | Same event sequence |
| Tool calling | Yes | Yes | Yes | Same format |
| Extended thinking | Yes | Yes | Yes* | *budget_tokens accepted, not enforced |
| Prompt caching | Yes | ? | No | Not supported |
| PDF input | Yes | ? | No | Not supported |
| Image URLs | Yes | ? | No | Base64 only |
| Token counting | Yes | ? | No | `/v1/messages/count_tokens` not available |

## Open Questions

1. **Z.AI Prompt Caching Support**
   - What we know: Z.AI is Anthropic-compatible
   - What's unclear: Whether Z.AI supports prompt caching headers
   - Recommendation: Test empirically; document findings

2. **Ollama Model Discovery**
   - What we know: Ollama has `/api/tags` for listing installed models
   - What's unclear: Should cc-relay auto-discover or use static config?
   - Recommendation: Use static config (user specifies models); simpler, more predictable

3. **Feature Detection Interface**
   - What we know: Ollama lacks some features (prompt caching, PDF)
   - What's unclear: Should Provider interface include feature detection methods?
   - Recommendation: Add optional methods in Phase 6+ if needed; keep Phase 5 simple

## Sources

### Primary (HIGH confidence)
- [Ollama Anthropic Compatibility Docs](https://docs.ollama.com/api/anthropic-compatibility) - Endpoint format, limitations
- [Ollama Blog: Claude Code](https://ollama.com/blog/claude) - Configuration, recommended models
- [Z.AI Developer Docs](https://docs.z.ai/scenario-example/develop-tools/claude) - API format, authentication
- Existing codebase: `internal/providers/zai.go`, `cmd/cc-relay/di/providers.go`

### Secondary (MEDIUM confidence)
- [Ollama GitHub API docs](https://github.com/ollama/ollama/blob/main/docs/api.md) - Native API reference
- Example.yaml in repository - Configuration patterns

### Tertiary (LOW confidence)
- Web search results about integration patterns - Community usage patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Based on existing codebase patterns
- Architecture: HIGH - Direct extension of proven BaseProvider pattern
- Pitfalls: HIGH - Verified against official Ollama documentation

**Research date:** 2026-01-23
**Valid until:** 60 days (Ollama's Anthropic API is stable since v0.14)
