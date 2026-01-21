package cache

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/olric-data/olric"
	olricconfig "github.com/olric-data/olric/config"
	"github.com/rs/zerolog"
)

// parseBindAddr parses a bind address string that may contain host:port or just host.
// Returns the host and port (0 if not specified).
func parseBindAddr(addr string) (h string, p int) {
	var err error
	h, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		// No port specified, return address as-is
		return addr, 0
	}
	p, err = strconv.Atoi(portStr)
	if err != nil {
		return h, 0
	}
	return h, p
}

// olricCache implements Cache using Olric as the backend.
// It provides distributed caching for high-availability deployments.
// Supports two modes:
//   - Embedded mode: Runs a local Olric node (for single-node HA or testing)
//   - Client mode: Connects to an existing Olric cluster
type olricCache struct {
	db     *olric.Olric    // Olric instance (only for embedded mode, nil for client mode)
	client olric.Client    // Client interface (works for both embedded and cluster)
	dmap   olric.DMap      // Distributed map handle
	log    *zerolog.Logger // Logger for cache operations
	name   string          // DMap name for reference
	mu     sync.RWMutex
	closed atomic.Bool
}

// Ensure olricCache implements the required interfaces.
var (
	_ Cache         = (*olricCache)(nil)
	_ StatsProvider = (*olricCache)(nil)
	_ Pinger        = (*olricCache)(nil)
)

// newOlricCache creates a new Olric distributed cache with the given configuration.
// In embedded mode, it starts a local Olric node.
// In client mode, it connects to an existing Olric cluster.
func newOlricCache(ctx context.Context, cfg *OlricConfig) (*olricCache, error) {
	olricLog := logger().With().Str("backend", "olric").Logger()

	dmapName := cfg.DMapName
	if dmapName == "" {
		dmapName = "cc-relay"
	}

	if cfg.Embedded {
		olricLog.Debug().Str("mode", "embedded").Msg("olric: starting embedded node")
		return newEmbeddedOlricCache(ctx, cfg, dmapName, &olricLog)
	}
	olricLog.Debug().Str("mode", "client").Strs("addresses", cfg.Addresses).Msg("olric: connecting to cluster")
	return newClientOlricCache(ctx, cfg, dmapName, &olricLog)
}

// newEmbeddedOlricCache starts an embedded Olric node.
func newEmbeddedOlricCache(
	ctx context.Context, cfg *OlricConfig, dmapName string, lg *zerolog.Logger,
) (*olricCache, error) {
	// Create embedded config
	c := olricconfig.New("local")

	// Parse bind address - it may contain host:port or just host
	bindAddr, bindPort := parseBindAddr(cfg.BindAddr)
	c.BindAddr = bindAddr
	if bindPort > 0 {
		c.BindPort = bindPort
	}

	// Set peers if provided
	if len(cfg.Peers) > 0 {
		c.Peers = cfg.Peers
	}

	// Suppress verbose Olric logging (especially useful for tests)
	// Set log output to discard to avoid cluttering test output
	c.LogOutput = io.Discard
	c.Logger = log.New(io.Discard, "", 0)

	// Channel to signal when Olric is ready
	// This must be set BEFORE calling olric.New()
	ready := make(chan struct{})
	c.Started = func() {
		close(ready)
	}

	// Create the Olric instance
	db, err := olric.New(c)
	if err != nil {
		lg.Error().Err(err).Msg("olric: failed to create embedded instance")
		return nil, err
	}

	// Start the node in the background
	startErr := make(chan error, 1)
	go func() {
		if err := db.Start(); err != nil {
			startErr <- err
		}
	}()

	// Wait for the node to be ready or context cancellation
	// Give it a reasonable timeout to start
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	select {
	case <-ready:
		lg.Debug().Msg("olric: embedded node ready")
	case err := <-startErr:
		lg.Error().Err(err).Msg("olric: embedded node failed to start")
		return nil, err
	case <-startupCtx.Done():
		// Timeout - the node is still starting but should be usable soon
		// Give it a tiny bit more time for the embedded client to be ready
		lg.Debug().Msg("olric: embedded node startup timeout, proceeding")
		time.Sleep(100 * time.Millisecond)
	}

	// Get the embedded client
	client := db.NewEmbeddedClient()

	// Get or create the DMap
	dm, err := client.NewDMap(dmapName)
	if err != nil {
		lg.Error().Err(err).Str("dmap", dmapName).Msg("olric: failed to create dmap")
		// Failed to create DMap, shutdown the embedded node
		if shutdownErr := db.Shutdown(context.Background()); shutdownErr != nil {
			lg.Error().Err(shutdownErr).Msg("olric: failed to shutdown after dmap creation error")
		}
		return nil, err
	}

	lg.Info().
		Str("bind_addr", bindAddr).
		Int("bind_port", bindPort).
		Str("dmap", dmapName).
		Int("peers", len(cfg.Peers)).
		Msg("olric embedded cache created")

	return &olricCache{
		client: client,
		dmap:   dm,
		db:     db,
		name:   dmapName,
		log:    lg,
	}, nil
}

