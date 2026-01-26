# Phase 7: Configuration Management - Research

**Researched:** 2026-01-25
**Domain:** Go configuration loading, hot-reload, multi-format support
**Confidence:** HIGH

## Summary

This phase implements comprehensive configuration management for cc-relay, adding TOML support alongside existing YAML, environment variable expansion (already implemented via `os.ExpandEnv`), validation on startup (partially implemented), and hot-reload via file watching. The codebase already has a solid foundation with `internal/config/loader.go` using `gopkg.in/yaml.v3` and `os.ExpandEnv` for environment variable expansion.

The standard approach for Go configuration hot-reload uses `fsnotify` for file system notifications combined with `sync/atomic.Pointer[T]` for thread-safe config swapping. This pattern allows zero-downtime configuration updates without affecting in-flight requests. For TOML support, `github.com/pelletier/go-toml/v2` is the performance leader (2.7-5.1x faster than BurntSushi/toml) and follows the same conventions as `encoding/json`.

**Primary recommendation:** Add `github.com/fsnotify/fsnotify` for file watching and `github.com/pelletier/go-toml/v2` for TOML support. Implement config reload using atomic pointer swap pattern with debouncing to handle rapid file changes.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) | v1.9.0 | File system notifications | De facto standard, 12,768+ downstream packages, cross-platform (inotify/kqueue/ReadDirectoryChangesW) |
| [pelletier/go-toml/v2](https://github.com/pelletier/go-toml/v2) | v2.x | TOML parsing | 2.7-5.1x faster than BurntSushi/toml, follows encoding/json conventions |
| gopkg.in/yaml.v3 | v3.0.1 | YAML parsing | Already in use, stable, de facto standard |
| sync/atomic | stdlib | Thread-safe config swap | Built-in, lock-free reads for high-performance concurrent access |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| path/filepath | stdlib | Path manipulation | Detecting file extension for format selection |
| os | stdlib | Environment expansion | `os.ExpandEnv` already used for ${VAR_NAME} syntax |
| time | stdlib | Debounce timer | Rate-limiting rapid file change events |
| context | stdlib | Graceful shutdown | Stopping file watcher on server shutdown |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-toml/v2 | BurntSushi/toml v1.6.0 | Slower (2.7-5.1x), but more widely used historically |
| fsnotify | polling | Works on NFS/SMB but higher CPU usage, not needed here |
| atomic.Pointer | sync.RWMutex | RWMutex works but atomic.Pointer has better read performance |

**Installation:**
```bash
go get github.com/fsnotify/fsnotify@v1.9.0
go get github.com/pelletier/go-toml/v2
```

## Architecture Patterns

### Recommended Project Structure
```
internal/config/
├── config.go        # Config struct definitions (exists)
├── loader.go        # Load/LoadFromReader (exists, extend)
├── loader_test.go   # Tests (exists, extend)
├── watcher.go       # NEW: File watcher with debounce
├── watcher_test.go  # NEW: Watcher tests
└── validator.go     # NEW: Comprehensive validation
```

### Pattern 1: Atomic Config Swap
**What:** Use `atomic.Pointer[Config]` for lock-free config reads, atomic swaps on reload
**When to use:** Hot-reload scenario with many concurrent readers, few writes
**Example:**
```go
// Source: Go stdlib sync/atomic, verified via official docs
type ConfigHolder struct {
    current atomic.Pointer[config.Config]
}

func (h *ConfigHolder) Get() *config.Config {
    return h.current.Load()
}

func (h *ConfigHolder) Swap(newCfg *config.Config) {
    h.current.Store(newCfg)
}
```

### Pattern 2: Debounced File Watcher
**What:** Watch parent directory (not file), debounce events, filter by filename
**When to use:** Any file watching scenario - editors create temp files, rapid saves
**Example:**
```go
// Source: fsnotify best practices + official docs
func (w *Watcher) watchLoop(ctx context.Context) {
    var debounceTimer *time.Timer
    debounceDelay := 100 * time.Millisecond

    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-w.watcher.Events:
            if !ok {
                return
            }
            // Filter: only our config file, only Write events
            if filepath.Base(event.Name) != w.filename {
                continue
            }
            if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
                continue
            }
            // Debounce: reset timer on each event
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            debounceTimer = time.AfterFunc(debounceDelay, w.reload)
        case err, ok := <-w.watcher.Errors:
            if !ok {
                return
            }
            w.logger.Error().Err(err).Msg("config watcher error")
        }
    }
}
```

### Pattern 3: Format Detection by Extension
**What:** Detect config format from file extension, support multiple formats
**When to use:** When supporting YAML and TOML from the same code path
**Example:**
```go
// Load detects format from extension and parses accordingly
func Load(path string) (*Config, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
    }

    // Expand environment variables
    expanded := os.ExpandEnv(string(content))

    var cfg Config
    switch strings.ToLower(filepath.Ext(path)) {
    case ".toml":
        if err := toml.Unmarshal([]byte(expanded), &cfg); err != nil {
            return nil, fmt.Errorf("failed to parse config TOML: %w", err)
        }
    case ".yaml", ".yml":
        if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
            return nil, fmt.Errorf("failed to parse config YAML: %w", err)
        }
    default:
        return nil, fmt.Errorf("unsupported config format: %s (use .yaml, .yml, or .toml)", filepath.Ext(path))
    }

    return &cfg, nil
}
```

### Pattern 4: Graceful Reload Without Dropping Requests
**What:** Reload config atomically, existing requests continue with old config, new requests use new config
**When to use:** Production hot-reload requirement (CONF-05)
**Example:**
```go
// Watcher notifies via callback when config changes
type ReloadCallback func(newCfg *config.Config, oldCfg *config.Config) error

// In DI container or serve command
watcher.OnReload(func(newCfg, oldCfg *config.Config) error {
    // Update atomic pointer - readers see new config immediately
    configHolder.Swap(newCfg)

    // Optionally: update dependent services if needed
    // Router, KeyPools, etc. can read from configHolder.Get()

    logger.Info().
        Str("path", configPath).
        Msg("configuration reloaded")
    return nil
})
```

### Anti-Patterns to Avoid
- **Watching files directly:** Editors use atomic writes (write to temp, rename). Watch the parent directory and filter by filename.
- **No debouncing:** Editors may trigger multiple events per save. Always debounce with 50-100ms delay.
- **Blocking on reload:** Never block request handling during config reload. Use atomic swap.
- **Ignoring Chmod events:** Spotlight, antivirus, and backup software trigger Chmod constantly. Filter them out.
- **Mutating config after Store:** Treat config as immutable once stored. Always create new config on reload.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| File watching | Custom inotify wrapper | fsnotify | Cross-platform, handles edge cases, battle-tested |
| TOML parsing | Regex/manual parsing | go-toml/v2 | Handles all TOML edge cases, proper error messages |
| Thread-safe config | Custom mutex wrapper | atomic.Pointer[T] | Lock-free reads, stdlib, no race conditions |
| Env var expansion | Custom ${VAR} parser | os.ExpandEnv | Handles edge cases ($VAR, ${VAR}, escaping) |
| Debouncing | Manual timer management | time.AfterFunc pattern | Simple, correct, well-understood |

**Key insight:** File watching looks simple but has many platform-specific edge cases (editor temp files, atomic renames, permissions). fsnotify handles all of these across Windows/Linux/macOS/BSD.

## Common Pitfalls

### Pitfall 1: Watching Files Instead of Directories
**What goes wrong:** Config file disappears on edit (editors do atomic write via temp file + rename)
**Why it happens:** Watching `/path/to/config.yaml` directly misses the temp file dance
**How to avoid:** Watch parent directory, filter events by `filepath.Base(event.Name)`
**Warning signs:** Config reload works in tests but not with Vim/VSCode/JetBrains

### Pitfall 2: No Debouncing on File Events
**What goes wrong:** Multiple reloads per save, potential race conditions, wasted CPU
**Why it happens:** Editors generate multiple events (Write, Chmod, sometimes Create)
**How to avoid:** Debounce with 50-100ms timer, reset timer on each event
**Warning signs:** Log shows multiple "config reloaded" messages per save

### Pitfall 3: Blocking Requests During Reload
**What goes wrong:** Request latency spikes during config reload, potential timeouts
**Why it happens:** Using RWMutex with write lock during reload
**How to avoid:** Use atomic.Pointer swap - readers never block
**Warning signs:** P99 latency spikes correlate with config changes

### Pitfall 4: Forgetting to Validate Before Swap
**What goes wrong:** Invalid config breaks the entire proxy
**Why it happens:** Reload path skips validation that startup path has
**How to avoid:** Always validate new config before atomic swap, reject invalid
**Warning signs:** Bad config file crashes running proxy instead of being rejected

### Pitfall 5: NFS/SMB/Network Filesystem Expectations
**What goes wrong:** File watching doesn't work on network filesystems
**Why it happens:** NFS/SMB protocols don't support inotify-style notifications
**How to avoid:** Document limitation, consider optional polling fallback
**Warning signs:** Hot-reload works locally but not when config is on NFS mount

### Pitfall 6: TOML Tag Mismatch with YAML Tags
**What goes wrong:** TOML file loads but fields are empty/zero
**Why it happens:** Using `yaml:"field_name"` tag but TOML uses `toml:"field_name"`
**How to avoid:** Add both tags to struct fields: `yaml:"field" toml:"field"`
**Warning signs:** YAML config works, identical TOML config has missing values

## Code Examples

Verified patterns from official sources:

### Complete Config Loader with Format Detection
```go
// Source: go-toml/v2 docs + yaml.v3 docs
package config

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    toml "github.com/pelletier/go-toml/v2"
    "gopkg.in/yaml.v3"
)

// Load reads and parses a configuration file, detecting format from extension.
// Environment variables in ${VAR_NAME} syntax are expanded before parsing.
func Load(path string) (*Config, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
    }

    // Expand environment variables before parsing
    expanded := os.ExpandEnv(string(content))

    var cfg Config
    ext := strings.ToLower(filepath.Ext(path))

    switch ext {
    case ".toml":
        if err := toml.Unmarshal([]byte(expanded), &cfg); err != nil {
            return nil, fmt.Errorf("failed to parse TOML config: %w", err)
        }
    case ".yaml", ".yml":
        if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
            return nil, fmt.Errorf("failed to parse YAML config: %w", err)
        }
    default:
        return nil, fmt.Errorf("unsupported config format %q (use .yaml, .yml, or .toml)", ext)
    }

    if err := cfg.Validate(); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }

    return &cfg, nil
}
```

### Config Watcher with Debounce
```go
// Source: fsnotify official docs + best practices
package config

import (
    "context"
    "path/filepath"
    "sync"
    "time"

    "github.com/fsnotify/fsnotify"
    "github.com/rs/zerolog"
)

// Watcher monitors a config file for changes and triggers reload.
type Watcher struct {
    watcher   *fsnotify.Watcher
    path      string
    dir       string
    filename  string
    logger    *zerolog.Logger
    callbacks []func(*Config) error
    mu        sync.RWMutex
}

// NewWatcher creates a config file watcher.
func NewWatcher(configPath string, logger *zerolog.Logger) (*Watcher, error) {
    fsw, err := fsnotify.NewWatcher()
    if err != nil {
        return nil, fmt.Errorf("failed to create file watcher: %w", err)
    }

    absPath, err := filepath.Abs(configPath)
    if err != nil {
        fsw.Close()
        return nil, fmt.Errorf("failed to resolve config path: %w", err)
    }

    w := &Watcher{
        watcher:  fsw,
        path:     absPath,
        dir:      filepath.Dir(absPath),
        filename: filepath.Base(absPath),
        logger:   logger,
    }

    // Watch the directory, not the file (handles atomic writes)
    if err := fsw.Add(w.dir); err != nil {
        fsw.Close()
        return nil, fmt.Errorf("failed to watch config directory: %w", err)
    }

    return w, nil
}

// OnReload registers a callback for config changes.
func (w *Watcher) OnReload(cb func(*Config) error) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.callbacks = append(w.callbacks, cb)
}

// Start begins watching for config changes.
func (w *Watcher) Start(ctx context.Context) {
    go w.watchLoop(ctx)
}

// Close stops watching and releases resources.
func (w *Watcher) Close() error {
    return w.watcher.Close()
}

func (w *Watcher) watchLoop(ctx context.Context) {
    var debounceTimer *time.Timer
    const debounceDelay = 100 * time.Millisecond

    for {
        select {
        case <-ctx.Done():
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            return

        case event, ok := <-w.watcher.Events:
            if !ok {
                return
            }

            // Filter: only our config file
            if filepath.Base(event.Name) != w.filename {
                continue
            }

            // Filter: only Write or Create events (ignore Chmod, Remove, Rename)
            if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
                continue
            }

            w.logger.Debug().
                Str("event", event.Op.String()).
                Str("file", event.Name).
                Msg("config file change detected")

            // Debounce: reset timer on each event
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            debounceTimer = time.AfterFunc(debounceDelay, func() {
                w.triggerReload()
            })

        case err, ok := <-w.watcher.Errors:
            if !ok {
                return
            }
            w.logger.Error().Err(err).Msg("config watcher error")
        }
    }
}

func (w *Watcher) triggerReload() {
    cfg, err := Load(w.path)
    if err != nil {
        w.logger.Error().Err(err).Msg("failed to reload config, keeping current")
        return
    }

    w.mu.RLock()
    callbacks := w.callbacks
    w.mu.RUnlock()

    for _, cb := range callbacks {
        if err := cb(cfg); err != nil {
            w.logger.Error().Err(err).Msg("config reload callback failed")
        }
    }

    w.logger.Info().Str("path", w.path).Msg("configuration reloaded successfully")
}
```

### Config Struct with Both YAML and TOML Tags
```go
// Source: Existing codebase pattern, extended
// For dual-format support, add toml tags alongside yaml tags

type Config struct {
    Providers []ProviderConfig `yaml:"providers" toml:"providers"`
    Routing   RoutingConfig    `yaml:"routing" toml:"routing"`
    Logging   LoggingConfig    `yaml:"logging" toml:"logging"`
    Health    health.Config    `yaml:"health" toml:"health"`
    Server    ServerConfig     `yaml:"server" toml:"server"`
    Cache     cache.Config     `yaml:"cache" toml:"cache"`
}

type ServerConfig struct {
    Listen        string     `yaml:"listen" toml:"listen"`
    APIKey        string     `yaml:"api_key" toml:"api_key"`
    Auth          AuthConfig `yaml:"auth" toml:"auth"`
    TimeoutMS     int        `yaml:"timeout_ms" toml:"timeout_ms"`
    MaxConcurrent int        `yaml:"max_concurrent" toml:"max_concurrent"`
    EnableHTTP2   bool       `yaml:"enable_http2" toml:"enable_http2"`
}

// ... repeat for all config structs
```

### Comprehensive Config Validation
```go
// Source: Best practices for fail-fast validation
package config

import (
    "errors"
    "fmt"
    "net"
    "strings"
)

// Validate performs comprehensive validation of the configuration.
// Returns a detailed error describing all validation failures.
func (c *Config) Validate() error {
    var errs []error

    // Server validation
    if c.Server.Listen == "" {
        errs = append(errs, errors.New("server.listen is required"))
    } else if _, _, err := net.SplitHostPort(c.Server.Listen); err != nil {
        errs = append(errs, fmt.Errorf("server.listen %q is not valid host:port: %w", c.Server.Listen, err))
    }

    if c.Server.TimeoutMS < 0 {
        errs = append(errs, errors.New("server.timeout_ms must be >= 0"))
    }

    if c.Server.MaxConcurrent < 0 {
        errs = append(errs, errors.New("server.max_concurrent must be >= 0"))
    }

    // Provider validation
    if len(c.Providers) == 0 {
        errs = append(errs, errors.New("at least one provider is required"))
    }

    providerNames := make(map[string]bool)
    for i, p := range c.Providers {
        prefix := fmt.Sprintf("providers[%d]", i)

        if p.Name == "" {
            errs = append(errs, fmt.Errorf("%s.name is required", prefix))
        } else if providerNames[p.Name] {
            errs = append(errs, fmt.Errorf("%s.name %q is duplicate", prefix, p.Name))
        } else {
            providerNames[p.Name] = true
        }

        if p.Type == "" {
            errs = append(errs, fmt.Errorf("%s.type is required", prefix))
        } else if !isValidProviderType(p.Type) {
            errs = append(errs, fmt.Errorf("%s.type %q is invalid (use: anthropic, zai, ollama, bedrock, vertex, azure)", prefix, p.Type))
        }

        if p.Enabled && len(p.Keys) == 0 {
            errs = append(errs, fmt.Errorf("%s: enabled provider must have at least one key", prefix))
        }

        for j, k := range p.Keys {
            if err := k.Validate(); err != nil {
                errs = append(errs, fmt.Errorf("%s.keys[%d]: %w", prefix, j, err))
            }
        }

        if err := p.ValidateCloudConfig(); err != nil {
            errs = append(errs, fmt.Errorf("%s: %w", prefix, err))
        }
    }

    // Routing validation
    if !isValidRoutingStrategy(c.Routing.Strategy) && c.Routing.Strategy != "" {
        errs = append(errs, fmt.Errorf("routing.strategy %q is invalid (use: round_robin, weighted_round_robin, shuffle, failover, model_based)", c.Routing.Strategy))
    }

    // Logging validation
    if c.Logging.Level != "" && !isValidLogLevel(c.Logging.Level) {
        errs = append(errs, fmt.Errorf("logging.level %q is invalid (use: debug, info, warn, error)", c.Logging.Level))
    }

    if len(errs) > 0 {
        return &ValidationError{Errors: errs}
    }
    return nil
}

// ValidationError contains all validation failures.
type ValidationError struct {
    Errors []error
}

func (e *ValidationError) Error() string {
    if len(e.Errors) == 1 {
        return e.Errors[0].Error()
    }
    var b strings.Builder
    b.WriteString(fmt.Sprintf("%d validation errors:\n", len(e.Errors)))
    for _, err := range e.Errors {
        b.WriteString("  - ")
        b.WriteString(err.Error())
        b.WriteString("\n")
    }
    return b.String()
}

func isValidProviderType(t string) bool {
    switch t {
    case "anthropic", "zai", "ollama", "bedrock", "vertex", "azure":
        return true
    }
    return false
}

func isValidRoutingStrategy(s string) bool {
    switch s {
    case "round_robin", "weighted_round_robin", "shuffle", "failover", "model_based":
        return true
    }
    return false
}

func isValidLogLevel(l string) bool {
    switch strings.ToLower(l) {
    case "debug", "info", "warn", "error":
        return true
    }
    return false
}
```

### DI Integration for Hot-Reload
```go
// Source: samber/do patterns + atomic operations
package di

import (
    "context"
    "sync/atomic"

    "github.com/omarluq/cc-relay/internal/config"
)

// ConfigService wraps the loaded configuration with hot-reload support.
type ConfigService struct {
    current atomic.Pointer[config.Config]
    path    string
    watcher *config.Watcher
}

// Get returns the current configuration (lock-free read).
func (s *ConfigService) Get() *config.Config {
    return s.current.Load()
}

// StartWatching begins watching for config file changes.
func (s *ConfigService) StartWatching(ctx context.Context) error {
    s.watcher.OnReload(func(newCfg *config.Config) error {
        s.current.Store(newCfg)
        return nil
    })
    s.watcher.Start(ctx)
    return nil
}

// Shutdown implements do.Shutdowner for graceful cleanup.
func (s *ConfigService) Shutdown() error {
    if s.watcher != nil {
        return s.watcher.Close()
    }
    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| sync.RWMutex for config | atomic.Pointer[T] (Go 1.19+) | Go 1.19 (2022) | Lock-free reads, better performance |
| BurntSushi/toml | pelletier/go-toml/v2 | 2022 | 2.7-5.1x faster parsing |
| Custom file polling | fsnotify | Stable since 2015 | Cross-platform, low CPU |
| Watch file directly | Watch parent directory | Best practice | Handles atomic editor writes |

**Deprecated/outdated:**
- `sync/atomic.Value`: Still works but `atomic.Pointer[T]` is type-safe (Go 1.19+)
- Manual inotify: Use fsnotify for cross-platform support
- Viper (for simple cases): Overkill for this project, brings many dependencies

## Open Questions

Things that couldn't be fully resolved:

1. **Partial Config Reload vs Full Reload**
   - What we know: Full reload is simpler and safer
   - What's unclear: Should we support partial updates (e.g., just add a key)?
   - Recommendation: Start with full reload, add partial if needed later

2. **SIGHUP Signal Handling**
   - What we know: SPEC mentions SIGHUP for manual reload trigger
   - What's unclear: Should this be in this phase or separate?
   - Recommendation: Include basic SIGHUP handler alongside file watcher

3. **Config Reload Notification to Services**
   - What we know: Some services (KeyPool, Router) cache config values
   - What's unclear: Do they need explicit notification or just re-read config?
   - Recommendation: Phase 1: services re-read config.Get() on next request

## Sources

### Primary (HIGH confidence)
- [fsnotify/fsnotify GitHub](https://github.com/fsnotify/fsnotify) - API, best practices, v1.9.0
- [pkg.go.dev/fsnotify](https://pkg.go.dev/github.com/fsnotify/fsnotify) - Full API documentation
- [pelletier/go-toml/v2 GitHub](https://github.com/pelletier/go-toml/v2) - API, performance benchmarks
- [pkg.go.dev/go-toml/v2](https://pkg.go.dev/github.com/pelletier/go-toml/v2) - Full API documentation
- [BurntSushi/toml GitHub](https://github.com/BurntSushi/toml) - Alternative TOML library, v1.6.0
- Go stdlib sync/atomic - atomic.Pointer[T] for config swap

### Secondary (MEDIUM confidence)
- [ITNEXT: Hot-reloading on Go applications](https://itnext.io/clean-and-simple-hot-reloading-on-uninterrupted-go-applications-5974230ab4c5) - Pattern validation
- [Medium: Config Hot-Reloading System](https://mediuntecharticles.medium.com/building-a-config-hot-reloading-system-in-go-without-restarting-the-server-b780e0a950b5) - Implementation patterns
- [Go Optimization Guide: Atomic Operations](https://goperf.dev/01-common-patterns/atomic-ops/) - Performance considerations
- [Openmymind: Golang Hot Configuration Reload](https://www.openmymind.net/Golang-Hot-Configuration-Reload/) - Practical patterns

### Tertiary (LOW confidence)
- Various Stack Overflow answers on fsnotify debouncing (common patterns)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Verified via official docs, go.mod shows existing yaml.v3 usage
- Architecture: HIGH - Patterns verified from multiple official sources, existing codebase structure analyzed
- Pitfalls: HIGH - Documented in fsnotify README and multiple community sources

**Research date:** 2026-01-25
**Valid until:** 60 days (stable libraries, patterns unlikely to change)

---

## Implementation Notes for Planner

### What Already Exists
1. `internal/config/loader.go` - YAML loading with `os.ExpandEnv` (CONF-03 partially done)
2. `internal/config/config.go` - Config struct with `yaml` tags, partial validation
3. `cmd/cc-relay/di/providers.go` - DI container with ConfigService wrapper
4. `internal/config/loader_test.go` - Tests for YAML loading and env expansion

### What Needs to Be Added
1. TOML tags on all config structs (CONF-02)
2. Format detection in Load() function (CONF-01, CONF-02)
3. Comprehensive Validate() method (CONF-04, CONF-06)
4. Watcher type with debounce (CONF-05)
5. DI integration for hot-reload (CONF-05)
6. Tests for all new functionality

### Linter Considerations
- Functions must stay under 80 lines (funlen)
- Cyclomatic complexity max 10 (gocyclo)
- Cognitive complexity max 15 (gocognit)
- Split validation into helper functions to meet limits
