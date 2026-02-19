package di

import (
	"sync/atomic"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/omarluq/cc-relay/internal/providers"
	"github.com/omarluq/cc-relay/internal/router"
)

// Exported for testing.
// This file provides access to unexported identifiers needed by tests in package di_test.

// GetConfigAtomic returns the atomic pointer for config storage.
func (c *ConfigService) GetConfigAtomic() *atomic.Pointer[config.Config] {
	return &c.config
}

// GetWatcher returns the watcher for testing purposes.
func (c *ConfigService) GetWatcher() *config.Watcher {
	return c.watcher
}

// mustTestConfig creates a minimal Config for testing with all required fields initialized.
func MustTestConfig() config.Config {
	return config.Config{
		Providers: []config.ProviderConfig{},
		Routing: config.RoutingConfig{
			ModelMapping:     map[string]string{},
			DefaultProvider: "",
			Strategy:        "",
			FailoverTimeout: 0,
			Debug:           false,
		},
		Logging: config.LoggingConfig{
			Level:        "info",
			Format:       "json",
			Output:       "stdout",
			Pretty:       false,
			DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
		},
		Health: health.Config{
			HealthCheck: health.CheckConfig{
				Enabled:    nil,
				IntervalMS: 0,
			},
			CircuitBreaker: health.CircuitBreakerConfig{
				OpenDurationMS:   0,
				FailureThreshold: 0,
				HalfOpenProbes:   0,
			},
		},
		Server: config.ServerConfig{
			Listen:        ":8787",
			APIKey:        "",
			Auth: config.AuthConfig{
			APIKey:           "",
			BearerSecret:     "",
			AllowBearer:      false,
			AllowSubscription: false,
		},
			TimeoutMS:     0,
			MaxConcurrent: 0,
			MaxBodyBytes:  0,
			EnableHTTP2:   false,
		},
		Cache: cache.Config{
			Mode: cache.ModeDisabled,
			Olric: cache.OlricConfig{
				DMapName:          "",
				BindAddr:          "",
				Environment:       "",
				Addresses:         nil,
				Peers:             nil,
				ReplicaCount:      0,
				ReadQuorum:        0,
				WriteQuorum:       0,
				LeaveTimeout:      0,
				MemberCountQuorum: 0,
				Embedded:          false,
			},
			Ristretto: cache.RistrettoConfig{
				NumCounters: 0,
				MaxCost:     0,
				BufferItems: 0,
			},
		},
	}
}

// mustTestRoutingConfig creates a minimal RoutingConfig for testing.
func MustTestRoutingConfig(strategy string) config.RoutingConfig {
	return config.RoutingConfig{
		ModelMapping:     map[string]string{},
		DefaultProvider: "",
		Strategy:        strategy,
		FailoverTimeout: 5000,
		Debug:           false,
	}
}

// mustTestProviderConfig creates a minimal ProviderConfig for testing.
func MustTestProviderConfig(name, pType, baseURL string, keys []config.KeyConfig) config.ProviderConfig {
	return config.ProviderConfig{
		ModelMapping:       map[string]string{},
		AWSRegion:          "",
		GCPProjectID:       "",
		AzureAPIVersion:    "",
		Name:               name,
		Type:               pType,
		BaseURL:            baseURL,
		AzureDeploymentID:  "",
		AWSAccessKeyID:     "",
		AzureResourceName:  "",
		AWSSecretAccessKey: "",
		GCPRegion:          "",
		Models:             nil,
		Pooling: config.PoolingConfig{
			Enabled:  false,
			Strategy: "",
		},
		Keys:               keys,
		Enabled:            true,
	}
}

// mustTestKeyConfig creates a minimal KeyConfig for testing.
func MustTestKeyConfig(key string) config.KeyConfig {
	return config.KeyConfig{
		Key:          key,
		RPMLimit:     0,
		ITPMLimit:    0,
		OTPMLimit:    0,
		Priority:     0,
		Weight:       0,
		TPMLimit:     0,
	}
}

// mustTestHealthConfig creates a minimal health.Config for testing.
func MustTestHealthConfig() health.Config {
	return health.Config{
		HealthCheck: health.CheckConfig{
			Enabled:    nil,
			IntervalMS: 0,
		},
		CircuitBreaker: health.CircuitBreakerConfig{
			OpenDurationMS:   0,
			FailureThreshold: 0,
			HalfOpenProbes:   0,
		},
	}
}

// mustTestServerConfig creates a minimal ServerConfig for testing.
func MustTestServerConfig(listen string) config.ServerConfig {
	return config.ServerConfig{
		Listen:        listen,
		APIKey:        "",
		Auth: config.AuthConfig{
			APIKey:           "",
			BearerSecret:     "",
			AllowBearer:      false,
			AllowSubscription: false,
		},
		TimeoutMS:     0,
		MaxConcurrent: 0,
		MaxBodyBytes:  0,
		EnableHTTP2:   false,
	}
}

