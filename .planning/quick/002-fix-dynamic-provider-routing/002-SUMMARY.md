---
phase: quick
plan: 002
subsystem: proxy
tags: [go, routing, provider, fix, bug]

requires: []
provides:
  - Per-provider proxy routing in Handler
  - KeyPoolMapService for DI
  - ProviderProxy type with dedicated ReverseProxy

tech-stack:
  added: []
  patterns:
    - Per-provider proxy map for dynamic URL routing
    - ModifyResponse hook pattern for callback injection

key-files:
  created:
    - internal/proxy/provider_proxy.go
    - internal/proxy/provider_proxy_test.go
  modified:
    - internal/proxy/handler.go
    - internal/proxy/handler_test.go
    - internal/proxy/routes.go
    - cmd/cc-relay/di/providers.go

decisions:
  - name: ProviderProxy per-provider architecture
    rationale: Each provider needs its own ReverseProxy with correct target URL and auth
    alternatives: Single proxy with URL rewriting in Rewrite function (rejected - harder to maintain)
  - name: ModifyResponse hook pattern
    rationale: Allow Handler to inject response processing without coupling ProviderProxy to Handler
    alternatives: Direct reference to Handler (rejected - creates circular dependency)

metrics:
  duration: 26 minutes
  completed: 2026-01-24
---

# Quick Fix 002: Dynamic Provider Routing Summary

## One-liner

Per-provider ReverseProxy map replaces static proxy to fix requests going to wrong provider.

## Problem Solved

The bug was that `Handler.proxy` used a static `targetURL` derived from the first provider's BaseURL at initialization time. When the router selected a different provider dynamically, the request still went to the first provider's URL with the wrong authentication.

**Before fix:**
```
Router selects: provider-zai (https://api.z.ai)
Request goes to: provider-anthropic (https://api.anthropic.com) <-- WRONG!
```

**After fix:**
```
Router selects: provider-zai (https://api.z.ai)
Request goes to: provider-zai (https://api.z.ai) <-- CORRECT!
```

## Implementation

### 1. Created ProviderProxy Type (`internal/proxy/provider_proxy.go`)

New type that bundles:
- `providers.Provider` - Provider interface for auth/headers
- `*httputil.ReverseProxy` - Dedicated proxy with correct URL baked in
- `*keypool.KeyPool` - Optional per-provider rate limiting
- `APIKey` - Fallback key when no pool
- `modifyResponseHook` - Callback for response processing

### 2. Refactored Handler (`internal/proxy/handler.go`)

Changed from:
```go
type Handler struct {
    provider providers.Provider
    proxy    *httputil.ReverseProxy  // Single proxy!
    keyPool  *keypool.KeyPool
    apiKey   string
    ...
}
```

To:
```go
type Handler struct {
    providerProxies map[string]*ProviderProxy  // Per-provider!
    defaultProvider providers.Provider
    ...
}
```

Key changes:
- `NewHandler` creates ProviderProxy for each provider in providerInfos
- `ServeHTTP` looks up correct proxy: `pp := h.providerProxies[selectedProvider.Name()]`
- Uses `pp.Proxy.ServeHTTP(w, r)` instead of `h.proxy.ServeHTTP(w, r)`

### 3. Updated DI Layer (`cmd/cc-relay/di/providers.go`)

Added `KeyPoolMapService` to manage per-provider key pools:
```go
type KeyPoolMapService struct {
    Pools map[string]*keypool.KeyPool  // Provider name -> KeyPool
    Keys  map[string]string            // Provider name -> API key
}
```

Updated `NewProxyHandler` to pass these maps to `SetupRoutesWithRouter`.

### 4. Updated Routes (`internal/proxy/routes.go`)

Extended `SetupRoutesWithRouter` signature to accept:
- `providerPools map[string]*keypool.KeyPool`
- `providerKeys map[string]string`

## Test Coverage

Created comprehensive tests in `internal/proxy/provider_proxy_test.go`:
- `TestNewProviderProxy_ValidProvider` - Basic creation
- `TestNewProviderProxy_InvalidURL` - Error handling
- `TestProviderProxy_SetsCorrectTargetURL` - URL routing
- `TestProviderProxy_UsesCorrectAuth` - Authentication
- `TestProviderProxy_TransparentModeForwardsClientAuth` - Passthrough auth
- `TestProviderProxy_NonTransparentProviderUsesConfiguredKey` - Z.AI mode
- `TestProviderProxy_ForwardsAnthropicHeaders` - Header forwarding
- `TestProviderProxy_SSEHeadersSet` - SSE streaming
- `TestProviderProxy_ModifyResponseHookCalled` - Hook pattern
- `TestProviderProxy_ErrorHandlerReturnsAnthropicFormat` - Error format

## Commits

| Hash | Message |
|------|---------|
| 2843640 | fix(proxy): use per-provider proxy for dynamic routing |

## Files Changed

| File | Changes |
|------|---------|
| `internal/proxy/provider_proxy.go` | +136 (new) |
| `internal/proxy/provider_proxy_test.go` | +280 (new) |
| `internal/proxy/handler.go` | +153/-134 (refactored) |
| `internal/proxy/handler_test.go` | +62/-70 (updated signatures) |
| `internal/proxy/routes.go` | +6/-2 (new params) |
| `cmd/cc-relay/di/providers.go` | +59/-6 (KeyPoolMapService) |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed cyclomatic complexity in ServeHTTP**
- **Found during:** Task 2
- **Issue:** ServeHTTP had cyclomatic complexity of 11 (> 10 threshold)
- **Fix:** Extracted `logAndSetDebugHeaders()` and `rewriteModelIfNeeded()` helper functions
- **Commit:** 2843640

**2. [Rule 3 - Blocking] Fixed test helper unparam lint warning**
- **Found during:** Task 6
- **Issue:** Lint reported `pool` and `healthTracker` always nil in `newTestHandler`
- **Fix:** Added `//nolint:unparam` directive with rationale
- **Commit:** 2843640
