// Package config provides configuration loading and parsing for cc-relay.
package config

import "sync/atomic"

// Runtime provides atomic access to configuration for hot-reload support.
// It uses sync/atomic.Pointer for lock-free reads, allowing in-flight requests
// to complete with the old config while new requests see the updated config.
//
// The Store() operation is called by the config watcher when a file change is detected.
// The Get() operation is called by components on each request (or per-operation basis)
// to ensure they observe the latest configuration.
//
// Example usage:
//
//	runtime := config.NewRuntime(initialConfig)
//
//	// In a request handler or component:
//	cfg := runtime.Get()
//	strategy := cfg.Routing.GetEffectiveStrategy()
//
//	// In the config watcher callback:
//	runtime.Store(newConfig)
type Runtime struct {
	ptr atomic.Pointer[Config]
}

// NewRuntime creates a new Runtime with the given initial configuration.
// The initial config is stored and immediately available via Get().
func NewRuntime(initial *Config) *Runtime {
	r := &Runtime{}
	r.ptr.Store(initial)
	return r
}

// Get returns the current configuration atomically.
// This is a lock-free read that returns the most recently stored config.
// Multiple concurrent calls are safe and efficient.
//
// Components should call Get() per-request or per-operation to ensure
// they observe the latest configuration after hot-reload.
func (r *Runtime) Get() *Config {
	return r.ptr.Load()
}

// Store atomically updates the configuration.
// This is called by the config watcher when a file change is detected.
// The swap is atomic - readers will either see the old config or the new config,
// never an inconsistent state.
//
// In-flight requests holding a reference to the old config continue to work
// with that config. New requests will see the new config via Get().
func (r *Runtime) Store(cfg *Config) {
	r.ptr.Store(cfg)
}

// RuntimeConfig interface implementation.
var _ RuntimeConfig = (*Runtime)(nil)
