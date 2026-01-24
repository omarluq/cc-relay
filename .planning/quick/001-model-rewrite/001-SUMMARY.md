# Quick Task 001 Summary: Model Rewrite Implementation

## Objective

Implement model name rewriting so incoming Claude model names are mapped to provider-specific model names using the existing `model_mapping` config field.

## Problem Solved

When Claude Code sends requests to cc-relay configured with Ollama, it sends `{"model": "claude-opus-4-5-20251101"}` but Ollama only knows its own models like `qwen3:8b`. This caused 404 errors. The `model_mapping` config field existed but was never applied to requests.

## Changes Made

### 1. Provider Interface (`internal/providers/provider.go`)

Added two new methods to the Provider interface:
- `GetModelMapping() map[string]string` - Returns the model mapping
- `MapModel(model string) string` - Maps a model name if found

### 2. Base Provider (`internal/providers/base.go`)

- Added `modelMapping` field to `BaseProvider` struct
- Added `NewBaseProviderWithMapping()` constructor
- Implemented `GetModelMapping()` and `MapModel()` methods

### 3. Provider Constructors

Updated all three providers with new `WithMapping` constructors:
- `providers.NewAnthropicProviderWithMapping(name, baseURL, models, modelMapping)`
- `providers.NewZAIProviderWithMapping(name, baseURL, models, modelMapping)`
- `providers.NewOllamaProviderWithMapping(name, baseURL, models, modelMapping)`

### 4. Model Rewriter (`internal/proxy/model_rewrite.go`)

New file implementing model rewriting logic:
- `NewModelRewriter(mapping)` - Creates rewriter with mapping
- `RewriteRequest(req, logger)` - Rewrites model in request body JSON
- `RewriteModel(model)` - Maps a single model name
- `HasMapping()` - Checks if mapping exists

Features:
- Graceful degradation (invalid JSON passes through unchanged)
- Logs original → mapped model for debugging
- Preserves all other request fields

### 5. Handler Integration (`internal/proxy/handler.go`)

Added model rewriting in `ServeHTTP` after provider selection:
```go
if mapping := selectedProvider.GetModelMapping(); len(mapping) > 0 {
    rewriter := NewModelRewriter(mapping)
    if err := rewriter.RewriteRequest(r, &logger); err != nil {
        logger.Warn().Err(err).Msg("failed to rewrite model in request body")
    }
}
```

### 6. DI Wiring (`cmd/cc-relay/di/providers.go`)

Updated provider creation to pass model mapping from config:
```go
case "ollama":
    prov = providers.NewOllamaProviderWithMapping(p.Name, p.BaseURL, p.Models, p.ModelMapping)
```

### 7. Tests

- `internal/proxy/model_rewrite_test.go` - 7 test functions covering all cases
- `internal/providers/ollama_test.go` - Added 4 new tests for model mapping

## Configuration Example

```yaml
providers:
  - name: ollama-local
    type: ollama
    enabled: true
    base_url: http://localhost:11434
    model_mapping:
      claude-opus-4-5-20251101: qwen3:8b
      claude-sonnet-4-20250514: qwen3:4b
      claude-haiku-3-5-20241022: qwen3:1b
```

## Verification

- [x] `go build ./...` - Compiles without errors
- [x] `go test -race -short ./...` - All 14 packages pass
- [x] `golangci-lint run ./...` - 0 issues
- [x] Model mapping from config passed to providers
- [x] Handler rewrites model in request body
- [x] Original model logged for debugging
- [x] Graceful degradation for invalid JSON

## Files Modified

| File | Lines Changed |
|------|---------------|
| `internal/providers/provider.go` | +9 |
| `internal/providers/base.go` | +30 |
| `internal/providers/anthropic.go` | +12 |
| `internal/providers/zai.go` | +15 |
| `internal/providers/ollama.go` | +15 |
| `internal/proxy/handler.go` | +8 |
| `internal/proxy/model_rewrite.go` | +117 (new) |
| `internal/proxy/model_rewrite_test.go` | +273 (new) |
| `internal/providers/ollama_test.go` | +106 |
| `cmd/cc-relay/di/providers.go` | +3 |
| `internal/proxy/handler_test.go` | +8 |
| `internal/router/weighted_round_robin_test.go` | +2 |

## Summary

Implemented transparent model name rewriting that enables Ollama (and other providers) to receive requests with Claude model names and translate them to provider-specific models. Users can now configure `model_mapping` in their provider config, and cc-relay will automatically rewrite the model field in request bodies before forwarding.
