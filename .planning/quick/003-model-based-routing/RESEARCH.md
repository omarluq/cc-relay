# Model-Based Routing Research

**Date:** 2026-01-23
**Complexity Rating:** MEDIUM (100-300 lines, interface modification needed)

## Executive Summary

Model-based routing is **already planned** (Phase 5, SPEC.md) but not yet implemented. Adding it requires modifying the `ProviderRouter` interface because the current `Select(ctx, providers)` method doesn't have access to the request body where the model name lives.

**Recommended Approach:** Option B (handle model routing before calling ProviderRouter in handler.go)
**Estimated Lines of Code:** 150-250 lines

## Current Architecture Analysis

### 1. ProviderRouter Interface Limitation

**File:** `internal/router/router.go`

```go
type ProviderRouter interface {
    // Select chooses a provider from the pool based on the strategy.
    Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error)
    Name() string
}
```

**Problem:** `Select()` receives only `context.Context` and `[]ProviderInfo`. The model name is in the request body, which isn't accessible at this level.

**Current call site in handler.go:**
```go
func (h *Handler) selectProvider(ctx context.Context) (router.ProviderInfo, error) {
    if h.router == nil || len(h.providers) == 0 {
        return router.ProviderInfo{Provider: h.defaultProvider, ...}, nil
    }
    return h.router.Select(ctx, h.providers) // ← No request body!
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ...
    selectedProviderInfo, err := h.selectProvider(r.Context())
    // Model rewriting happens AFTER provider selection
    h.rewriteModelIfNeeded(r, &logger, selectedProvider)
}
```

### 2. Model Extraction Logic EXISTS ✓

**File:** `internal/proxy/model_rewrite.go`

```go
// ModelRewriter already knows how to extract model from request body
func (r *ModelRewriter) RewriteRequest(req *http.Request, logger *zerolog.Logger) error {
    // Read body
    bodyBytes, err := io.ReadAll(req.Body)
    // Parse JSON
    var body map[string]any
    json.Unmarshal(bodyBytes, &body)
    // Get model field
    originalModel, ok := body["model"].(string)
    // ...
}
```

**Reusable code:** YES. The model extraction logic can be factored out into a helper function.

### 3. Config Structure - Model Mapping

**File:** `internal/config/config.go`

```go
type ProviderConfig struct {
    ModelMapping map[string]string `yaml:"model_mapping"`
    // ...
}
```

**Current usage:** Maps outbound model names (e.g., `claude-opus` → `qwen3:8b`) for rewriting requests TO providers.

**Model-based routing needs:** Inverse - map inbound model name to provider name (e.g., `claude-opus` → `anthropic`, `glm-4.7` → `zai`).

### 4. Strategy Implementation Pattern

**Example:** `internal/router/round_robin.go`

```go
type RoundRobinRouter struct {
    index uint64
}

func (r *RoundRobinRouter) Select(_ context.Context, providers []ProviderInfo) (ProviderInfo, error) {
    healthy := FilterHealthy(providers)
    // ... select logic
    return healthy[idx], nil
}

func (r *RoundRobinRouter) Name() string {
    return StrategyRoundRobin
}
```

**Pattern:** Stateless selection based only on provider list and internal state (counter, weights, etc.).

## Implementation Options

### Option A: Extend ProviderRouter Interface (REJECTED)

**Changes:**
```go
type ProviderRouter interface {
    Select(ctx context.Context, providers []ProviderInfo, req *http.Request) (ProviderInfo, error)
    Name() string
}
```

**Pros:**
- Model-based router fits the same interface
- Centralized routing logic

**Cons:**
- **BREAKING CHANGE** - All 4 existing routers must be updated
- Request is irrelevant for round-robin, shuffle, weighted-round-robin, failover
- Violates interface segregation principle (not all strategies need request)

**Verdict:** Too invasive, poor design.

---

### Option B: Pre-Router Model Filtering (RECOMMENDED)

