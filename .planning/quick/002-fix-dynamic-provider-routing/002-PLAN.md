# Feature Plan: Fix Dynamic Provider Routing Bug

Created: 2026-01-23
Author: architect-agent

## Overview

Fix the dynamic provider routing bug where requests are sent to the wrong backend URL despite correct router selection. The bug occurs because `Handler.proxy` captures a static `targetURL` from the first provider, and `Handler.Rewrite` uses the static `h.provider` for authentication and headers, ignoring the dynamically selected provider.

## Requirements

- [ ] Requests must be sent to the URL of the selected provider
- [ ] Authentication must use the selected provider's method
- [ ] Headers must be forwarded according to selected provider's rules
- [ ] Model mapping must continue to work correctly
- [ ] Health tracking/circuit breaker must continue to work
- [ ] Backward compatibility with single-provider mode
- [ ] No performance regression (avoid parsing request body twice)

## Problem Summary

| Component | Current Behavior | Expected Behavior |
|-----------|------------------|-------------------|
| `targetURL` | Static, from first provider | Dynamic, from selected provider |
| `h.provider.Authenticate()` | Uses first provider | Uses selected provider |
| `h.provider.ForwardHeaders()` | Uses first provider | Uses selected provider |
| `h.provider.SupportsTransparentAuth()` | Uses first provider | Uses selected provider |

## Design Analysis: Three Approaches

### Option A: Fix Current Architecture (Context-Based Dynamic Provider)

Store the selected provider in request context, retrieve it in `Rewrite` closure.

```go
// In ServeHTTP:
r = r.WithContext(context.WithValue(r.Context(), selectedProviderContextKey, selectedProvider))

// In Rewrite:
selectedProvider := r.In.Context().Value(selectedProviderContextKey).(providers.Provider)
targetURL, _ := url.Parse(selectedProvider.BaseURL())
r.SetURL(targetURL)
selectedProvider.Authenticate(r.Out, selectedKey)
```

**Pros:**
- Minimal code change (~50 lines modified)
- Preserves existing ProviderRouter integration (failover, round-robin, etc.)
- No new types or files needed
- Easy to review and test

**Cons:**
- URL parsing happens on every request (small overhead)
- Error handling in Rewrite closure is awkward (no return value)
- `Rewrite` closure becomes more complex
- Type assertion in closure could panic (needs defensive coding)

### Option B: Adopt Fork's ModelRouter Pattern (Handler Per Provider)

Create separate `Handler` instances for each provider at startup, route to correct handler per request.

```go
type ProviderHandler struct {
    Provider providers.Provider
    Handler  *Handler              // Each has its own ReverseProxy
    KeyPool  *keypool.KeyPool
}

type ModelRouter struct {
    modelToProvider map[string]*ProviderHandler
    defaultHandler  *ProviderHandler
}
```

**Pros:**
- Clean separation - each handler is self-contained
- No per-request URL parsing
- No context juggling in Rewrite closure
- Simpler `Handler` struct (no router reference needed)

**Cons:**
- Significant refactoring (~300+ lines)
- Does NOT integrate with existing ProviderRouter strategies (failover, round-robin)
- Fork's ModelRouter only supports model-based routing
- Memory overhead (multiple handlers instead of one)
- Creates two parallel routing systems

### Option C: Hybrid (Recommended)

Extend existing architecture with per-provider handler map, while keeping ProviderRouter for strategy selection.

```go
// Handler gets a map of provider handlers
type Handler struct {
    providerHandlers map[string]*ProviderProxy  // NEW: Per-provider proxies
    router           router.ProviderRouter      // Existing router
    // ... other fields
}

// ProviderProxy bundles provider-specific proxy config
type ProviderProxy struct {
    Provider providers.Provider
    Proxy    *httputil.ReverseProxy
    KeyPool  *keypool.KeyPool
}
```

Flow:
1. `ServeHTTP` calls `router.Select()` (existing failover/round-robin/etc.)
2. Uses `providerHandlers[selectedProvider.Name()]` to get the right proxy
3. That proxy has the correct URL/auth/headers baked in

**Pros:**
- Best of both worlds - keeps ProviderRouter strategies AND correct routing
- No per-request URL parsing
- Clean separation of concerns
- Can evolve toward fork's model-based routing later
- Testable in isolation

**Cons:**
- Medium refactoring (~150 lines)
- Memory overhead (multiple proxies)
- Slightly more complex initialization

## Recommendation: Option C (Hybrid)

**Justification:**

