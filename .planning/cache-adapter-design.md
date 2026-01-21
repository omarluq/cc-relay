# Cache Adapter Interface Design

**Created:** 2026-01-21
**Author:** architect-agent
**Status:** Design Document

## Overview

This document defines a unified cache adapter interface for cc-relay that abstracts over three cache backends:
1. **Single mode (Ristretto)** - High-performance local in-memory cache
2. **HA mode (Olric)** - Distributed cache for high-availability deployments
3. **Disabled mode (Noop)** - Passthrough when caching is disabled

The design prioritizes a consistent API surface while accommodating the different semantics of local vs distributed caching.

## Requirements

- [ ] Unified interface for all cache backends
- [ ] Context support for cancellation and timeouts (required by Olric)
- [ ] Error-based return values (distributed operations can fail)
- [ ] Serialization flexibility via `[]byte` values
- [ ] TTL support for cache entries
- [ ] Metrics/stats collection for observability
- [ ] Factory pattern for configuration-driven instantiation
- [ ] Graceful shutdown support

## Design

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     cc-relay core                           │
│                          │                                  │
│                          ▼                                  │
│                    Cache Interface                          │
│                          │                                  │
│          ┌───────────────┼───────────────┐                  │
│          │               │               │                  │
│          ▼               ▼               ▼                  │
│   ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│   │  Ristretto │  │    Olric   │  │    Noop    │           │
│   │  Adapter   │  │   Adapter  │  │   Adapter  │           │
│   └────────────┘  └────────────┘  └────────────┘           │
│          │               │               │                  │
│          ▼               ▼               ▼                  │
│   ┌────────────┐  ┌────────────┐  ┌────────────┐           │
│   │ dgraph.io  │  │  buraksezer│  │   (none)   │           │
│   │ /ristretto │  │   /olric   │  │            │           │
│   └────────────┘  └────────────┘  └────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

### Interface Definition

```go
// Package cache provides a unified caching interface for cc-relay.
package cache

import (
	"context"
	"errors"
	"time"
)

// Standard errors for cache operations.
var (
	// ErrNotFound is returned when a key does not exist in the cache.
	ErrNotFound = errors.New("cache: key not found")

	// ErrClosed is returned when operations are attempted on a closed cache.
	ErrClosed = errors.New("cache: cache is closed")

	// ErrSerializationFailed is returned when value serialization fails.
	ErrSerializationFailed = errors.New("cache: serialization failed")
)

// Cache defines the interface for cache operations.
// All implementations must be safe for concurrent use.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns ErrNotFound if the key does not exist.
	// Returns ErrClosed if the cache has been closed.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with no expiration.
	// Returns ErrClosed if the cache has been closed.
	Set(ctx context.Context, key string, value []byte) error

	// SetWithTTL stores a value in the cache with a time-to-live.
	// After the TTL expires, the key will no longer be retrievable.
	// Returns ErrClosed if the cache has been closed.
	SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key from the cache.
	// Returns nil if the key does not exist (idempotent).
	// Returns ErrClosed if the cache has been closed.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	// Returns ErrClosed if the cache has been closed.
	Exists(ctx context.Context, key string) (bool, error)

	// Close releases resources associated with the cache.
	// After Close is called, all operations will return ErrClosed.
	// Close is idempotent.
	Close() error
}

// Stats provides cache statistics for observability.
type Stats struct {
	// Hits is the number of cache hits.
	Hits uint64 `json:"hits"`

	// Misses is the number of cache misses.
	Misses uint64 `json:"misses"`

	// KeyCount is the current number of keys in the cache.
	KeyCount uint64 `json:"key_count"`

	// BytesUsed is the approximate memory used by cached values.
	BytesUsed uint64 `json:"bytes_used"`

	// Evictions is the number of keys evicted due to capacity limits.
	Evictions uint64 `json:"evictions"`
}

// StatsProvider is an optional interface for caches that support statistics.
type StatsProvider interface {
	// Stats returns current cache statistics.
	Stats() Stats
}

// Pinger is an optional interface for caches that support health checks.
type Pinger interface {
	// Ping verifies the cache connection is alive.
	// For local caches, this always returns nil.
	// For distributed caches, this validates cluster connectivity.
	Ping(ctx context.Context) error
}

// MultiGetter is an optional interface for batch operations.
type MultiGetter interface {
	// GetMulti retrieves multiple values from the cache.
	// Missing keys are not included in the result map (no error).
	// Returns a map of key -> value for found keys.
	GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)
}

// MultiSetter is an optional interface for batch operations.
type MultiSetter interface {
	// SetMulti stores multiple values in the cache.
	// If any key fails, returns error but may have set some keys.
	SetMulti(ctx context.Context, items map[string][]byte) error

	// SetMultiWithTTL stores multiple values with a common TTL.
	SetMultiWithTTL(ctx context.Context, items map[string][]byte, ttl time.Duration) error
}
```

