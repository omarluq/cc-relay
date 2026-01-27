// Package config provides configuration loading and parsing for cc-relay.
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
	"github.com/omarluq/cc-relay/internal/health"
	"github.com/rs/zerolog"
	"github.com/samber/mo"
)

// Configuration errors.
var (
	ErrKeyRequired = errors.New("config: key is required")
)

// RuntimeConfig defines the interface for accessing runtime configuration that supports hot-reload.
// Components that need to observe config changes should use this interface instead of
// holding a direct *Config pointer, which would become stale after hot-reload.
//
// Usage pattern:
//
//	func (r *Router) Select(ctx context.Context, providers []ProviderInfo) (ProviderInfo, error) {
//		cfg := r.runtime.Get()
//		strategy := cfg.Routing.GetEffectiveStrategy()
//		// Use strategy for this request...
//	}
type RuntimeConfig interface {
	Get() *Config
}

// InvalidPriorityError is returned when priority is outside valid range.
type InvalidPriorityError struct {
	Priority int
}

func (e InvalidPriorityError) Error() string {
	return fmt.Sprintf("config: priority must be 0-2, got %d", e.Priority)
}

// InvalidWeightError is returned when weight is negative.
type InvalidWeightError struct {
	Weight int
}

func (e InvalidWeightError) Error() string {
	return fmt.Sprintf("config: weight must be >= 0, got %d", e.Weight)
}

// Log level constants.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// Config represents the complete cc-relay configuration.
type Config struct {
	Providers []ProviderConfig `yaml:"providers" toml:"providers"`
	Routing   RoutingConfig    `yaml:"routing" toml:"routing"`
	Logging   LoggingConfig    `yaml:"logging" toml:"logging"`
	Health    health.Config    `yaml:"health" toml:"health"`
	Server    ServerConfig     `yaml:"server" toml:"server"`
	Cache     cache.Config     `yaml:"cache" toml:"cache"`
}

// RoutingConfig defines provider-level routing strategy behavior.
// This controls how requests are distributed across multiple providers.
type RoutingConfig struct {
	// ModelMapping maps model name prefixes to provider names for model-based routing.
	// Example: {"claude-opus": "anthropic", "glm-4": "zai", "qwen": "ollama"}
	// Uses longest prefix match for specificity.
	// Only used when Strategy is "model_based".
	ModelMapping map[string]string `yaml:"model_mapping" toml:"model_mapping"`

	// Strategy defines the provider selection algorithm.
	// Options: round_robin, weighted_round_robin, shuffle, failover (default), model_based
	Strategy string `yaml:"strategy" toml:"strategy"`

	// DefaultProvider is the fallback provider when no model mapping matches.
	// Only used when Strategy is "model_based".
	DefaultProvider string `yaml:"default_provider" toml:"default_provider"`

	// FailoverTimeout is the timeout in milliseconds for failover attempts.
	// When a provider fails, the router will try the next provider within this timeout.
	// Default: 5000ms (5 seconds)
	FailoverTimeout int `yaml:"failover_timeout" toml:"failover_timeout"`

	// Debug enables routing debug headers (X-CC-Relay-Provider, X-CC-Relay-Strategy).
	// Useful for debugging routing decisions but may leak internal info.
	Debug bool `yaml:"debug" toml:"debug"`
}

// GetEffectiveStrategy returns the routing strategy with default fallback.
// Returns "failover" if Strategy is empty string.
func (r *RoutingConfig) GetEffectiveStrategy() string {
	if r.Strategy == "" {
		return "failover"
	}
	return r.Strategy
}

// GetFailoverTimeoutOption returns the failover timeout as a duration Option.
// Returns None if FailoverTimeout is zero or negative.
func (r *RoutingConfig) GetFailoverTimeoutOption() mo.Option[time.Duration] {
	if r.FailoverTimeout <= 0 {
		return mo.None[time.Duration]()
	}
	return mo.Some(time.Duration(r.FailoverTimeout) * time.Millisecond)
}