1. **Preserves existing investment** - ProviderRouter strategies (failover, round-robin, weighted) are valuable and tested
2. **Solves the bug correctly** - Each provider gets its own proxy with correct URL/auth
3. **No runtime overhead** - URL parsing happens once at init, not per-request
4. **Clear upgrade path** - Can add model-based routing as another strategy later
5. **Testable** - Each ProviderProxy is isolated and testable

## Architecture

### Component Diagram

```
Request
    |
    v
+-------------------+
|   Handler         |
|   .router         |---> ProviderRouter (failover/round-robin/etc.)
|   .providerProxies|     |
+-------------------+     v
         |          ProviderInfo (Provider, IsHealthy, Weight)
         |                |
         v                v
    +----+----------------+----+
    |    providerProxies map   |
    +----+----+----+----+------+
         |    |    |
         v    v    v
   +-----+ +-----+ +-----+
   |Proxy| |Proxy| |Proxy|  <- Each has own targetURL, auth, headers
   |Anthr| |Z.AI | |Ollam|
   +-----+ +-----+ +-----+
```

### New Types

```go
// ProviderProxy bundles a provider with its dedicated reverse proxy.
// Each proxy has the provider's URL and auth baked in at creation time.
type ProviderProxy struct {
    Provider providers.Provider
    Proxy    *httputil.ReverseProxy
    KeyPool  *keypool.KeyPool  // May be nil for single-key mode
    apiKey   string            // Fallback key when no pool
}
```

### Data Flow

1. Request arrives at `Handler.ServeHTTP()`
2. Router selects provider: `selectedProviderInfo := h.router.Select(ctx, h.providers)`
3. Get provider's proxy: `providerProxy := h.providerProxies[selectedProviderInfo.Provider.Name()]`
4. Handle key selection (from `providerProxy.KeyPool` or `providerProxy.apiKey`)
5. Store selected key in request header: `r.Header.Set("X-Selected-Key", key)`
6. Delegate to proxy: `providerProxy.Proxy.ServeHTTP(w, r)`

## Dependencies

| Dependency | Type | Reason |
|------------|------|--------|
| `providers.Provider` | Internal | Provider interface |
| `router.ProviderRouter` | Internal | Existing routing strategies |
| `router.ProviderInfo` | Internal | Provider metadata for routing |
| `keypool.KeyPool` | Internal | Key selection per provider |
| `health.Tracker` | Internal | Circuit breaker per provider |

## Implementation Phases

### Phase 1: Add ProviderProxy Type

**Files to create/modify:**
- `internal/proxy/provider_proxy.go` (NEW) - ProviderProxy type and constructor

**Code:**

```go
// internal/proxy/provider_proxy.go
package proxy

import (
    "net/http"
    "net/http/httputil"
    "net/url"
    "github.com/samber/lo"
    "github.com/omarluq/cc-relay/internal/config"
    "github.com/omarluq/cc-relay/internal/keypool"
    "github.com/omarluq/cc-relay/internal/providers"
)

// ProviderProxy bundles a provider with its dedicated reverse proxy.
type ProviderProxy struct {
    Provider  providers.Provider
    Proxy     *httputil.ReverseProxy
    KeyPool   *keypool.KeyPool
    apiKey    string
    debugOpts config.DebugOptions
}

// NewProviderProxy creates a provider-specific proxy with correct URL and auth.
func NewProviderProxy(
    provider providers.Provider,
    apiKey string,
    pool *keypool.KeyPool,
    debugOpts config.DebugOptions,
) (*ProviderProxy, error) {
    targetURL, err := url.Parse(provider.BaseURL())
    if err != nil {
        return nil, fmt.Errorf("invalid provider base URL: %w", err)
    }

    pp := &ProviderProxy{
        Provider:  provider,
        KeyPool:   pool,
        apiKey:    apiKey,
        debugOpts: debugOpts,
    }

    pp.Proxy = &httputil.ReverseProxy{
        Rewrite: pp.rewrite(targetURL),
        FlushInterval: -1,  // Immediate flush for SSE
        ModifyResponse: pp.modifyResponse,
        ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
            WriteError(w, http.StatusBadGateway, "api_error", "upstream connection failed")
        },
    }

    return pp, nil
}

// rewrite creates the Rewrite function for this provider's proxy.
func (pp *ProviderProxy) rewrite(targetURL *url.URL) func(r *httputil.ProxyRequest) {
    return func(r *httputil.ProxyRequest) {
        r.SetURL(targetURL)
        r.SetXForwarded()

        clientAuth := r.In.Header.Get("Authorization")
        clientAPIKey := r.In.Header.Get("x-api-key")
        hasClientAuth := clientAuth != "" || clientAPIKey != ""

        if hasClientAuth && pp.Provider.SupportsTransparentAuth() {
            // TRANSPARENT MODE
            lo.ForEach(lo.Entries(r.In.Header), func(entry lo.Entry[string, []string], _ int) {
                canonicalKey := http.CanonicalHeaderKey(entry.Key)
                if len(canonicalKey) >= 10 && canonicalKey[:10] == "Anthropic-" {
                    r.Out.Header[canonicalKey] = entry.Value
                }
            })
            r.Out.Header.Set("Content-Type", "application/json")
        } else {
            // CONFIGURED KEY MODE
            r.Out.Header.Del("Authorization")
            r.Out.Header.Del("x-api-key")

            selectedKey := r.In.Header.Get("X-Selected-Key")
            if selectedKey == "" {
                selectedKey = pp.apiKey
            }

            if selectedKey != "" {
                pp.Provider.Authenticate(r.Out, selectedKey)
            }

            forwardHeaders := pp.Provider.ForwardHeaders(r.In.Header)
            lo.ForEach(lo.Entries(forwardHeaders), func(entry lo.Entry[string, []string], _ int) {
                r.Out.Header[entry.Key] = entry.Value
            })
        }
    }
}

// modifyResponse handles SSE headers and key pool updates.
func (pp *ProviderProxy) modifyResponse(resp *http.Response) error {
    if resp.Header.Get("Content-Type") == "text/event-stream" {
        SetSSEHeaders(resp.Header)
    }

    // Key pool updates happen in Handler.modifyResponse wrapper
    return nil
}
```