// newClientOlricCache connects to an external Olric cluster.
func newClientOlricCache(
	ctx context.Context, cfg *OlricConfig, dmapName string, lg *zerolog.Logger,
) (*olricCache, error) {
	if len(cfg.Addresses) == 0 {
		lg.Error().Msg("olric: addresses required for client mode")
		return nil, errors.New("cache: olric addresses required for client mode")
	}

	// Create cluster client
	client, err := olric.NewClusterClient(cfg.Addresses)
	if err != nil {
		lg.Error().Err(err).Strs("addresses", cfg.Addresses).Msg("olric: failed to connect to cluster")
		return nil, err
	}

	// Get or create the DMap
	dm, err := client.NewDMap(dmapName)
	if err != nil {
		lg.Error().Err(err).Str("dmap", dmapName).Msg("olric: failed to create dmap")
		// Failed to create DMap, close the client connection
		if closeErr := client.Close(ctx); closeErr != nil {
			lg.Error().Err(closeErr).Msg("olric: failed to close client after dmap creation error")
		}
		return nil, err
	}

	lg.Info().
		Strs("addresses", cfg.Addresses).
		Str("dmap", dmapName).
		Msg("olric cluster cache created")

	return &olricCache{
		client: client,
		dmap:   dm,
		db:     nil, // nil for client mode
		name:   dmapName,
		log:    lg,
	}, nil
}

