package di

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/config"
)

// ConfigService wraps the loaded configuration with hot-reload support.
// It uses atomic.Pointer for lock-free config reads, allowing in-flight
// requests to continue uninterrupted while new requests use reloaded config.
type ConfigService struct {
	config  atomic.Pointer[config.Config]
	watcher *config.Watcher
	Config  *config.Config
	path    string
}

// Get returns the current configuration via atomic load (lock-free read).
// This is the preferred method for accessing config during request handling.
func (c *ConfigService) Get() *config.Config {
	return c.config.Load()
}

// StartWatching begins watching the config file for changes.
// It registers a callback to atomically swap the config on reload.
// This should be called after the DI container is fully initialized.
// The context controls the watcher lifecycle - cancel to stop watching.
func (c *ConfigService) StartWatching(ctx context.Context) {
	if c.watcher == nil {
		return
	}

	// Register callback to swap config atomically
	c.watcher.OnReload(func(newCfg *config.Config) error {
		c.config.Store(newCfg)
		// Keep legacy Config pointer in sync for backward compatibility.
		c.Config = newCfg
		log.Info().Str("path", c.path).Msg("config hot-reloaded successfully")
		return nil
	})

	// Start watching in background
	go func() {
		if err := c.watcher.Watch(ctx); err != nil {
			log.Error().Err(err).Msg("config watcher error")
		}
	}()

	log.Info().Str("path", c.path).Msg("config file watcher started")
}

// Shutdown implements do.Shutdowner for graceful watcher cleanup.
func (c *ConfigService) Shutdown() error {
	if c.watcher != nil {
		return c.watcher.Close()
	}
	return nil
}

// NewConfig loads the configuration from the config path and creates a watcher.
// The watcher is created but not started - call StartWatching() after container init.
func NewConfig(i do.Injector) (*ConfigService, error) {
	path := do.MustInvokeNamed[string](i, ConfigPathKey)

	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
	}

	svc := &ConfigService{
		Config: cfg,
		path:   path,
	}

	// Store initial config in atomic pointer
	svc.config.Store(cfg)

	// Create watcher (warn on failure, don't error - hot-reload is optional)
	watcher, err := config.NewWatcher(path)
	if err != nil {
		log.Warn().Err(err).Str("path", path).Msg("config watcher creation failed, hot-reload disabled")
	} else {
		svc.watcher = watcher
	}

	return svc, nil
}