**Changes:**
```go
// handler.go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // NEW: Extract model name and filter providers BEFORE routing
    modelName := extractModelFromRequest(r) // New helper
    
    candidateProviders := h.providers
    if h.routingConfig.Strategy == "model_based" {
        candidateProviders = filterProvidersByModel(modelName, h.providers, h.modelToProviderMap)
        if len(candidateProviders) == 0 {
            // No provider supports this model
            WriteError(w, http.StatusBadRequest, "unsupported_model", ...)
            return
        }
    }
    
    // Use filtered providers for routing
    selectedProviderInfo, err := h.router.Select(r.Context(), candidateProviders)
    // ...
}
```

**Pros:**
- No interface changes
- Clear separation: model filtering → provider routing → key selection
- Works with existing routers (filtered list goes to failover/round-robin/shuffle/weighted)
- Easy to test in isolation

**Cons:**
- Model extraction happens twice (once for routing, once for rewrite)
  - **Mitigation:** Cache extracted model in request context
- Slightly more logic in handler.go
  - **Mitigation:** Extract to separate `modelRouter.go` helper

**Verdict:** Best balance of simplicity and correctness.

---

### Option C: New ModelBasedRouter with Request Context (CONSIDERED)

**Changes:**
```go
// router/model_based.go
type ModelBasedRouter struct {
    modelToProvider map[string]string // "claude-opus" → "anthropic"
    fallback        ProviderRouter     // If no match
}

// Store request in context before routing
const modelNameKey contextKey = "modelName"

func (h *Handler) ServeHTTP(w, r) {
    // Extract model and store in context
    model := extractModelFromRequest(r)
    ctx := context.WithValue(r.Context(), modelNameKey, model)
    
    selectedProviderInfo, err := h.router.Select(ctx, h.providers)
}

func (r *ModelBasedRouter) Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error) {
    modelName, ok := ctx.Value(modelNameKey).(string)
    // ... match model to provider
}
```

**Pros:**
- Follows ProviderRouter interface
- Self-contained router implementation

**Cons:**
- Hidden dependency on context key (fragile)
- Context key must be set by handler (coupling)
- Less obvious than Option B

**Verdict:** Clever but too implicit.

---

## Recommended Approach: Option B

### Config Schema

```yaml
routing:
  strategy: model_based
  model_mapping:
    # Pattern matching (prefix)
    claude-opus: anthropic
    claude-sonnet: anthropic
    claude-haiku: anthropic
    glm-4: zai
    glm-3: zai
    qwen: ollama
    llama: ollama
    # Fallback if no match
    default: anthropic
```

### Implementation Plan

#### 1. Add Model Extraction Helper

**File:** `internal/proxy/model_extract.go` (NEW)

```go
// ExtractModelFromRequest reads the model field from request body.
// Returns empty string if body is missing, malformed, or has no model field.
// Restores request body for subsequent reads.
func ExtractModelFromRequest(r *http.Request) string {
    if r.Body == nil {
        return ""
    }
    
    bodyBytes, _ := io.ReadAll(r.Body)
    r.Body.Close()
    r.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // Restore
    
    var body map[string]any
    if err := json.Unmarshal(bodyBytes, &body); err != nil {
        return ""
    }
    
    model, _ := body["model"].(string)
    return model
}

// CacheModelInContext stores extracted model in context to avoid re-reading body.
const modelNameContextKey contextKey = "extractedModel"

func CacheModelInContext(ctx context.Context, model string) context.Context {
    return context.WithValue(ctx, modelNameContextKey, model)
}

func GetModelFromContext(ctx context.Context) (string, bool) {
    model, ok := ctx.Value(modelNameContextKey).(string)
    return model, ok
}
```

**Lines:** ~50

#### 2. Add Model-to-Provider Mapping

**File:** `internal/config/config.go`

```go
type RoutingConfig struct {
    Strategy string `yaml:"strategy"`
    FailoverTimeout int `yaml:"failover_timeout"`
    Debug bool `yaml:"debug"`
    
    // NEW: Model-based routing configuration
    ModelMapping map[string]string `yaml:"model_mapping"` // model prefix → provider name
    DefaultProvider string `yaml:"default_provider"` // Fallback if no match
}
```