// IsDebugEnabled returns true if routing debug headers are enabled.
func (r *RoutingConfig) IsDebugEnabled() bool {
	return r.Debug
}

// ServerConfig defines server-level settings.
type ServerConfig struct {
	Listen        string     `yaml:"listen" toml:"listen"`
	APIKey        string     `yaml:"api_key" toml:"api_key"` // Legacy: use Auth.APIKey instead
	Auth          AuthConfig `yaml:"auth" toml:"auth"`
	TimeoutMS     int        `yaml:"timeout_ms" toml:"timeout_ms"`
	MaxConcurrent int        `yaml:"max_concurrent" toml:"max_concurrent"`
	EnableHTTP2   bool       `yaml:"enable_http2" toml:"enable_http2"` // Enable HTTP/2 cleartext (h2c) support
}

// AuthConfig defines authentication settings for the proxy.
type AuthConfig struct {
	// APIKey is the expected value for x-api-key header authentication.
	// If empty, API key authentication is disabled.
	APIKey string `yaml:"api_key" toml:"api_key"`

	// BearerSecret is the expected Bearer token value.
	// If empty but AllowBearer is true, any bearer token is accepted.
	BearerSecret string `yaml:"bearer_secret" toml:"bearer_secret"`

	// AllowBearer enables Authorization: Bearer token authentication.
	// Used by Claude Code subscription users.
	AllowBearer bool `yaml:"allow_bearer" toml:"allow_bearer"`

	// AllowSubscription is an alias for AllowBearer, provided for user-friendly config.
	// Claude Code subscription users authenticate with Bearer tokens, so this enables
	// the same passthrough Bearer authentication.
	AllowSubscription bool `yaml:"allow_subscription" toml:"allow_subscription"`
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
//
//nolint:govet // Field order optimized for readability, not memory alignment
type ProviderConfig struct {
	ModelMapping map[string]string `yaml:"model_mapping" toml:"model_mapping"`
	Name         string            `yaml:"name" toml:"name"`
	Type         string            `yaml:"type" toml:"type"`
	BaseURL      string            `yaml:"base_url" toml:"base_url"`
	Keys         []KeyConfig       `yaml:"keys" toml:"keys"`
	Models       []string          `yaml:"models" toml:"models"`
	Pooling      PoolingConfig     `yaml:"pooling" toml:"pooling"`
	Enabled      bool              `yaml:"enabled" toml:"enabled"`

	// Cloud provider fields (used when Type is bedrock, vertex, or azure)

	// AWSRegion is the AWS region for Bedrock (e.g., "us-east-1", "us-west-2").
	// Required when Type is "bedrock".
	AWSRegion string `yaml:"aws_region" toml:"aws_region"`

	// AWSAccessKeyID and AWSSecretAccessKey for explicit credentials.
	// If empty, uses AWS SDK default credential chain (env vars, IAM role, etc.).
	AWSAccessKeyID     string `yaml:"aws_access_key_id" toml:"aws_access_key_id"`
	AWSSecretAccessKey string `yaml:"aws_secret_access_key" toml:"aws_secret_access_key"`

	// GCPProjectID is the Google Cloud project ID for Vertex AI.
	// Required when Type is "vertex".
	GCPProjectID string `yaml:"gcp_project_id" toml:"gcp_project_id"`

	// GCPRegion is the Google Cloud region for Vertex AI (e.g., "us-central1").
	// Required when Type is "vertex".
	GCPRegion string `yaml:"gcp_region" toml:"gcp_region"`

	// AzureResourceName is the Azure resource name for Foundry.
	// Required when Type is "azure".
	AzureResourceName string `yaml:"azure_resource_name" toml:"azure_resource_name"`

	// AzureDeploymentID is the deployment ID (model) for Azure Foundry.
	// Optional - can be derived from model mapping.
	AzureDeploymentID string `yaml:"azure_deployment_id" toml:"azure_deployment_id"`

	// AzureAPIVersion is the Azure API version (e.g., "2024-06-01").
	// Defaults to "2024-06-01" if not specified.
	AzureAPIVersion string `yaml:"azure_api_version" toml:"azure_api_version"`
}

// PoolingConfig defines key pool behavior for a provider.
type PoolingConfig struct {
	Strategy string `yaml:"strategy" toml:"strategy"` // least_loaded (default), round_robin, random, weighted
	Enabled  bool   `yaml:"enabled" toml:"enabled"`   // Enable pooling (default: true if multiple keys)
}

// GetEffectiveStrategy returns the selection strategy with default fallback.
func (p *ProviderConfig) GetEffectiveStrategy() string {
	if p.Pooling.Strategy != "" {
		return p.Pooling.Strategy
	}
	return "least_loaded" // Default strategy
}

// IsPoolingEnabled returns true if key pooling should be used.
func (p *ProviderConfig) IsPoolingEnabled() bool {
	// Explicit setting takes precedence
	if p.Pooling.Enabled {
		return true
	}
	// Default: enable if multiple keys
	return len(p.Keys) > 1
}

// GetAzureAPIVersion returns the Azure API version with default fallback.
func (p *ProviderConfig) GetAzureAPIVersion() string {
	if p.AzureAPIVersion == "" {
		return "2024-06-01"
	}
	return p.AzureAPIVersion
}

// ValidateCloudConfig validates cloud provider-specific configuration.
func (p *ProviderConfig) ValidateCloudConfig() error {
	switch p.Type {
	case ProviderBedrock:
		if p.AWSRegion == "" {
			return errors.New("config: aws_region required for bedrock provider")
		}
	case ProviderVertex:
		if p.GCPProjectID == "" {
			return errors.New("config: gcp_project_id required for vertex provider")
		}
		if p.GCPRegion == "" {
			return errors.New("config: gcp_region required for vertex provider")
		}
	case ProviderAzure:
		if p.AzureResourceName == "" {
			return errors.New("config: azure_resource_name required for azure provider")
		}
	}
	return nil
}

// KeyConfig defines an API key with rate limits and selection metadata.
type KeyConfig struct {
	Key       string `yaml:"key" toml:"key"`               // API key value (supports ${ENV_VAR})
	RPMLimit  int    `yaml:"rpm_limit" toml:"rpm_limit"`   // Requests per minute (0 = unlimited/learn)
	ITPMLimit int    `yaml:"itpm_limit" toml:"itpm_limit"` // Input tokens per minute (0 = unlimited/learn)
	OTPMLimit int    `yaml:"otpm_limit" toml:"otpm_limit"` // Output tokens per minute (0 = unlimited/learn)
	Priority  int    `yaml:"priority" toml:"priority"`     // Selection priority: 0=low, 1=normal (default), 2=high
	Weight    int    `yaml:"weight" toml:"weight"`         // For weighted selection strategy (default: 1)

	// Deprecated: Use ITPMLimit + OTPMLimit instead
	TPMLimit int `yaml:"tpm_limit" toml:"tpm_limit"`
}

// GetEffectiveTPM returns the combined TPM limit for backwards compatibility.
// Prefers ITPMLimit + OTPMLimit if set, falls back to TPMLimit.
func (k *KeyConfig) GetEffectiveTPM() (itpm, otpm int) {
	if k.ITPMLimit > 0 || k.OTPMLimit > 0 {
		return k.ITPMLimit, k.OTPMLimit
	}
	// Legacy: split TPMLimit equally between input/output
	if k.TPMLimit > 0 {
		return k.TPMLimit / 2, k.TPMLimit / 2
	}
	return 0, 0
}

// Validate checks KeyConfig for errors.
func (k *KeyConfig) Validate() error {
	if k.Key == "" {
		return ErrKeyRequired
	}
	if k.Priority < 0 || k.Priority > 2 {
		return InvalidPriorityError{Priority: k.Priority}
	}
	if k.Weight < 0 {
		return InvalidWeightError{Weight: k.Weight}
	}
	return nil
}

// LoggingConfig defines logging behavior.
type LoggingConfig struct {
	Level        string       `yaml:"level" toml:"level"`                 // debug, info, warn, error
	Format       string       `yaml:"format" toml:"format"`               // json, console
	Output       string       `yaml:"output" toml:"output"`               // stdout, stderr, or file path
	Pretty       bool         `yaml:"pretty" toml:"pretty"`               // enable colored console output
	DebugOptions DebugOptions `yaml:"debug_options" toml:"debug_options"` // granular debug logging controls
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
	LogRequestBody bool `yaml:"log_request_body" toml:"log_request_body"`

	// LogResponseHeaders enables logging of response headers in debug mode.
	LogResponseHeaders bool `yaml:"log_response_headers" toml:"log_response_headers"`

	// LogTLSMetrics enables logging of TLS connection metrics (version, handshake time, reuse).
	LogTLSMetrics bool `yaml:"log_tls_metrics" toml:"log_tls_metrics"`

	// MaxBodyLogSize is the maximum number of bytes to log from request/response bodies.
	// Default: 1000 bytes. Set to 0 for unlimited (not recommended).
	MaxBodyLogSize int `yaml:"max_body_log_size" toml:"max_body_log_size"`
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

// GetMaxBodyLogSizeOption returns the max body log size as an Option.
// Returns None if the value is not explicitly set (zero or negative).
func (d *DebugOptions) GetMaxBodyLogSizeOption() mo.Option[int] {
	if d.MaxBodyLogSize <= 0 {
		return mo.None[int]()
	}
	return mo.Some(d.MaxBodyLogSize)
}

// ServerConfig Option helpers for type-safe access to optional configuration values.
// These methods expose configuration fields as mo.Option[T] for composable handling.

// GetTimeoutOption returns the timeout as an Option.
// Returns None if TimeoutMS is zero (use default).
func (s *ServerConfig) GetTimeoutOption() mo.Option[time.Duration] {
	if s.TimeoutMS <= 0 {
		return mo.None[time.Duration]()
	}
	return mo.Some(time.Duration(s.TimeoutMS) * time.Millisecond)
}

// GetMaxConcurrentOption returns the max concurrent setting as an Option.
// Returns None if MaxConcurrent is zero (unlimited).
func (s *ServerConfig) GetMaxConcurrentOption() mo.Option[int] {
	if s.MaxConcurrent <= 0 {
		return mo.None[int]()
	}
	return mo.Some(s.MaxConcurrent)
}

// KeyConfig Option helpers for type-safe access to optional rate limit values.

// GetRPMLimitOption returns the RPM limit as an Option.
// Returns None if RPMLimit is zero (unlimited/learn from headers).
func (k *KeyConfig) GetRPMLimitOption() mo.Option[int] {
	if k.RPMLimit <= 0 {
		return mo.None[int]()
	}
	return mo.Some(k.RPMLimit)
}

// GetITPMLimitOption returns the ITPM limit as an Option.
// Returns None if ITPMLimit is zero (unlimited/learn from headers).
func (k *KeyConfig) GetITPMLimitOption() mo.Option[int] {
	if k.ITPMLimit <= 0 {
		return mo.None[int]()
	}
	return mo.Some(k.ITPMLimit)
}

// GetOTPMLimitOption returns the OTPM limit as an Option.
// Returns None if OTPMLimit is zero (unlimited/learn from headers).
func (k *KeyConfig) GetOTPMLimitOption() mo.Option[int] {
	if k.OTPMLimit <= 0 {
		return mo.None[int]()
	}
	return mo.Some(k.OTPMLimit)
}
