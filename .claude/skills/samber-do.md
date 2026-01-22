# samber/do - Dependency Injection

A comprehensive guide to using samber/do v2 in cc-relay for type-safe dependency injection.

**Version:** v2.0.0
**Import:** `github.com/samber/do/v2`
**Docs:** https://do.samber.dev/

## Quick Reference

### Core Functions

| Function | Purpose | Example |
|----------|---------|---------|
| `do.New()` | Create root container | `injector := do.New()` |
| `do.Provide` | Register lazy service | `do.Provide(i, NewCache)` |
| `do.ProvideValue` | Register pre-built value | `do.ProvideValue(i, cfg)` |
| `do.ProvideNamed` | Register with name | `do.ProvideNamed(i, "primary", NewDB)` |
| `do.ProvideTransient` | New instance per invoke | `do.ProvideTransient(i, NewLogger)` |
| `do.Invoke` | Resolve service | `svc, err := do.Invoke[*Cache](i)` |
| `do.MustInvoke` | Resolve or panic | `svc := do.MustInvoke[*Cache](i)` |
| `do.InvokeNamed` | Resolve by name | `db := do.InvokeNamed[*DB](i, "primary")` |
| `do.NewScope` | Create child scope | `scope := do.NewScope(i)` |
| `do.Shutdown` | Graceful shutdown | `do.Shutdown(i)` |

### Service Lifecycles

| Type | When Created | When Destroyed | Use Case |
|------|--------------|----------------|----------|
| Singleton (default) | First invoke | Container shutdown | Config, pools, providers |
| Transient | Every invoke | Never (GC) | Request loggers, temp objects |
| Lazy (default) | First invoke | Container shutdown | Heavy initialization |
| Eager | At registration | Container shutdown | Preload services |

### Scope Types

| Type | Description | Use Case |
|------|-------------|----------|
| Root | Main container | Application lifetime services |
| Scope (child) | Inherits from parent | Request-scoped services |
| Named | Multiple instances by name | Multi-tenant, multi-db |

## cc-relay Examples

### Basic DI Setup for serve.go

```go
import "github.com/samber/do/v2"

// cmd/cc-relay/serve.go
func runServer(configPath string) error {
    // Create root container
    injector := do.New()

    // Register services
    registerServices(injector, configPath)

    // Resolve handler and start server
    handler := do.MustInvoke[http.Handler](injector)

    server := &http.Server{
        Addr:    ":8787",
        Handler: handler,
    }

    // Graceful shutdown
    go func() {
        <-shutdownChan
        do.Shutdown(injector)
        server.Shutdown(context.Background())
    }()

    return server.ListenAndServe()
}

func registerServices(i do.Injector, configPath string) {
    // Config (eager - needed immediately)
    do.ProvideValue(i, configPath)
    do.Provide(i, NewConfig)

    // Core services (singletons)
    do.Provide(i, NewCache)
    do.Provide(i, NewKeyPool)
    do.Provide(i, NewProviderMap)

    // HTTP handler
    do.Provide(i, NewProxyHandler)
}
```

### Service Provider Functions

