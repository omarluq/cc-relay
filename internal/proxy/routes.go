// Package proxy implements the HTTP proxy server for cc-relay.
package proxy

import (
	"fmt"
	"net/http"

	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

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
	handler, err := NewHandler(provider, nil, nil, providerKey, pool, nil, nil, nil, debugOpts, false, nil, nil)
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
func SetupRoutesWithRouter(
	cfg *config.Config,
	provider providers.Provider,
	providerInfos []router.ProviderInfo,
	providerRouter router.ProviderRouter,
	providerKey string,
	pool *keypool.KeyPool,
	providerPools map[string]*keypool.KeyPool,
	providerKeys map[string]string,
	allProviders []providers.Provider,
	healthTracker *health.Tracker,
	signatureCache *SignatureCache,
) (http.Handler, error) {
	runtimeCfg := config.NewRuntime(cfg)
	return SetupRoutesWithRouterLive(
		runtimeCfg,
		provider,
		func() []router.ProviderInfo { return providerInfos },
		providerRouter,
		providerKey,
		pool,
		func() map[string]*keypool.KeyPool { return providerPools },
		func() map[string]string { return providerKeys },
		allProviders,
		healthTracker,
		signatureCache,
	)
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
func SetupRoutesWithRouterLive(
	cfgProvider config.RuntimeConfig,
	provider providers.Provider,
	providerInfosFunc ProviderInfoFunc,
	providerRouter router.ProviderRouter,
	providerKey string,
	pool *keypool.KeyPool,
	getProviderPools KeyPoolsFunc,
	getProviderKeys KeysFunc,
	allProviders []providers.Provider,
	healthTracker *health.Tracker,
	signatureCache *SignatureCache,
) (http.Handler, error) {
	return SetupRoutesWithLiveKeyPools(
		cfgProvider,
		provider,
		providerInfosFunc,
		providerRouter,
		providerKey,
		pool,
		getProviderPools,
		getProviderKeys,
		allProviders,
		healthTracker,
		signatureCache,
	)
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
func SetupRoutesWithLiveKeyPools(
	cfgProvider config.RuntimeConfig,
	provider providers.Provider,
	providerInfosFunc ProviderInfoFunc,
	providerRouter router.ProviderRouter,
	providerKey string,
	pool *keypool.KeyPool,
	getProviderPools KeyPoolsFunc,
	getProviderKeys KeysFunc,
	allProviders []providers.Provider,
	healthTracker *health.Tracker,
	signatureCache *SignatureCache,
) (http.Handler, error) {
	mux := http.NewServeMux()

	// Create proxy handler with full hot-reload support
	cfg := cfgProvider.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config provider returned nil config")
	}
	debugOpts := cfg.Logging.DebugOptions
	routingDebug := cfg.Routing.IsDebugEnabled()

	handler, err := NewHandlerWithLiveKeyPools(
		provider, providerInfosFunc, providerRouter,
		providerKey, pool, getProviderPools, getProviderKeys,
		&cfg.Routing, debugOpts, routingDebug, healthTracker, signatureCache,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}
	handler.SetRuntimeConfig(cfgProvider)

	// Apply middleware in order:
	// 1. RequestIDMiddleware (first - generates ID)
	// 2. LoggingMiddleware (second - logs with ID)
	// 3. AuthMiddleware (third - auth logs include ID)
	// 4. Handler
	var messagesHandler http.Handler = handler

	messagesHandler = LiveAuthMiddleware(cfgProvider)(messagesHandler)
	messagesHandler = LoggingMiddlewareWithProvider(func() config.DebugOptions {
		cfg := cfgProvider.Get()
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
