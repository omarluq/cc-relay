# Cache System

cc-relay provides a unified caching layer that supports multiple backends. This document covers configuration, usage patterns, high-availability clustering, and how to extend the cache with custom backends.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Cache Modes](#cache-modes)
4. [Cache Key Conventions](#cache-key-conventions)
5. [Configuration Reference](#configuration-reference)
6. [Cache Busting Strategies](#cache-busting-strategies)
7. [HA Clustering Guide](#ha-clustering-guide)
8. [Extending with New Backends](#extending-with-new-backends)
9. [Troubleshooting](#troubleshooting)
10. [API Reference](#api-reference)

## Overview

The cc-relay cache subsystem abstracts over three cache backends:

| Mode | Backend | Use Case |
|------|---------|----------|
| `single` | [Ristretto](https://github.com/dgraph-io/ristretto) | Single-instance deployments, high performance |
| `ha` | [Olric](https://github.com/buraksezer/olric) | Multi-instance deployments, shared state |
| `disabled` | Noop | No caching, passthrough |

### When to Use Each Mode

**Choose `single` (default) when:**
- Running a single cc-relay instance
- Maximum performance is critical
- No shared state needed between instances

**Choose `ha` when:**
- Running multiple cc-relay instances
- Cache state must be shared across instances
- Fault tolerance is required

**Choose `disabled` when:**
- Debugging cache-related issues
- Cache is handled at another layer
- Testing without caching overhead

All cache implementations are safe for concurrent use and share a common interface.

## Quick Start

### Minimal Configuration (Single Mode)

```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000
    max_cost: 104857600  # 100 MB
    buffer_items: 64
```

### Basic Usage in Go

```go
package main

import (
    "context"
    "errors"
    "log"
    "time"

    "github.com/omarluq/cc-relay/internal/cache"
)

func main() {
    // Create configuration
    cfg := &cache.Config{
        Mode:      cache.ModeSingle,
        Ristretto: cache.DefaultRistrettoConfig(),
    }

    // Initialize cache
    ctx := context.Background()
    c, err := cache.New(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Store a value with TTL
    key := "user:profile:12345"
    value := []byte(`{"name": "Alice", "role": "admin"}`)
    err = c.SetWithTTL(ctx, key, value, 5*time.Minute)
    if err != nil {
        log.Fatal(err)
    }

    // Retrieve a value
    data, err := c.Get(ctx, key)
    if errors.Is(err, cache.ErrNotFound) {
        log.Println("Cache miss - fetch from source")
        return
    }
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Cache hit: %s", data)
}
```

## Cache Modes

### Single Mode (Ristretto)

Ristretto is a high-performance, concurrent in-memory cache based on research from the Caffeine library. It uses a TinyLFU admission policy for optimal hit rates.

```yaml
cache:
  mode: single
  ristretto:
    num_counters: 1000000  # 10x expected max items
    max_cost: 104857600    # 100 MB
    buffer_items: 64       # Admission buffer size
```

**Characteristics:**
- Sub-microsecond Get/Set operations
- Automatic memory management
- TinyLFU admission for near-optimal eviction
- No network overhead

**Best for:** Single-instance deployments prioritizing latency.

### HA Mode (Olric)

Olric is a distributed in-memory key/value store with clustering support. In HA mode, cc-relay runs an embedded Olric node that participates in cluster membership.

```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "other-node:3322"
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```

**Characteristics:**
- Distributed hash table with consistent hashing
- Automatic partition rebalancing
- Configurable replication and quorum
- Built-in cluster membership via memberlist

**Best for:** Multi-instance deployments requiring shared cache state.

### Disabled Mode (Noop)

The noop cache is a passthrough that performs no actual caching. All operations return immediately.

```yaml
cache:
  mode: disabled
```

**Characteristics:**
- All Get operations return `ErrNotFound`
- All Set operations succeed immediately (no-op)
- Zero memory usage
- Useful for debugging and testing

**Best for:** Debugging, testing, or when caching is handled elsewhere.

## Cache Key Conventions

cc-relay uses structured cache keys for organization and debugging. While the cache accepts any string key, following these conventions improves maintainability.

### Key Format

```
{domain}:{type}:{identifier}
```

### Examples

| Domain | Type | Example Key | What It Caches |
|--------|------|-------------|----------------|
| provider | health | `provider:health:anthropic` | Provider health check result |
| provider | config | `provider:config:zai` | Provider configuration |
| response | hash | `response:hash:a1b2c3d4...` | Cached API response |
| model | list | `model:list:global` | Aggregated model list |
| user | session | `user:session:uuid-here` | User session data |
| rate | limit | `rate:limit:key-abc` | Rate limit counter |

### Best Practices

1. **Use lowercase**: Keys are case-sensitive; lowercase prevents confusion
2. **Use colons as separators**: Standard convention in caching systems
3. **Keep identifiers deterministic**: Same input should produce same key
4. **Avoid special characters**: Stick to alphanumeric, colons, hyphens, underscores
5. **Include version for cached responses**: `response:v1:hash` allows cache busting on format changes

### Key Generation Example

```go
func cacheKey(domain, keyType, id string) string {
    return fmt.Sprintf("%s:%s:%s", domain, keyType, id)
}

// Usage
key := cacheKey("provider", "health", "anthropic")
// Result: "provider:health:anthropic"
```

## Configuration Reference

### RistrettoConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `num_counters` | int64 | 1,000,000 | Number of 4-bit access counters. Recommended: 10x expected max items for optimal admission policy. |
| `max_cost` | int64 | 104,857,600 (100 MB) | Maximum memory in bytes the cache can hold. |
| `buffer_items` | int64 | 64 | Number of keys per Get buffer. Controls admission buffer size. |

**Default Configuration:**

```go
cache.DefaultRistrettoConfig() // Returns:
// NumCounters: 1_000_000
// MaxCost:     100 << 20 (100 MB)
// BufferItems: 64
```

**Sizing Guidelines:**

| Expected Items | NumCounters | MaxCost (assuming 1KB avg) |
|----------------|-------------|---------------------------|
| 10,000 | 100,000 | 10 MB |
| 100,000 | 1,000,000 | 100 MB |
| 1,000,000 | 10,000,000 | 1 GB |

### OlricConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dmap_name` | string | "cc-relay" | Name of the distributed map. All nodes must use the same name. |
| `bind_addr` | string | required | Address for Olric client connections (e.g., "0.0.0.0:3320"). |
| `environment` | string | "local" | Memberlist preset: "local", "lan", or "wan". See below. |
| `addresses` | []string | - | External Olric cluster addresses (client mode only). |
| `peers` | []string | - | Memberlist addresses for peer discovery (embedded mode). |
| `replica_count` | int | 1 | Number of copies per key. 1 = no replication. |
| `read_quorum` | int | 1 | Minimum successful reads for response. Must be <= replica_count. |
| `write_quorum` | int | 1 | Minimum successful writes for response. Must be <= replica_count. |
| `member_count_quorum` | int32 | 1 | Minimum cluster members required to operate. |
| `leave_timeout` | duration | 5s | Time to broadcast leave message before shutdown. |
| `embedded` | bool | false | Run embedded Olric node (true) vs. connect to external cluster (false). |

**Default Configuration:**

```go
cache.DefaultOlricConfig() // Returns:
// DMapName:          "cc-relay"
// Environment:       "local"
// ReplicaCount:      1
// ReadQuorum:        1
// WriteQuorum:       1
// MemberCountQuorum: 1
// LeaveTimeout:      5 * time.Second
```

**Environment Presets:**

| Environment | Use Case | Failure Detection | Network Overhead |
|-------------|----------|-------------------|------------------|
| `local` | Development, same-host | Fast (100ms) | Low |
| `lan` | Local network | Medium (500ms) | Medium |
| `wan` | Cross-datacenter | Slow (2s+) | Low |

### Complete Configuration Example

```yaml
# Single mode (development)
cache:
  mode: single
  ristretto:
    num_counters: 1000000
    max_cost: 104857600
    buffer_items: 64

---
# HA mode (production)
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "cc-relay-2:3322"
      - "cc-relay-3:3322"
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```

## Cache Busting Strategies

### 1. TTL-Based Expiration

Use `SetWithTTL()` to automatically expire entries after a specified duration.

```go
// Set with 5-minute TTL
err := cache.SetWithTTL(ctx, key, value, 5*time.Minute)

// Set with no expiration (permanent until evicted)
err := cache.Set(ctx, key, value)
```

**Recommended TTLs by Use Case:**

| Use Case | Recommended TTL | Rationale |
|----------|-----------------|-----------|
| Health checks | 10s - 30s | Frequent updates, stale data problematic |
| Model lists | 5m - 15m | Changes infrequently, moderate staleness OK |
| API responses | 1m - 5m | Depends on upstream cache headers |
| Configuration | 1m - 10m | Balance between freshness and performance |
| Session data | 30m - 24h | Depends on security requirements |

### 2. Manual Invalidation

Use `Delete()` to explicitly remove entries when the underlying data changes.

```go
// Delete a specific key
err := cache.Delete(ctx, "provider:health:anthropic")

// Delete is idempotent - no error if key doesn't exist
err := cache.Delete(ctx, "nonexistent:key")  // err == nil
```

**When to Use Manual Invalidation:**
- Data changed in source system
- Configuration updated
- User logged out (clear session)
- Provider status changed

### 3. Cluster Events (HA Mode)

In HA mode, Olric automatically handles cache consistency during cluster topology changes:

**Node Leave:**
- Olric detects node departure via memberlist
- Partitions owned by departed node are redistributed
- Replicated data remains available on surviving nodes
- No manual intervention required

**Node Join:**
- New node announced via memberlist
- Partition ownership rebalanced automatically
- New node begins accepting requests immediately

**Network Partition (Split-Brain Protection):**
- `member_count_quorum` prevents split-brain scenarios
- Minority partition becomes read-only or unavailable
- Majority partition continues operating normally

```yaml
# Require at least 2 nodes to operate
olric:
  member_count_quorum: 2
  replica_count: 2
```

## HA Clustering Guide

### Prerequisites

Before configuring HA mode:

1. **Network connectivity**: All nodes must be able to reach each other
2. **Port accessibility**: Both Olric and memberlist ports must be open
3. **Consistent configuration**: All nodes must use the same `dmap_name` and `environment`

### Port Requirements

**Critical:** Olric uses two ports:

| Port | Purpose | Default |
|------|---------|---------|
| `bind_addr` port | Olric client connections | 3320 |
| `bind_addr` port + 2 | Memberlist gossip protocol | 3322 |

**Example:** If `bind_addr: "0.0.0.0:3320"`, memberlist automatically uses port 3322.

Ensure both ports are open in firewalls:

```bash
# Allow Olric client port
sudo ufw allow 3320/tcp

# Allow memberlist gossip port (bind_addr port + 2)
sudo ufw allow 3322/tcp
```

### Environment Settings

| Setting | Gossip Interval | Probe Interval | Probe Timeout | Use When |
|---------|-----------------|----------------|---------------|----------|
| `local` | 100ms | 100ms | 200ms | Same host, development |
| `lan` | 200ms | 1s | 500ms | Same datacenter |
| `wan` | 500ms | 3s | 2s | Cross-datacenter |

**All nodes in a cluster must use the same environment setting.**

### Two-Node Cluster Example

**Node 1 (cc-relay-1):**

```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "cc-relay-2:3322"  # Memberlist port of node 2
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```

**Node 2 (cc-relay-2):**

```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "cc-relay-1:3322"  # Memberlist port of node 1
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```

### Three-Node Docker Compose Example

```yaml
version: '3.8'

services:
  cc-relay-1:
    image: cc-relay:latest
    environment:
      - CC_RELAY_CONFIG=/config/config.yaml
    volumes:
      - ./config-node1.yaml:/config/config.yaml:ro
    ports:
      - "8787:8787"   # HTTP proxy
      - "3320:3320"   # Olric client port
      - "3322:3322"   # Memberlist gossip port
    networks:
      - cc-relay-net

  cc-relay-2:
    image: cc-relay:latest
    environment:
      - CC_RELAY_CONFIG=/config/config.yaml
    volumes:
      - ./config-node2.yaml:/config/config.yaml:ro
    ports:
      - "8788:8787"
      - "3330:3320"
      - "3332:3322"
    networks:
      - cc-relay-net

  cc-relay-3:
    image: cc-relay:latest
    environment:
      - CC_RELAY_CONFIG=/config/config.yaml
    volumes:
      - ./config-node3.yaml:/config/config.yaml:ro
    ports:
      - "8789:8787"
      - "3340:3320"
      - "3342:3322"
    networks:
      - cc-relay-net

networks:
  cc-relay-net:
    driver: bridge
```

**config-node1.yaml:**

```yaml
cache:
  mode: ha
  olric:
    embedded: true
    bind_addr: "0.0.0.0:3320"
    dmap_name: "cc-relay"
    environment: lan
    peers:
      - "cc-relay-2:3322"
      - "cc-relay-3:3322"
    replica_count: 2
    read_quorum: 1
    write_quorum: 1
    member_count_quorum: 2
    leave_timeout: 5s
```

**config-node2.yaml and config-node3.yaml:** Same as node1, but with different peers lists pointing to the other nodes.

### Replication and Quorum Explained

**replica_count:** Number of copies of each key stored in the cluster.

| replica_count | Behavior |
|---------------|----------|
| 1 | No replication (single copy) |
| 2 | One primary + one backup |
| 3 | One primary + two backups |

**read_quorum / write_quorum:** Minimum successful operations before returning success.

| Setting | Consistency | Availability |
|---------|-------------|--------------|
| quorum = 1 | Eventual | High |
| quorum = replica_count | Strong | Lower |
| quorum = (replica_count/2)+1 | Majority | Balanced |

**Recommendations:**

| Cluster Size | replica_count | read_quorum | write_quorum | Fault Tolerance |
|--------------|---------------|-------------|--------------|-----------------|
| 2 nodes | 2 | 1 | 1 | 1 node failure |
| 3 nodes | 2 | 1 | 1 | 1 node failure |
| 3 nodes | 3 | 2 | 2 | 1 node failure (strong consistency) |

## Extending with New Backends

To add a new cache backend (e.g., Redis, Memcached), implement the `Cache` interface.

### Required Interface

```go
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
```

### Implementation Checklist

When implementing a new backend:

- [ ] Return `ErrNotFound` on cache miss (not nil, not custom error)
- [ ] Return `ErrClosed` after `Close()` is called
- [ ] Be safe for concurrent use (use mutexes or atomic operations)
- [ ] Copy byte slices before storing (don't retain caller's slice)
- [ ] Return copies from Get (don't share internal storage)
- [ ] Respect context cancellation and deadlines
- [ ] Handle connection failures gracefully
- [ ] Make `Close()` idempotent (safe to call multiple times)
- [ ] Make `Delete()` idempotent (no error if key doesn't exist)

### Optional Interfaces

Implement these for enhanced functionality:

```go
// StatsProvider - for metrics and observability
type StatsProvider interface {
    Stats() Stats
}

// Pinger - for health checks
type Pinger interface {
    Ping(ctx context.Context) error
}

// ClusterInfo - for distributed caches
type ClusterInfo interface {
    MemberlistAddr() string
    ClusterMembers() int
    IsEmbedded() bool
}

// MultiGetter - for efficient batch reads
type MultiGetter interface {
    GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)
}

// MultiSetter - for efficient batch writes
type MultiSetter interface {
    SetMulti(ctx context.Context, items map[string][]byte) error
    SetMultiWithTTL(ctx context.Context, items map[string][]byte, ttl time.Duration) error
}
```

### Example: Redis Backend Skeleton

```go
package cache

import (
    "context"
    "sync"
    "time"

    "github.com/redis/go-redis/v9"
)

type redisCache struct {
    client *redis.Client
    closed bool
    mu     sync.RWMutex
}

func newRedisCache(addr string) (*redisCache, error) {
    client := redis.NewClient(&redis.Options{
        Addr: addr,
    })

    // Verify connection
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, err
    }

    return &redisCache{client: client}, nil
}

func (r *redisCache) Get(ctx context.Context, key string) ([]byte, error) {
    r.mu.RLock()
    if r.closed {
        r.mu.RUnlock()
        return nil, ErrClosed
    }
    r.mu.RUnlock()

    data, err := r.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, ErrNotFound
    }
    if err != nil {
        return nil, err
    }

    // Return a copy
    result := make([]byte, len(data))
    copy(result, data)
    return result, nil
}

func (r *redisCache) Set(ctx context.Context, key string, value []byte) error {
    r.mu.RLock()
    if r.closed {
        r.mu.RUnlock()
        return ErrClosed
    }
    r.mu.RUnlock()

    // Copy the value
    data := make([]byte, len(value))
    copy(data, value)

    return r.client.Set(ctx, key, data, 0).Err()
}

func (r *redisCache) SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error {
    r.mu.RLock()
    if r.closed {
        r.mu.RUnlock()
        return ErrClosed
    }
    r.mu.RUnlock()

    // Copy the value
    data := make([]byte, len(value))
    copy(data, value)

    return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *redisCache) Delete(ctx context.Context, key string) error {
    r.mu.RLock()
    if r.closed {
        r.mu.RUnlock()
        return ErrClosed
    }
    r.mu.RUnlock()

    // DEL is idempotent - no error if key doesn't exist
    return r.client.Del(ctx, key).Err()
}

func (r *redisCache) Exists(ctx context.Context, key string) (bool, error) {
    r.mu.RLock()
    if r.closed {
        r.mu.RUnlock()
        return false, ErrClosed
    }
    r.mu.RUnlock()

    n, err := r.client.Exists(ctx, key).Result()
    if err != nil {
        return false, err
    }
    return n > 0, nil
}

func (r *redisCache) Close() error {
    r.mu.Lock()
    defer r.mu.Unlock()

    if r.closed {
        return nil // Idempotent
    }
    r.closed = true

    return r.client.Close()
}

// Optional: Implement Pinger for health checks
func (r *redisCache) Ping(ctx context.Context) error {
    r.mu.RLock()
    if r.closed {
        r.mu.RUnlock()
        return ErrClosed
    }
    r.mu.RUnlock()

    return r.client.Ping(ctx).Err()
}

// Optional: Implement StatsProvider
func (r *redisCache) Stats() Stats {
    // Redis INFO command provides stats
    // Parse and return relevant metrics
    return Stats{}
}
```

### Registering a New Backend

To integrate with the factory, modify `factory.go`:

```go
func New(ctx context.Context, cfg *Config) (Cache, error) {
    switch cfg.Mode {
    case ModeSingle:
        return newRistrettoCache(cfg.Ristretto)
    case ModeHA:
        return newOlricCache(ctx, &cfg.Olric)
    case ModeDisabled:
        return newNoopCache(), nil
    case ModeRedis:  // Add new mode
        return newRedisCache(cfg.Redis.Addr)
    default:
        return nil, fmt.Errorf("cache: unknown mode %q", cfg.Mode)
    }
}
```

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `ErrNotFound` | Key doesn't exist (cache miss) | Normal behavior. Fetch from source and populate cache. |
| `ErrClosed` | Operations after `Close()` | Check application lifecycle. Don't use cache after shutdown. |
| `ErrSerializationFailed` | Value encoding/decoding failed | Check value format. Ensure consistent serialization. |

### HA Mode Issues

#### Nodes Cannot Join Cluster

**Symptom:** Nodes start but don't discover each other.

**Causes and Solutions:**

1. **Wrong peer port:** Peers must use memberlist port (bind_addr + 2), not Olric port.
   ```yaml
   # Wrong
   peers:
     - "other-node:3320"  # This is the Olric port

   # Correct
   peers:
     - "other-node:3322"  # Memberlist port = 3320 + 2
   ```

2. **Firewall blocking:** Ensure both Olric and memberlist ports are open.
   ```bash
   # Check connectivity
   nc -zv other-node 3320  # Olric port
   nc -zv other-node 3322  # Memberlist port
   ```

3. **DNS resolution:** Verify hostnames resolve correctly.
   ```bash
   getent hosts other-node
   ```

4. **Environment mismatch:** All nodes must use the same `environment` setting.

#### Quorum Errors

**Symptom:** "not enough members" or operations fail despite nodes being up.

**Solution:** Ensure `member_count_quorum` is less than or equal to actual running nodes.

```yaml
# For 2-node cluster
member_count_quorum: 2  # Requires both nodes

# For 3-node cluster with 1-node fault tolerance
member_count_quorum: 2  # Allows 1 node to be down
```

#### Data Not Replicated

**Symptom:** Data disappears when a node goes down.

**Solution:** Ensure `replica_count` > 1 and have enough nodes.

```yaml
replica_count: 2          # Store 2 copies
member_count_quorum: 2    # Need 2 nodes to write
```

### Single Mode Issues

#### High Memory Usage

**Symptom:** cc-relay memory grows unbounded.

**Solution:** Adjust `max_cost` to limit cache size.

```yaml
ristretto:
  max_cost: 52428800  # Reduce to 50 MB
```

#### Low Hit Rate

**Symptom:** Frequent cache misses despite caching.

**Solutions:**
1. Increase `num_counters` for better admission policy
2. Increase `max_cost` if cache is evicting too aggressively
3. Check TTLs - too short may cause premature expiration

### General Issues

#### Cache Data Lost on Restart

**This is expected behavior.** The cache is ephemeral by design:
- Restarting cc-relay clears all cached data
- Switching modes clears all cached data
- This is a cache, not a database

If you need persistence, consider:
- External Redis/Memcached with persistence enabled
- Warm-up scripts to pre-populate cache on startup

#### Debugging Cache Operations

Enable debug logging to trace cache operations:

```yaml
logging:
  level: debug
```

Look for log entries from `component=cache_factory` and cache backend operations.

## API Reference

### Core Interface

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte) error
    SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    Close() error
}
```

### Stats Structure

```go
type Stats struct {
    Hits      uint64 `json:"hits"`       // Cache hit count
    Misses    uint64 `json:"misses"`     // Cache miss count
    KeyCount  uint64 `json:"key_count"`  // Current number of keys
    BytesUsed uint64 `json:"bytes_used"` // Approximate memory usage
    Evictions uint64 `json:"evictions"`  // Keys evicted due to capacity
}
```

### Optional Interfaces

| Interface | Methods | Purpose |
|-----------|---------|---------|
| `StatsProvider` | `Stats() Stats` | Metrics and observability |
| `Pinger` | `Ping(ctx context.Context) error` | Health checks |
| `ClusterInfo` | `MemberlistAddr() string`, `ClusterMembers() int`, `IsEmbedded() bool` | HA cluster status |
| `MultiGetter` | `GetMulti(ctx context.Context, keys []string) (map[string][]byte, error)` | Batch reads |
| `MultiSetter` | `SetMulti(...)`, `SetMultiWithTTL(...)` | Batch writes |

### Error Types

```go
var (
    ErrNotFound            = errors.New("cache: key not found")
    ErrClosed              = errors.New("cache: cache is closed")
    ErrSerializationFailed = errors.New("cache: serialization failed")
)
```

### Using Optional Interfaces

```go
// Check for stats support
if sp, ok := cache.(cache.StatsProvider); ok {
    stats := sp.Stats()
    log.Printf("Hits: %d, Misses: %d", stats.Hits, stats.Misses)
}

// Check for health check support
if p, ok := cache.(cache.Pinger); ok {
    if err := p.Ping(ctx); err != nil {
        log.Printf("Cache unhealthy: %v", err)
    }
}

// Check for cluster info (HA mode)
if ci, ok := cache.(cache.ClusterInfo); ok {
    if ci.IsEmbedded() {
        log.Printf("Cluster members: %d", ci.ClusterMembers())
        log.Printf("Memberlist addr: %s", ci.MemberlistAddr())
    }
}

// Check for batch operations
if mg, ok := cache.(cache.MultiGetter); ok {
    results, err := mg.GetMulti(ctx, []string{"key1", "key2", "key3"})
    // results contains only found keys
}
```

### Factory Function

```go
// New creates a new Cache based on the configuration.
// Returns error if configuration is invalid or backend fails to initialize.
func New(ctx context.Context, cfg *Config) (Cache, error)
```

### Default Configurations

```go
// DefaultRistrettoConfig returns sensible defaults for single mode.
// NumCounters: 1,000,000 (for ~100K items)
// MaxCost: 100 MB
// BufferItems: 64
func DefaultRistrettoConfig() RistrettoConfig

// DefaultOlricConfig returns sensible defaults for HA mode.
// DMapName: "cc-relay"
// Environment: "local"
// ReplicaCount: 1
// ReadQuorum: 1
// WriteQuorum: 1
// MemberCountQuorum: 1
// LeaveTimeout: 5s
func DefaultOlricConfig() OlricConfig
```
