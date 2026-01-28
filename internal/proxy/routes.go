// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// RoutesOptions configures route setup with optional hot-reload support.
type RoutesOptions struct {
	ProviderRouter    router.ProviderRouter
	Provider          providers.Provider
	ConfigProvider    config.RuntimeConfigGetter
	Pool              *keypool.KeyPool
	ProviderInfosFunc ProviderInfoFunc
	ProviderPools     map[string]*keypool.KeyPool
	ProviderKeys      map[string]string
	GetProviderPools  KeyPoolsFunc
	GetProviderKeys   KeysFunc
	HealthTracker     *health.Tracker
	SignatureCache    *SignatureCache
	ProviderKey       string
	ProviderInfos     []router.ProviderInfo
	AllProviders      []providers.Provider
}

const routesOptionsRequiredMsg = "routes options are required"

// SetupRoutes creates the HTTP handler with all routes configured.
// Routes:
//   - POST /v1/messages - Proxy to backend provider (with auth if configured)
//   - GET /v1/models - List available models (no auth required)
//   - GET /health - Health check endpoint (no auth required)
//
// This is a convenience wrapper that calls SetupRoutesWithProviders with nil for pool and allProviders.
func SetupRoutes(
	cfg *config.Config,
	provider providers.Provider,
	providerKey string,
	pool *keypool.KeyPool,
) (http.Handler, error) {
	return SetupRoutesWithProviders(cfg, provider, providerKey, pool, nil)
}

// SetupRoutesWithProviders creates the HTTP handler with all routes configured.
// Routes:
//   - POST /v1/messages - Proxy to backend provider (with auth if configured)
//   - GET /v1/models - List available models from all providers (no auth required)
//   - GET /v1/providers - List active providers with metadata (no auth required)
//   - GET /health - Health check endpoint (no auth required)
//
// The allProviders parameter is used for the /v1/models and /v1/providers endpoints
// to list models and providers from all configured providers. If nil, only the primary
// provider's models are listed.
func SetupRoutesWithProviders(
	cfg *config.Config,
	provider providers.Provider,
	providerKey string,
	pool *keypool.KeyPool,
	allProviders []providers.Provider,
) (http.Handler, error) {
	mux := http.NewServeMux()

	// Create proxy handler with debug options from config
	debugOpts := cfg.Logging.DebugOptions
	// Use keypool for multi-key routing if provided
	// Note: SetupRoutesWithProviders doesn't use router - for DI integration use NewProxyHandler
	// Pass nil for healthTracker - this legacy function doesn't support health tracking
	// Single-provider mode: nil maps for providerPools, providerKeys, and routingConfig
	handler, err := NewHandler(&HandlerOptions{
		Provider:     provider,
		APIKey:       providerKey,
		Pool:         pool,
		DebugOptions: debugOpts,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	// Apply middleware in order:
	// 1. RequestIDMiddleware (first - generates ID)
	// 2. LoggingMiddleware (second - logs with ID)
	// 3. AuthMiddleware (third - auth logs include ID)
	// 4. Handler
	var messagesHandler http.Handler = handler

	// Use new multi-auth middleware if Auth config is present
	if cfg.Server.Auth.IsEnabled() {
		messagesHandler = MultiAuthMiddleware(&cfg.Server.Auth)(messagesHandler)
	} else if cfg.Server.APIKey != "" {
		// Fallback to legacy API key auth for backward compatibility
		messagesHandler = AuthMiddleware(cfg.Server.APIKey)(messagesHandler)
	}

	messagesHandler = LoggingMiddleware(debugOpts)(messagesHandler)
	messagesHandler = RequestIDMiddleware()(messagesHandler)

	// Register routes
	mux.Handle("POST /v1/messages", messagesHandler)

	// Models endpoint (no auth required for discovery)
	// Use allProviders if provided, otherwise fall back to just the primary provider
	modelsProviders := allProviders
	if modelsProviders == nil {
		modelsProviders = []providers.Provider{provider}
	}
	modelsHandler := NewModelsHandler(modelsProviders)
	mux.Handle("GET /v1/models", modelsHandler)

	// Providers endpoint (no auth required for discovery)
	providersHandler := NewProvidersHandler(modelsProviders)
	mux.Handle("GET /v1/providers", providersHandler)

	// Health check endpoint (no auth required)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Health check write error is non-critical
		w.Write([]byte(`{"status":"ok"}`))
	})

	return mux, nil
}

