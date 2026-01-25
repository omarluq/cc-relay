package di

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
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

// ConfigService wraps the loaded configuration.
type ConfigService struct {
	Config *config.Config
}

// CacheService wraps the cache implementation.
type CacheService struct {
	Cache cache.Cache
}

// KeyPoolService wraps the optional key pool for the primary provider.
type KeyPoolService struct {
	Pool *keypool.KeyPool
}

// KeyPoolMapService wraps per-provider key pools for multi-provider routing.
type KeyPoolMapService struct {
	Pools map[string]*keypool.KeyPool // Provider name -> KeyPool
	Keys  map[string]string           // Provider name -> API key (fallback)
}

// ProviderMapService wraps the map of providers.
type ProviderMapService struct {
	PrimaryProvider providers.Provider
	Providers       map[string]providers.Provider
	PrimaryKey      string
	AllProviders    []providers.Provider
}

// RouterService wraps the provider router for DI.
type RouterService struct {
	Router router.ProviderRouter
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
// 10. SignatureCache (depends on Cache)
// 11. Handler (depends on Config, KeyPool, KeyPoolMap, Providers, Router, HealthTracker, SignatureCache)
// 12. Server (depends on Handler, Config).
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
	do.Provide(i, NewSignatureCache)
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
		}), nil
	default:
		return nil, ErrUnknownProviderType
	}
}

// supportedProviderTypes is the list of supported provider types for error messages.
const supportedProviderTypes = "anthropic, zai, ollama, bedrock, vertex, azure"

// NewProviderMap creates the map of enabled providers.
func NewProviderMap(i do.Injector) (*ProviderMapService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	providerMap := make(map[string]providers.Provider)
	var allProviders []providers.Provider
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
		return nil, fmt.Errorf("no enabled provider found (supported: %s)", supportedProviderTypes)
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

// NewKeyPoolMap creates key pools for all enabled providers.
// This enables dynamic provider routing with per-provider rate limiting.
func NewKeyPoolMap(i do.Injector) (*KeyPoolMapService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	cfg := cfgSvc.Config

	pools := make(map[string]*keypool.KeyPool)
	keys := make(map[string]string)

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

		pools[p.Name] = pool
	}

	return &KeyPoolMapService{Pools: pools, Keys: keys}, nil
}

// NewRouter creates the provider router based on configuration.
func NewRouter(i do.Injector) (*RouterService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	routingCfg := cfgSvc.Config.Routing

	// Get timeout with default fallback (5 seconds)
	timeout := routingCfg.GetFailoverTimeoutOption().OrElse(5 * time.Second)

	r, err := router.NewRouter(routingCfg.GetEffectiveStrategy(), timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	return &RouterService{Router: r}, nil
}

// NewProxyHandler creates the HTTP handler with all middleware.
func NewProxyHandler(i do.Injector) (*HandlerService, error) {
	cfgSvc := do.MustInvoke[*ConfigService](i)
	providerSvc := do.MustInvoke[*ProviderMapService](i)
	poolSvc := do.MustInvoke[*KeyPoolService](i)
	poolMapSvc := do.MustInvoke[*KeyPoolMapService](i)
	routerSvc := do.MustInvoke[*RouterService](i)
	trackerSvc := do.MustInvoke[*HealthTrackerService](i)
	sigCacheSvc := do.MustInvoke[*SignatureCacheService](i)

	cfg := cfgSvc.Config

	// Build ProviderInfo list from config and providers
	var providerInfos []router.ProviderInfo
	for idx := range cfg.Providers {
		pc := &cfg.Providers[idx]
		if !pc.Enabled {
			continue
		}

		prov, ok := providerSvc.Providers[pc.Name]
		if !ok {
			continue
		}

		// Get weight and priority from first key (provider-level defaults)
		var weight, priority int
		if len(pc.Keys) > 0 {
			weight = pc.Keys[0].Weight
			priority = pc.Keys[0].Priority
		}

		// Wire IsHealthy from tracker (replaces stub)
		providerName := pc.Name
		providerInfos = append(providerInfos, router.ProviderInfo{
			Provider:  prov,
			Weight:    weight,
			Priority:  priority,
			IsHealthy: trackerSvc.Tracker.IsHealthyFunc(providerName),
		})
	}

	handler, err := proxy.SetupRoutesWithRouter(
		cfg,
		providerSvc.PrimaryProvider,
		providerInfos,
		routerSvc.Router,
		providerSvc.PrimaryKey,
		poolSvc.Pool,
		poolMapSvc.Pools,
		poolMapSvc.Keys,
		providerSvc.AllProviders,
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