// mustTestLoggingConfig creates a minimal LoggingConfig for testing.
func MustTestLoggingConfig(level string) config.LoggingConfig {
	return config.LoggingConfig{
		Level:        level,
		Format:       "json",
		Output:       "stdout",
		Pretty:       false,
		DebugOptions: config.DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
	}
}

// MustTestCacheConfig creates a minimal cache.Config for testing.
func MustTestCacheConfig(mode cache.Mode) cache.Config {
	return cache.Config{
		Mode: mode,
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 0,
			MaxCost:     0,
			BufferItems: 0,
		},
	}
}

// MustTestProviderInfo creates a minimal router.ProviderInfo for testing.
func MustTestProviderInfo(provider providers.Provider, weight, priority int) router.ProviderInfo {
	return router.ProviderInfo{
		Provider:  provider,
		IsHealthy: func() bool { return true },
		Weight:    weight,
		Priority:  priority,
	}
}

// NewConfigServiceUninitialized creates a ConfigService without initialization.
func NewConfigServiceUninitialized() *ConfigService {
	cfg := MustTestConfig()
	svc := &ConfigService{
		config:  atomic.Pointer[config.Config]{},
		watcher: nil,
		Config:  nil,
		path:    "",
	}
	svc.config.Store(&cfg)
	return svc
}

// NewRouterServiceWithConfigService creates a RouterService with a specific ConfigService.
func NewRouterServiceWithConfigService(cfgSvc *ConfigService) *RouterService {
	svc := &RouterService{
		cfgSvc: cfgSvc,
		router: atomic.Pointer[routerCacheEntry]{},
	}
	svc.router.Store(&routerCacheEntry{
		router:   nil,
		strategy: "",
		timeout:  0,
	})
	return svc
}

// NewConfigServiceWithConfig creates a ConfigService with config and nil watcher.
func NewConfigServiceWithConfig(cfg *config.Config) *ConfigService {
	svc := &ConfigService{
		config:  atomic.Pointer[config.Config]{},
		watcher: nil,
		Config:  cfg,
		path:    "",
	}
	svc.config.Store(cfg)
	return svc
}

// NewConfigServiceWithNilWatcher creates a ConfigService with config and explicit nil watcher.
func NewConfigServiceWithNilWatcher(cfg *config.Config) *ConfigService {
	svc := &ConfigService{
		config:  atomic.Pointer[config.Config]{},
		watcher: nil,
		Config:  cfg,
		path:    "",
	}
	svc.config.Store(cfg)
	return svc
}

// NewProviderMapServiceWithConfigService creates a ProviderMapService with a specific ConfigService.
func NewProviderMapServiceWithConfigService(cfgSvc *ConfigService) *ProviderMapService {
	svc := &ProviderMapService{
		data:            atomic.Pointer[providerMapData]{},
		cfgSvc:          cfgSvc,
		PrimaryProvider: nil,
		Providers:       map[string]providers.Provider{},
		PrimaryKey:      "",
		AllProviders:    nil,
	}
	svc.data.Store(&providerMapData{
		PrimaryProvider: nil,
		Providers:       nil,
		PrimaryKey:      "",
		AllProviders:    nil,
	})
	return svc
}

// NewProviderInfoService creates a ProviderInfoService with all dependencies.
func NewProviderInfoService(
	cfgSvc *ConfigService,
	providerSvc *ProviderMapService,
	trackerSvc *HealthTrackerService,
) *ProviderInfoService {
	svc := &ProviderInfoService{
		infos:       atomic.Pointer[[]router.ProviderInfo]{},
		cfgSvc:      cfgSvc,
		providerSvc: providerSvc,
		trackerSvc:  trackerSvc,
	}
	svc.infos.Store(&[]router.ProviderInfo{})
	return svc
}

// StoreInfos stores the provider info slice in the atomic pointer (for testing).
func (s *ProviderInfoService) StoreInfos(infos *[]router.ProviderInfo) {
	s.infos.Store(infos)
}

// StoreProviderMapData stores the provider map data in the atomic pointer (for testing).
func (s *ProviderMapService) StoreProviderMapData(data *providerMapData) {
	s.data.Store(data)
}

// NewHealthTrackerServiceWithTracker creates a HealthTrackerService with a specific tracker.
func NewHealthTrackerServiceWithTracker(tracker *health.Tracker) *HealthTrackerService {
	return &HealthTrackerService{
		Tracker: tracker,
		cfgSvc:  nil,
		logger:  nil,
	}
}
