package config

import (
	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/health"
)

// DetectFormat exports detectFormat for testing.
var DetectFormat = detectFormat

// Test helpers with all fields initialized for exhaustruct compliance.

// MakeTestConfig returns a minimal valid Config with all fields set.
func MakeTestConfig() *Config {
	return &Config{
		Providers: []ProviderConfig{},
		Routing:   MakeTestRoutingConfig(),
		Logging:   MakeTestLoggingConfig(),
		Health:    MakeTestHealthConfig(),
		Server:    MakeTestServerConfig(),
		Cache:     MakeTestCacheConfig(),
	}
}

// MakeTestServerConfig returns a minimal ServerConfig with all fields set.
func MakeTestServerConfig() ServerConfig {
	return ServerConfig{
		Listen:        "127.0.0.1:8787",
		APIKey:        "",
		Auth:          MakeTestAuthConfig(),
		TimeoutMS:     60000,
		MaxConcurrent: 0,
		MaxBodyBytes:  0,
		EnableHTTP2:   false,
	}
}

// MakeTestAuthConfig returns a minimal AuthConfig with all fields set.
func MakeTestAuthConfig() AuthConfig {
	return AuthConfig{
		APIKey:            "",
		BearerSecret:      "",
		AllowBearer:       false,
		AllowSubscription: false,
	}
}

// MakeTestProviderConfig returns a minimal ProviderConfig with all fields set.
func MakeTestProviderConfig() ProviderConfig {
	return ProviderConfig{
		ModelMapping:       map[string]string{},
		AWSRegion:          "",
		GCPProjectID:       "",
		AzureAPIVersion:    "",
		Name:               "test",
		Type:               "anthropic",
		BaseURL:            "",
		AzureDeploymentID:  "",
		AWSAccessKeyID:     "",
		AzureResourceName:  "",
		AWSSecretAccessKey: "",
		GCPRegion:          "",
		Keys:               []KeyConfig{},
		Models:             []string{},
		Pooling:            MakeTestPoolingConfig(),
		Enabled:            true,
	}
}

// MakeTestPoolingConfig returns a minimal PoolingConfig with all fields set.
func MakeTestPoolingConfig() PoolingConfig {
	return PoolingConfig{
		Strategy: "",
		Enabled:  false,
	}
}

// MakeTestKeyConfig returns a minimal KeyConfig with all fields set.
func MakeTestKeyConfig(key string) KeyConfig {
	return KeyConfig{
		Key:       key,
		RPMLimit:  0,
		ITPMLimit: 0,
		OTPMLimit: 0,
		Priority:  1,
		Weight:    1,
		TPMLimit:  0,
	}
}

// MakeTestLoggingConfig returns a minimal LoggingConfig with all fields set.
func MakeTestLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:        "info",
		Format:       "json",
		Output:       "stdout",
		Pretty:       false,
		DebugOptions: MakeTestDebugOptions(),
	}
}

// MakeTestDebugOptions returns a minimal DebugOptions with all fields set.
func MakeTestDebugOptions() DebugOptions {
	return DebugOptions{
		LogRequestBody:     false,
		LogResponseHeaders: false,
		LogTLSMetrics:      false,
		MaxBodyLogSize:     1000,
	}
}

// MakeTestRoutingConfig returns a minimal RoutingConfig with all fields set.
func MakeTestRoutingConfig() RoutingConfig {
	return RoutingConfig{
		ModelMapping:    map[string]string{},
		Strategy:        "",
		DefaultProvider: "",
		FailoverTimeout: 5000,
		Debug:           false,
	}
}

// MakeTestHealthConfig returns a minimal health.Config with all fields set.
func MakeTestHealthConfig() health.Config {
	return health.Config{
		HealthCheck: health.CheckConfig{
			Enabled:    boolPtr(true),
			IntervalMS: 10000,
		},
		CircuitBreaker: health.CircuitBreakerConfig{
			OpenDurationMS:   30000,
			FailureThreshold: 5,
			HalfOpenProbes:   3,
		},
	}
}

// MakeTestCacheConfig returns a minimal cache.Config with all fields set.
func MakeTestCacheConfig() cache.Config {
	return cache.Config{
		Mode:      cache.ModeDisabled,
		Olric:     cache.DefaultOlricConfig(),
		Ristretto: cache.DefaultRistrettoConfig(),
	}
}

// MakeTestValidationError returns a ValidationError with Errors initialized.
func MakeTestValidationError() *ValidationError {
	return &ValidationError{
		Errors: []string{},
	}
}

// boolPtr returns a pointer to a bool.
func boolPtr(b bool) *bool {
	return &b
}