**Acceptance:**
- [ ] `ProviderProxy` compiles
- [ ] Constructor returns error for invalid URL
- [ ] Unit test verifies URL is set correctly

**Estimated effort:** Small (1-2 hours)

### Phase 2: Refactor Handler to Use ProviderProxy Map

**Files to modify:**
- `internal/proxy/handler.go` - Replace single proxy with map of ProviderProxy

**Changes:**

1. Replace fields:
```go
type Handler struct {
    // REMOVE:
    // provider      providers.Provider
    // proxy         *httputil.ReverseProxy
    // apiKey        string

    // ADD:
    providerProxies map[string]*ProviderProxy  // Per-provider proxies
    defaultProvider providers.Provider          // Fallback for single-provider mode

    // KEEP:
    router        router.ProviderRouter
    providers     []router.ProviderInfo
    healthTracker *health.Tracker
    debugOpts     config.DebugOptions
    routingDebug  bool
}
```

2. Update `NewHandler`:
```go
func NewHandler(
    provider providers.Provider,
    providerInfos []router.ProviderInfo,
    providerRouter router.ProviderRouter,
    apiKey string,
    pool *keypool.KeyPool,
    providerPools map[string]*keypool.KeyPool,  // NEW param
    providerKeys map[string]string,              // NEW param
    debugOpts config.DebugOptions,
    routingDebug bool,
    healthTracker *health.Tracker,
) (*Handler, error) {
    h := &Handler{
        providerProxies: make(map[string]*ProviderProxy),
        defaultProvider: provider,
        router:          providerRouter,
        providers:       providerInfos,
        healthTracker:   healthTracker,
        debugOpts:       debugOpts,
        routingDebug:    routingDebug,
    }

    // Create ProviderProxy for each provider
    if len(providerInfos) > 0 {
        for _, info := range providerInfos {
            prov := info.Provider
            key := providerKeys[prov.Name()]
            provPool := providerPools[prov.Name()]

            pp, err := NewProviderProxy(prov, key, provPool, debugOpts)
            if err != nil {
                return nil, fmt.Errorf("failed to create proxy for %s: %w", prov.Name(), err)
            }
            h.providerProxies[prov.Name()] = pp
        }
    } else {
        // Single provider mode
        pp, err := NewProviderProxy(provider, apiKey, pool, debugOpts)
        if err != nil {
            return nil, err
        }
        h.providerProxies[provider.Name()] = pp
    }

    return h, nil
}
```