```go
import "github.com/samber/do/v2"

// Provider functions follow pattern: func(do.Injector) (T, error)

func NewConfig(i do.Injector) (*config.Config, error) {
    path := do.MustInvoke[string](i) // Get config path
    return config.Load(path)
}

func NewCache(i do.Injector) (*cache.Cache, error) {
    cfg := do.MustInvoke[*config.Config](i)
    return cache.New(cfg.Cache.Size), nil
}

func NewKeyPool(i do.Injector) (*keypool.KeyPool, error) {
    cfg := do.MustInvoke[*config.Config](i)
    c := do.MustInvoke[*cache.Cache](i)

    pool := keypool.NewKeyPool()
    for _, key := range cfg.Keys {
        pool.AddKey(keypool.NewKeyMetadata(
            key.Key,
            key.ProviderName,
            key.RPMLimit,
            key.ITPMLimit,
            key.OTPMLimit,
        ))
    }

    return pool, nil
}

func NewProviderMap(i do.Injector) (map[string]providers.Provider, error) {
    cfg := do.MustInvoke[*config.Config](i)

    providerMap := make(map[string]providers.Provider)
    for _, p := range cfg.Providers {
        if !p.Enabled {
            continue
        }
        providerMap[p.Name] = providers.NewProvider(p)
    }

    return providerMap, nil
}

func NewProxyHandler(i do.Injector) (http.Handler, error) {
    cfg := do.MustInvoke[*config.Config](i)
    pool := do.MustInvoke[*keypool.KeyPool](i)
    providerMap := do.MustInvoke[map[string]providers.Provider](i)
    c := do.MustInvoke[*cache.Cache](i)

    return proxy.NewHandler(cfg, pool, providerMap, c)
}
```

### Request-Scoped Services

```go
import "github.com/samber/do/v2"

// Middleware that creates request scope
func RequestScopeMiddleware(rootInjector do.Injector) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Create child scope for this request
            scope := do.NewScope(rootInjector)

            // Provide request-specific values
            requestID := r.Header.Get("X-Request-ID")
            if requestID == "" {
                requestID = uuid.New().String()
            }
            do.ProvideValue(scope, requestID)
            do.ProvideValue(scope, r.Context())

            // Provide request-scoped logger
            do.ProvideTransient(scope, func(i do.Injector) (*zerolog.Logger, error) {
                reqID := do.MustInvoke[string](i)
                logger := log.With().Str("request_id", reqID).Logger()
                return &logger, nil
            })

            // Store scope in context for handlers to use
            ctx := context.WithValue(r.Context(), scopeKey, scope)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Handler using request scope
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    scope := r.Context().Value(scopeKey).(do.Injector)

    // Get request-scoped logger
    logger := do.MustInvoke[*zerolog.Logger](scope)
    logger.Info().Msg("handling request")

    // ... handle request
}
```

### Named Services for Multi-Provider

```go
import "github.com/samber/do/v2"

// Register multiple providers by name
func registerProviders(i do.Injector) {
    do.ProvideNamed(i, "anthropic", NewAnthropicProvider)
    do.ProvideNamed(i, "zai", NewZAIProvider)
    do.ProvideNamed(i, "ollama", NewOllamaProvider)
}

func NewAnthropicProvider(i do.Injector) (providers.Provider, error) {
    cfg := do.MustInvoke[*config.Config](i)
    return providers.NewAnthropic(cfg.Anthropic), nil
}

// Resolve by name
func getProvider(i do.Injector, name string) (providers.Provider, error) {
    return do.InvokeNamed[providers.Provider](i, name)
}
```

### Shutdown Hooks for Cleanup

```go
import "github.com/samber/do/v2"

// Services implementing Shutdowner get called on container shutdown
type Cache struct {
    data *ristretto.Cache
}

func NewCache(i do.Injector) (*Cache, error) {
    cache, err := ristretto.NewCache(&ristretto.Config{
        NumCounters: 1e7,
        MaxCost:     1 << 30,
        BufferItems: 64,
    })
    if err != nil {
        return nil, err
    }
    return &Cache{data: cache}, nil
}

// Implement do.Shutdowner interface
func (c *Cache) Shutdown() error {
    c.data.Close()
    return nil
}

// On do.Shutdown(injector), Cache.Shutdown() is called automatically
```

### Health Check Service

```go
import "github.com/samber/do/v2"

// Health check service that inspects container
type HealthChecker struct {
    injector do.Injector
}

func NewHealthChecker(i do.Injector) (*HealthChecker, error) {
    return &HealthChecker{injector: i}, nil
}

func (h *HealthChecker) Check() HealthStatus {
    status := HealthStatus{Healthy: true, Services: make(map[string]bool)}

    // Check if services are initialized
    _, err := do.Invoke[*cache.Cache](h.injector)
    status.Services["cache"] = err == nil

    _, err = do.Invoke[*keypool.KeyPool](h.injector)
    status.Services["keypool"] = err == nil

    for name, healthy := range status.Services {
        if !healthy {
            status.Healthy = false
            status.Message = fmt.Sprintf("%s not healthy", name)
            break
        }
    }

    return status
}
```

