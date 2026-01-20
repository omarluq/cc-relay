# Coding Conventions

**Analysis Date:** 2026-01-20

## Naming Patterns

**Files:**
- Source files use lowercase with hyphens for multi-word names (following Go convention)
- Component-based organization: Provider implementations in separate files (`anthropic.go`, `zai.go`, `ollama.go`, `bedrock.go`, `azure.go`, `vertex.go`)
- Router strategy implementations in dedicated files under `internal/router/strategies/` (`shuffle.go`, `roundrobin.go`, `leastbusy.go`, etc.)
- Test files follow Go convention: `filename_test.go` co-located with source files
- Example: `internal/proxy/server.go`, `internal/router/strategies/failover.go`, `internal/health/circuit.go`

**Packages:**
- Lowercase, single-word package names following Go standard library patterns
- Main packages: `proxy`, `router`, `providers`, `health`, `config`, `grpc`
- Subpackages for related implementations: `router/strategies`, `ui/tui`

**Functions:**
- PascalCase for exported functions (Go convention): `TransformRequest()`, `HealthCheck()`, `StreamStats()`
- camelCase for unexported functions: `selectProvider()`, `routeRequest()`, `trackMetrics()`
- Interface methods are action verbs: `Authenticate()`, `Transform*()`, `Check()`, `Handle()`
- Test functions follow pattern: `Test[FunctionName]` or `Test[FunctionName]_[Scenario]`
  - Examples: `TestProxyHandler`, `TestCircuitBreakerTransition`, `TestRoundRobinDistribution_WithMultipleKeys`

**Variables:**
- camelCase for all variables (Go convention)
- Boolean variables prefixed with `is`, `has`, `can`, `should`: `isHealthy`, `hasCapacity`, `shouldFailover`
- Slice/map variables use plural nouns: `providers`, `keys`, `requests`, `strategies`
- Context variables: `ctx` (standard Go convention)
- Error variables: `err` (single letter in tight scopes, descriptive names in broader scopes)
- Constants: UPPERCASE_WITH_UNDERSCORES for package-level constants (Go convention)

**Types:**
- PascalCase for all type names: `ProviderTransformer`, `CircuitBreaker`, `RoutingStrategy`, `HealthStatus`
- Interface types suffix with `er`: `ProviderTransformer`, `StrategySelector` (or just named for behavior like `Reader`)
- Request/response types use `Request`/`Response` suffix: `AddKeyRequest`, `KeyUsage`, `ProviderStats`
- Struct fields use PascalCase: `Name`, `Type`, `Keys`, `StatusCode`

## Code Style

**Formatting:**
- `go fmt` - automatic formatting (standard Go tooling)
- `goimports` recommended for import organization
- Line length: follow Go convention (~100-120 characters, but no strict limit)
- Indentation: tabs (Go standard)

**Linting:**
- Follow `golangci-lint` recommendations where applicable
- Run: `golangci-lint run ./...`
- Pre-commit hook can include: `go vet` for all Go files

**Error Handling:**
- Always return error as last return value: `(result T, err error)`
- Check errors immediately after operations that can fail
- Use explicit error checks, avoid `panic()` in library code
- Wrap errors with context: `fmt.Errorf("provider %s: %w", name, err)`
- Define custom error types for provider-specific failures

**Logging:**
- Use structured logging (consider `log/slog` in stdlib or `zerolog` for production)
- Log levels: `DEBUG`, `INFO`, `WARN`, `ERROR`
- Include context: timestamp, provider/key name, operation, result
- Pattern: Log at operation boundaries (request received, provider selected, response sent)

## Import Organization

**Order:**
1. Standard library imports (`fmt`, `net/http`, `context`, `encoding/json`)
2. External third-party imports (`google.golang.org/grpc`, `github.com/charmbracelet/bubbletea`)
3. Internal project imports (`github.com/omarluq/cc-relay/internal/proxy`)

**Path Aliases:**
- `proto` for gRPC generated code: `import protoRelay "github.com/omarluq/cc-relay/proto"`
- Short aliases for frequently used packages: `proto`, `cfg` (for config)
- No dots in import paths (avoid importing as `.`)

**Grouping:**
- Blank lines separate each group
- Use `goimports` to maintain automatic organization

## Error Handling

**Patterns:**
- Custom error types for provider-specific failures:
  ```go
  type ProviderError struct {
      Provider string
      Code     int
      Message  string
      Wrapped  error
  }
  ```
- Rate limit errors (429) trigger health tracker
- Timeout errors trigger circuit breaker open state
- 5xx errors increment failure counter
- Explicit nil checks before operations: `if provider == nil { return errors.New("provider not found") }`
- Use `errors.Is()` and `errors.As()` for error type checking (Go 1.13+)

**Error Messages:**
- Start with lowercase (Go convention)
- Be descriptive about what failed and why
- Include context: `"failed to route request to provider %s: %w"`
- Avoid generic "error" strings

## Comments

