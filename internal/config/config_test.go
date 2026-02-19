package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/config"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/rs/zerolog"
	"github.com/samber/mo"
)

// assertOption is a generic helper for testing mo.Option methods.
// It eliminates duplication across tests for GetMaxBodyLogSizeOption,
// GetMaxConcurrentOption, GetMaxBodyBytesOption, GetITPMLimitOption,
// and GetOTPMLimitOption.
func assertOption[T comparable](
	t *testing.T, name string, get func() mo.Option[T], wantSome bool, wantValue T,
) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		opt := get()
		if opt.IsPresent() != wantSome {
			t.Errorf("IsPresent() = %v, want %v", opt.IsPresent(), wantSome)
		}
		if wantSome {
			if got := opt.MustGet(); got != wantValue {
				t.Errorf("MustGet() = %v, want %v", got, wantValue)
			}
		}
	})
}

// zeroServerConfig returns a ServerConfig with all fields zeroed.
func zeroServerConfig() config.ServerConfig {
	return config.ServerConfig{
		Listen: "",
		APIKey: "",
		Auth: config.AuthConfig{
			APIKey: "", BearerSecret: "",
			AllowBearer: false, AllowSubscription: false,
		},
		TimeoutMS: 0, MaxConcurrent: 0, MaxBodyBytes: 0, EnableHTTP2: false,
	}
}

// zeroRoutingConfig returns a RoutingConfig with all fields zeroed.
func zeroRoutingConfig() config.RoutingConfig {
	return config.RoutingConfig{
		ModelMapping: nil, Strategy: "", DefaultProvider: "",
		FailoverTimeout: 0, Debug: false,
	}
}

// zeroProviderConfig returns a ProviderConfig with all fields zeroed.
func zeroProviderConfig() config.ProviderConfig {
	return config.ProviderConfig{
		ModelMapping: nil, AWSRegion: "", GCPProjectID: "",
		AzureAPIVersion: "", Name: "", Type: "", BaseURL: "",
		AzureDeploymentID: "", AWSAccessKeyID: "", AzureResourceName: "",
		AWSSecretAccessKey: "", GCPRegion: "",
		Keys: nil, Models: nil,
		Pooling: config.PoolingConfig{Strategy: "", Enabled: false},
		Enabled: false,
	}
}

// zeroAuthConfig returns an AuthConfig with all fields zeroed.
func zeroAuthConfig() config.AuthConfig {
	return config.AuthConfig{
		APIKey: "", BearerSecret: "",
		AllowBearer: false, AllowSubscription: false,
	}
}

// zeroDebugOptions returns a DebugOptions with all fields zeroed.
func zeroDebugOptions() config.DebugOptions {
	return config.DebugOptions{
		LogRequestBody: false, LogResponseHeaders: false,
		LogTLSMetrics: false, MaxBodyLogSize: 0,
	}
}

// zeroKeyConfig returns a KeyConfig with all fields zeroed.
func zeroKeyConfig() config.KeyConfig {
	return config.KeyConfig{
		Key: "", RPMLimit: 0, ITPMLimit: 0, OTPMLimit: 0,
		Priority: 0, Weight: 0, TPMLimit: 0,
	}
}

// zeroLoggingConfig returns a LoggingConfig with all fields zeroed.
func zeroLoggingConfig() config.LoggingConfig {
	return config.LoggingConfig{
		Level: "", Format: "", Output: "", Pretty: false,
		DebugOptions: zeroDebugOptions(),
	}
}

// providerWithKeys creates a zero ProviderConfig with specified keys and pooling.
func providerWithKeys(
	keys []config.KeyConfig, poolingEnabled bool,
) config.ProviderConfig {
	prov := zeroProviderConfig()
	prov.Keys = keys
	prov.Pooling.Enabled = poolingEnabled
	return prov
}

// providerWithPooling creates a zero ProviderConfig with specified pooling strategy.
func providerWithPooling(strategy string) config.ProviderConfig {
	prov := zeroProviderConfig()
	prov.Pooling.Strategy = strategy
	return prov
}

