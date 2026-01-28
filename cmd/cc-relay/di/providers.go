package di

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/samber/do/v2"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/keypool"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/proxy"
	"github.com/omarluq/cc-relay/internal/router"
)

// Service wrapper types for DI registration.
// These provide type safety and allow distinguishing between similar types.

// ConfigService wraps the loaded configuration with hot-reload support.
// It uses atomic.Pointer for lock-free config reads, allowing in-flight
// requests to continue uninterrupted while new requests use reloaded config.
//
//nolint:govet // Field order optimized for readability over memory alignment
type ConfigService struct {
	// config holds the current config via atomic pointer for lock-free reads
	config atomic.Pointer[config.Config]

	// watcher monitors the config file for changes (may be nil if watch fails)
	watcher *config.Watcher

	// path is the config file path for reload operations
	path string

	// Config is the initial config pointer (kept for backward compatibility).
	//
	// Deprecated: Use Get() for thread-safe access.
	Config *config.Config
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

// CacheService wraps the cache implementation.
type CacheService struct {
	Cache cache.Cache
}

// keyPoolData holds the primary key pool for atomic swap.
type keyPoolData struct {
	Pool         *keypool.KeyPool
	ProviderName string
}

// KeyPoolService wraps the optional key pool for the primary provider.
// Supports hot-reload: primary key pool can be rebuilt on config reload.
type KeyPoolService struct {
	data   atomic.Pointer[keyPoolData]
	cfgSvc *ConfigService

	// For backward compatibility during transition
	Pool         *keypool.KeyPool
	ProviderName string
}

// Get returns the current primary key pool (live, hot-reload aware).
func (s *KeyPoolService) Get() *keypool.KeyPool {
	d := s.data.Load()
	if d == nil {
		return s.Pool
	}
	return d.Pool
}

// RebuildFrom rebuilds the primary key pool from the given config.
// Uses the first enabled provider with pooling enabled.
func (s *KeyPoolService) RebuildFrom(cfg *config.Config) error {
	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		if !p.IsPoolingEnabled() {
			s.data.Store(&keyPoolData{ProviderName: p.Name, Pool: nil})
			s.Pool = nil
			s.ProviderName = p.Name
			return nil
		}

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
			return fmt.Errorf("failed to create key pool for provider %s: %w", p.Name, err)
		}

		s.data.Store(&keyPoolData{ProviderName: p.Name, Pool: pool})
		s.Pool = pool
		s.ProviderName = p.Name
		return nil
	}

	// No enabled providers found
	s.data.Store(&keyPoolData{ProviderName: "", Pool: nil})
	s.Pool = nil
	s.ProviderName = ""
	return nil
}

// StartWatching begins watching config changes for primary key pool updates.
func (s *KeyPoolService) StartWatching() {
	if s.cfgSvc == nil || s.cfgSvc.watcher == nil {
		return
	}

	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		if err := s.RebuildFrom(newCfg); err != nil {
			log.Error().Err(err).Msg("failed to rebuild primary key pool after config reload")
			return err
		}
		log.Info().Msg("primary key pool rebuilt after config reload")
		return nil
	})
}

// keyPoolMapData holds the key pools and keys for atomic swap.
type keyPoolMapData struct {
	Pools map[string]*keypool.KeyPool // Provider name -> KeyPool
	Keys  map[string]string           // Provider name -> API key (fallback)
}

// KeyPoolMapService wraps per-provider key pools for multi-provider routing.
// Supports hot-reload: key pools for newly enabled providers are created on reload.
type KeyPoolMapService struct {
	data   atomic.Pointer[keyPoolMapData]
	cfgSvc *ConfigService

	// For backward compatibility during transition
	Pools map[string]*keypool.KeyPool // Provider name -> KeyPool
	Keys  map[string]string           // Provider name -> API key (fallback)
}

// GetPools returns the current key pools (live, hot-reload aware).
func (s *KeyPoolMapService) GetPools() map[string]*keypool.KeyPool {
	d := s.data.Load()
	if d == nil {
		return s.Pools // Fallback to legacy field
	}
	return d.Pools
}

