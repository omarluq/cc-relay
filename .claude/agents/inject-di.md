# Inject-DI Refactoring Agent

Wire new services into the samber/do dependency injection container.

## Purpose

Properly integrate new services into the DI container, following established patterns for service registration, dependency resolution, and lifecycle management.

## Input

- **Service interface or struct** to register
- **Dependencies** the service requires
- Example: `NewMetricsService` depending on `ConfigService`

## Process

### 1. Analyze Service Requirements

Determine:
- **Service type**: Singleton (one instance) or Transient (new per invocation)
- **Dependencies**: What other services does this service need?
- **Lifecycle**: Does it need cleanup on shutdown?
- **Naming**: Does it need a named registration (multiple instances)?

### 2. Create Service Wrapper Type

Reference: @.claude/skills/samber-do.md

From cc-relay di/providers.go pattern:

```go
// Service wrapper types provide type safety and allow distinguishing
// between similar types in the DI container.

// MetricsService wraps the metrics collector.
type MetricsService struct {
    Collector *metrics.Collector
}
```

**Why wrappers?**
- Type safety: `*ConfigService` vs `*config.Config` prevents accidental resolution
- Documentation: Wrapper struct shows what the DI provides
- Lifecycle: Wrapper can implement `do.Shutdowner`

### 3. Create Provider Function

Provider functions follow this signature: `func(do.Injector) (T, error)`

```go
// NewMetrics creates the metrics collector.
func NewMetrics(i do.Injector) (*MetricsService, error) {
    // Resolve dependencies
    cfgSvc := do.MustInvoke[*ConfigService](i)

    // Create service
    collector, err := metrics.NewCollector(cfgSvc.Config.Metrics)
    if err != nil {
        return nil, fmt.Errorf("failed to create metrics collector: %w", err)
    }

    return &MetricsService{Collector: collector}, nil
}
```

**Error handling:**
- Return wrapped errors with context
- Use `do.MustInvoke` for required dependencies (panics on missing)
- Use `do.Invoke` for optional dependencies (returns error)

### 4. Implement Shutdown (if needed)

If the service holds resources that need cleanup:

```go
// Shutdown implements do.Shutdowner for graceful cleanup.
func (m *MetricsService) Shutdown() error {
    if m.Collector != nil {
        return m.Collector.Close()
    }
    return nil
}
```

From cc-relay CacheService example:
```go
func (c *CacheService) Shutdown() error {
    if c.Cache != nil {
        return c.Cache.Close()
    }
    return nil
}
```

### 5. Register in RegisterSingletons

Add to `cmd/cc-relay/di/providers.go`:

```go
func RegisterSingletons(i do.Injector) {
    // Existing registrations...
    do.Provide(i, NewConfig)
    do.Provide(i, NewCache)
    do.Provide(i, NewProviderMap)
    do.Provide(i, NewKeyPool)
    do.Provide(i, NewProxyHandler)
    do.Provide(i, NewHTTPServer)

    // New registration
    do.Provide(i, NewMetrics)  // Add in dependency order
}
```

**Registration order matters:**
1. Config (no dependencies)
2. Infrastructure (Cache, depends on Config)
3. Domain services (Providers, KeyPool, depends on Config)
4. Application services (Handler, depends on domain)
5. Server (depends on Handler)

### 6. Use Named Registration (if multiple instances)

For services with multiple named instances:

```go
// Register multiple providers by name
func registerNamedProviders(i do.Injector) {
    do.ProvideNamed(i, "anthropic", NewAnthropicProvider)
    do.ProvideNamed(i, "zai", NewZAIProvider)
    do.ProvideNamed(i, "ollama", NewOllamaProvider)
}

// Resolve by name
provider, err := do.InvokeNamed[providers.Provider](i, "anthropic")
```

### 7. Add Tests

Create test file `di/providers_test.go`:

```go
func TestNewMetrics(t *testing.T) {
    // Setup container with dependencies
    i := do.New()
    do.ProvideNamedValue(i, ConfigPathKey, "../../testdata/config.yaml")
    do.Provide(i, NewConfig)
    do.Provide(i, NewMetrics)

    // Test invocation
    svc, err := do.Invoke[*MetricsService](i)
    require.NoError(t, err)
    require.NotNil(t, svc)
    require.NotNil(t, svc.Collector)

    // Test shutdown
    err = svc.Shutdown()
    require.NoError(t, err)
}

func TestNewMetrics_DependencyOrder(t *testing.T) {
    // Verify service can be resolved with full container
    i := do.New()
    do.ProvideNamedValue(i, ConfigPathKey, "../../testdata/config.yaml")
    RegisterSingletons(i)

    // Should resolve without error
    svc, err := do.Invoke[*MetricsService](i)
    require.NoError(t, err)
    require.NotNil(t, svc)
}
```

## cc-relay Examples

### CacheService (di/providers.go)

