// Package config provides configuration loading and parsing for cc-relay.
package config

import (
	"strings"

	"github.com/rs/zerolog"
)

// Config represents the complete cc-relay configuration.
type Config struct {
	Logging   LoggingConfig    `yaml:"logging"`
	Providers []ProviderConfig `yaml:"providers"`
	Server    ServerConfig     `yaml:"server"`
}

// ServerConfig defines server-level settings.
type ServerConfig struct {
	Listen        string `yaml:"listen"`
	APIKey        string `yaml:"api_key"`
	TimeoutMS     int    `yaml:"timeout_ms"`
	MaxConcurrent int    `yaml:"max_concurrent"`
}

// ProviderConfig defines configuration for a backend LLM provider.
type ProviderConfig struct {
	ModelMapping map[string]string `yaml:"model_mapping"`
	Name         string            `yaml:"name"`
	Type         string            `yaml:"type"`
	BaseURL      string            `yaml:"base_url"`
	Keys         []KeyConfig       `yaml:"keys"`
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
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, console
	Output string `yaml:"output"` // stdout, stderr, or file path
	Pretty bool   `yaml:"pretty"` // enable colored console output
}

// ParseLevel converts a string log level to zerolog.Level.
// Returns zerolog.InfoLevel if the level string is invalid.
func (l *LoggingConfig) ParseLevel() zerolog.Level {
	switch strings.ToLower(l.Level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