// GetKeys returns the current fallback keys (live, hot-reload aware).
func (s *KeyPoolMapService) GetKeys() map[string]string {
	d := s.data.Load()
	if d == nil {
		return s.Keys // Fallback to legacy field
	}
	return d.Keys
}

// RebuildFrom rebuilds key pools from the given config.
// Called from reload callbacks to create pools for newly enabled providers.
func (s *KeyPoolMapService) RebuildFrom(cfg *config.Config) error {
	pools := make(map[string]*keypool.KeyPool)
	keys := make(map[string]string)
	var rebuildErr error

	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		// Store fallback key (first key in list)
		if len(p.Keys) > 0 {
			keys[p.Name] = p.Keys[0].Key
		}

		// Skip pool creation if pooling not enabled for this provider
		if !p.IsPoolingEnabled() {
			continue
		}

		// Build pool configuration for provider
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
			log.Error().Err(err).Str("provider", p.Name).Msg("failed to create key pool on reload")
			rebuildErr = err
			continue // Log and skip, don't fail the entire reload
		}

		pools[p.Name] = pool
	}

	s.data.Store(&keyPoolMapData{Pools: pools, Keys: keys})
	// Also update legacy fields for backward compatibility
	s.Pools = pools
	s.Keys = keys

	return rebuildErr
}

// StartWatching begins watching config changes for key pool updates.
func (s *KeyPoolMapService) StartWatching() {
	if s.cfgSvc == nil || s.cfgSvc.watcher == nil {
		return
	}

	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		if err := s.RebuildFrom(newCfg); err != nil {
			log.Error().Err(err).Msg("failed to rebuild key pools after config reload")
			// Don't return error to avoid blocking other callbacks
		}
		log.Info().Msg("key pools rebuilt after config reload")
		return nil
	})
}

// providerMapData holds the provider map data for atomic swap.
type providerMapData struct {
	PrimaryProvider providers.Provider
	Providers       map[string]providers.Provider
	PrimaryKey      string
	AllProviders    []providers.Provider
}

// ProviderMapService wraps the map of providers with hot-reload support.
// Providers are rebuilt on config reload to support enabling/disabling providers dynamically.
type ProviderMapService struct {
	data   atomic.Pointer[providerMapData]
	cfgSvc *ConfigService

	// For backward compatibility
	PrimaryProvider providers.Provider
	Providers       map[string]providers.Provider
	PrimaryKey      string
	AllProviders    []providers.Provider
}

// GetPrimaryProvider returns the current primary provider (live, hot-reload aware).
func (s *ProviderMapService) GetPrimaryProvider() providers.Provider {
	d := s.data.Load()
	if d == nil {
		return s.PrimaryProvider
	}
	return d.PrimaryProvider
}

// GetPrimaryKey returns the current primary provider key (live, hot-reload aware).
func (s *ProviderMapService) GetPrimaryKey() string {
	d := s.data.Load()
	if d == nil {
		return s.PrimaryKey
	}
	return d.PrimaryKey
}

// GetProviders returns the current provider map (live, hot-reload aware).
func (s *ProviderMapService) GetProviders() map[string]providers.Provider {
	d := s.data.Load()
	if d == nil {
		return s.Providers // Fallback to legacy field
	}
	return d.Providers
}

// GetAllProviders returns the current all providers slice (live, hot-reload aware).
func (s *ProviderMapService) GetAllProviders() []providers.Provider {
	d := s.data.Load()
	if d == nil {
		return s.AllProviders // Fallback to legacy field
	}
	return d.AllProviders
}

// GetProvider returns a provider by name (live, hot-reload aware).
func (s *ProviderMapService) GetProvider(name string) (providers.Provider, bool) {
	providersMap := s.GetProviders()
	if providersMap == nil {
		return nil, false
	}
	prov, ok := providersMap[name]
	return prov, ok
}