### Adapter Implementations Overview

#### 1. Ristretto Adapter (`internal/cache/ristretto.go`)

```go
// ristrettoCache adapts dgraph-io/ristretto to the Cache interface.
type ristrettoCache struct {
	cache  *ristretto.Cache
	closed atomic.Bool
	mu     sync.RWMutex
}

// Implementation notes:
// - Get: cache.Get(key) returns (interface{}, bool)
//        Convert to []byte, return ErrNotFound if !found
// - Set: cache.Set(key, value, cost) returns bool
//        cost = len(value) for memory tracking
//        Ristretto Set is async; use SetWithTTL for sync
// - SetWithTTL: cache.SetWithTTL(key, value, cost, ttl)
// - Delete: cache.Del(key) - no return value, always succeeds
// - Exists: cache.Get(key) and check bool
// - Close: cache.Close() - wait for buffers to flush
// - Stats: cache.Metrics provides hits/misses/etc
//
// Context handling:
// - Ristretto is local, so context is only checked for cancellation
// - All operations check ctx.Err() before proceeding
```

#### 2. Olric Adapter (`internal/cache/olric.go`)

```go
// olricCache adapts buraksezer/olric to the Cache interface.
type olricCache struct {
	client *olric.ClusterClient  // or embedded *olric.Olric
	dmap   olric.DMap            // distributed map handle
	name   string                // DMap name
	closed atomic.Bool
}

// Implementation notes:
// - Get: dmap.Get(ctx, key) returns (olric.GetResponse, error)
//        response.Byte() to get []byte value
//        Check for olric.ErrKeyNotFound -> ErrNotFound
// - Set: dmap.Put(ctx, key, value) with options
//        dmap.Put(ctx, key, value, olric.EX(ttl)) for TTL
// - Delete: dmap.Delete(ctx, key) - supports multiple keys
// - Exists: Use Get and check error (no native Exists)
// - Close: client.Close() / olric.Shutdown(ctx)
// - Ping: client.Ping(ctx) or Stats() call
//
// Context handling:
// - All Olric operations accept context natively
// - Context used for timeout, cancellation, and deadline
//
// Cluster modes:
// - Embedded: olric.New(cfg) starts local node
// - Client: olric.NewClusterClient(addrs) connects to existing cluster
```

#### 3. Noop Adapter (`internal/cache/noop.go`)

```go
// noopCache is a no-operation cache that stores nothing.
// Used when caching is disabled.
type noopCache struct {
	closed atomic.Bool
}

// Implementation notes:
// - Get: always returns ErrNotFound
// - Set: always returns nil (success, but stores nothing)
// - SetWithTTL: always returns nil
// - Delete: always returns nil
// - Exists: always returns false, nil
// - Close: sets closed flag, idempotent
// - Stats: returns zeroed Stats{}
```

### Factory Function Design

```go
// Mode represents the cache operating mode.
type Mode string

const (
	// ModeSingle uses local Ristretto cache (default).
	ModeSingle Mode = "single"

	// ModeHA uses distributed Olric cache for high availability.
	ModeHA Mode = "ha"

	// ModeDisabled uses noop cache (caching disabled).
	ModeDisabled Mode = "disabled"
)

// Config defines cache configuration.
type Config struct {
	// Mode selects the cache backend: "single", "ha", or "disabled".
	Mode Mode `yaml:"mode"`

	// Ristretto configuration (used when mode: single).
	Ristretto RistrettoConfig `yaml:"ristretto"`

	// Olric configuration (used when mode: ha).
	Olric OlricConfig `yaml:"olric"`
}

// RistrettoConfig configures the Ristretto local cache.
type RistrettoConfig struct {
	// NumCounters is the number of 4-bit access counters.
	// Recommended: 10x expected max items.
	NumCounters int64 `yaml:"num_counters"`

	// MaxCost is the maximum cost (memory) the cache can hold.
	// Cost is measured in bytes of cached values.
	MaxCost int64 `yaml:"max_cost"`

	// BufferItems is the number of keys per Get buffer.
	// Recommended: 64 (default).
	BufferItems int64 `yaml:"buffer_items"`
}

// OlricConfig configures the Olric distributed cache.
type OlricConfig struct {
	// Addresses is a list of Olric cluster member addresses.
	// Used when connecting as a client to an existing cluster.
	Addresses []string `yaml:"addresses"`

	// DMapName is the name of the distributed map to use.
	// Default: "cc-relay".
	DMapName string `yaml:"dmap_name"`

	// Embedded starts an embedded Olric node instead of connecting as client.
	// Useful for single-node HA setups or development.
	Embedded bool `yaml:"embedded"`

	// BindAddr is the address for the embedded node to bind to.
	// Only used when Embedded: true.
	BindAddr string `yaml:"bind_addr"`

	// Peers is a list of peer addresses for cluster discovery.
	// Only used when Embedded: true.
	Peers []string `yaml:"peers"`
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	switch c.Mode {
	case ModeSingle:
		if c.Ristretto.MaxCost <= 0 {
			return errors.New("cache: ristretto.max_cost must be positive")
		}
	case ModeHA:
		if !c.Olric.Embedded && len(c.Olric.Addresses) == 0 {
			return errors.New("cache: olric.addresses required when not embedded")
		}
	case ModeDisabled:
		// No validation needed
	case "":
		return errors.New("cache: mode is required")
	default:
		return fmt.Errorf("cache: unknown mode %q", c.Mode)
	}
	return nil
}

// New creates a new Cache based on the configuration.
func New(ctx context.Context, cfg Config) (Cache, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	switch cfg.Mode {
	case ModeSingle:
		return newRistrettoCache(cfg.Ristretto)
	case ModeHA:
		return newOlricCache(ctx, cfg.Olric)
	case ModeDisabled:
		return newNoopCache(), nil
	default:
		return nil, fmt.Errorf("cache: unknown mode %q", cfg.Mode)
	}
}
```