### Testing with DI

```go
import (
    "testing"
    "github.com/samber/do/v2"
)

func TestHandler(t *testing.T) {
    // Create test container
    i := do.New()

    // Provide mock services
    do.ProvideValue(i, &MockConfig{})
    do.ProvideValue(i, &MockKeyPool{})
    do.ProvideValue(i, &MockCache{})
    do.Provide(i, NewProxyHandler)

    // Resolve handler
    handler := do.MustInvoke[http.Handler](i)

    // Test handler
    req := httptest.NewRequest("POST", "/v1/messages", nil)
    rec := httptest.NewRecorder()
    handler.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandlerWithMockProvider(t *testing.T) {
    i := do.New()

    // Register real config, mock provider
    do.Provide(i, NewConfig)
    do.ProvideNamed(i, "anthropic", func(i do.Injector) (providers.Provider, error) {
        return &MockProvider{}, nil
    })

    // ...
}
```

## Wiring Pattern for cc-relay serve.go

Current serve.go has manual wiring. Here's how to refactor with do:

```go
// cmd/cc-relay/di/container.go
package di

import (
    "github.com/samber/do/v2"
    "github.com/omarluq/cc-relay/internal/cache"
    "github.com/omarluq/cc-relay/internal/config"
    "github.com/omarluq/cc-relay/internal/keypool"
    "github.com/omarluq/cc-relay/internal/providers"
    "github.com/omarluq/cc-relay/internal/proxy"
)

// NewContainer creates the DI container with all services
func NewContainer(configPath string) do.Injector {
    i := do.New()

    // Configuration
    do.ProvideValue(i, configPath)
    do.Provide(i, LoadConfig)

    // Core services (singletons)
    do.Provide(i, NewCache)
    do.Provide(i, NewKeyPool)
    do.Provide(i, NewProviders)

    // HTTP layer
    do.Provide(i, NewMiddlewareChain)
    do.Provide(i, NewHandler)

    return i
}

func LoadConfig(i do.Injector) (*config.Config, error) {
    path := do.MustInvoke[string](i)
    return config.Load(path)
}

func NewCache(i do.Injector) (*cache.Cache, error) {
    cfg := do.MustInvoke[*config.Config](i)
    return cache.New(cfg.Cache)
}

func NewKeyPool(i do.Injector) (*keypool.KeyPool, error) {
    cfg := do.MustInvoke[*config.Config](i)
    pool := keypool.NewKeyPool()
    // ... populate from config
    return pool, nil
}

func NewProviders(i do.Injector) (map[string]providers.Provider, error) {
    cfg := do.MustInvoke[*config.Config](i)
    // ... build provider map
    return nil, nil
}

func NewMiddlewareChain(i do.Injector) ([]func(http.Handler) http.Handler, error) {
    cfg := do.MustInvoke[*config.Config](i)
    // Return middleware in order
    return []func(http.Handler) http.Handler{
        RequestIDMiddleware,
        LoggingMiddleware(cfg.Logging),
        AuthMiddleware(cfg.Server.APIKey),
    }, nil
}

func NewHandler(i do.Injector) (http.Handler, error) {
    cfg := do.MustInvoke[*config.Config](i)
    pool := do.MustInvoke[*keypool.KeyPool](i)
    providerMap := do.MustInvoke[map[string]providers.Provider](i)
    c := do.MustInvoke[*cache.Cache](i)
    middleware := do.MustInvoke[[]func(http.Handler) http.Handler](i)

    handler := proxy.NewHandler(cfg, pool, providerMap, c)

    // Apply middleware
    var h http.Handler = handler
    for i := len(middleware) - 1; i >= 0; i-- {
        h = middleware[i](h)
    }

    return h, nil
}
```

