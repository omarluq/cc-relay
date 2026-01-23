# Dependency Injection Patterns

Patterns and best practices for using samber/do v2 dependency injection in Go applications.

**Reference:** @.claude/skills/samber-do.md for API details

## Core Concepts

### Service Lifecycles

| Lifecycle | Registration | Created | Destroyed | Use Case |
|-----------|--------------|---------|-----------|----------|
| **Singleton** | `do.Provide` | First invoke | Container shutdown | Config, pools, connections |
| **Transient** | `do.ProvideTransient` | Every invoke | Never (GC) | Loggers, temp objects |
| **Eager** | `do.Provide` + immediate invoke | At registration | Container shutdown | Preload services |
| **Request-scoped** | `do.Provide` on scope | First invoke in scope | Scope shutdown | Request loggers, context |

### cc-relay Example: Service Wrapper Pattern

```go
// di/providers.go

// Service wrapper types provide type safety and clear documentation.
// They allow distinguishing between similar types in the DI container.

// ConfigService wraps the loaded configuration.
type ConfigService struct {
    Config *config.Config
}

// CacheService wraps the cache implementation.
type CacheService struct {
    Cache cache.Cache
}

// KeyPoolService wraps the optional key pool.
type KeyPoolService struct {
    Pool *keypool.KeyPool  // Can be nil if pooling disabled
}
```

**Benefits:**
- Type safety: `*ConfigService` vs `*config.Config`
- Documentation: Clear what DI provides
- Lifecycle: Wrapper can implement `do.Shutdowner`
- Nil handling: Wrapper exists even when underlying is nil

## Pattern 1: Singleton Services

Most services are singletons - created once, shared across application.

```go
// Registration
func RegisterSingletons(i do.Injector) {
    do.Provide(i, NewConfig)
    do.Provide(i, NewCache)
    do.Provide(i, NewKeyPool)
    do.Provide(i, NewHandler)
}

// Provider function - created on first invoke
func NewConfig(i do.Injector) (*ConfigService, error) {
    path := do.MustInvokeNamed[string](i, ConfigPathKey)
    cfg, err := config.Load(path)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }
    return &ConfigService{Config: cfg}, nil
}
```

**When to use:**
- Long-lived services (config, pools, connections)
- Expensive initialization
- Shared state (with proper synchronization)

## Pattern 2: Transient Services

New instance created on every invoke.

```go
// Registration
do.ProvideTransient(i, NewRequestLogger)

// Provider - new instance each time
func NewRequestLogger(i do.Injector) (*zerolog.Logger, error) {
    requestID := uuid.New().String()
    logger := log.With().Str("request_id", requestID).Logger()
    return &logger, nil
}

// Usage - each invoke gets fresh logger
logger1 := do.MustInvoke[*zerolog.Logger](i)
logger2 := do.MustInvoke[*zerolog.Logger](i)
// logger1 != logger2 (different request IDs)
```

**When to use:**
- Per-request loggers
- Temporary objects
- Stateless utilities

## Pattern 3: Request-Scoped Services

Child scopes inherit parent services but can have request-specific overrides.

```go
// Middleware creating request scope
func RequestScopeMiddleware(root do.Injector) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Create child scope
            scope := do.NewScope(root)

            // Provide request-specific values
            requestID := r.Header.Get("X-Request-ID")
            if requestID == "" {
                requestID = uuid.New().String()
            }
            do.ProvideValue(scope, requestID)
            do.ProvideValue(scope, r.Context())

            // Request-scoped logger
            do.Provide(scope, func(i do.Injector) (*zerolog.Logger, error) {
                reqID := do.MustInvoke[string](i)
                logger := log.With().Str("request_id", reqID).Logger()
                return &logger, nil
            })

            // Store scope in context
            ctx := context.WithValue(r.Context(), scopeKey, scope)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

// Handler using request scope
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    scope := r.Context().Value(scopeKey).(do.Injector)

    // Get request-scoped logger (inherits parent config)
    logger := do.MustInvoke[*zerolog.Logger](scope)
    logger.Info().Msg("handling request")

    // Get singleton from parent
    cfg := do.MustInvoke[*ConfigService](scope)  // Resolved from parent
}
```

**When to use:**
- Per-request logging with request ID
- Auth context
- Tracing spans
- Request-specific configuration

## Pattern 4: Named Services

Multiple instances of the same type, distinguished by name.

```go
// Registration with names
func registerProviders(i do.Injector) {
    do.ProvideNamed(i, "anthropic", NewAnthropicProvider)
    do.ProvideNamed(i, "zai", NewZAIProvider)
    do.ProvideNamed(i, "ollama", NewOllamaProvider)
}

// Resolution by name
func getProvider(i do.Injector, name string) (providers.Provider, error) {
    return do.InvokeNamed[providers.Provider](i, name)
}

// cc-relay example: Config path as named value
const ConfigPathKey = "config_path"

// Registration
do.ProvideNamedValue(i, ConfigPathKey, "/path/to/config.yaml")

// Resolution
path := do.MustInvokeNamed[string](i, ConfigPathKey)
```

**When to use:**
- Multiple database connections (primary, replica)
- Multiple providers (anthropic, zai, ollama)
- Configuration values
- Multi-tenant services

## Pattern 5: Lifecycle Management (Shutdown)

Services implementing `do.Shutdowner` get automatic cleanup.