### Configuration Structure

Add to `internal/config/config.go`:

```go
// Config represents the complete cc-relay configuration.
type Config struct {
	Providers []ProviderConfig `yaml:"providers"`
	Logging   LoggingConfig    `yaml:"logging"`
	Server    ServerConfig     `yaml:"server"`
	Cache     CacheConfig      `yaml:"cache"` // NEW
}

// CacheConfig defines cache settings.
// Imported from internal/cache package.
type CacheConfig = cache.Config
```

Example YAML configuration:

```yaml
# ============================================================================
# Cache Configuration
# ============================================================================
cache:
  # Mode: "single" (local), "ha" (distributed), or "disabled"
  mode: single

  # Ristretto settings (used when mode: single)
  ristretto:
    # Number of counters for admission policy (~10x expected items)
    num_counters: 1000000  # 1M counters for ~100K items
    # Maximum memory for cached values (in bytes)
    max_cost: 104857600    # 100 MB
    # Buffer items per Get shard
    buffer_items: 64

  # Olric settings (used when mode: ha)
  olric:
    # Connect to existing cluster
    addresses:
      - "olric-1:3320"
      - "olric-2:3320"
    # Distributed map name
    dmap_name: "cc-relay"

    # OR run embedded node
    # embedded: true
    # bind_addr: "0.0.0.0:3320"
    # peers:
    #   - "olric-1:3320"
    #   - "olric-2:3320"
```

## Data Flow

### Cache Hit Flow

```
1. Request arrives at proxy
2. Generate cache key from request (model, hash of prompt, etc.)
3. cache.Get(ctx, key)
   ├── Ristretto: sync memory lookup
   ├── Olric: network call to cluster
   └── Noop: returns ErrNotFound
4. If found: deserialize and return cached response
5. If ErrNotFound: proceed to backend provider
```

### Cache Write Flow

```
1. Receive response from backend provider
2. Determine if cacheable (non-streaming, successful, etc.)
3. Serialize response to []byte
4. Compute cost (response size)
5. cache.SetWithTTL(ctx, key, value, ttl)
   ├── Ristretto: async write to local memory
   ├── Olric: sync write to cluster
   └── Noop: no-op, returns nil
```

## Dependencies

| Dependency | Type | Reason | Version |
|------------|------|--------|---------|
| `github.com/dgraph-io/ristretto` | External | Local high-performance cache | v0.1.1+ |
| `github.com/buraksezer/olric` | External | Distributed cache with clustering | v0.5.4+ |
| `sync/atomic` | Standard | Thread-safe closed flag | N/A |
| `context` | Standard | Cancellation and timeout | N/A |

## Implementation Phases

### Phase 1: Foundation
**Files to create:**
- `internal/cache/cache.go` - Interface and type definitions
- `internal/cache/errors.go` - Standard errors
- `internal/cache/config.go` - Configuration types

**Acceptance:**
- [ ] Interface compiles
- [ ] Config validation works
- [ ] Errors are documented

**Estimated effort:** Small

### Phase 2: Noop Implementation
**Files to create:**
- `internal/cache/noop.go` - Noop adapter
- `internal/cache/noop_test.go` - Unit tests

**Dependencies:** Phase 1

**Acceptance:**
- [ ] All interface methods implemented
- [ ] Unit tests pass
- [ ] Thread-safe operations verified

**Estimated effort:** Small

### Phase 3: Ristretto Implementation
**Files to create:**
- `internal/cache/ristretto.go` - Ristretto adapter
- `internal/cache/ristretto_test.go` - Unit tests