```go
// cmd/cc-relay/serve.go (refactored)
package main

import (
    "github.com/omarluq/cc-relay/cmd/cc-relay/di"
    "github.com/samber/do/v2"
)

func runServe(cmd *cobra.Command, args []string) error {
    // Create DI container
    container := di.NewContainer(configPath)

    // Resolve handler
    handler := do.MustInvoke[http.Handler](container)

    // Get config for server settings
    cfg := do.MustInvoke[*config.Config](container)

    server := &http.Server{
        Addr:         cfg.Server.Address,
        Handler:      handler,
        ReadTimeout:  cfg.Server.ReadTimeout,
        WriteTimeout: cfg.Server.WriteTimeout,
    }

    // Graceful shutdown
    go func() {
        <-shutdownSignal
        do.Shutdown(container)
        server.Shutdown(context.Background())
    }()

    return server.ListenAndServe()
}
```

## When to Use

**Use do for:**
- Service initialization with dependencies
- Singleton services (config, pools, connections)
- Testable code (swap implementations)
- Lifecycle management (shutdown hooks)
- Multi-tenant applications (named services)

**Use request scopes for:**
- Request-specific loggers
- Request context/ID
- Per-request metrics
- Auth context per request

## When NOT to Use

**Avoid do for:**
- Simple value types (pass directly)
- Pure functions with no dependencies
- Performance-critical hot paths
- Very small applications (overkill)

**Anti-patterns:**
- Storing request data in singletons
- Creating scopes without cleanup
- Circular dependencies (will fail)
- Over-injecting (inject interfaces, not implementations)

## Common Pitfalls

### 1. Circular Dependencies

```go
// BAD: A needs B, B needs A
func NewA(i do.Injector) (*A, error) {
    b := do.MustInvoke[*B](i) // B calls NewA...
    return &A{b: b}, nil
}

// GOOD: Break cycle with interface or lazy init
func NewA(i do.Injector) (*A, error) {
    return &A{getB: func() *B {
        return do.MustInvoke[*B](i)
    }}, nil
}
```

### 2. Scope Leaks

```go
// BAD: Storing scope-created object in singleton
type Handler struct {
    logger *zerolog.Logger // Different per request!
}

// GOOD: Resolve per request
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    scope := r.Context().Value(scopeKey).(do.Injector)
    logger := do.MustInvoke[*zerolog.Logger](scope)
    // Use logger for this request only
}
```

### 3. Missing Error Handling

```go
// BAD: Ignoring invoke errors
svc := do.MustInvoke[*Service](i) // Panics if service fails

// GOOD: Handle errors in production code
svc, err := do.Invoke[*Service](i)
if err != nil {
    return fmt.Errorf("failed to invoke service: %w", err)
}
```

### 4. Not Implementing Shutdowner

```go
// BAD: Resource leak
type DB struct {
    conn *sql.DB
}

// GOOD: Implement Shutdowner
func (d *DB) Shutdown() error {
    return d.conn.Close()
}
```

## Performance Tips

1. **Use ProvideValue for pre-built objects** - avoids lazy initialization overhead
2. **Keep scope creation lightweight** - only provide request-specific values
3. **Use MustInvoke in init paths** - errors are programming bugs, not runtime
4. **Profile scope creation** - if slow, cache more at root level

## Related Skills

- [samber-lo.md](samber-lo.md) - Functional collection utilities
- [samber-mo.md](samber-mo.md) - Monads for error handling
- [samber-ro.md](samber-ro.md) - Reactive streams

## References

- [Official Documentation](https://do.samber.dev/)
- [GitHub Repository](https://github.com/samber/do)
- [API Reference](https://pkg.go.dev/github.com/samber/do/v2)