// providerWithType creates a zero ProviderConfig with specified type and cloud fields.
func providerWithType(
	pType, awsRegion, gcpProject, gcpRegion, azureResource string,
) config.ProviderConfig {
	prov := zeroProviderConfig()
	prov.Type = pType
	prov.AWSRegion = awsRegion
	prov.GCPProjectID = gcpProject
	prov.GCPRegion = gcpRegion
	prov.AzureResourceName = azureResource
	return prov
}

// serverWithTimeout returns a zero ServerConfig with the given TimeoutMS.
func serverWithTimeout(ms int) config.ServerConfig {
	s := zeroServerConfig()
	s.TimeoutMS = ms
	return s
}

// serverWithMaxConcurrent returns a zero ServerConfig with the given MaxConcurrent.
func serverWithMaxConcurrent(n int) config.ServerConfig {
	s := zeroServerConfig()
	s.MaxConcurrent = n
	return s
}

// serverWithMaxBodyBytes returns a zero ServerConfig with the given MaxBodyBytes.
func serverWithMaxBodyBytes(n int64) config.ServerConfig {
	s := zeroServerConfig()
	s.MaxBodyBytes = n
	return s
}

// serverWithAPIKeys returns a zero ServerConfig with legacy and auth API keys.
func serverWithAPIKeys(legacy, auth string) config.ServerConfig {
	s := zeroServerConfig()
	s.APIKey = legacy
	s.Auth.APIKey = auth
	return s
}

// keyWithField creates a key config with a specific field set.
func keyWithField(key, field string, value int) config.KeyConfig {
	keyCfg := zeroKeyConfig()
	keyCfg.Key = key
	switch field {
	case "RPMLimit":
		keyCfg.RPMLimit = value
	case "ITPMLimit":
		keyCfg.ITPMLimit = value
	case "OTPMLimit":
		keyCfg.OTPMLimit = value
	case "Priority":
		keyCfg.Priority = value
	case "Weight":
		keyCfg.Weight = value
	case "TPMLimit":
		keyCfg.TPMLimit = value
	}
	return keyCfg
}

// routingWithStrategy returns a zero RoutingConfig with the given strategy.
func routingWithStrategy(s string) config.RoutingConfig {
	r := zeroRoutingConfig()
	r.Strategy = s
	return r
}

// routingWithTimeout returns a zero RoutingConfig with the given failover timeout.
func routingWithTimeout(ms int) config.RoutingConfig {
	r := zeroRoutingConfig()
	r.FailoverTimeout = ms
	return r
}

// routingWithDebug returns a zero RoutingConfig with the given debug setting.
func routingWithDebug(debug bool) config.RoutingConfig {
	r := zeroRoutingConfig()
	r.Debug = debug
	return r
}

func TestLoggingConfigParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		level    string
		expected zerolog.Level
	}{
		{"debug level", "debug", zerolog.DebugLevel},
		{"info level", "info", zerolog.InfoLevel},
		{"warn level", "warn", zerolog.WarnLevel},
		{"error level", "error", zerolog.ErrorLevel},
		{"uppercase DEBUG", "DEBUG", zerolog.DebugLevel},
		{"mixed case Info", "Info", zerolog.InfoLevel},
		{"invalid level defaults to info", "invalid", zerolog.InfoLevel},
		{"empty level defaults to info", "", zerolog.InfoLevel},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cfg := zeroLoggingConfig()
			cfg.Level = testCase.level

			got := cfg.ParseLevel()
			if got != testCase.expected {
				t.Errorf("ParseLevel() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestAuthConfigIsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   config.AuthConfig
		expected bool
	}{
		{"no auth configured", zeroAuthConfig(), false},
		{
			"api key only",
			config.AuthConfig{APIKey: "test-key", BearerSecret: "",
				AllowBearer: false, AllowSubscription: false},
			true,
		},
		{
			"bearer only",
			config.AuthConfig{APIKey: "", BearerSecret: "",
				AllowBearer: true, AllowSubscription: false},
			true,
		},
		{
			"both configured",
			config.AuthConfig{APIKey: "test-key", BearerSecret: "",
				AllowBearer: true, AllowSubscription: false},
			true,
		},
		{
			"bearer secret without allow bearer",
			config.AuthConfig{APIKey: "", BearerSecret: "secret",
				AllowBearer: false, AllowSubscription: false},
			false,
		},
		{
			"subscription only",
			config.AuthConfig{APIKey: "", BearerSecret: "",
				AllowBearer: false, AllowSubscription: true},
			true,
		},
		{
			"subscription and api key",
			config.AuthConfig{APIKey: "test-key", BearerSecret: "",
				AllowBearer: false, AllowSubscription: true},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.IsEnabled(); got != testCase.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestAuthConfigIsBearerEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   config.AuthConfig
		expected bool
	}{
		{"no bearer configured", zeroAuthConfig(), false},
		{
			"allow_bearer true",
			config.AuthConfig{APIKey: "", BearerSecret: "",
				AllowBearer: true, AllowSubscription: false},
			true,
		},
		{
			"allow_subscription true",
			config.AuthConfig{APIKey: "", BearerSecret: "",
				AllowBearer: false, AllowSubscription: true},
			true,
		},
		{
			"both bearer and subscription",
			config.AuthConfig{APIKey: "", BearerSecret: "",
				AllowBearer: true, AllowSubscription: true},
			true,
		},
		{
			"api key only does not enable bearer",
			config.AuthConfig{APIKey: "test-key", BearerSecret: "",
				AllowBearer: false, AllowSubscription: false},
			false,
		},
		{
			"bearer secret without allow flag",
			config.AuthConfig{APIKey: "", BearerSecret: "secret",
				AllowBearer: false, AllowSubscription: false},
			false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.IsBearerEnabled(); got != testCase.expected {
				t.Errorf("IsBearerEnabled() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestServerConfigGetEffectiveAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		config   config.ServerConfig
	}{
		{"no api key", "", zeroServerConfig()},
		{"legacy api key only", "legacy-key", serverWithAPIKeys("legacy-key", "")},
		{"auth api key only", "auth-key", serverWithAPIKeys("", "auth-key")},
		{"both - auth takes precedence", "auth-key", serverWithAPIKeys("legacy-key", "auth-key")},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.GetEffectiveAPIKey(); got != testCase.expected {
				t.Errorf("GetEffectiveAPIKey() = %q, want %q", got, testCase.expected)
			}
		})
	}
}

func TestLoggingConfigEnableAllDebugOptions(t *testing.T) {
	t.Parallel()

	cfg := zeroLoggingConfig()
	cfg.Level = "info"

	cfg.EnableAllDebugOptions()

	if cfg.Level != config.LevelDebug {
		t.Errorf("Expected level '%s', got %q", config.LevelDebug, cfg.Level)
	}
	if !cfg.DebugOptions.LogRequestBody {
		t.Error("Expected LogRequestBody to be true")
	}
	if !cfg.DebugOptions.LogResponseHeaders {
		t.Error("Expected LogResponseHeaders to be true")
	}
	if !cfg.DebugOptions.LogTLSMetrics {
		t.Error("Expected LogTLSMetrics to be true")
	}
	if cfg.DebugOptions.MaxBodyLogSize != 1000 {
		t.Errorf("Expected MaxBodyLogSize 1000, got %d", cfg.DebugOptions.MaxBodyLogSize)
	}
}

func TestDebugOptionsGetMaxBodyLogSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		size     int
		expected int
	}{
		{"default value when zero", 0, 1000},
		{"default value when negative", -1, 1000},
		{"custom value", 5000, 5000},
		{"small custom value", 100, 100},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			opts := zeroDebugOptions()
			opts.MaxBodyLogSize = testCase.size
			if got := opts.GetMaxBodyLogSize(); got != testCase.expected {
				t.Errorf("GetMaxBodyLogSize() = %d, want %d", got, testCase.expected)
			}
		})
	}
}

func TestDebugOptionsIsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     config.DebugOptions
		expected bool
	}{
		{"all disabled", zeroDebugOptions(), false},
		{
			"only LogRequestBody",
			config.DebugOptions{
				LogRequestBody: true, LogResponseHeaders: false,
				LogTLSMetrics: false, MaxBodyLogSize: 0},
			true,
		},
		{
			"only LogResponseHeaders",
			config.DebugOptions{
				LogRequestBody: false, LogResponseHeaders: true,
				LogTLSMetrics: false, MaxBodyLogSize: 0},
			true,
		},
		{
			"only LogTLSMetrics",
			config.DebugOptions{
				LogRequestBody: false, LogResponseHeaders: false,
				LogTLSMetrics: true, MaxBodyLogSize: 0},
			true,
		},
		{
			"all enabled",
			config.DebugOptions{
				LogRequestBody: true, LogResponseHeaders: true,
				LogTLSMetrics: true, MaxBodyLogSize: 0},
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.opts.IsEnabled(); got != testCase.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestDebugOptionsIsEnabledMaxBodyLogSizeAlone(t *testing.T) {
	t.Parallel()

	opts := zeroDebugOptions()
	opts.MaxBodyLogSize = 5000
	if opts.IsEnabled() {
		t.Error("MaxBodyLogSize alone should not enable debug options")
	}
}

func TestKeyConfigGetEffectiveTPM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       config.KeyConfig
		expectedITPM int
		expectedOTPM int
	}{
		{
			"ITPM and OTPM set",
			config.KeyConfig{Key: "", RPMLimit: 0, ITPMLimit: 30000,
				OTPMLimit: 10000, Priority: 0, Weight: 0, TPMLimit: 0},
			30000, 10000,
		},
		{
			"only ITPM set",
			config.KeyConfig{Key: "", RPMLimit: 0, ITPMLimit: 30000,
				OTPMLimit: 0, Priority: 0, Weight: 0, TPMLimit: 0},
			30000, 0,
		},
		{
			"only OTPM set",
			config.KeyConfig{Key: "", RPMLimit: 0, ITPMLimit: 0,
				OTPMLimit: 10000, Priority: 0, Weight: 0, TPMLimit: 0},
			0, 10000,
		},
		{
			"legacy TPMLimit",
			config.KeyConfig{Key: "", RPMLimit: 0, ITPMLimit: 0,
				OTPMLimit: 0, Priority: 0, Weight: 0, TPMLimit: 40000},
			20000, 20000,
		},
		{
			"ITPM/OTPM preferred",
			config.KeyConfig{Key: "", RPMLimit: 0, ITPMLimit: 30000,
				OTPMLimit: 10000, Priority: 0, Weight: 0, TPMLimit: 40000},
			30000, 10000,
		},
		{"no limits set", zeroKeyConfig(), 0, 0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			itpm, otpm := testCase.config.GetEffectiveTPM()
			if itpm != testCase.expectedITPM {
				t.Errorf("ITPM = %d, want %d", itpm, testCase.expectedITPM)
			}
			if otpm != testCase.expectedOTPM {
				t.Errorf("OTPM = %d, want %d", otpm, testCase.expectedOTPM)
			}
		})
	}
}

// runKeyValidation is a helper to test KeyConfig.Validate.
func runKeyValidation(
	t *testing.T, name string, cfg config.KeyConfig,
	wantErr bool, check func(t *testing.T, err error),
) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		err := cfg.Validate()
		if wantErr {
			if err == nil {
				t.Error("Validate() expected error, got nil")
				return
			}
			if check != nil {
				check(t, err)
			}
		} else if err != nil {
			t.Errorf("Validate() expected no error, got %v", err)
		}
	})
}