**Dependencies:** Phase 1, Phase 2 (for test patterns)

**Acceptance:**
- [ ] All interface methods implemented
- [ ] Stats interface implemented
- [ ] TTL behavior verified
- [ ] Context cancellation tested
- [ ] Benchmark tests added

**Estimated effort:** Medium

### Phase 4: Olric Implementation
**Files to create:**
- `internal/cache/olric.go` - Olric adapter
- `internal/cache/olric_test.go` - Unit tests (requires test cluster)

**Dependencies:** Phase 1, Phase 2, Phase 3

**Acceptance:**
- [ ] Client mode implemented
- [ ] Embedded mode implemented
- [ ] Ping health check works
- [ ] Context timeout respected
- [ ] Integration tests with docker-compose

**Estimated effort:** Medium-Large

### Phase 5: Factory and Integration
**Files to create:**
- `internal/cache/factory.go` - New() factory function
- `internal/cache/factory_test.go` - Factory tests

**Files to modify:**
- `internal/config/config.go` - Add CacheConfig
- `internal/config/loader.go` - Load cache config

**Dependencies:** Phases 1-4

**Acceptance:**
- [ ] Factory creates correct backend per config
- [ ] Config hot-reload for non-HA modes
- [ ] Integration with cc-relay startup

**Estimated effort:** Small

### Phase 6: Documentation
**Files to create:**
- `docs/caching.md` - User documentation
- `internal/cache/README.md` - Developer guide

**Files to modify:**
- `example.yaml` - Add cache configuration examples

**Estimated effort:** Small

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Olric cluster complexity | High | Start with embedded mode for simpler deployment; document cluster setup |
| Ristretto async writes | Medium | Use SetWithTTL which is synchronous; document behavior |
| Serialization overhead | Medium | Use efficient formats (msgpack, protobuf); benchmark |
| Cache stampede on cold start | High | Implement singleflight for concurrent requests to same key |
| Memory pressure (Ristretto) | Medium | Proper cost accounting; configurable limits |
| Network partitions (Olric) | High | Circuit breaker; fallback to no-cache on errors |

## Open Questions

- [ ] Should we implement singleflight to prevent cache stampede?
- [ ] What serialization format for cached responses? (JSON, msgpack, protobuf)
- [ ] Should cache key include user identity for per-user caching?
- [ ] How to handle streaming responses? (likely skip caching)
- [ ] Should stats be exposed via gRPC management API?

## Success Criteria

1. All three cache modes (single, ha, disabled) work correctly
2. Context cancellation is properly handled across all backends
3. TTL expiration is accurate within reasonable tolerance
4. No data races under concurrent access (verified by race detector)
5. Stats are accurate and exposed for metrics collection
6. Configuration validation catches common errors early
7. Graceful degradation when HA backend is unavailable

## Usage Examples

### Basic Usage

```go
import "github.com/yourusername/cc-relay/internal/cache"

func main() {
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1e6,
			MaxCost:     100 << 20, // 100 MB
			BufferItems: 64,
		},
	}

	c, err := cache.New(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Store a value
	err = c.SetWithTTL(ctx, "prompt:abc123", responseBytes, 5*time.Minute)
	if err != nil {
		log.Printf("cache set failed: %v", err)
	}

	// Retrieve a value
	data, err := c.Get(ctx, "prompt:abc123")
	if errors.Is(err, cache.ErrNotFound) {
		// Cache miss - fetch from backend
	} else if err != nil {
		log.Printf("cache get failed: %v", err)
	} else {
		// Cache hit - use data
	}
}
```

### Checking Stats

```go
if sp, ok := c.(cache.StatsProvider); ok {
	stats := sp.Stats()
	hitRate := float64(stats.Hits) / float64(stats.Hits+stats.Misses)
	log.Printf("Cache hit rate: %.2f%%, keys: %d, bytes: %d",
		hitRate*100, stats.KeyCount, stats.BytesUsed)
}
```

### Health Check for HA Mode

```go
if p, ok := c.(cache.Pinger); ok {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := p.Ping(ctx); err != nil {
		log.Printf("Cache cluster unhealthy: %v", err)
	}
}
```

### Type Assertion Pattern for Optional Interfaces

```go
// Check if batch operations are supported
if mg, ok := c.(cache.MultiGetter); ok {
	keys := []string{"key1", "key2", "key3"}
	results, err := mg.GetMulti(ctx, keys)
	// Use batch result
}

// Fallback to individual gets if not supported
for _, key := range keys {
	data, err := c.Get(ctx, key)
	// Handle each individually
}
```

## Related Documents

- [SPEC.md](/home/omarluq/sandbox/go/cc-relay/SPEC.md) - Project specification
- [example.yaml](/home/omarluq/sandbox/go/cc-relay/example.yaml) - Configuration examples
