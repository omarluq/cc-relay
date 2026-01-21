// Package config provides configuration loading and parsing for cc-relay.
package config

import (
	"strings"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/rs/zerolog"
)

// Log level constants.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Config represents the complete cc-relay configuration.
type Config struct {
	Providers []ProviderConfig `yaml:"providers"`
	Logging   LoggingConfig    `yaml:"logging"`
	Server    ServerConfig     `yaml:"server"`
	Cache     cache.Config     `yaml:"cache"`
}

// ServerConfig defines server-level settings.
type ServerConfig struct {
	Listen        string     `yaml:"listen"`
	APIKey        string     `yaml:"api_key"` // Legacy: use Auth.APIKey instead
	Auth          AuthConfig `yaml:"auth"`
	TimeoutMS     int        `yaml:"timeout_ms"`
	MaxConcurrent int        `yaml:"max_concurrent"`
	EnableHTTP2   bool       `yaml:"enable_http2"` // Enable HTTP/2 cleartext (h2c) support
}

// AuthConfig defines authentication settings for the proxy.
type AuthConfig struct {
	// APIKey is the expected value for x-api-key header authentication.
	// If empty, API key authentication is disabled.
	APIKey string `yaml:"api_key"`

	// BearerSecret is the expected Bearer token value.
	// If empty but AllowBearer is true, any bearer token is accepted.
	BearerSecret string `yaml:"bearer_secret"`

	// AllowBearer enables Authorization: Bearer token authentication.
	// Used by Claude Code subscription users.
	AllowBearer bool `yaml:"allow_bearer"`

	// AllowSubscription is an alias for AllowBearer, provided for user-friendly config.
	// Claude Code subscription users authenticate with Bearer tokens, so this enables
	// the same passthrough Bearer authentication.
	AllowSubscription bool `yaml:"allow_subscription"`
}

// IsEnabled returns true if any authentication method is configured.
func (a *AuthConfig) IsEnabled() bool {
	return a.APIKey != "" || a.AllowBearer || a.AllowSubscription
}

// IsBearerEnabled returns true if Bearer token authentication is enabled.
// This checks both AllowBearer and AllowSubscription (which is an alias).
func (a *AuthConfig) IsBearerEnabled() bool {
	return a.AllowBearer || a.AllowSubscription
}

// GetEffectiveAPIKey returns the API key from Auth config or falls back to legacy ServerConfig.APIKey.
func (s *ServerConfig) GetEffectiveAPIKey() string {
	if s.Auth.APIKey != "" {
		return s.Auth.APIKey
	}
	return s.APIKey
}

// ProviderConfig defines configuration for a backend LLM provider.
type ProviderConfig struct {
	ModelMapping map[string]string `yaml:"model_mapping"`
	Name         string            `yaml:"name"`
	Type         string            `yaml:"type"`
	BaseURL      string            `yaml:"base_url"`
	Keys         []KeyConfig       `yaml:"keys"`
	Models       []string          `yaml:"models"`
	Enabled      bool              `yaml:"enabled"`
}

// KeyConfig defines an API key with rate limits.
type KeyConfig struct {
	Key      string `yaml:"key"`
	RPMLimit int    `yaml:"rpm_limit"`
	TPMLimit int    `yaml:"tpm_limit"`
}

// LoggingConfig defines logging behavior.
type LoggingConfig struct {
	Level        string       `yaml:"level"`         // debug, info, warn, error
	Format       string       `yaml:"format"`        // json, console
	Output       string       `yaml:"output"`        // stdout, stderr, or file path
	Pretty       bool         `yaml:"pretty"`        // enable colored console output
	DebugOptions DebugOptions `yaml:"debug_options"` // granular debug logging controls
}

// ParseLevel converts a string log level to zerolog.Level.
// Returns zerolog.InfoLevel if the level string is invalid.
func (l *LoggingConfig) ParseLevel() zerolog.Level {
	switch strings.ToLower(l.Level) {
	case LevelDebug:
		return zerolog.DebugLevel
	case LevelInfo:
		return zerolog.InfoLevel
	case LevelWarn:
		return zerolog.WarnLevel
	case LevelError:
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// EnableAllDebugOptions turns on all debug logging features.
// Used by --debug CLI flag shortcut.
func (l *LoggingConfig) EnableAllDebugOptions() {
	l.Level = LevelDebug
	l.DebugOptions = DebugOptions{
		LogRequestBody:     true,
		LogResponseHeaders: true,
		LogTLSMetrics:      true,
		MaxBodyLogSize:     1000,
	}
}

// DebugOptions defines granular debug logging controls.
type DebugOptions struct {
	// LogRequestBody enables logging of request body in debug mode.
	// Body is truncated to MaxBodyLogSize to prevent massive logs.
	LogRequestBody bool `yaml:"log_request_body"`

	// LogResponseHeaders enables logging of response headers in debug mode.
	LogResponseHeaders bool `yaml:"log_response_headers"`

	// LogTLSMetrics enables logging of TLS connection metrics (version, handshake time, reuse).
	LogTLSMetrics bool `yaml:"log_tls_metrics"`

	// MaxBodyLogSize is the maximum number of bytes to log from request/response bodies.
	// Default: 1000 bytes. Set to 0 for unlimited (not recommended).
	MaxBodyLogSize int `yaml:"max_body_log_size"`
}

// GetMaxBodyLogSize returns the effective max body log size with default fallback.
func (d *DebugOptions) GetMaxBodyLogSize() int {
	if d.MaxBodyLogSize <= 0 {
		return 1000 // Default: 1KB
	}
	return d.MaxBodyLogSize
}

// IsEnabled returns true if any debug option is enabled.
func (d *DebugOptions) IsEnabled() bool {
	return d.LogRequestBody || d.LogResponseHeaders || d.LogTLSMetrics
}