**Lines:** ~5

#### 3. Add Model Filtering Helper

**File:** `internal/proxy/model_filter.go` (NEW)

```go
// FilterProvidersByModel returns providers that support the given model.
// Uses prefix matching on model names (e.g., "claude-opus" matches "claude-opus-*").
// Returns all providers if model is empty or no mapping configured.
func FilterProvidersByModel(
    model string,
    providers []router.ProviderInfo,
    modelMapping map[string]string,
    defaultProviderName string,
) []router.ProviderInfo {
    if model == "" || len(modelMapping) == 0 {
        return providers
    }
    
    // Find provider name from model mapping (prefix match)
    var targetProviderName string
    for modelPrefix, providerName := range modelMapping {
        if strings.HasPrefix(model, modelPrefix) {
            targetProviderName = providerName
            break
        }
    }
    
    // Fallback to default if no match
    if targetProviderName == "" {
        if defaultProviderName != "" {
            targetProviderName = defaultProviderName
        } else {
            return providers // No filtering
        }
    }
    
    // Filter providers by name
    filtered := lo.Filter(providers, func(p router.ProviderInfo, _ int) bool {
        return p.Provider.Name() == targetProviderName
    })
    
    return filtered
}
```

**Lines:** ~35

#### 4. Integrate into Handler

**File:** `internal/proxy/handler.go`

```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    start := time.Now()
    
    // NEW: Extract model and cache in context
    model := ExtractModelFromRequest(r)
    r = r.WithContext(CacheModelInContext(r.Context(), model))
    
    // NEW: Filter providers if model-based routing
    candidateProviders := h.providers
    if h.routingConfig.Strategy == router.StrategyModelBased {
        candidateProviders = FilterProvidersByModel(
            model,
            h.providers,
            h.routingConfig.ModelMapping,
            h.routingConfig.DefaultProvider,
        )
        if len(candidateProviders) == 0 {
            WriteError(w, http.StatusBadRequest, "unsupported_model",
                fmt.Sprintf("no provider configured for model %q", model))
            return
        }
    }
    
    // Select provider from filtered candidates
    selectedProviderInfo, err := h.router.Select(r.Context(), candidateProviders)
    // ... rest unchanged
}
```

**Changes:** Add Handler.routingConfig field, update constructor to accept RoutingConfig

**Lines:** ~30 (plus ~20 for constructor changes)

#### 5. Add Strategy Constant

**File:** `internal/router/router.go`

```go
const (
    StrategyRoundRobin         = "round_robin"
    StrategyWeightedRoundRobin = "weighted_round_robin"
    StrategyShuffle            = "shuffle"
    StrategyFailover           = "failover"
    StrategyModelBased         = "model_based" // NEW
)

func NewRouter(strategy string, timeout time.Duration) (ProviderRouter, error) {
    // ...
    case StrategyModelBased:
        // Model-based filtering happens in handler, use failover as default router
        return NewFailoverRouter(timeout), nil
    // ...
}
```

**Lines:** ~5

#### 6. Update Model Rewriter to Use Cached Model

**File:** `internal/proxy/model_rewrite.go`

```go
func (r *ModelRewriter) RewriteRequest(req *http.Request, logger *zerolog.Logger) error {
    // Skip if no mapping configured
    if len(r.mapping) == 0 {
        return nil
    }
    
    // Try to get cached model from context first
    originalModel, ok := GetModelFromContext(req.Context())
    if !ok || originalModel == "" {
        // Fallback to extracting from body (existing logic)
        // ... current implementation
    }
    
    // ... rest of rewriting logic
}
```

**Lines:** ~10 changes

#### 7. Tests

**File:** `internal/proxy/model_filter_test.go` (NEW)

- Test prefix matching
- Test default provider fallback
- Test empty model handling
- Test no mapping case

**Lines:** ~80

**Total Estimated LOC:** ~235 lines