// RebuildFrom rebuilds the provider map from the given config.
// Called from reload callbacks to create providers for newly enabled ones.
// Reuses existing providers when possible to preserve state.
func (s *ProviderMapService) RebuildFrom(cfg *config.Config) error {
	ctx := context.Background()

	providerMap := make(map[string]providers.Provider)
	var allProviders []providers.Provider
	var primaryProvider providers.Provider
	var primaryKey string

	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		prov, err := createProvider(ctx, p)
		if errors.Is(err, ErrUnknownProviderType) {
			log.Warn().Str("provider", p.Name).Str("type", p.Type).Msg("skipping unknown provider type on reload")
			continue // Skip unknown provider types
		}
		if err != nil {
			log.Error().Err(err).Str("provider", p.Name).Msg("failed to create provider on reload")
			continue // Log and skip, don't fail the entire reload
		}

		providerMap[p.Name] = prov
		allProviders = append(allProviders, prov)

		if primaryProvider == nil {
			primaryProvider = prov
			if len(p.Keys) > 0 {
				primaryKey = p.Keys[0].Key
			}
		}
	}

	if primaryProvider == nil {
		// Keep using current providers if no enabled providers in new config
		log.Warn().Msg("no enabled providers in new config, keeping current providers")
		return nil
	}

	s.data.Store(&providerMapData{
		PrimaryProvider: primaryProvider,
		Providers:       providerMap,
		PrimaryKey:      primaryKey,
		AllProviders:    allProviders,
	})
	// Also update legacy fields for backward compatibility
	s.PrimaryProvider = primaryProvider
	s.Providers = providerMap
	s.PrimaryKey = primaryKey
	s.AllProviders = allProviders

	return nil
}

// StartWatching begins watching config changes for provider map updates.
func (s *ProviderMapService) StartWatching() {
	if s.cfgSvc == nil || s.cfgSvc.watcher == nil {
		return
	}

	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		if err := s.RebuildFrom(newCfg); err != nil {
			log.Error().Err(err).Msg("failed to rebuild provider map after config reload")
		}
		log.Info().Msg("provider map rebuilt after config reload")
		return nil
	})
}

// ProviderInfoService holds live provider routing information with atomic swap support.
// Provider info (enabled/disabled, weights, priorities) is rebuilt on config reload
// and atomically swapped for thread-safe access without mutex overhead.
type ProviderInfoService struct {
	// infos holds the current provider info slice via atomic pointer
	infos atomic.Pointer[[]router.ProviderInfo]

	// cfgSvc provides access to current config for rebuilding
	cfgSvc *ConfigService

	// providerSvc gives access to provider instances
	providerSvc *ProviderMapService

	// trackerSvc provides health check functions
	trackerSvc *HealthTrackerService
}

// Get returns the current provider info slice (lock-free read).
// Returns a shallow copy to prevent callers from mutating the internal slice.
func (s *ProviderInfoService) Get() []router.ProviderInfo {
	ptr := s.infos.Load()
	if ptr == nil {
		return nil
	}
	// Return shallow copy (append to nil) to prevent mutation of internal slice
	return append([]router.ProviderInfo(nil), (*ptr)...)
}

// Rebuild rebuilds the provider info slice from current config.
// This should be called on config reload to update provider routing inputs.
func (s *ProviderInfoService) Rebuild() {
	cfg := s.cfgSvc.Get()
	s.RebuildFrom(cfg)
}

