package config

import (
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

	tests := []struct {
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