func TestKeyConfigValidateValid(t *testing.T) {
	t.Parallel()

	runKeyValidation(t, "valid key with all fields", config.KeyConfig{
		Key: "sk-test123", RPMLimit: 50, ITPMLimit: 30000,
		OTPMLimit: 10000, Priority: 2, Weight: 5, TPMLimit: 0,
	}, false, nil)

	runKeyValidation(t, "valid key with defaults", config.KeyConfig{
		Key: "sk-test123", RPMLimit: 0, ITPMLimit: 0,
		OTPMLimit: 0, Priority: 0, Weight: 0, TPMLimit: 0,
	}, false, nil)

	for _, p := range []int{0, 1, 2} {
		k := zeroKeyConfig()
		k.Key = "sk-test123"
		k.Priority = p
		runKeyValidation(t, "valid priority "+strings.Repeat("I", p), k, false, nil)
	}

	k := zeroKeyConfig()
	k.Key = "sk-test123"
	runKeyValidation(t, "zero weight is valid", k, false, nil)
}

func TestKeyConfigValidateErrors(t *testing.T) {
	t.Parallel()

	runKeyValidation(t, "empty key", zeroKeyConfig(), true, func(t *testing.T, err error) {
		t.Helper()
		if !errors.Is(err, config.ErrKeyRequired) {
			t.Errorf("Expected ErrKeyRequired, got %v", err)
		}
	})

	priorityHigh := keyWithField("sk-test123", "Priority", 3)
	runKeyValidation(t, "priority too high", priorityHigh, true,
		func(t *testing.T, err error) {
			t.Helper()
			var priorityErr config.InvalidPriorityError
			if !errors.As(err, &priorityErr) {
				t.Errorf("Expected InvalidPriorityError, got %T", err)
			}
		})

	priorityNeg := keyWithField("sk-test123", "Priority", -1)
	runKeyValidation(t, "priority negative", priorityNeg, true,
		func(t *testing.T, err error) {
			t.Helper()
			var priorityErr config.InvalidPriorityError
			if !errors.As(err, &priorityErr) {
				t.Errorf("Expected InvalidPriorityError, got %T", err)
			}
		})

	weightNeg := keyWithField("sk-test123", "Weight", -1)
	runKeyValidation(t, "negative weight", weightNeg, true,
		func(t *testing.T, err error) {
			t.Helper()
			var weightErr config.InvalidWeightError
			if !errors.As(err, &weightErr) {
				t.Errorf("Expected InvalidWeightError, got %T", err)
			}
		})
}

func TestProviderConfigGetEffectiveStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		config   config.ProviderConfig
	}{
		{"least_loaded", "least_loaded", providerWithPooling("least_loaded")},
		{"round_robin", "round_robin", providerWithPooling("round_robin")},
		{"weighted", "weighted", providerWithPooling("weighted")},
		{"no strategy defaults", "least_loaded", zeroProviderConfig()},
		{"empty strategy defaults", "least_loaded", providerWithPooling("")},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.GetEffectiveStrategy(); got != testCase.expected {
				t.Errorf("GetEffectiveStrategy() = %q, want %q", got, testCase.expected)
			}
		})
	}
}

