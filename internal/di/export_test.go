package di

import (
	"net/http"
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
			ModelMapping:    map[string]string{},
			DefaultProvider: "",
			Strategy:        "",
			FailoverTimeout: 0,
			Debug:           false,
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
			Pretty: false,
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
			Listen: ":8787",
			APIKey: "",
			Auth: config.AuthConfig{
				APIKey:            "",
				BearerSecret:      "",
				AllowBearer:       false,
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
		ModelMapping:    map[string]string{},
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
		Keys:    keys,
		Enabled: true,
	}
}

// mustTestKeyConfig creates a minimal KeyConfig for testing.
func MustTestKeyConfig(key string) config.KeyConfig {
	return config.KeyConfig{
		Key:       key,
		RPMLimit:  0,
		ITPMLimit: 0,
		OTPMLimit: 0,
		Priority:  0,
		Weight:    0,
		TPMLimit:  0,
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
		Listen: listen,
		APIKey: "",
		Auth: config.AuthConfig{
			APIKey:            "",
			BearerSecret:      "",
			AllowBearer:       false,
			AllowSubscription: false,
		},
		TimeoutMS:     0,
		MaxConcurrent: 0,
		MaxBodyBytes:  0,
		EnableHTTP2:   false,
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
// Does not initialize the atomic data pointer - callers must call StoreProviderMapData to set data,
// or rely on the legacy Providers field fallback.
func NewProviderMapServiceWithConfigService(cfgSvc *ConfigService) *ProviderMapService {
	return &ProviderMapService{
		data:            atomic.Pointer[providerMapData]{},
		cfgSvc:          cfgSvc,
		PrimaryProvider: nil,
		Providers:       map[string]providers.Provider{},
		PrimaryKey:      "",
		AllProviders:    nil,
	}
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

// CreateCloudProvider exports createCloudProvider for testing.
var CreateCloudProvider = createCloudProvider

// TestProviderMapData is an alias for providerMapData for testing.
type TestProviderMapData = providerMapData

// SetLegacyProviders sets the legacy Providers field for testing.
func (s *ProviderMapService) SetLegacyProviders(provs map[string]Provider) {
	s.Providers = provs
}

// Provider is a type alias for providers.Provider to allow mock providers in tests.
type Provider = providers.Provider

// MockProvider is a minimal implementation of providers.Provider for testing.
type MockProvider struct {
	ModelMappingVal    map[string]string
	NameVal            string
	BaseURLVal         string
	OwnerVal           string
	StreamingTypeVal   string
	StreamingVal       bool
	TransparentAuthVal bool
	BodyTransformVal   bool
}

func (m *MockProvider) Name() string    { return m.NameVal }
func (m *MockProvider) BaseURL() string { return m.BaseURLVal }
func (m *MockProvider) Owner() string   { return m.OwnerVal }
func (m *MockProvider) Authenticate(_ *http.Request, _ string) error {
	return nil
}
func (m *MockProvider) ForwardHeaders(h http.Header) http.Header { return h }
func (m *MockProvider) SupportsStreaming() bool                  { return m.StreamingVal }
func (m *MockProvider) SupportsTransparentAuth() bool            { return m.TransparentAuthVal }
func (m *MockProvider) ListModels() []providers.Model            { return nil }
func (m *MockProvider) GetModelMapping() map[string]string       { return m.ModelMappingVal }
func (m *MockProvider) MapModel(model string) string {
	if m, ok := m.ModelMappingVal[model]; ok {
		return m
	}
	return model
}
func (m *MockProvider) TransformRequest(
	body []byte, endpoint string,
) (transformedBody []byte, transformedEndpoint string, err error) {
	return body, endpoint, nil
}
func (m *MockProvider) TransformResponse(_ *http.Response, _ http.ResponseWriter) error {
	return nil
}
func (m *MockProvider) RequiresBodyTransform() bool { return m.BodyTransformVal }
func (m *MockProvider) StreamingContentType() string {
	if m.StreamingTypeVal != "" {
		return m.StreamingTypeVal
	}
	return "text/event-stream"
}
