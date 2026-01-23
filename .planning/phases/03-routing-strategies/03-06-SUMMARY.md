---
phase: 03-routing-strategies
plan: 06
subsystem: di-integration
tags: [di, router, proxy, handler, debug-headers]
requires: ["03-01", "03-02", "03-03", "03-04", "03-05"]
provides: ["router-di-integration", "debug-headers", "provider-selection"]
affects: []
tech-stack:
  added: []
  patterns: ["dependency-injection", "provider-routing"]
key-files:
  created:
    - internal/proxy/routes.go (SetupRoutesWithRouter function)
  modified:
    - cmd/cc-relay/di/providers.go
    - internal/proxy/handler.go
    - internal/proxy/handler_test.go
decisions:
  - title: "Router before Handler in DI order"
    rationale: "Handler depends on Router for provider selection"
  - title: "IsHealthy stub returns true"
    rationale: "Health tracking deferred to Phase 4"
  - title: "Weight/priority from first key"
    rationale: "Provider-level defaults from first key config"
metrics:
  duration: 16min
  completed: 2026-01-23
---

# Phase 3 Plan 6: DI and Handler Integration Summary

Router strategies fully wired into application via DI container with debug header support.

## Changes

### DI Container (`cmd/cc-relay/di/providers.go`)

**RouterService type added:**
```go
type RouterService struct {
    Router router.ProviderRouter
}
```

**NewRouter provider:**
- Creates router from config.Routing.GetEffectiveStrategy()
- Uses GetFailoverTimeoutOption().OrElse(5*time.Second) for timeout
- Returns RouterService wrapping ProviderRouter

**Registration order updated:**
1. Config
2. Cache
3. Providers
4. KeyPool
5. **Router** (new)
6. Handler
7. Server

**NewProxyHandler updated:**
- Injects RouterService via `do.MustInvoke`
- Builds `[]router.ProviderInfo` from config providers
- Extracts weight/priority from first key of each provider
- Calls `proxy.SetupRoutesWithRouter` with full context

### Proxy Handler (`internal/proxy/handler.go`)

**Handler struct extended:**
```go
type Handler struct {
    provider     providers.Provider
    providers    []router.ProviderInfo  // NEW
    router       router.ProviderRouter  // NEW
    proxy        *httputil.ReverseProxy
    keyPool      *keypool.KeyPool
    apiKey       string
    debugOpts    config.DebugOptions
    routingDebug bool                   // NEW
}
```

**NewHandler signature updated:**
```go
func NewHandler(
    provider providers.Provider,
    providerInfos []router.ProviderInfo,  // NEW
    providerRouter router.ProviderRouter, // NEW
    apiKey string,
    pool *keypool.KeyPool,
    debugOpts config.DebugOptions,
    routingDebug bool,                    // NEW
) (*Handler, error)
```

**selectProvider method added:**
- Uses router.Select() when router is available
- Falls back to static provider in single-provider mode
- Returns ProviderInfo with provider and health status

**ServeHTTP updated:**
- Calls selectProvider at request start
- Adds debug headers when routingDebug=true:
  - `X-CC-Relay-Strategy`: Router strategy name
  - `X-CC-Relay-Provider`: Selected provider name
- Logs strategy and provider with zerolog

### Routes (`internal/proxy/routes.go`)

**SetupRoutesWithRouter function added:**
```go
func SetupRoutesWithRouter(
    cfg *config.Config,
    provider providers.Provider,
    providerInfos []router.ProviderInfo,
    providerRouter router.ProviderRouter,
    providerKey string,
    pool *keypool.KeyPool,
    allProviders []providers.Provider,
) (http.Handler, error)
```

- Creates handler with full router support
- Reads routingDebug from config.Routing.IsDebugEnabled()
- Same middleware chain as SetupRoutesWithProviders

## Test Coverage

### DI Tests (`container_test.go`)
- `TestRouterService/creates_router_with_default_strategy` - Default failover
- `TestRouterService/creates_router_with_configured_strategy` - round_robin
- `TestRouterService/router_depends_on_config` - Dependency order
- `TestRouterService/supports_all_routing_strategies` - All 4 strategies

### Handler Tests (`handler_test.go`)
- `TestHandler_SingleProviderMode` - Backwards compat (router=nil)
- `TestHandler_MultiProviderModeUsesRouter` - Router-based selection
- `TestHandler_DebugHeadersDisabledByDefault` - No headers when false
- `TestHandler_DebugHeadersWhenEnabled` - Headers added when true
- `TestHandler_RouterSelectionError` - 503 on router failure
- `TestHandler_SelectProviderSingleMode` - selectProvider unit test
- `TestHandler_SelectProviderMultiMode` - selectProvider with router

## Commits

| Hash | Description |
|------|-------------|
| 3bfa22f | Add RouterService to DI container |
| d806763 | Update proxy handler to use ProviderRouter |
| 97b3e9d | Wire router into DI and proxy handler setup |

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Router registered after KeyPool | Handler needs both; Router has simpler deps |
| Default timeout 5 seconds | Balance between retry attempts and latency |
| IsHealthy stub returns true | Phase 4 adds circuit breaker health tracking |
| Weight/priority from first key | Provider-level defaults, per-key config future work |
| Debug headers only with router | No strategy name in single-provider mode |

## Deviations from Plan

None - plan executed exactly as written.

## Phase 3 Complete

This plan completes Phase 3 (Routing Strategies):

| Plan | Description | Status |
|------|-------------|--------|
| 03-01 | ProviderRouter interface and types | Complete |
| 03-02 | RoundRobinRouter implementation | Complete |
| 03-03 | ShuffleRouter and WeightedRoundRobinRouter | Complete |
| 03-04 | Failover trigger system | Complete |
| 03-05 | FailoverRouter with parallel retry | Complete |
| 03-06 | DI and handler integration | Complete |

**Routing strategies are now fully operational:**
- All 4 strategies implemented (failover, round_robin, weighted_round_robin, shuffle)
- Router integrated into DI container
- Handler selects provider via router
- Debug headers available for troubleshooting
- Comprehensive test coverage

## Next Phase Readiness

Phase 4 (Health Tracking) prerequisites met:
- ProviderInfo.IsHealthy interface ready for circuit breaker
- FailoverRouter uses IsHealthy for provider filtering
- RouterService can be extended with health service injection
