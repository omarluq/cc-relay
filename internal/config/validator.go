// Package config provides configuration loading, parsing, and validation for cc-relay.
package config

import (
	"fmt"
	"net"
	"strings"
)

// Provider type constants.
const (
	ProviderBedrock = "bedrock"
	ProviderVertex  = "vertex"
	ProviderAzure   = "azure"
)

// Valid routing strategies.
var validRoutingStrategies = map[string]bool{
	"":                     true, // Empty defaults to failover
	"failover":             true,
	"round_robin":          true,
	"weighted_round_robin": true,
	"shuffle":              true,
	"model_based":          true,
	"least_loaded":         true,
	"weighted_failover":    true,
}

// Valid keypool strategies.
var validPoolingStrategies = map[string]bool{
	"":             true, // Empty defaults to least_loaded
	"least_loaded": true,
	"round_robin":  true,
	"random":       true,
	"weighted":     true,
}

// Valid provider types.
var validProviderTypes = map[string]bool{
	"anthropic":     true,
	"zai":           true,
	"minimax":       true,
	"ollama":        true,
	ProviderBedrock: true,
	ProviderVertex:  true,
	ProviderAzure:   true,
}

// Valid logging levels.
var validLogLevels = map[string]bool{
	"":      true, // Empty defaults to info
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Valid logging formats.
var validLogFormats = map[string]bool{
	"":        true, // Empty defaults to json
	"json":    true,
	"console": true,
	"text":    true, // Alias for console
	"pretty":  true,
}

// Validate checks the configuration for errors.
// It validates all required fields, valid values, and cross-field constraints.
// Returns a ValidationError containing all errors found, or nil if valid.
func (c *Config) Validate() error {
	errs := &ValidationError{Errors: nil}

	validateServer(c, errs)
	validateProviders(c, errs)
	validateRouting(c, errs)
	validateLogging(c, errs)

	return errs.ToError()
}

// validateServer validates the server configuration section.
func validateServer(cfg *Config, errs *ValidationError) {
	// Server.Listen is required
	if cfg.Server.Listen == "" {
		errs.Add("server.listen is required")
	} else {
		// Validate listen address format (host:port)
		validateListenAddress(cfg.Server.Listen, errs)
	}

	// Validate timeout if set
	if cfg.Server.TimeoutMS < 0 {
		errs.Add("server.timeout_ms must be >= 0")
	}

	// Validate max_concurrent if set
	if cfg.Server.MaxConcurrent < 0 {
		errs.Add("server.max_concurrent must be >= 0")
	}

	// Validate max_body_bytes if set
	if cfg.Server.MaxBodyBytes < 0 {
		errs.Add("server.max_body_bytes must be >= 0")
	}
}

// validateListenAddress validates a listen address in host:port format.
func validateListenAddress(addr string, errs *ValidationError) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		errs.Addf("server.listen must be in host:port format (got %q)", addr)
		return
	}

	// Host can be empty (listen on all interfaces) or a valid IP/hostname
	if host != "" {
		// Try to parse as IP
		if ip := net.ParseIP(host); ip == nil {
			// Not an IP, treat as hostname - basic validation
			if strings.ContainsAny(host, " \t\n") {
				errs.Add("server.listen host contains invalid characters")
			}
		}
	}

	// Port must be a number (SplitHostPort doesn't validate this)
	if port == "" {
		errs.Add("server.listen port is required")
	}
}

// validateProviders validates the providers configuration section.
func validateProviders(cfg *Config, errs *ValidationError) {
	if len(cfg.Providers) == 0 {
		// No providers is valid - might be used for testing or placeholder config
		return
	}

	seenNames := make(map[string]bool)

	for idx := range cfg.Providers {
		validateProvider(&cfg.Providers[idx], idx, seenNames, errs)
	}
}