// RebuildFrom rebuilds the provider info slice from the given config.
// This is called from reload callbacks to ensure we use the fresh config
// rather than racing with the atomic config swap.
// Uses the live provider map to pick up newly enabled providers.
func (s *ProviderInfoService) RebuildFrom(cfg *config.Config) {
	var providerInfos []router.ProviderInfo

	// Use live provider map to pick up newly enabled providers
	providerMap := s.providerSvc.GetProviders()

	for idx := range cfg.Providers {
		pc := &cfg.Providers[idx]
		if !pc.Enabled {
			continue
		}

		prov, ok := providerMap[pc.Name]
		if !ok {
			continue
		}

		// Get weight and priority from first key (provider-level defaults)
		var weight, priority int
		if len(pc.Keys) > 0 {
			weight = pc.Keys[0].Weight
			priority = pc.Keys[0].Priority
		}

		// Wire IsHealthy from tracker
		providerName := pc.Name
		providerInfos = append(providerInfos, router.ProviderInfo{
			Provider:  prov,
			Weight:    weight,
			Priority:  priority,
			IsHealthy: s.trackerSvc.Tracker.IsHealthyFunc(providerName),
		})
	}

	s.infos.Store(&providerInfos)
}

// StartWatching begins watching config changes for provider info updates.
// Registers a callback with the config watcher to rebuild provider info on reload.
func (s *ProviderInfoService) StartWatching() {
	if s.cfgSvc.watcher == nil {
		return
	}

	// Register callback to rebuild provider info on config reload.
	// Important: We rebuild from the newCfg passed to the callback, not from
	// cfgSvc.Get(), to ensure we use the freshly loaded config regardless of
	// callback registration order.
	s.cfgSvc.watcher.OnReload(func(newCfg *config.Config) error {
		s.RebuildFrom(newCfg)
		log.Info().Msg("provider info rebuilt after config reload")
		return nil
	})
}

// RouterService wraps the provider router for DI.
// It provides hot-reloadable router access with caching to preserve router state.
//
// Router instances are cached and only rebuilt when strategy or timeout changes.
// This preserves state for stateful routers (round_robin, weighted_round_robin)
// while still allowing config changes to take effect without restart.
type RouterService struct {
	cfgSvc *ConfigService

	// Cached router with atomic swap support
	router atomic.Pointer[routerCacheEntry]
}

// routerCacheEntry holds a cached router with its configuration key.
type routerCacheEntry struct {
	router   router.ProviderRouter
	strategy string
	timeout  time.Duration
}

// GetRouter returns the current router, using cache when config unchanged.
// This method is safe for concurrent use and preserves router state.
func (s *RouterService) GetRouter() router.ProviderRouter {
	cfg := s.cfgSvc.Get()
	strategy := cfg.Routing.GetEffectiveStrategy()
	timeout := cfg.Routing.GetFailoverTimeoutOption().OrElse(5 * time.Second)

	// Check cache for existing router with same config
	cached := s.router.Load()
	if cached != nil && cached.strategy == strategy && cached.timeout == timeout {
		return cached.router
	}

	// Create new router for updated config
	r, err := router.NewRouter(strategy, timeout)
	if err != nil {
		// Fallback to failover if strategy is invalid
		var fallbackErr error
		r, fallbackErr = router.NewRouter(router.StrategyFailover, timeout)
		// If even failover fails, we have a configuration problem
		if fallbackErr != nil {
			// Return a failover router with default timeout as last resort
			// At this point we log the error but continue with a known-good fallback
			log.Error().Err(fallbackErr).Msg("failed to create failover router, using default")
			r, err = router.NewRouter(router.StrategyFailover, 5*time.Second)
			if err != nil {
				// This should never happen unless there's a code bug
				panic("router: failed to create default failover router")
			}
		}
	}

	// Atomically store new router (racing updates may overwrite, last wins)
	newEntry := &routerCacheEntry{
		router:   r,
		strategy: strategy,
		timeout:  timeout,
	}
	s.router.Store(newEntry)

	return r
}

// GetRouterFunc returns a function that fetches the current router.
// This is used with LiveRouter for per-request router access.
func (s *RouterService) GetRouterFunc() router.ProviderRouterFunc {
	return s.GetRouter
}

// GetRouterAsFunc returns the router getter as a ProviderRouterFunc directly.
// This is a convenience wrapper for passing to NewLiveRouter.
// Delegates to GetRouterFunc for deduplication.
func (s *RouterService) GetRouterAsFunc() router.ProviderRouterFunc {
	return s.GetRouterFunc()
}