```go
// CacheService wraps the cache implementation.
type CacheService struct {
    Cache cache.Cache
}

// NewCache creates the cache based on configuration.
func NewCache(i do.Injector) (*CacheService, error) {
    cfgSvc := do.MustInvoke[*ConfigService](i)

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    c, err := cache.New(ctx, &cfgSvc.Config.Cache)
    if err != nil {
        return nil, fmt.Errorf("failed to create cache: %w", err)
    }

    return &CacheService{Cache: c}, nil
}

// Shutdown implements do.Shutdowner for graceful cache cleanup.
func (c *CacheService) Shutdown() error {
    if c.Cache != nil {
        return c.Cache.Close()
    }
    return nil
}
```

### KeyPoolService (di/providers.go)

```go
// KeyPoolService wraps the optional key pool.
type KeyPoolService struct {
    Pool *keypool.KeyPool
}

// NewKeyPool creates the key pool for the primary provider if pooling is enabled.
func NewKeyPool(i do.Injector) (*KeyPoolService, error) {
    cfgSvc := do.MustInvoke[*ConfigService](i)
    cfg := cfgSvc.Config

    for idx := range cfg.Providers {
        p := &cfg.Providers[idx]
        if !p.Enabled {
            continue
        }

        if !p.IsPoolingEnabled() {
            return &KeyPoolService{Pool: nil}, nil
        }

        // Build pool configuration...
        pool, err := keypool.NewKeyPool(p.Name, poolCfg)
        if err != nil {
            return nil, fmt.Errorf("failed to create key pool: %w", err)
        }

        return &KeyPoolService{Pool: pool}, nil
    }

    return &KeyPoolService{Pool: nil}, nil
}
```

### ServerService with Shutdown (di/providers.go)

```go
// ServerService wraps the HTTP server.
type ServerService struct {
    Server *proxy.Server
}

// Shutdown implements do.Shutdowner for graceful server shutdown.
func (s *ServerService) Shutdown() error {
    if s.Server != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        return s.Server.Shutdown(ctx)
    }
    return nil
}
```

## Output

- Service wrapper type in `di/providers.go`
- Provider function in `di/providers.go`
- Registration in `RegisterSingletons()`
- Shutdown method (if resources need cleanup)
- Tests in `di/providers_test.go`

## Verification Checklist

- [ ] Service wrapper type created with clear documentation
- [ ] Provider function follows signature `func(do.Injector) (T, error)`
- [ ] Dependencies resolved with `do.MustInvoke`
- [ ] Errors wrapped with context
- [ ] Registered in `RegisterSingletons()` in correct order
- [ ] `Shutdown()` implemented if service holds resources
- [ ] Tests verify creation and shutdown
- [ ] Full container test verifies dependency resolution

## Anti-patterns to Avoid

### 1. Circular Dependencies

```go
// DON'T create A -> B -> A cycles
func NewA(i do.Injector) (*A, error) {
    b := do.MustInvoke[*B](i)  // B invokes A...
    return &A{B: b}, nil
}

// DO break cycle with lazy resolution or interface
func NewA(i do.Injector) (*A, error) {
    return &A{
        getB: func() *B { return do.MustInvoke[*B](i) },
    }, nil
}
```

### 2. Storing Request Data in Singletons

```go
// DON'T store per-request data in singleton
type Handler struct {
    CurrentRequest *http.Request  // Wrong! Shared across requests
}

// DO resolve per-request data from context/scope
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    scope := r.Context().Value(scopeKey).(do.Injector)
    logger := do.MustInvoke[*zerolog.Logger](scope)  // Request-scoped
}
```

### 3. Not Implementing Shutdowner

```go
// DON'T forget cleanup for resources
type DBService struct {
    DB *sql.DB  // Connection needs closing!
}

// DO implement Shutdowner
func (d *DBService) Shutdown() error {
    if d.DB != nil {
        return d.DB.Close()
    }
    return nil
}
```

### 4. Over-injecting

```go
// DON'T inject everything
func NewHandler(i do.Injector) (*Handler, error) {
    cfg := do.MustInvoke[*ConfigService](i)
    cache := do.MustInvoke[*CacheService](i)
    pool := do.MustInvoke[*KeyPoolService](i)
    providers := do.MustInvoke[*ProviderMapService](i)
    metrics := do.MustInvoke[*MetricsService](i)
    tracer := do.MustInvoke[*TracerService](i)
    logger := do.MustInvoke[*LoggerService](i)
    // 7+ dependencies = code smell
}

// DO compose smaller services or use facades
```

## Related Skills

- @.claude/skills/samber-do.md - Full do reference
- @.claude/skills/di-patterns.md - DI patterns guide

## Example Invocation

```
/refactor inject-di MetricsService --deps ConfigService
```

Or for a new service:

```
/refactor inject-di TracingService --deps ConfigService,LoggerService --shutdown
```