3. Update `ServeHTTP`:
```go
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    start := time.Now()

    // Select provider
    selectedProviderInfo, err := h.selectProvider(r.Context())
    if err != nil {
        WriteError(w, http.StatusServiceUnavailable, "api_error",
            fmt.Sprintf("failed to select provider: %v", err))
        return
    }
    selectedProvider := selectedProviderInfo.Provider

    // Get provider's proxy
    providerProxy, ok := h.providerProxies[selectedProvider.Name()]
    if !ok {
        WriteError(w, http.StatusInternalServerError, "internal_error",
            fmt.Sprintf("no proxy configured for provider %s", selectedProvider.Name()))
        return
    }

    // ... logger setup, context setup (existing code) ...

    // Model rewriting (existing code)
    if mapping := selectedProvider.GetModelMapping(); len(mapping) > 0 {
        rewriter := NewModelRewriter(mapping)
        if err := rewriter.RewriteRequest(r, &logger); err != nil {
            logger.Warn().Err(err).Msg("failed to rewrite model")
        }
    }

    // Key selection from THIS provider's pool
    r, ok = h.handleAuthAndKeySelection(w, r, &logger, providerProxy)
    if !ok {
        return
    }

    // Proxy using THIS provider's proxy
    logger.Debug().Msg("proxying request to backend")
    backendStart := time.Now()
    providerProxy.Proxy.ServeHTTP(w, r)
    backendTime := time.Since(backendStart)

    // ... metrics logging (existing code) ...
}
```

4. Update `handleAuthAndKeySelection` to accept `*ProviderProxy`:
```go
func (h *Handler) handleAuthAndKeySelection(
    w http.ResponseWriter, r *http.Request, logger *zerolog.Logger,
    providerProxy *ProviderProxy,  // NEW param
) (*http.Request, bool) {
    // Use providerProxy.Provider instead of h.provider
    useTransparentAuth := hasClientAuth && providerProxy.Provider.SupportsTransparentAuth()

    if useTransparentAuth {
        // ...
        return r, true
    }

    if providerProxy.KeyPool != nil {
        return h.handleKeyPoolSelection(w, r, logger, providerProxy)
    }

    r.Header.Set("X-Selected-Key", providerProxy.apiKey)
    return r, true
}
```

**Acceptance:**
- [ ] Handler compiles with new structure
- [ ] Single-provider mode works (backward compatible)
- [ ] Multi-provider mode routes to correct provider

**Estimated effort:** Medium (3-4 hours)

### Phase 3: Update Routes and DI

**Files to modify:**
- `internal/proxy/routes.go` - Update `SetupRoutesWithRouter` signature
- `cmd/cc-relay/di/container.go` - Update DI wiring

**Changes to routes.go:**

```go
func SetupRoutesWithRouter(
    cfg *config.Config,
    providerInfos []router.ProviderInfo,
    providerRouter router.ProviderRouter,
    providerPools map[string]*keypool.KeyPool,  // NEW: per-provider pools
    providerKeys map[string]string,              // NEW: per-provider keys
    allProviders []providers.Provider,
    healthTracker *health.Tracker,
) (http.Handler, error) {
    // Get first provider as default
    var defaultProvider providers.Provider
    var defaultKey string
    if len(providerInfos) > 0 {
        defaultProvider = providerInfos[0].Provider
        defaultKey = providerKeys[defaultProvider.Name()]
    }

    handler, err := NewHandler(
        defaultProvider,
        providerInfos,
        providerRouter,
        defaultKey,
        providerPools[defaultProvider.Name()],
        providerPools,
        providerKeys,
        cfg.Logging.DebugOptions,
        cfg.Routing.IsDebugEnabled(),
        healthTracker,
    )
    // ... rest unchanged
}
```

**Acceptance:**
- [ ] Routes compile with new signature
- [ ] DI container properly injects per-provider pools and keys
- [ ] Existing tests pass

**Estimated effort:** Small (1-2 hours)

### Phase 4: Testing

**Files to create:**
- `internal/proxy/provider_proxy_test.go` (NEW)
- `internal/proxy/handler_routing_test.go` (NEW)

**Test Cases:**

1. **Unit: ProviderProxy URL**
```go
func TestProviderProxy_SetsCorrectURL(t *testing.T) {
    provider := providers.NewAnthropicProvider("test", "https://test.example.com")
    pp, _ := NewProviderProxy(provider, "key", nil, config.DebugOptions{})

    // Verify proxy sets correct URL
    backend := httptest.NewServer(...)
    // Assert request.URL.Host == "test.example.com"
}
```

2. **Unit: ProviderProxy Auth**
```go
func TestProviderProxy_AuthenticatesCorrectly(t *testing.T) {
    anthropic := providers.NewAnthropicProvider("anthropic", "https://api.anthropic.com")
    zai := providers.NewZAIProvider("zai", "https://api.zai.com")

    // Anthropic uses x-api-key
    // Z.AI uses Authorization: Bearer
    // Verify each proxy authenticates correctly
}
```