// validateProvider validates a single provider configuration.
func validateProvider(provider *ProviderConfig, index int, seenNames map[string]bool, errs *ValidationError) {
	prefix := func(field string) string {
		if provider.Name != "" {
			return fmt.Sprintf("provider[%s].%s", provider.Name, field)
		}
		return fmt.Sprintf("providers[%d].%s", index, field)
	}

	// Name is required
	if provider.Name == "" {
		errs.Addf("providers[%d].name is required", index)
	} else {
		// Check for duplicate names
		if seenNames[provider.Name] {
			errs.Addf("duplicate provider name: %s", provider.Name)
		}
		seenNames[provider.Name] = true
	}

	// Type is required
	if provider.Type == "" {
		errs.Addf("%s is required", prefix("type"))
	} else if !validProviderTypes[provider.Type] {
		errs.Addf("%s is invalid (got %q, valid: anthropic, zai, ollama, bedrock, vertex, azure)",
			prefix("type"), provider.Type)
	}

	// Validate cloud provider fields
	validateCloudProviderConfig(provider, prefix, errs)

	// Validate keys
	for keyIdx, key := range provider.Keys {
		validateProviderKey(&key, provider.Name, keyIdx, errs)
	}

	// Validate pooling strategy if set
	if provider.Pooling.Strategy != "" && !validPoolingStrategies[provider.Pooling.Strategy] {
		errs.Addf("%s is invalid (got %q)", prefix("pooling.strategy"), provider.Pooling.Strategy)
	}
}

// validateCloudProviderConfig validates cloud provider-specific fields.
func validateCloudProviderConfig(provider *ProviderConfig, prefix func(string) string, errs *ValidationError) {
	switch provider.Type {
	case ProviderBedrock:
		if provider.AWSRegion == "" {
			errs.Addf("%s is required for bedrock provider", prefix("aws_region"))
		}
	case ProviderVertex:
		if provider.GCPProjectID == "" {
			errs.Addf("%s is required for vertex provider", prefix("gcp_project_id"))
		}
		if provider.GCPRegion == "" {
			errs.Addf("%s is required for vertex provider", prefix("gcp_region"))
		}
	case ProviderAzure:
		if provider.AzureResourceName == "" {
			errs.Addf("%s is required for azure provider", prefix("azure_resource_name"))
		}
	}
}

// validateProviderKey validates a single API key configuration.
func validateProviderKey(keyCfg *KeyConfig, providerName string, index int, errs *ValidationError) {
	prefix := func(field string) string {
		if providerName != "" {
			return fmt.Sprintf("provider[%s].keys[%d].%s", providerName, index, field)
		}
		return fmt.Sprintf("keys[%d].%s", index, field)
	}

	// Key is required (will be expanded from env var later)
	if keyCfg.Key == "" {
		errs.Addf("%s is required", prefix("key"))
	}

	// Priority must be 0-2
	if keyCfg.Priority < 0 || keyCfg.Priority > 2 {
		errs.Addf("%s must be 0-2 (got %d)", prefix("priority"), keyCfg.Priority)
	}

	// Weight must be non-negative
	if keyCfg.Weight < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("weight"), keyCfg.Weight)
	}

	// Rate limits must be non-negative
	if keyCfg.RPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("rpm_limit"), keyCfg.RPMLimit)
	}
	if keyCfg.TPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("tpm_limit"), keyCfg.TPMLimit)
	}
	if keyCfg.ITPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("itpm_limit"), keyCfg.ITPMLimit)
	}
	if keyCfg.OTPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("otpm_limit"), keyCfg.OTPMLimit)
	}
}

// validateRouting validates the routing configuration section.
func validateRouting(cfg *Config, errs *ValidationError) {
	// Strategy must be valid if set
	if cfg.Routing.Strategy != "" && !validRoutingStrategies[cfg.Routing.Strategy] {
		errs.Addf("routing.strategy is invalid (got %q, valid: failover, round_robin, "+
			"weighted_round_robin, shuffle, model_based, least_loaded, weighted_failover)",
			cfg.Routing.Strategy)
	}

	// FailoverTimeout must be non-negative
	if cfg.Routing.FailoverTimeout < 0 {
		errs.Add("routing.failover_timeout must be >= 0")
	}

	// Model-based routing requires model_mapping
	if cfg.Routing.Strategy == "model_based" && len(cfg.Routing.ModelMapping) == 0 {
		errs.Add("routing.model_mapping is required when strategy is model_based")
	}
}

// validateLogging validates the logging configuration section.
func validateLogging(cfg *Config, errs *ValidationError) {
	// Level must be valid if set
	if !validLogLevels[cfg.Logging.Level] {
		errs.Addf("logging.level is invalid (got %q, valid: debug, info, warn, error)",
			cfg.Logging.Level)
	}

	// Format must be valid if set
	if !validLogFormats[cfg.Logging.Format] {
		errs.Addf("logging.format is invalid (got %q, valid: json, console, text, pretty)",
			cfg.Logging.Format)
	}

	// MaxBodyLogSize must be non-negative
	if cfg.Logging.DebugOptions.MaxBodyLogSize < 0 {
		errs.Add("logging.debug_options.max_body_log_size must be >= 0")
	}
}