---

## Complexity Assessment

**Rating:** MEDIUM

**Reasons:**
- No interface changes (avoids breaking existing routers)
- Reuses existing model extraction logic pattern
- Requires new helper functions (~85 lines)
- Requires handler.go integration (~50 lines)
- Config changes minimal (~10 lines)
- Tests needed (~80 lines)

**Risk Areas:**
1. **Request body consumption** - Must restore body after reading for downstream use
2. **Model caching** - Context key collision risk (use unique private type)
3. **Config validation** - Ensure model_mapping provider names exist
4. **Error handling** - Clear errors when no provider matches model

---

## Files to Modify

| File | Type | Lines Changed | Description |
|------|------|---------------|-------------|
| `internal/proxy/model_extract.go` | NEW | ~50 | Model extraction and caching helpers |
| `internal/proxy/model_filter.go` | NEW | ~35 | Provider filtering by model |
| `internal/proxy/model_filter_test.go` | NEW | ~80 | Tests for filtering logic |
| `internal/proxy/handler.go` | MODIFY | ~50 | Integrate model filtering into ServeHTTP |
| `internal/proxy/model_rewrite.go` | MODIFY | ~10 | Use cached model to avoid re-reading body |
| `internal/config/config.go` | MODIFY | ~5 | Add ModelMapping and DefaultProvider fields |
| `internal/router/router.go` | MODIFY | ~5 | Add StrategyModelBased constant |

**Total:** ~235 lines across 7 files

---

## Key Code Snippets

### How It Works

**1. Request arrives with model in body:**
```json
{
  "model": "claude-opus-4",
  "messages": [...]
}
```

**2. Handler extracts model and caches:**
```go
model := ExtractModelFromRequest(r) // → "claude-opus-4"
r = r.WithContext(CacheModelInContext(r.Context(), model))
```

**3. Filter providers by model mapping:**
```go
// Config: {"claude-opus": "anthropic", "glm-4": "zai"}
candidateProviders := FilterProvidersByModel(
    "claude-opus-4",           // model
    h.providers,               // all providers
    {"claude-opus": "anthropic"},
    "anthropic",               // default
)
// Result: Only providers with Name() == "anthropic"
```

**4. Route within filtered providers:**
```go
selectedProviderInfo, err := h.router.Select(r.Context(), candidateProviders)
// Uses failover/round-robin/shuffle within the filtered set
```

**5. Model rewriting happens later (unchanged):**
```go
h.rewriteModelIfNeeded(r, &logger, selectedProvider)
// Uses cached model from context (no re-read)
```

---

## Alternative: Pattern Matching

For advanced use cases, support regex patterns:

```yaml
routing:
  model_mapping:
    "^claude-.*": anthropic
    "^glm-.*": zai
    "^(llama|qwen).*": ollama
```

**Implementation:** Replace `strings.HasPrefix` with `regexp.MatchString`

**Trade-off:** Adds ~30 lines for pattern compilation/caching, but enables more flexible routing.

---

## Comparison to Fork Implementation

The fork (`.planning/quick/002-fix-dynamic-provider-routing/FORK-RESEARCH.md`) uses **dedicated handlers per provider** with **model-based routing mode** as a top-level switch.

**Fork approach:**
```
model-based mode → Multiple handlers (one per provider) → Mux routes by model prefix
```

**Our approach (Option B):**
```
model-based strategy → Single handler → Filter providers → Existing router
```

**Advantages of Option B:**
- Reuses all existing routing strategies (can do model-based + failover)
- No handler duplication
- Clear separation of concerns
- Composable (can add model-based to any strategy)

---

## Next Steps

1. Implement model extraction helpers (`model_extract.go`)
2. Implement provider filtering (`model_filter.go`)
3. Add config fields and validation
4. Integrate into handler.go with feature flag (config.routing.strategy)
5. Write tests for edge cases (empty model, no match, malformed JSON)
6. Update documentation and example.yaml

**Estimated Implementation Time:** 3-4 hours for experienced Go developer