```go
// cc-relay CacheService with shutdown
type CacheService struct {
    Cache cache.Cache
}

// Shutdown implements do.Shutdowner
func (c *CacheService) Shutdown() error {
    if c.Cache != nil {
        return c.Cache.Close()
    }
    return nil
}

// cc-relay ServerService with shutdown
type ServerService struct {
    Server *proxy.Server
}

func (s *ServerService) Shutdown() error {
    if s.Server != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        return s.Server.Shutdown(ctx)
    }
    return nil
}

// Container shutdown calls all Shutdowners
func gracefulShutdown(container *di.Container) {
    if err := container.Shutdown(); err != nil {
        log.Error().Err(err).Msg("Shutdown error")
    }
}
```

**Shutdown order:**
- Services shut down in reverse dependency order
- If A depends on B, A shuts down before B
- Errors are collected, not short-circuited

## Pattern 6: Eager Initialization

Force service creation at startup (fail fast).

```go
// cc-relay NewContainer with eager validation
func NewContainer(configPath string) (*Container, error) {
    i := do.New()

    // Register services
    do.ProvideNamedValue(i, ConfigPathKey, configPath)
    RegisterSingletons(i)

    // Eagerly invoke config to fail fast on invalid config
    _, err := do.Invoke[*ConfigService](i)
    if err != nil {
        return nil, fmt.Errorf("container initialization failed: %w", err)
    }

    return &Container{injector: i}, nil
}
```

**When to use:**
- Config validation at startup
- Database connection verification
- Service health checks during boot

## Pattern 7: Testing with DI

Easy mocking by providing test implementations.

```go
func TestHandler(t *testing.T) {
    // Create test container
    i := do.New()

    // Provide mock config
    mockCfg := &config.Config{
        Server: config.ServerConfig{Port: 8080},
    }
    do.ProvideValue(i, &ConfigService{Config: mockCfg})

    // Provide mock cache
    mockCache := &MockCache{}
    do.ProvideValue(i, &CacheService{Cache: mockCache})

    // Provide mock key pool (nil = single key mode)
    do.ProvideValue(i, &KeyPoolService{Pool: nil})

    // Provider under test still uses do.Provide
    do.Provide(i, NewHandler)

    // Resolve and test
    handler := do.MustInvoke[*HandlerService](i)
    // ... test handler
}

func TestContainerIntegration(t *testing.T) {
    // Full integration test
    i := do.New()
    do.ProvideNamedValue(i, ConfigPathKey, "testdata/config.yaml")
    RegisterSingletons(i)

    // Verify all services resolve
    _, err := do.Invoke[*ConfigService](i)
    require.NoError(t, err)

    _, err = do.Invoke[*CacheService](i)
    require.NoError(t, err)

    // Cleanup
    err = do.Shutdown(i)
    require.NoError(t, err)
}
```

## Anti-patterns

### 1. Circular Dependencies

```go
// BAD: A -> B -> A
func NewA(i do.Injector) (*A, error) {
    b := do.MustInvoke[*B](i)  // B invokes A = panic
    return &A{b: b}, nil
}

// GOOD: Break cycle with lazy resolution
func NewA(i do.Injector) (*A, error) {
    return &A{
        getB: func() *B { return do.MustInvoke[*B](i) },
    }, nil
}

// BETTER: Restructure to avoid cycle
type A struct { /* no B reference */ }
type B struct { a *A }
```

### 2. Storing Request Data in Singletons

```go
// BAD: Shared across all requests
type Handler struct {
    CurrentUser *User  // Race condition!
}

// GOOD: Resolve per-request
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    scope := r.Context().Value(scopeKey).(do.Injector)
    user := do.MustInvoke[*User](scope)  // Request-scoped
}
```

### 3. Scope Leaks

```go
// BAD: Storing scoped object in long-lived struct
type Handler struct {
    logger *zerolog.Logger  // Scoped logger in singleton!
}

func NewHandler(scope do.Injector) (*Handler, error) {
    logger := do.MustInvoke[*zerolog.Logger](scope)
    return &Handler{logger: logger}, nil  // Wrong!
}

// GOOD: Resolve per-request
func (h *Handler) handle(r *http.Request) {
    scope := r.Context().Value(scopeKey).(do.Injector)
    logger := do.MustInvoke[*zerolog.Logger](scope)
    logger.Info().Msg("request")
}
```

### 4. Over-injection (God Service)

```go
// BAD: 7+ dependencies = code smell
func NewMegaService(i do.Injector) (*MegaService, error) {
    cfg := do.MustInvoke[*ConfigService](i)
    cache := do.MustInvoke[*CacheService](i)
    pool := do.MustInvoke[*KeyPoolService](i)
    providers := do.MustInvoke[*ProviderMapService](i)
    metrics := do.MustInvoke[*MetricsService](i)
    tracer := do.MustInvoke[*TracerService](i)
    logger := do.MustInvoke[*LoggerService](i)
    // Too many!
}

// GOOD: Compose smaller services
type RequestPipeline struct {
    Auth     *AuthService
    Routing  *RoutingService
    Handler  *HandlerService
}
```

### 5. Not Implementing Shutdowner

```go
// BAD: Resource leak on shutdown
type DBService struct {
    conn *sql.DB
}

// GOOD: Implement Shutdowner
func (d *DBService) Shutdown() error {
    if d.conn != nil {
        return d.conn.Close()
    }
    return nil
}
```

## Decision Guide

| Question | Answer | Pattern |
|----------|--------|---------|
| One instance for whole app? | Yes | Singleton (`do.Provide`) |
| New instance per call? | Yes | Transient (`do.ProvideTransient`) |
| Per-request data? | Yes | Request scope (`do.NewScope`) |
| Multiple instances by name? | Yes | Named (`do.ProvideNamed`) |
| Needs cleanup? | Yes | Implement `Shutdown()` |
| Must validate at startup? | Yes | Eager invoke after registration |

## Related Skills

- @.claude/skills/samber-do.md - API reference
- @.claude/agents/inject-di.md - Wiring new services
