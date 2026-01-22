package config

import (
	"errors"
	"testing"

	"github.com/rs/zerolog"
)

func TestLoggingConfig_ParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected zerolog.Level
	}{
		{
			name:     "debug level",
			level:    "debug",
			expected: zerolog.DebugLevel,
		},
		{
			name:     "info level",
			level:    "info",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "warn level",
			level:    "warn",
			expected: zerolog.WarnLevel,
		},
		{
			name:     "error level",
			level:    "error",
			expected: zerolog.ErrorLevel,
		},
		{
			name:     "uppercase DEBUG",
			level:    "DEBUG",
			expected: zerolog.DebugLevel,
		},
		{
			name:     "mixed case Info",
			level:    "Info",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "invalid level defaults to info",
			level:    "invalid",
			expected: zerolog.InfoLevel,
		},
		{
			name:     "empty level defaults to info",
			level:    "",
			expected: zerolog.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := LoggingConfig{Level: tt.level}

			got := cfg.ParseLevel()
			if got != tt.expected {
				t.Errorf("ParseLevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthConfig_IsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   AuthConfig
		expected bool
	}{
		{
			name:     "no auth configured",
			config:   AuthConfig{},
			expected: false,
		},
		{
			name:     "api key only",
			config:   AuthConfig{APIKey: "test-key"},
			expected: true,
		},
		{
			name:     "bearer only",
			config:   AuthConfig{AllowBearer: true},
			expected: true,
		},
		{
			name:     "both configured",
			config:   AuthConfig{APIKey: "test-key", AllowBearer: true},
			expected: true,
		},
		{
			name:     "bearer secret without allow bearer",
			config:   AuthConfig{BearerSecret: "secret"},
			expected: false,
		},
		{
			name:     "subscription only",
			config:   AuthConfig{AllowSubscription: true},
			expected: true,
		},
		{
			name:     "subscription and api key",
			config:   AuthConfig{APIKey: "test-key", AllowSubscription: true},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.config.IsEnabled()
			if got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthConfig_IsBearerEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   AuthConfig
		expected bool
	}{
		{
			name:     "no bearer configured",
			config:   AuthConfig{},
			expected: false,
		},
		{
			name:     "allow_bearer true",
			config:   AuthConfig{AllowBearer: true},
			expected: true,
		},
		{
			name:     "allow_subscription true",
			config:   AuthConfig{AllowSubscription: true},
			expected: true,
		},
		{
			name:     "both bearer and subscription",
			config:   AuthConfig{AllowBearer: true, AllowSubscription: true},
			expected: true,
		},
		{
			name:     "api key only does not enable bearer",
			config:   AuthConfig{APIKey: "test-key"},
			expected: false,
		},
		{
			name:     "bearer secret without allow flag",
			config:   AuthConfig{BearerSecret: "secret"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.config.IsBearerEnabled()
			if got != tt.expected {
				t.Errorf("IsBearerEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServerConfig_GetEffectiveAPIKey(t *testing.T) {
	t.Parallel()

	tests := []struct { //nolint:govet // test table struct alignment
		name     string
		config   ServerConfig
		expected string
	}{
		{
			name:     "no api key",
			config:   ServerConfig{},
			expected: "",
		},
		{
			name:     "legacy api key only",
			config:   ServerConfig{APIKey: "legacy-key"},
			expected: "legacy-key",
		},
		{
			name:     "auth api key only",
			config:   ServerConfig{Auth: AuthConfig{APIKey: "auth-key"}},
			expected: "auth-key",
		},
		{
			name:     "both - auth takes precedence",
			config:   ServerConfig{APIKey: "legacy-key", Auth: AuthConfig{APIKey: "auth-key"}},
			expected: "auth-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.config.GetEffectiveAPIKey()
			if got != tt.expected {
				t.Errorf("GetEffectiveAPIKey() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLoggingConfig_EnableAllDebugOptions(t *testing.T) {
	t.Parallel()

	cfg := LoggingConfig{
		Level: "info",
		DebugOptions: DebugOptions{
			LogRequestBody:     false,
			LogResponseHeaders: false,
			LogTLSMetrics:      false,
			MaxBodyLogSize:     0,
		},
	}

	cfg.EnableAllDebugOptions()

	// Verify level is set to debug
	if cfg.Level != LevelDebug {
		t.Errorf("Expected level '%s', got %q", LevelDebug, cfg.Level)
	}

	// Verify all debug options are enabled
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

func TestDebugOptions_GetMaxBodyLogSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     DebugOptions
		expected int
	}{
		{
			name:     "default value when zero",
			opts:     DebugOptions{MaxBodyLogSize: 0},
			expected: 1000,
		},
		{
			name:     "default value when negative",
			opts:     DebugOptions{MaxBodyLogSize: -1},
			expected: 1000,
		},
		{
			name:     "custom value",
			opts:     DebugOptions{MaxBodyLogSize: 5000},
			expected: 5000,
		},
		{
			name:     "small custom value",
			opts:     DebugOptions{MaxBodyLogSize: 100},
			expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.opts.GetMaxBodyLogSize()
			if got != tt.expected {
				t.Errorf("GetMaxBodyLogSize() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestDebugOptions_IsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     DebugOptions
		expected bool
	}{
		{
			name:     "all disabled",
			opts:     DebugOptions{},
			expected: false,
		},
		{
			name:     "only LogRequestBody",
			opts:     DebugOptions{LogRequestBody: true},
			expected: true,
		},
		{
			name:     "only LogResponseHeaders",
			opts:     DebugOptions{LogResponseHeaders: true},
			expected: true,
		},
		{
			name:     "only LogTLSMetrics",
			opts:     DebugOptions{LogTLSMetrics: true},
			expected: true,
		},
		{
			name:     "all enabled",
			opts:     DebugOptions{LogRequestBody: true, LogResponseHeaders: true, LogTLSMetrics: true},
			expected: true,
		},
		{
			name:     "MaxBodyLogSize alone does not enable",
			opts:     DebugOptions{MaxBodyLogSize: 5000},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.opts.IsEnabled()
			if got != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestKeyConfig_GetEffectiveTPM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       KeyConfig
		expectedITPM int
		expectedOTPM int
	}{
		{
			name: "ITPM and OTPM set",
			config: KeyConfig{
				ITPMLimit: 30000,
				OTPMLimit: 10000,
			},
			expectedITPM: 30000,
			expectedOTPM: 10000,
		},
		{
			name: "only ITPM set",
			config: KeyConfig{
				ITPMLimit: 30000,
			},
			expectedITPM: 30000,
			expectedOTPM: 0,
		},
		{
			name: "only OTPM set",
			config: KeyConfig{
				OTPMLimit: 10000,
			},
			expectedITPM: 0,
			expectedOTPM: 10000,
		},
		{
			name: "legacy TPMLimit",
			config: KeyConfig{
				TPMLimit: 40000,
			},
			expectedITPM: 20000,
			expectedOTPM: 20000,
		},
		{
			name: "ITPM/OTPM preferred over TPMLimit",
			config: KeyConfig{
				ITPMLimit: 30000,
				OTPMLimit: 10000,
				TPMLimit:  40000,
			},
			expectedITPM: 30000,
			expectedOTPM: 10000,
		},
		{
			name:         "no limits set",
			config:       KeyConfig{},
			expectedITPM: 0,
			expectedOTPM: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			itpm, otpm := tt.config.GetEffectiveTPM()
			if itpm != tt.expectedITPM {
				t.Errorf("GetEffectiveTPM() ITPM = %d, want %d", itpm, tt.expectedITPM)
			}
			if otpm != tt.expectedOTPM {
				t.Errorf("GetEffectiveTPM() OTPM = %d, want %d", otpm, tt.expectedOTPM)
			}
		})
	}
}

//nolint:gocognit // Test function complexity is acceptable for comprehensive coverage
func TestKeyConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		checkFunc func(t *testing.T, err error)
		name      string
		config    KeyConfig
		wantError bool
	}{
		{
			name: "valid key with all fields",
			config: KeyConfig{
				Key:       "sk-test123",
				RPMLimit:  50,
				ITPMLimit: 30000,
				OTPMLimit: 10000,
				Priority:  2,
				Weight:    5,
			},
			wantError: false,
		},
		{
			name: "valid key with defaults",
			config: KeyConfig{
				Key: "sk-test123",
			},
			wantError: false,
		},
		{
			name: "empty key",
			config: KeyConfig{
				Key: "",
			},
			wantError: true,
			checkFunc: func(t *testing.T, err error) {
				if !errors.Is(err, ErrKeyRequired) {
					t.Errorf("Expected ErrKeyRequired, got %v", err)
				}
			},
		},
		{
			name: "invalid priority too high",
			config: KeyConfig{
				Key:      "sk-test123",
				Priority: 3,
			},
			wantError: true,
			checkFunc: func(t *testing.T, err error) {
				var priorityErr InvalidPriorityError
				if !errors.As(err, &priorityErr) {
					t.Errorf("Expected InvalidPriorityError, got %T", err)
				}
			},
		},
		{
			name: "invalid priority negative",
			config: KeyConfig{
				Key:      "sk-test123",
				Priority: -1,
			},
			wantError: true,
			checkFunc: func(t *testing.T, err error) {
				var priorityErr InvalidPriorityError
				if !errors.As(err, &priorityErr) {
					t.Errorf("Expected InvalidPriorityError, got %T", err)
				}
			},
		},
		{
			name: "negative weight",
			config: KeyConfig{
				Key:    "sk-test123",
				Weight: -1,
			},
			wantError: true,
			checkFunc: func(t *testing.T, err error) {
				var weightErr InvalidWeightError
				if !errors.As(err, &weightErr) {
					t.Errorf("Expected InvalidWeightError, got %T", err)
				}
			},
		},
		{
			name: "valid priority 0 (low)",
			config: KeyConfig{
				Key:      "sk-test123",
				Priority: 0,
			},
			wantError: false,
		},
		{
			name: "valid priority 1 (normal)",
			config: KeyConfig{
				Key:      "sk-test123",
				Priority: 1,
			},
			wantError: false,
		},
		{
			name: "valid priority 2 (high)",
			config: KeyConfig{
				Key:      "sk-test123",
				Priority: 2,
			},
			wantError: false,
		},
		{
			name: "zero weight is valid",
			config: KeyConfig{
				Key:    "sk-test123",
				Weight: 0,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if tt.wantError {
				if err == nil {
					t.Error("Validate() expected error, got nil")
					return
				}
				if tt.checkFunc != nil {
					tt.checkFunc(t, err)
				}
			} else if err != nil {
				t.Errorf("Validate() expected no error, got %v", err)
			}
		})
	}
}

func TestProviderConfig_GetEffectiveStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		expected string
		config   ProviderConfig
	}{
		{
			name: "configured strategy least_loaded",
			config: ProviderConfig{
				Pooling: PoolingConfig{
					Strategy: "least_loaded",
				},
			},
			expected: "least_loaded",
		},
		{
			name: "configured strategy round_robin",
			config: ProviderConfig{
				Pooling: PoolingConfig{
					Strategy: "round_robin",
				},
			},
			expected: "round_robin",
		},
		{
			name: "configured strategy weighted",
			config: ProviderConfig{
				Pooling: PoolingConfig{
					Strategy: "weighted",
				},
			},
			expected: "weighted",
		},
		{
			name:     "no strategy configured - defaults to least_loaded",
			config:   ProviderConfig{},
			expected: "least_loaded",
		},
		{
			name: "empty strategy - defaults to least_loaded",
			config: ProviderConfig{
				Pooling: PoolingConfig{
					Strategy: "",
				},
			},
			expected: "least_loaded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.config.GetEffectiveStrategy()
			if got != tt.expected {
				t.Errorf("GetEffectiveStrategy() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestProviderConfig_IsPoolingEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   ProviderConfig
		expected bool
	}{
		{
			name: "explicitly enabled",
			config: ProviderConfig{
				Pooling: PoolingConfig{
					Enabled: true,
				},
				Keys: []KeyConfig{{Key: "key1"}},
			},
			expected: true,
		},
		{
			name: "explicitly enabled with multiple keys",
			config: ProviderConfig{
				Pooling: PoolingConfig{
					Enabled: true,
				},
				Keys: []KeyConfig{
					{Key: "key1"},
					{Key: "key2"},
				},
			},
			expected: true,
		},
		{
			name: "not explicitly enabled but multiple keys",
			config: ProviderConfig{
				Keys: []KeyConfig{
					{Key: "key1"},
					{Key: "key2"},
				},
			},
			expected: true,
		},
		{
			name: "not explicitly enabled with three keys",
			config: ProviderConfig{
				Keys: []KeyConfig{
					{Key: "key1"},
					{Key: "key2"},
					{Key: "key3"},
				},
			},
			expected: true,
		},
		{
			name: "not enabled with single key",
			config: ProviderConfig{
				Keys: []KeyConfig{{Key: "key1"}},
			},
			expected: false,
		},
		{
			name: "not enabled with no keys",
			config: ProviderConfig{
				Keys: []KeyConfig{},
			},
			expected: false,
		},
		{
			name: "explicitly enabled overrides single key",
			config: ProviderConfig{
				Pooling: PoolingConfig{
					Enabled: true,
				},
				Keys: []KeyConfig{{Key: "key1"}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.config.IsPoolingEnabled()
			if got != tt.expected {
				t.Errorf("IsPoolingEnabled() = %v, want %v", got, tt.expected)
			}
		})
	}
}
