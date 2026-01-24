# Quick Task 001: Model Rewrite Implementation

## Goal

Implement model name rewriting so that incoming Claude model names (e.g., `claude-opus-4-5-20251101`) are mapped to provider-specific model names (e.g., `qwen3:8b` for Ollama) using the existing `model_mapping` config field.

## Problem

The `model_mapping` field exists in `ProviderConfig` but is never applied to requests. When Claude Code sends a request for a Claude model to an Ollama provider, the request fails with 404 because Ollama doesn't recognize Claude model names.

## Tasks

### Task 1: Add Model Mapping to Provider Interface

**Files:**
- `internal/providers/provider.go` - Add `GetModelMapping()` method to interface
- `internal/providers/base.go` - Store modelMapping in BaseProvider, implement getter

**Changes:**
```go
// provider.go - Add to Provider interface:
GetModelMapping() map[string]string

// base.go - Add field and method:
type BaseProvider struct {
    // ... existing fields
    modelMapping map[string]string
}

func (p *BaseProvider) GetModelMapping() map[string]string {
    return p.modelMapping
}
```

### Task 2: Update Provider Constructors

**Files:**
- `internal/providers/base.go` - Update NewBaseProvider signature
- `internal/providers/anthropic.go` - Update constructor
- `internal/providers/zai.go` - Update constructor
- `internal/providers/ollama.go` - Update constructor

**Changes:**
- Add `modelMapping map[string]string` parameter to all constructors
- Pass through to BaseProvider

### Task 3: Implement Model Rewriting in Handler

**Files:**
- `internal/proxy/handler.go` - Add model rewrite in Rewrite function
- `internal/proxy/model_rewrite.go` - New file for rewrite logic (keeps handler clean)

**Changes:**
```go
// model_rewrite.go
func RewriteModelInBody(body []byte, mapping map[string]string) ([]byte, string, error)

// In handler Rewrite function:
// 1. Read request body
// 2. Call provider.GetModelMapping()
// 3. If mapping exists and model matches, rewrite
// 4. Replace request body with modified version
```

### Task 4: Update DI to Pass Model Mapping

**Files:**
- `cmd/cc-relay/di/providers.go` - Pass p.ModelMapping to provider constructors

**Changes:**
```go
case "anthropic":
    prov = providers.NewAnthropicProviderWithModels(p.Name, p.BaseURL, p.Models, p.ModelMapping)
case "zai":
    prov = providers.NewZAIProviderWithModels(p.Name, p.BaseURL, p.Models, p.ModelMapping)
case "ollama":
    prov = providers.NewOllamaProviderWithModels(p.Name, p.BaseURL, p.Models, p.ModelMapping)
```

### Task 5: Add Tests

**Files:**
- `internal/proxy/model_rewrite_test.go` - Unit tests for rewrite logic
- `internal/providers/base_test.go` - Test GetModelMapping
- `internal/proxy/handler_test.go` - Integration test for model rewriting

**Test Cases:**
1. Model rewrite when mapping exists
2. Model passthrough when no mapping
3. Model passthrough when model not in mapping
4. Empty/nil mapping handling
5. Invalid JSON handling (should error or passthrough?)

## Acceptance Criteria

- [ ] `model_mapping` from config is passed to providers
- [ ] Provider interface has `GetModelMapping()` method
- [ ] Handler rewrites model in request body before forwarding
- [ ] Original model preserved in logs for debugging
- [ ] Tests cover all edge cases
- [ ] All existing tests pass
- [ ] Linter passes (0 issues)

## Example Config

```yaml
providers:
  - name: ollama-local
    type: ollama
    enabled: true
    base_url: http://localhost:11434
    model_mapping:
      claude-opus-4-5-20251101: qwen3:8b
      claude-sonnet-4-20250514: qwen3:4b
```

## Estimated Scope

- ~150 lines new code
- ~200 lines tests
- 5-6 files modified