func TestProviderConfigIsPoolingEnabled(t *testing.T) {
	t.Parallel()

	const testKey1, testKey2, testKey3 = "key1", "key2", "key3"

	oneKey := []config.KeyConfig{zeroKeyConfig()}
	oneKey[0].Key = testKey1

	twoKeys := []config.KeyConfig{zeroKeyConfig(), zeroKeyConfig()}
	twoKeys[0].Key = testKey1
	twoKeys[1].Key = testKey2

	threeKeys := []config.KeyConfig{zeroKeyConfig(), zeroKeyConfig(), zeroKeyConfig()}
	threeKeys[0].Key = testKey1
	threeKeys[1].Key = testKey2
	threeKeys[2].Key = testKey3

	tests := []struct {
		name     string
		config   config.ProviderConfig
		expected bool
	}{
		{"explicitly enabled single key", providerWithKeys(oneKey, true), true},
		{"explicitly enabled multi keys", providerWithKeys(twoKeys, true), true},
		{"multi keys auto-enabled", providerWithKeys(twoKeys, false), true},
		{"three keys auto-enabled", providerWithKeys(threeKeys, false), true},
		{"single key not enabled", providerWithKeys(oneKey, false), false},
		{"no keys not enabled", providerWithKeys([]config.KeyConfig{}, false), false},
		{"explicitly enabled overrides single", providerWithKeys(oneKey, true), true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.IsPoolingEnabled(); got != testCase.expected {
				t.Errorf("IsPoolingEnabled() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

// Tests for mo.Option helper methods.

func TestDebugOptionsGetMaxBodyLogSizeOption(t *testing.T) {
	t.Parallel()

	opts1 := zeroDebugOptions()
	assertOption(t, "zero returns None", opts1.GetMaxBodyLogSizeOption, false, 0)

	opts2 := zeroDebugOptions()
	opts2.MaxBodyLogSize = -1
	assertOption(t, "negative returns None", opts2.GetMaxBodyLogSizeOption, false, 0)

	opts3 := zeroDebugOptions()
	opts3.MaxBodyLogSize = 5000
	assertOption(t, "positive returns Some", opts3.GetMaxBodyLogSizeOption, true, 5000)
}

func TestServerConfigGetTimeoutOption(t *testing.T) {
	t.Parallel()

	cfg0 := serverWithTimeout(0)
	assertOption(t, "zero returns None", cfg0.GetTimeoutOption, false, time.Duration(0))

	cfgNeg := serverWithTimeout(-1)
	assertOption(t, "negative returns None", cfgNeg.GetTimeoutOption, false, time.Duration(0))

	cfgPos := serverWithTimeout(5000)
	assertOption(t, "positive returns Some", cfgPos.GetTimeoutOption, true, 5*time.Second)
}

func TestServerConfigGetMaxConcurrentOption(t *testing.T) {
	t.Parallel()

	cfg0 := serverWithMaxConcurrent(0)
	assertOption(t, "zero returns None", cfg0.GetMaxConcurrentOption, false, 0)

	cfgNeg := serverWithMaxConcurrent(-1)
	assertOption(t, "negative returns None", cfgNeg.GetMaxConcurrentOption, false, 0)

	cfgPos := serverWithMaxConcurrent(100)
	assertOption(t, "positive returns Some", cfgPos.GetMaxConcurrentOption, true, 100)
}

func TestServerConfigGetMaxBodyBytesOption(t *testing.T) {
	t.Parallel()

	cfg0 := serverWithMaxBodyBytes(0)
	assertOption(t, "zero returns None", cfg0.GetMaxBodyBytesOption, false, int64(0))

	cfgNeg := serverWithMaxBodyBytes(-1)
	assertOption(t, "negative returns None", cfgNeg.GetMaxBodyBytesOption, false, int64(0))

	cfgPos := serverWithMaxBodyBytes(10485760)
	assertOption(t, "positive returns Some", cfgPos.GetMaxBodyBytesOption, true, int64(10485760))
}

func TestKeyConfigGetRPMLimitOption(t *testing.T) {
	t.Parallel()

	cfg0 := keyWithField("test", "RPMLimit", 0)
	assertOption(t, "zero returns None", cfg0.GetRPMLimitOption, false, 0)

	cfgNeg := keyWithField("test", "RPMLimit", -1)
	assertOption(t, "negative returns None", cfgNeg.GetRPMLimitOption, false, 0)

	cfgPos := keyWithField("test", "RPMLimit", 50)
	assertOption(t, "positive returns Some", cfgPos.GetRPMLimitOption, true, 50)
}

func TestKeyConfigGetITPMLimitOption(t *testing.T) {
	t.Parallel()

	cfg0 := keyWithField("test", "ITPMLimit", 0)
	assertOption(t, "zero returns None", cfg0.GetITPMLimitOption, false, 0)

	cfgPos := keyWithField("test", "ITPMLimit", 30000)
	assertOption(t, "positive returns Some", cfgPos.GetITPMLimitOption, true, 30000)
}

func TestKeyConfigGetOTPMLimitOption(t *testing.T) {
	t.Parallel()

	cfg0 := keyWithField("test", "OTPMLimit", 0)
	assertOption(t, "zero returns None", cfg0.GetOTPMLimitOption, false, 0)

	cfgPos := keyWithField("test", "OTPMLimit", 10000)
	assertOption(t, "positive returns Some", cfgPos.GetOTPMLimitOption, true, 10000)
}

// Test Option usage with OrElse pattern.

func TestOptionOrElseTimeout(t *testing.T) {
	t.Parallel()

	defaultTimeout := 30 * time.Second

	cfg0 := serverWithTimeout(0)
	timeout := cfg0.GetTimeoutOption().OrElse(defaultTimeout)
	if timeout != defaultTimeout {
		t.Errorf("Expected default timeout %v, got %v", defaultTimeout, timeout)
	}

	cfg1 := serverWithTimeout(5000)
	timeout2 := cfg1.GetTimeoutOption().OrElse(defaultTimeout)
	if timeout2 != 5*time.Second {
		t.Errorf("Expected 5s timeout, got %v", timeout2)
	}
}

func TestOptionOrElseMaxConcurrent(t *testing.T) {
	t.Parallel()

	defaultMax := 1000

	cfg0 := serverWithMaxConcurrent(0)
	maxConc := cfg0.GetMaxConcurrentOption().OrElse(defaultMax)
	if maxConc != defaultMax {
		t.Errorf("Expected default %d, got %d", defaultMax, maxConc)
	}

	cfg1 := serverWithMaxConcurrent(50)
	maxConc2 := cfg1.GetMaxConcurrentOption().OrElse(defaultMax)
	if maxConc2 != 50 {
		t.Errorf("Expected 50, got %d", maxConc2)
	}
}

// Tests for RoutingConfig.

func TestRoutingConfigGetEffectiveStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		config   config.RoutingConfig
	}{
		{"empty defaults to failover", "failover", routingWithStrategy("")},
		{"zero value defaults to failover", "failover", zeroRoutingConfig()},
		{"configured failover", "failover", routingWithStrategy("failover")},
		{"configured round_robin", "round_robin", routingWithStrategy("round_robin")},
		{"configured weighted_round_robin", "weighted_round_robin", routingWithStrategy("weighted_round_robin")},
		{"configured shuffle", "shuffle", routingWithStrategy("shuffle")},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.GetEffectiveStrategy(); got != testCase.expected {
				t.Errorf("GetEffectiveStrategy() = %q, want %q", got, testCase.expected)
			}
		})
	}
}

func TestRoutingConfigGetFailoverTimeoutOption(t *testing.T) {
	t.Parallel()

	r0 := routingWithTimeout(0)
	assertOption(t, "zero returns None", r0.GetFailoverTimeoutOption, false, time.Duration(0))

	rNeg := routingWithTimeout(-100)
	assertOption(t, "negative returns None", rNeg.GetFailoverTimeoutOption, false, time.Duration(0))

	rPos := routingWithTimeout(5000)
	assertOption(t, "positive returns Some", rPos.GetFailoverTimeoutOption, true, 5*time.Second)

	rSmall := routingWithTimeout(100)
	assertOption(t, "small value returns correct duration", rSmall.GetFailoverTimeoutOption, true, 100*time.Millisecond)
}

func TestRoutingConfigIsDebugEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   config.RoutingConfig
		expected bool
	}{
		{"default is false", zeroRoutingConfig(), false},
		{"explicit false", routingWithDebug(false), false},
		{"explicit true", routingWithDebug(true), true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.IsDebugEnabled(); got != testCase.expected {
				t.Errorf("IsDebugEnabled() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestRoutingConfigOptionOrElsePattern(t *testing.T) {
	t.Parallel()

	defaultTimeout := 5 * time.Second

	r0 := routingWithTimeout(0)
	timeout := r0.GetFailoverTimeoutOption().OrElse(defaultTimeout)
	if timeout != defaultTimeout {
		t.Errorf("Expected default timeout %v, got %v", defaultTimeout, timeout)
	}

	r1 := routingWithTimeout(10000)
	timeout2 := r1.GetFailoverTimeoutOption().OrElse(defaultTimeout)
	if timeout2 != 10*time.Second {
		t.Errorf("Expected 10s timeout, got %v", timeout2)
	}
}

func TestProviderConfigGetAzureAPIVersion(t *testing.T) {
	t.Parallel()

	configured := zeroProviderConfig()
	configured.AzureAPIVersion = "2023-12-01"

	tests := []struct {
		name     string
		expected string
		config   config.ProviderConfig
	}{
		{"returns default when empty", "2024-06-01", zeroProviderConfig()},
		{"returns configured version", "2023-12-01", configured},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := testCase.config.GetAzureAPIVersion(); got != testCase.expected {
				t.Errorf("GetAzureAPIVersion() = %v, want %v", got, testCase.expected)
			}
		})
	}
}

func TestProviderConfigValidateCloudConfigNonCloud(t *testing.T) {
	t.Parallel()

	for _, pType := range []string{"anthropic", "zai", "ollama"} {
		p := providerWithType(pType, "", "", "", "")
		t.Run(pType+" passes", func(t *testing.T) {
			t.Parallel()
			if err := p.ValidateCloudConfig(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestProviderConfigValidateCloudConfigBedrock(t *testing.T) {
	t.Parallel()

	t.Run("with region passes", func(t *testing.T) {
		t.Parallel()
		p := providerWithType("bedrock", "us-east-1", "", "", "")
		if err := p.ValidateCloudConfig(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("without region fails", func(t *testing.T) {
		t.Parallel()
		p := providerWithType("bedrock", "", "", "", "")
		err := p.ValidateCloudConfig()
		if err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "aws_region required") {
			t.Errorf("error = %v, want containing 'aws_region required'", err)
		}
	})
}

func TestProviderConfigValidateCloudConfigVertex(t *testing.T) {
	t.Parallel()

	t.Run("with all fields passes", func(t *testing.T) {
		t.Parallel()
		p := providerWithType("vertex", "", "my-project", "us-central1", "")
		if err := p.ValidateCloudConfig(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("without project ID fails", func(t *testing.T) {
		t.Parallel()
		p := providerWithType("vertex", "", "", "us-central1", "")
		err := p.ValidateCloudConfig()
		if err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "gcp_project_id required") {
			t.Errorf("error = %v, want containing 'gcp_project_id required'", err)
		}
	})

	t.Run("without region fails", func(t *testing.T) {
		t.Parallel()
		p := providerWithType("vertex", "", "my-project", "", "")
		err := p.ValidateCloudConfig()
		if err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "gcp_region required") {
			t.Errorf("error = %v, want containing 'gcp_region required'", err)
		}
	})
}

func TestProviderConfigValidateCloudConfigAzure(t *testing.T) {
	t.Parallel()

	t.Run("with resource name passes", func(t *testing.T) {
		t.Parallel()
		p := providerWithType("azure", "", "", "", "my-resource")
		if err := p.ValidateCloudConfig(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("without resource name fails", func(t *testing.T) {
		t.Parallel()
		p := providerWithType("azure", "", "", "", "")
		err := p.ValidateCloudConfig()
		if err == nil {
			t.Error("expected error, got nil")
		} else if !strings.Contains(err.Error(), "azure_resource_name required") {
			t.Errorf("error = %v, want containing 'azure_resource_name required'", err)
		}
	})
}

// Ensure unused imports are referenced.
var (
	_ cache.Config
	_ health.Config
)