// SetupRoutesWithRouter creates the HTTP handler with all routes configured and router support.
// This is the DI-friendly version that accepts a ProviderRouter for multi-provider routing.
// Routes:
//   - POST /v1/messages - Proxy to backend provider with router-based selection
//   - GET /v1/models - List available models from all providers (no auth required)
//   - GET /v1/providers - List active providers with metadata (no auth required)
//   - GET /health - Health check endpoint (no auth required)
func SetupRoutesWithRouter(cfg *config.Config, opts *RoutesOptions) (http.Handler, error) {
	if opts == nil {
		return nil, errors.New(routesOptionsRequiredMsg)
	}
	opts.ConfigProvider = config.NewRuntime(cfg)
	opts.ProviderInfosFunc = func() []router.ProviderInfo { return opts.ProviderInfos }
	opts.GetProviderPools = func() map[string]*keypool.KeyPool { return opts.ProviderPools }
	opts.GetProviderKeys = func() map[string]string { return opts.ProviderKeys }

	return SetupRoutesWithRouterLive(opts)
}

// SetupRoutesWithRouterLive creates the HTTP handler with hot-reloadable provider info and router.
// This is the DI-friendly version that accepts functions for dynamic provider/router access.
// ProviderInfosFunc is called per-request to get current provider routing information,
// allowing changes to enabled/disabled, weights, and priorities to take effect without restart.
// Routes:
//   - POST /v1/messages - Proxy to backend provider with router-based selection
//   - GET /v1/models - List available models from all providers (no auth required)
//   - GET /v1/providers - List active providers with metadata (no auth required)
//   - GET /health - Health check endpoint (no auth required)
func SetupRoutesWithRouterLive(opts *RoutesOptions) (http.Handler, error) {
	if opts == nil {
		return nil, errors.New(routesOptionsRequiredMsg)
	}
	return SetupRoutesWithLiveKeyPools(opts)
}

// SetupRoutesWithLiveKeyPools creates the HTTP handler with full hot-reload support.
// Extends SetupRoutesWithRouterLive with live key pool accessors, enabling:
// - Hot-reloadable provider info (enabled/disabled, weights, priorities)
// - Hot-reloadable routing strategy and timeout
// - Hot-reloadable key pools (newly enabled providers get keys immediately)
// Routes:
//   - POST /v1/messages - Proxy to backend provider with router-based selection
//   - GET /v1/models - List available models from all providers (no auth required)
//   - GET /v1/providers - List active providers with metadata (no auth required)
//   - GET /health - Health check endpoint (no auth required)
func SetupRoutesWithLiveKeyPools(opts *RoutesOptions) (http.Handler, error) {
	if opts == nil {
		return nil, errors.New(routesOptionsRequiredMsg)
	}
	mux := http.NewServeMux()

	// Create proxy handler with full hot-reload support
	cfg := opts.ConfigProvider.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config provider returned nil config")
	}
	debugOpts := cfg.Logging.DebugOptions
	routingDebug := cfg.Routing.IsDebugEnabled()

	handler, err := NewHandlerWithLiveKeyPools(&HandlerOptions{
		Provider:          opts.Provider,
		ProviderInfosFunc: opts.ProviderInfosFunc,
		ProviderRouter:    opts.ProviderRouter,
		APIKey:            opts.ProviderKey,
		Pool:              opts.Pool,
		GetProviderPools:  opts.GetProviderPools,
		GetProviderKeys:   opts.GetProviderKeys,
		RoutingConfig:     &cfg.Routing,
		DebugOptions:      debugOpts,
		RoutingDebug:      routingDebug,
		HealthTracker:     opts.HealthTracker,
		SignatureCache:    opts.SignatureCache,
	},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}
	handler.SetRuntimeConfigGetter(opts.ConfigProvider)

	// Apply middleware in order:
	// 1. RequestIDMiddleware (first - generates ID)
	// 2. LoggingMiddleware (second - logs with ID)
	// 3. AuthMiddleware (third - auth logs include ID)
	// 4. Handler
	var messagesHandler http.Handler = handler

	messagesHandler = LiveAuthMiddleware(opts.ConfigProvider)(messagesHandler)
	messagesHandler = LoggingMiddlewareWithProvider(func() config.DebugOptions {
		cfg := opts.ConfigProvider.Get()
		if cfg == nil {
			return config.DebugOptions{}
		}
		return cfg.Logging.DebugOptions
	})(messagesHandler)
	messagesHandler = RequestIDMiddleware()(messagesHandler)

	// Register routes
	mux.Handle("POST /v1/messages", messagesHandler)

	// Models endpoint (no auth required for discovery)
	// Use allProviders if provided, otherwise fall back to just the primary provider
	modelsProviders := opts.AllProviders
	if modelsProviders == nil {
		modelsProviders = []providers.Provider{opts.Provider}
	}
	modelsHandler := NewModelsHandler(modelsProviders)
	mux.Handle("GET /v1/models", modelsHandler)

	// Providers endpoint (no auth required for discovery)
	providersHandler := NewProvidersHandler(modelsProviders)
	mux.Handle("GET /v1/providers", providersHandler)

	// Health check endpoint (no auth required)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//nolint:errcheck // Health check write error is non-critical
		w.Write([]byte(`{"status":"ok"}`))
	})

	return mux, nil
}
