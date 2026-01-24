# Quick Task 003: Model-Based Routing Strategy

**Date:** 2026-01-23
**Status:** IN PROGRESS
**Research:** [RESEARCH.md](RESEARCH.md)

## Goal

Add model-based routing as a fifth strategy that composes with existing routing strategies (failover, round-robin, shuffle, weighted-round-robin).

## Approach: Pre-Router Model Filtering

Based on research, Option B is the cleanest approach:
1. Extract model name from request body
2. Filter providers that can serve that model
3. Pass filtered list to existing router

This enables composable routing: "model-based + failover" or "model-based + round-robin".

## Implementation Tasks

### Task 1: Create Model Extraction Helper
**File:** `internal/proxy/model_extract.go` (NEW ~50 lines)

- `ExtractModelFromRequest(r *http.Request) string` - reads model field from body, restores body
- `CacheModelInContext(ctx, model)` - stores model in context
- `GetModelFromContext(ctx)` - retrieves cached model

### Task 2: Create Provider Filtering Helper
**File:** `internal/proxy/model_filter.go` (NEW ~40 lines)

- `FilterProvidersByModel(model, providers, modelMapping, defaultProvider) []ProviderInfo`
- Prefix matching on model names
- Falls back to default provider if no match

### Task 3: Add Config Fields
**File:** `internal/config/config.go` (MODIFY ~10 lines)

Add to `RoutingConfig`:
```go
ModelMapping    map[string]string `yaml:"model_mapping"`    // model prefix → provider name
DefaultProvider string            `yaml:"default_provider"` // fallback
```

### Task 4: Add Strategy Constant
**File:** `internal/router/router.go` (MODIFY ~5 lines)

```go
const StrategyModelBased = "model_based"
```

Update `NewRouter` to return FailoverRouter for model_based (it's just a filter, not a full strategy).

### Task 5: Integrate into Handler
**File:** `internal/proxy/handler.go` (MODIFY ~40 lines)

In `ServeHTTP`:
1. Extract model from request
2. Cache in context
3. If strategy is model_based, filter providers
4. Pass filtered list to router

Add `routingConfig *config.RoutingConfig` field to Handler.

### Task 6: Update Model Rewriter
**File:** `internal/proxy/model_rewrite.go` (MODIFY ~10 lines)

Use cached model from context to avoid re-reading body.

### Task 7: Write Tests
**Files:**
- `internal/proxy/model_extract_test.go` (~60 lines)
- `internal/proxy/model_filter_test.go` (~80 lines)
- `internal/proxy/handler_test.go` (add integration tests)

### Task 8: Update Documentation
**File:** `example.yaml`

Add example model-based routing config.

## Config Example

```yaml
routing:
  strategy: model_based
  model_mapping:
    claude-opus: anthropic
    claude-sonnet: anthropic
    claude-haiku: anthropic
    glm-4: zai
    glm-3: zai
    qwen: ollama
    llama: ollama
  default_provider: anthropic
```

## Estimated LOC

| File | Type | Lines |
|------|------|-------|
| model_extract.go | NEW | ~50 |
| model_filter.go | NEW | ~40 |
| model_extract_test.go | NEW | ~60 |
| model_filter_test.go | NEW | ~80 |
| config.go | MODIFY | ~10 |
| router.go | MODIFY | ~5 |
| handler.go | MODIFY | ~40 |
| model_rewrite.go | MODIFY | ~10 |
| handler_test.go | MODIFY | ~40 |
| example.yaml | MODIFY | ~15 |

**Total:** ~350 lines

## Success Criteria

1. ✅ Model-based routing strategy configurable
2. ✅ Requests route to correct provider based on model name
3. ✅ Fallback to default provider when no mapping matches
4. ✅ Composes with existing strategies (filtered list goes to failover/round-robin)
5. ✅ All tests pass
6. ✅ Linters pass
