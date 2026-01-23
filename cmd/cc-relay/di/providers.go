package di

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
)

// Service wrapper types for DI registration.
// These provide type safety and allow distinguishing between similar types.

// ConfigService wraps the loaded configuration.
type ConfigService struct {
	Config *config.Config
}

// CacheService wraps the cache implementation.
type CacheService struct {
	Cache cache.Cache
}

// KeyPoolService wraps the optional key pool.
type KeyPoolService struct {
	Pool *keypool.KeyPool
}

// ProviderMapService wraps the map of providers.
type ProviderMapService struct {
	PrimaryProvider providers.Provider
	Providers       map[string]providers.Provider
	PrimaryKey      string
	AllProviders    []providers.Provider
}

// HandlerService wraps the HTTP handler.
type HandlerService struct {
	Handler http.Handler
}

// ServerService wraps the HTTP server.
type ServerService struct {
	Server *proxy.Server
}

// RegisterSingletons registers all service providers as singletons.
// Services are registered in dependency order:
// 1. Config (no dependencies)
// 2. Cache (depends on Config)
// 3. Providers (depends on Config)
// 4. KeyPool (depends on Config)
// 5. Handler (depends on Config, KeyPool, Providers)
// 6. Server (depends on Handler, Config).
func RegisterSingletons(i do.Injector) {
	do.Provide(i, NewConfig)
	do.Provide(i, NewCache)
	do.Provide(i, NewProviderMap)
	do.Provide(i, NewKeyPool)
	do.Provide(i, NewProxyHandler)
	do.Provide(i, NewHTTPServer)
}

// NewConfig loads the configuration from the config path.
func NewConfig(i do.Injector) (*ConfigService, error) {
	path := do.MustInvokeNamed[string](i, ConfigPathKey)

	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", path, err)
	}

	return &ConfigService{Config: cfg}, nil
}

// NewCache creates the cache based on configuration.
func NewCache(i do.Injector) (*CacheService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)

	// Use a background context with timeout for cache initialization
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := cache.New(ctx, &cfgSvc.Config.Cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	return &CacheService{Cache: c}, nil
}

// Shutdown implements do.Shutdowner for graceful cache cleanup.
func (c *CacheService) Shutdown() error {
	if c.Cache != nil {
		return c.Cache.Close()
	}
	return nil
}

// NewProviderMap creates the map of enabled providers.
func NewProviderMap(i do.Injector) (*ProviderMapService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	providerMap := make(map[string]providers.Provider)
	var allProviders []providers.Provider
	var primaryProvider providers.Provider
	var primaryKey string

	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		var prov providers.Provider
		switch p.Type {
		case "anthropic":
			prov = providers.NewAnthropicProviderWithModels(p.Name, p.BaseURL, p.Models)
		case "zai":
			prov = providers.NewZAIProviderWithModels(p.Name, p.BaseURL, p.Models)
		default:
			continue // Skip unknown provider types
		}

		providerMap[p.Name] = prov
		allProviders = append(allProviders, prov)

		// First enabled provider becomes the primary
		if primaryProvider == nil {
			primaryProvider = prov
			if len(p.Keys) > 0 {
				primaryKey = p.Keys[0].Key
			}
		}
	}

	if primaryProvider == nil {
		return nil, fmt.Errorf("no enabled provider found (supported types: anthropic, zai)")
	}

	return &ProviderMapService{
		Providers:       providerMap,
		AllProviders:    allProviders,
		PrimaryProvider: primaryProvider,
		PrimaryKey:      primaryKey,
	}, nil
}

// NewKeyPool creates the key pool for the primary provider if pooling is enabled.
func NewKeyPool(i do.Injector) (*KeyPoolService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	// Find first enabled provider with pooling enabled
	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		if !p.IsPoolingEnabled() {
			// No pooling for this provider
			return &KeyPoolService{Pool: nil}, nil
		}

		// Build pool configuration
		poolCfg := keypool.PoolConfig{
			Strategy: p.GetEffectiveStrategy(),
			Keys:     make([]keypool.KeyConfig, len(p.Keys)),
		}

		for j, k := range p.Keys {
			itpm, otpm := k.GetEffectiveTPM()
			poolCfg.Keys[j] = keypool.KeyConfig{
				APIKey:    k.Key,
				RPMLimit:  k.RPMLimit,
				ITPMLimit: itpm,
				OTPMLimit: otpm,
				Priority:  k.Priority,
				Weight:    k.Weight,
			}
		}

		pool, err := keypool.NewKeyPool(p.Name, poolCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create key pool for provider %s: %w", p.Name, err)
		}

		return &KeyPoolService{Pool: pool}, nil
	}

	// No enabled providers found
	return &KeyPoolService{Pool: nil}, nil
}

// NewProxyHandler creates the HTTP handler with all middleware.
func NewProxyHandler(i do.Injector) (*HandlerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	providerSvc := do.MustInvoke[*ProviderMapService](i)
	poolSvc := do.MustInvoke[*KeyPoolService](i)

	handler, err := proxy.SetupRoutesWithProviders(
		cfgSvc.Config,
		providerSvc.PrimaryProvider,
		providerSvc.PrimaryKey,
		poolSvc.Pool,
		providerSvc.AllProviders,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to setup proxy handler: %w", err)
	}

	return &HandlerService{Handler: handler}, nil
}

// NewHTTPServer creates the HTTP server.
func NewHTTPServer(i do.Injector) (*ServerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	handlerSvc := do.MustInvoke[*HandlerService](i)

	server := proxy.NewServer(
		cfgSvc.Config.Server.Listen,
		handlerSvc.Handler,
		cfgSvc.Config.Server.EnableHTTP2,
	)

	return &ServerService{Server: server}, nil
}

// Shutdown implements do.Shutdowner for graceful server shutdown.
func (s *ServerService) Shutdown() error {
	if s.Server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return s.Server.Shutdown(ctx)
	}
	return nil
}