3. **Integration: Multi-Provider Routing**
```go
func TestHandler_RoutesToCorrectProvider(t *testing.T) {
    // Setup: 3 backends (mock servers)
    anthropicBackend := httptest.NewServer(...)
    zaiBackend := httptest.NewServer(...)
    ollamaBackend := httptest.NewServer(...)

    // Create providers pointing to mocks
    providers := []router.ProviderInfo{...}

    // Create handler with round-robin router
    handler, _ := NewHandler(...)

    // Send 3 requests, verify they go to different backends
    // Assert: anthropicBackend got 1 request
    // Assert: zaiBackend got 1 request
    // Assert: ollamaBackend got 1 request
}
```

4. **Integration: Model Mapping with Correct Provider**
```go
func TestHandler_ModelMappingUsesSelectedProvider(t *testing.T) {
    // Setup: Ollama with model_mapping: claude-opus-4-5-20251101 -> qwen3:8b
    // Create handler with failover router
    // Force selection of Ollama

    // Send request with model: claude-opus-4-5-20251101
    // Verify: Request body has model: qwen3:8b
    // Verify: Request went to Ollama backend URL
}
```

5. **Unit: Key Pool Per Provider**
```go
func TestHandler_UsesCorrectKeyPoolPerProvider(t *testing.T) {
    // Setup: Anthropic with pool of 2 keys, Z.AI with pool of 3 keys
    // Verify: When routing to Anthropic, uses Anthropic's pool
    // Verify: When routing to Z.AI, uses Z.AI's pool
}
```

**Acceptance:**
- [ ] All new tests pass
- [ ] Existing tests still pass
- [ ] Coverage for new code >= 80%

**Estimated effort:** Medium (3-4 hours)

### Phase 5: Documentation

**Files to modify:**
- `README.md` - Update multi-provider section
- `docs/routing.md` (NEW) - Document routing architecture

**Content:**

```markdown
## Multi-Provider Routing

cc-relay supports multiple LLM providers with various routing strategies.

### How It Works

When multiple providers are configured:
1. Each provider gets its own dedicated reverse proxy
2. The router selects which provider handles each request
3. The request is forwarded to that provider's backend

### Routing Strategies

- `failover`: Try providers in priority order (default)
- `round_robin`: Rotate through providers
- `shuffle`: Random selection
- `weighted_round_robin`: Weighted rotation

### Key Pools

Each provider can have its own key pool for rate limit management.
Keys are only used with their configured provider.

### Model Mapping

Model names are transformed AFTER provider selection:
1. Router selects provider (based on strategy)
2. Model name is mapped using that provider's `model_mapping`
3. Request is sent to provider with mapped model name
```

**Acceptance:**
- [ ] README updated
- [ ] Routing docs explain new architecture
- [ ] Example configs show multi-provider setup

**Estimated effort:** Small (1 hour)

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking backward compatibility | High | Keep single-provider mode unchanged; use same `NewHandler` signature with optional params |
| Memory overhead from multiple proxies | Low | Each proxy is ~100 bytes; for 10 providers = 1KB negligible |
| Key pool selection race condition | Medium | Ensure key pool selection uses same provider as proxy selection |
| Test flakiness with multiple backends | Medium | Use deterministic routing strategies in tests (round-robin with known seed) |

## Open Questions

- [x] Should we adopt fork's OAuth/Stainless header handling? **Answer: Out of scope for this PR, can be done separately**
- [x] Should model-based routing be a new strategy? **Answer: Yes, can be added later as `StrategyModelBased`**
- [ ] Should we deprecate single-provider SetupRoutes? **Decision needed: Keep for backward compat or migrate all?**

## Success Criteria

1. Requests are sent to the URL of the selected provider (verified by integration test)
2. Authentication uses the selected provider's method (verified by unit test)
3. Existing single-provider mode continues to work (verified by existing tests)
4. All routing strategies (failover, round-robin, shuffle, weighted) work with multiple providers
5. No performance regression (benchmark < 5% slower)

## Implementation Checklist

- [ ] Phase 1: Create `ProviderProxy` type
- [ ] Phase 2: Refactor `Handler` to use `map[string]*ProviderProxy`
- [ ] Phase 3: Update routes and DI
- [ ] Phase 4: Add unit and integration tests
- [ ] Phase 5: Update documentation
- [ ] Final: Run full test suite, benchmark, manual verification
