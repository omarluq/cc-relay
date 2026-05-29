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
	"github.com/rs/zerolog/log"
)

// RoutesOptions configures route setup with optional hot-reload support.
type RoutesOptions struct {
	ProviderRouter     router.ProviderRouter
	Provider           providers.Provider
	ConfigProvider     config.RuntimeConfigGetter
	Pool               *keypool.KeyPool
	ProviderInfosFunc  ProviderInfoFunc
	ProviderPools      map[string]*keypool.KeyPool
	ProviderKeys       map[string]string
	GetProviderPools   KeyPoolsFunc
	GetProviderKeys    KeysFunc
	GetAllProviders    ProvidersGetter
	HealthTracker      *health.Tracker
	SignatureCache     *SignatureCache
	ConcurrencyLimiter *ConcurrencyLimiter
	ProviderKey        string
	ProviderInfos      []router.ProviderInfo
	AllProviders       []providers.Provider
}

const routesOptionsRequiredMsg = "routes options are required"

// SetupRoutesWithLiveKeyPools creates the HTTP handler with full hot-reload support.
// Provides live key pool accessors, enabling:
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

	messagesHandler, err := buildMessagesHandler(opts)
	if err != nil {
		return nil, err
	}
	mux.Handle("POST /v1/messages", messagesHandler)

	providersGetter := liveProvidersGetter(opts)
	mux.Handle("GET /v1/models", NewModelsHandler(providersGetter))
	mux.Handle("GET /v1/providers", NewProvidersHandler(providersGetter))

	registerHealthRoute(mux)

	return mux, nil
}

func buildMessagesHandler(opts *RoutesOptions) (http.Handler, error) {
	handler, err := buildProxyHandler(opts)
	if err != nil {
		return nil, err
	}

	// Apply middleware in order (outermost first):
	// 1. RequestIDMiddleware - generates request ID
	// 2. LoggingMiddleware - logs with request ID
	// 3. ConcurrencyMiddleware - enforces max_concurrent limit (early rejection)
	// 4. MaxBodyBytesMiddleware - enforces max_body_bytes limit
	// 5. AuthMiddleware - validates authentication
	// 6. Handler
	var messagesHandler http.Handler = handler

	messagesHandler = LiveAuthMiddleware(opts.ConfigProvider)(messagesHandler)

	// Apply max_body_bytes limit (hot-reloadable)
	messagesHandler = MaxBodyBytesMiddleware(func() int64 {
		cfg := opts.ConfigProvider.Get()
		if cfg == nil {
			return 0
		}
		return cfg.Server.MaxBodyBytes
	})(messagesHandler)

	// Apply concurrency limit if limiter provided
	if opts.ConcurrencyLimiter != nil {
		messagesHandler = ConcurrencyMiddleware(opts.ConcurrencyLimiter)(messagesHandler)
	}

	messagesHandler = LoggingMiddlewareWithProvider(func() config.DebugOptions {
		cfg := opts.ConfigProvider.Get()
		if cfg == nil {
			return config.DebugOptions{
				LogRequestBody:     false,
				LogResponseHeaders: false,
				LogTLSMetrics:      false,
				MaxBodyLogSize:     0,
			}
		}
		return cfg.Logging.DebugOptions
	})(messagesHandler)
	messagesHandler = RequestIDMiddleware()(messagesHandler)

	return messagesHandler, nil
}

func buildProxyHandler(opts *RoutesOptions) (*Handler, error) {
	cfg := opts.ConfigProvider.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config provider returned nil config")
	}
	handler, err := NewHandler(&HandlerOptions{
		Provider:          opts.Provider,
		ProviderInfosFunc: opts.ProviderInfosFunc,
		ProviderRouter:    opts.ProviderRouter,
		ProviderPools:     nil,
		ProviderKeys:      nil,
		APIKey:            opts.ProviderKey,
		Pool:              opts.Pool,
		GetProviderPools:  opts.GetProviderPools,
		GetProviderKeys:   opts.GetProviderKeys,
		RoutingConfig:     &cfg.Routing,
		DebugOptions:      cfg.Logging.DebugOptions,
		RoutingDebug:      cfg.Routing.IsDebugEnabled(),
		HealthTracker:     opts.HealthTracker,
		SignatureCache:    opts.SignatureCache,
		ProviderInfos:     nil,
	},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}
	handler.SetRuntimeConfigGetter(opts.ConfigProvider)
	return handler, nil
}

func liveProvidersGetter(opts *RoutesOptions) func() []providers.Provider {
	return func() []providers.Provider {
		if opts.GetAllProviders != nil {
			if providersList := opts.GetAllProviders(); providersList != nil {
				return providersList
			}
		}
		return []providers.Provider{opts.Provider}
	}
}

func registerHealthRoute(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			log.Error().Err(err).Msg("failed to write health response")
		}
	})
}