// Get retrieves a value from the cache.
// Returns ErrNotFound if the key does not exist.
// Returns ErrClosed if the cache has been closed.
func (o *olricCache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if o.closed.Load() {
		return nil, ErrClosed
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed.Load() {
		return nil, ErrClosed
	}

	resp, err := o.dmap.Get(ctx, key)
	if err != nil {
		if errors.Is(err, olric.ErrKeyNotFound) {
			o.log.Debug().
				Str("key", key).
				Bool("hit", false).
				Msg("cache get")
			return nil, ErrNotFound
		}
		o.log.Debug().
			Str("key", key).
			Err(err).
			Msg("cache get error")
		return nil, err
	}

	value, err := resp.Byte()
	if err != nil {
		o.log.Debug().
			Str("key", key).
			Err(err).
			Msg("cache get: failed to decode value")
		return nil, err
	}

	o.log.Debug().
		Str("key", key).
		Bool("hit", true).
		Int("size", len(value)).
		Msg("cache get")

	// Return a copy to prevent mutation of cached data
	result := make([]byte, len(value))
	copy(result, value)
	return result, nil
}

// Set stores a value in the cache with no expiration.
// Returns ErrClosed if the cache has been closed.
func (o *olricCache) Set(ctx context.Context, key string, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if o.closed.Load() {
		return ErrClosed
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed.Load() {
		return ErrClosed
	}

	// Make a copy to prevent caller from mutating cached data
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	err := o.dmap.Put(ctx, key, valueCopy)
	if err != nil {
		o.log.Debug().
			Str("key", key).
			Int("size", len(value)).
			Err(err).
			Msg("cache set error")
		return err
	}

	o.log.Debug().
		Str("key", key).
		Int("size", len(value)).
		Msg("cache set")

	return nil
}

// SetWithTTL stores a value in the cache with a time-to-live.
// After the TTL expires, the key will no longer be retrievable.
// Returns ErrClosed if the cache has been closed.
func (o *olricCache) SetWithTTL(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if o.closed.Load() {
		return ErrClosed
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed.Load() {
		return ErrClosed
	}

	// Make a copy to prevent caller from mutating cached data
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	err := o.dmap.Put(ctx, key, valueCopy, olric.EX(ttl))
	if err != nil {
		o.log.Debug().
			Str("key", key).
			Int("size", len(value)).
			Dur("ttl", ttl).
			Err(err).
			Msg("cache set error")
		return err
	}

	o.log.Debug().
		Str("key", key).
		Int("size", len(value)).
		Dur("ttl", ttl).
		Msg("cache set")

	return nil
}

// Delete removes a key from the cache.
// Returns nil if the key does not exist (idempotent).
// Returns ErrClosed if the cache has been closed.
func (o *olricCache) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if o.closed.Load() {
		return ErrClosed
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed.Load() {
		return ErrClosed
	}

	_, err := o.dmap.Delete(ctx, key)
	if errors.Is(err, olric.ErrKeyNotFound) {
		o.log.Debug().
			Str("key", key).
			Msg("cache delete")
		return nil // Idempotent: deleting nonexistent key is not an error
	}
	if err != nil {
		o.log.Debug().
			Str("key", key).
			Err(err).
			Msg("cache delete error")
		return err
	}

	o.log.Debug().
		Str("key", key).
		Msg("cache delete")

	return nil
}

// Exists checks if a key exists in the cache.
// Returns ErrClosed if the cache has been closed.
func (o *olricCache) Exists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	if o.closed.Load() {
		return false, ErrClosed
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed.Load() {
		return false, ErrClosed
	}

	_, err := o.dmap.Get(ctx, key)
	if errors.Is(err, olric.ErrKeyNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Close releases resources associated with the cache.
// After Close is called, all operations will return ErrClosed.
// Close is idempotent.
func (o *olricCache) Close() error {
	if o.closed.Load() {
		return nil
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if o.closed.Load() {
		return nil
	}

	o.closed.Store(true)

	ctx := context.Background()

	// Close the DMap
	if o.dmap != nil {
		// DMap close error is not critical, we're already shutting down
		if dmapErr := o.dmap.Close(ctx); dmapErr != nil {
			o.log.Debug().Err(dmapErr).Msg("olric: dmap close error during shutdown")
		}
	}

	var err error
	if o.db != nil {
		// Embedded mode: shutdown the Olric node
		err = o.db.Shutdown(ctx)
		if err != nil {
			o.log.Error().Err(err).Msg("olric: embedded node shutdown error")
		} else {
			o.log.Info().Msg("olric embedded cache closed")
		}
		return err
	}

	// Client mode: close the cluster client
	if o.client != nil {
		err = o.client.Close(ctx)
		if err != nil {
			o.log.Error().Err(err).Msg("olric: client disconnect error")
		} else {
			o.log.Info().Msg("olric cluster cache closed")
		}
		return err
	}

	return nil
}

// Stats returns current cache statistics.
// Note: Olric stats are available via the client.Stats() method,
// but require a specific member address. For simplicity, this
// returns empty stats. Use Stats() from the Olric client directly
// for detailed cluster statistics.
func (o *olricCache) Stats() Stats {
	if o.closed.Load() {
		return Stats{}
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed.Load() {
		return Stats{}
	}

	// Olric's Stats() requires a member address and returns different
	// statistics than what our Stats struct expects. For a full
	// implementation, you would need to aggregate stats from all
	// cluster members or use a specific member's stats.
	//
	// For now, return empty stats. The cache is still fully functional;
	// this just means stats won't be available through this interface.
	// Use client.Stats(ctx, addr) directly for detailed Olric stats.
	return Stats{}
}

// Ping verifies the cache connection is alive.
// For embedded mode, this always succeeds if not closed.
// For client mode, this validates cluster connectivity by attempting
// a simple operation.
func (o *olricCache) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if o.closed.Load() {
		return ErrClosed
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.closed.Load() {
		return ErrClosed
	}

	// Try a simple Get operation to verify connectivity
	// ErrKeyNotFound is expected and means the connection is working
	_, err := o.dmap.Get(ctx, "__ping_healthcheck__")
	if errors.Is(err, olric.ErrKeyNotFound) {
		o.log.Debug().Msg("cache ping: healthy")
		return nil // Expected - connection is working
	}
	if err != nil {
		o.log.Debug().Err(err).Msg("cache ping: unhealthy")
		return err
	}

	o.log.Debug().Msg("cache ping: healthy")
	return nil
}