// LoggerService wraps the zerolog logger for DI.
type LoggerService struct {
	Logger *zerolog.Logger
}

// HealthTrackerService wraps the health tracker for DI.
type HealthTrackerService struct {
	Tracker *health.Tracker
}

// CheckerService wraps the health checker for DI.
type CheckerService struct {
	Checker *health.Checker
}

// SignatureCacheService wraps the thinking signature cache for DI.
type SignatureCacheService struct {
	Cache *proxy.SignatureCache
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
// 2. Logger (depends on Config)
// 3. Cache (depends on Config)
// 4. Providers (depends on Config)
// 5. KeyPool (depends on Config) - primary provider only
// 6. KeyPoolMap (depends on Config) - all providers
// 7. Router (depends on Config)
// 8. HealthTracker (depends on Config, Logger)
// 9. Checker (depends on HealthTracker, Config, Logger)
// 10. ProviderInfo (depends on Config, Providers, HealthTracker)
// 11. SignatureCache (depends on Cache)
// 12. Handler (depends on Config, KeyPool, KeyPoolMap, Providers, Router, ProviderInfo, HealthTracker, SignatureCache)
// 13. Server (depends on Handler, Config).
func RegisterSingletons(i do.Injector) {
	do.Provide(i, NewConfig)
	do.Provide(i, NewLogger)
	do.Provide(i, NewCache)
	do.Provide(i, NewProviderMap)
	do.Provide(i, NewKeyPool)
	do.Provide(i, NewKeyPoolMap)
	do.Provide(i, NewRouter)
	do.Provide(i, NewHealthTracker)
	do.Provide(i, NewChecker)
	do.Provide(i, NewProviderInfo)
	do.Provide(i, NewSignatureCache)
	do.Provide(i, NewProxyHandler)
	do.Provide(i, NewHTTPServer)
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

// NewLogger creates the zerolog logger from configuration.
func NewLogger(i do.Injector) (*LoggerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)

	logger, err := proxy.NewLogger(cfgSvc.Config.Logging)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &LoggerService{Logger: &logger}, nil
}

// NewHealthTracker creates the health tracker from configuration.
func NewHealthTracker(i do.Injector) (*HealthTrackerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	loggerSvc := do.MustInvoke[*LoggerService](i)

	tracker := health.NewTracker(
		cfgSvc.Config.Health.CircuitBreaker,
		loggerSvc.Logger,
	)
	return &HealthTrackerService{Tracker: tracker}, nil
}

// NewChecker creates the health checker from configuration.
func NewChecker(i do.Injector) (*CheckerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)
	loggerSvc := do.MustInvoke[*LoggerService](i)

	checker := health.NewChecker(
		trackerSvc.Tracker,
		cfgSvc.Config.Health.HealthCheck,
		loggerSvc.Logger,
	)

	// Register health checks for all enabled providers
	for idx := range cfgSvc.Config.Providers {
		pc := &cfgSvc.Config.Providers[idx]
		if !pc.Enabled {
			continue
		}

		// Construct base URL based on provider type
		baseURL := pc.BaseURL
		switch pc.Type {
		case "bedrock":
			// Bedrock base URL: https://bedrock-runtime.{region}.amazonaws.com
			if pc.AWSRegion != "" {
				baseURL = fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", pc.AWSRegion)
			}
		case "vertex":
			// Vertex base URL: https://{region}-aiplatform.googleapis.com
			if pc.GCPRegion != "" {
				baseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com", pc.GCPRegion)
			}
		case "azure":
			// Azure base URL: https://{resource}.services.ai.azure.com
			if pc.AzureResourceName != "" {
				baseURL = fmt.Sprintf("https://%s.services.ai.azure.com", pc.AzureResourceName)
			}
		}

		// NewProviderHealthCheck handles empty BaseURL (returns NoOpHealthCheck)
		healthCheck := health.NewProviderHealthCheck(pc.Name, baseURL, nil)
		checker.RegisterProvider(healthCheck)
		loggerSvc.Logger.Debug().
			Str("provider", pc.Name).
			Str("base_url", baseURL).
			Msg("registered health check")
	}

	return &CheckerService{Checker: checker}, nil
}