**When to Comment:**
- Export all public functions/types with doc comments (Go convention)
- Complex business logic (e.g., circuit breaker transitions, rate limit calculation)
- Non-obvious algorithm choices
- Security-sensitive code (auth, key handling)
- Breaking changes or deprecations

**Doc Comments:**
- Start with the symbol name: `// HealthCheck validates provider connectivity`
- First sentence should stand alone as a summary
- Exported packages should have a `// Package <name>` doc comment at the top
- Examples in doc comments for complex types

**Inline Comments:**
- Use `//` for inline comments
- Keep brief (same line or few lines above)
- Avoid obvious comments: don't comment `i++` in loops

**Pattern:**
```go
// ProviderTransformer adapts requests/responses between Anthropic API format
// and provider-specific formats. Each provider implementation must handle
// authentication, request transformation, and response normalization.
type ProviderTransformer interface {
    // TransformRequest converts an Anthropic Messages API request to provider format
    TransformRequest(req *AnthropicRequest) (*http.Request, error)
}
```

## Function Design

**Size:**
- Prefer small, focused functions (typical range: 10-40 lines)
- Extract helper functions for repeated logic
- Maximum nesting depth: 3 levels (triggers refactoring)

**Parameters:**
- Avoid function parameter lists longer than 3-4 items
- Use struct for option-like parameters: `func NewServer(cfg *config.Config, opts *ServerOptions) (*Server, error)`
- Context parameter always first: `func (p *Provider) HealthCheck(ctx context.Context) error`

**Return Values:**
- Return errors as last value (Go convention)
- Use `(T, error)` pattern for functions that can fail
- Named return values only when needed for clarity (rare in this project)
- Single responsibility: function returns one main result type plus error

**Receiver Type:**
- Use pointer receivers for methods that modify state: `func (r *Router) SelectProvider() (...)`
- Use value receivers for read-only operations or small types: `func (s Status) String() string`

## Module Design

**Exports:**
- Exported types: `ProviderConfig`, `RoutingStrategy`, `HealthStatus`
- Exported functions: `New[Type]()` for constructors, action verbs for operations
- Keep package APIs small and focused (single responsibility)
- Use internal packages (`internal/`) to hide implementation details

**Barrel Files:**
- No barrel exports pattern used; each subpackage is independent
- `internal/router/strategies/` contains individual strategy implementations
- Import specific strategy packages as needed

**Package Structure:**
```
internal/proxy/        # HTTP server, SSE handler, middleware
internal/router/       # Router interface, key pool, strategy selection
  └── strategies/      # Individual routing strategy implementations
internal/providers/    # Provider transformer interface and implementations
internal/health/       # Circuit breaker, health tracking
internal/config/       # Configuration loading and validation
internal/grpc/         # gRPC server implementation
ui/tui/               # TUI application (Bubble Tea)
cmd/cc-relay/         # CLI entry point
proto/                # gRPC protobuf definitions (generated code)
```

## Concurrency Patterns

**Goroutines:**
- Use goroutines for I/O-bound operations (provider calls, health checks)
- Spawn goroutines sparingly; pool them when possible
- Always pass `context.Context` for cancellation
- Use channels for synchronization, not global mutexes where possible

**Mutexes:**
- Protect shared state with `sync.RWMutex` or `sync.Mutex`
- Lock scope should be minimal (lock right before access, unlock right after)
- Avoid holding locks during I/O operations
- Document which mutex protects which fields in struct comments

**Context Usage:**
- Pass context through all request paths for timeout/cancellation
- Propagate deadline from client to provider calls
- Use `context.WithTimeout()` for health checks to avoid hangs
- Always check `ctx.Done()` in long-running operations

## API Design

**HTTP Endpoints:**
- Proxy endpoint: `POST /v1/messages` (Anthropic API compatible)
- Response format: Match Anthropic API exactly for SSE streaming
- Headers: Preserve required headers (`x-api-key`, `anthropic-version`, `content-type`)
- Status codes: Return same status as upstream provider (for error semantics)

**gRPC Service:**
- Service defined in `relay.proto` (see proto file for details)
- Message naming: `PascalCase` for types
- RPC naming: camelCase for method names
- Streaming: Use `stream` keyword for server-streaming responses

## Configuration

**Config Structure:**
- Top-level: `server`, `routing`, `providers`, `grpc`, `logging`, `metrics`, `health`
- Environment variable expansion: `${VAR_NAME}` syntax
- YAML primary format, TOML supported
- Example: `example.yaml` (216 lines, comprehensive reference)

**Validation:**
- Validate provider configs on startup
- Ensure routing strategy is valid before use
- Check API keys are not empty
- Verify listen address is valid

## Type Organization

**Request/Response Structs:**
- Follow Anthropic API format for messages
- Include metadata: timestamp, provider name, strategy used
- Use `json` tags for HTTP marshaling: `json:"field_name"`
- Use `protobuf` tags for gRPC: `protobuf:"bytes,1,opt,name=field_name"`

---

*Convention analysis: 2026-01-20*