// Shutdown implements do.Shutdowner for graceful checker cleanup.
func (h *CheckerService) Shutdown() error {
	if h.Checker != nil {
		h.Checker.Stop()
	}
	return nil
}

// NewSignatureCache creates the thinking signature cache using the main cache backend.
func NewSignatureCache(i do.Injector) (*SignatureCacheService, error) {
	cacheSvc := do.MustInvoke[*CacheService](i)

	// SignatureCache wraps the main cache for thinking block signatures
	sigCache := proxy.NewSignatureCache(cacheSvc.Cache)

	return &SignatureCacheService{Cache: sigCache}, nil
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

// ErrUnknownProviderType is returned when the provider type is not recognized.
var ErrUnknownProviderType = fmt.Errorf("unknown provider type")

// createProvider creates a provider instance from configuration.
// Returns ErrUnknownProviderType for unknown provider types.
func createProvider(ctx context.Context, p *config.ProviderConfig) (providers.Provider, error) {
	switch p.Type {
	case "anthropic":
		return providers.NewAnthropicProviderWithMapping(
			p.Name, p.BaseURL, p.Models, p.ModelMapping,
		), nil
	case "zai":
		return providers.NewZAIProviderWithMapping(
			p.Name, p.BaseURL, p.Models, p.ModelMapping,
		), nil
	case "ollama":
		return providers.NewOllamaProviderWithMapping(
			p.Name, p.BaseURL, p.Models, p.ModelMapping,
		), nil
	case "bedrock":
		if err := p.ValidateCloudConfig(); err != nil {
			return nil, fmt.Errorf("bedrock provider %s: %w", p.Name, err)
		}
		return providers.NewBedrockProvider(ctx, &providers.BedrockConfig{
			Name:         p.Name,
			Region:       p.AWSRegion,
			Models:       p.Models,
			ModelMapping: p.ModelMapping,
		})
	case "vertex":
		if err := p.ValidateCloudConfig(); err != nil {
			return nil, fmt.Errorf("vertex provider %s: %w", p.Name, err)
		}
		return providers.NewVertexProvider(ctx, &providers.VertexConfig{
			Name:         p.Name,
			ProjectID:    p.GCPProjectID,
			Region:       p.GCPRegion,
			Models:       p.Models,
			ModelMapping: p.ModelMapping,
		})
	case "azure":
		if err := p.ValidateCloudConfig(); err != nil {
			return nil, fmt.Errorf("azure provider %s: %w", p.Name, err)
		}
		return providers.NewAzureProvider(&providers.AzureConfig{
			Name:         p.Name,
			ResourceName: p.AzureResourceName,
			DeploymentID: p.AzureDeploymentID,
			APIVersion:   p.GetAzureAPIVersion(),
			Models:       p.Models,
			ModelMapping: p.ModelMapping,
		})
	default:
		return nil, ErrUnknownProviderType
	}
}

// supportedProviderTypes is the list of supported provider types for error messages.
const supportedProviderTypes = "anthropic, zai, ollama, bedrock, vertex, azure"

// NewProviderMap creates the map of enabled providers with hot-reload support.
// Supports hot-reload: call StartWatching() is invoked automatically.
func NewProviderMap(i do.Injector) (*ProviderMapService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	svc := &ProviderMapService{
		cfgSvc:       cfgSvc,
		Providers:    make(map[string]providers.Provider),
		AllProviders: nil,
	}

	var primaryProvider providers.Provider
	var primaryKey string

	ctx := context.Background()

	for idx := range cfg.Providers {
		p := &cfg.Providers[idx]
		if !p.Enabled {
			continue
		}

		prov, err := createProvider(ctx, p)
		if errors.Is(err, ErrUnknownProviderType) {
			continue // Skip unknown provider types
		}
		if err != nil {
			return nil, err
		}

		svc.Providers[p.Name] = prov
		svc.AllProviders = append(svc.AllProviders, prov)

		// First enabled provider becomes the primary
		if primaryProvider == nil {
			primaryProvider = prov
			if len(p.Keys) > 0 {
				primaryKey = p.Keys[0].Key
			}
		}
	}

	if primaryProvider == nil {
		return nil, fmt.Errorf("no enabled provider found (supported: %s)", supportedProviderTypes)
	}

	svc.PrimaryProvider = primaryProvider
	svc.PrimaryKey = primaryKey

	// Store initial data in atomic pointer
	svc.data.Store(&providerMapData{
		PrimaryProvider: primaryProvider,
		Providers:       svc.Providers,
		PrimaryKey:      primaryKey,
		AllProviders:    svc.AllProviders,
	})

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}

// NewKeyPool creates the key pool for the primary provider if pooling is enabled.
func NewKeyPool(i do.Injector) (*KeyPoolService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	svc := &KeyPoolService{cfgSvc: cfgSvc}
	if err := svc.RebuildFrom(cfg); err != nil {
		return nil, err
	}

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}

// NewKeyPoolMap creates key pools for all enabled providers.
// This enables dynamic provider routing with per-provider rate limiting.
// Supports hot-reload: call StartWatching() after container init.
func NewKeyPoolMap(i do.Injector) (*KeyPoolMapService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	svc := &KeyPoolMapService{cfgSvc: cfgSvc}
	if err := svc.RebuildFrom(cfg); err != nil {
		return nil, err
	}

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}

// NewRouter creates the provider router service with hot-reload support.
// The router is created dynamically per-request based on current config.
func NewRouter(i do.Injector) (*RouterService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	return &RouterService{cfgSvc: cfgSvc}, nil
}

// NewProviderInfo creates the provider info service with hot-reload support.
// Provider info (enabled/disabled, weights, priorities) is rebuilt on config reload.
func NewProviderInfo(i do.Injector) (*ProviderInfoService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	providerSvc := do.MustInvoke[*ProviderMapService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)

	svc := &ProviderInfoService{
		cfgSvc:      cfgSvc,
		providerSvc: providerSvc,
		trackerSvc:  trackerSvc,
	}

	// Build initial provider info
	svc.Rebuild()

	// Start watching for config changes
	svc.StartWatching()

	return svc, nil
}

// NewProxyHandler creates the HTTP handler with all middleware.
func NewProxyHandler(i do.Injector) (*HandlerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	providerSvc := do.MustInvoke[*ProviderMapService](i)
	poolSvc := do.MustInvoke[*KeyPoolService](i)
	poolMapSvc := do.MustInvoke[*KeyPoolMapService](i)
	routerSvc := do.MustInvoke[*RouterService](i)
	providerInfoSvc := do.MustInvoke[*ProviderInfoService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)
	sigCacheSvc := do.MustInvoke[*SignatureCacheService](i)

	// Use SetupRoutesWithLiveKeyPools for full hot-reload support:
	// - Live provider info (enabled/disabled, weights, priorities)
	// - Live router (strategy/timeout changes without restart)
	// - Live key pools (newly enabled providers get keys immediately)
	liveRouter := router.NewLiveRouter(routerSvc.GetRouterAsFunc())
	handler, err := proxy.SetupRoutesWithLiveKeyPools(
		cfgSvc,
		providerSvc.GetPrimaryProvider(),
		providerInfoSvc.Get, // Hot-reloadable provider info
		liveRouter,          // Live router for strategy changes
		providerSvc.GetPrimaryKey(),
		poolSvc.Get(),
		poolMapSvc.GetPools, // Live key pools accessor
		poolMapSvc.GetKeys,  // Live fallback keys accessor
		providerSvc.GetAllProviders(),
		trackerSvc.Tracker,
		sigCacheSvc.Cache,
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
