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
	"":                    true, // Empty defaults to failover
	"failover":            true,
	"round_robin":         true,
	"weighted_round_robin": true,
	"shuffle":             true,
	"model_based":         true,
	"least_loaded":        true,
	"weighted_failover":   true,
}

// Valid keypool strategies.
var validPoolingStrategies = map[string]bool{
	"":            true, // Empty defaults to least_loaded
	"least_loaded": true,
	"round_robin":  true,
	"random":       true,
	"weighted":     true,
}

// Valid provider types.
var validProviderTypes = map[string]bool{
	"anthropic":     true,
	"zai":           true,
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
	errs := &ValidationError{}

	validateServer(c, errs)
	validateProviders(c, errs)
	validateRouting(c, errs)
	validateLogging(c, errs)

	return errs.ToError()
}

// validateServer validates the server configuration section.
func validateServer(c *Config, errs *ValidationError) {
	// Server.Listen is required
	if c.Server.Listen == "" {
		errs.Add("server.listen is required")
	} else {
		// Validate listen address format (host:port)
		validateListenAddress(c.Server.Listen, errs)
	}

	// Validate timeout if set
	if c.Server.TimeoutMS < 0 {
		errs.Add("server.timeout_ms must be >= 0")
	}

	// Validate max_concurrent if set
	if c.Server.MaxConcurrent < 0 {
		errs.Add("server.max_concurrent must be >= 0")
	}

	// Validate max_body_bytes if set
	if c.Server.MaxBodyBytes < 0 {
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
func validateProviders(c *Config, errs *ValidationError) {
	if len(c.Providers) == 0 {
		// No providers is valid - might be used for testing or placeholder config
		return
	}

	seenNames := make(map[string]bool)

	for i := range c.Providers {
		validateProvider(&c.Providers[i], i, seenNames, errs)
	}
}

// validateProvider validates a single provider configuration.
func validateProvider(p *ProviderConfig, index int, seenNames map[string]bool, errs *ValidationError) {
	prefix := func(field string) string {
		if p.Name != "" {
			return fmt.Sprintf("provider[%s].%s", p.Name, field)
		}
		return fmt.Sprintf("providers[%d].%s", index, field)
	}

	// Name is required
	if p.Name == "" {
		errs.Addf("providers[%d].name is required", index)
	} else {
		// Check for duplicate names
		if seenNames[p.Name] {
			errs.Addf("duplicate provider name: %s", p.Name)
		}
		seenNames[p.Name] = true
	}

	// Type is required
	if p.Type == "" {
		errs.Addf("%s is required", prefix("type"))
	} else if !validProviderTypes[p.Type] {
		errs.Addf("%s is invalid (got %q, valid: anthropic, zai, ollama, bedrock, vertex, azure)",
			prefix("type"), p.Type)
	}

	// Validate cloud provider fields
	validateCloudProviderConfig(p, prefix, errs)

	// Validate keys
	for j, key := range p.Keys {
		validateProviderKey(&key, p.Name, j, errs)
	}

	// Validate pooling strategy if set
	if p.Pooling.Strategy != "" && !validPoolingStrategies[p.Pooling.Strategy] {
		errs.Addf("%s is invalid (got %q)", prefix("pooling.strategy"), p.Pooling.Strategy)
	}
}

// validateCloudProviderConfig validates cloud provider-specific fields.
func validateCloudProviderConfig(p *ProviderConfig, prefix func(string) string, errs *ValidationError) {
	switch p.Type {
	case ProviderBedrock:
		if p.AWSRegion == "" {
			errs.Addf("%s is required for bedrock provider", prefix("aws_region"))
		}
	case ProviderVertex:
		if p.GCPProjectID == "" {
			errs.Addf("%s is required for vertex provider", prefix("gcp_project_id"))
		}
		if p.GCPRegion == "" {
			errs.Addf("%s is required for vertex provider", prefix("gcp_region"))
		}
	case ProviderAzure:
		if p.AzureResourceName == "" {
			errs.Addf("%s is required for azure provider", prefix("azure_resource_name"))
		}
	}
}

// validateProviderKey validates a single API key configuration.
func validateProviderKey(k *KeyConfig, providerName string, index int, errs *ValidationError) {
	prefix := func(field string) string {
		if providerName != "" {
			return fmt.Sprintf("provider[%s].keys[%d].%s", providerName, index, field)
		}
		return fmt.Sprintf("keys[%d].%s", index, field)
	}

	// Key is required (will be expanded from env var later)
	if k.Key == "" {
		errs.Addf("%s is required", prefix("key"))
	}

	// Priority must be 0-2
	if k.Priority < 0 || k.Priority > 2 {
		errs.Addf("%s must be 0-2 (got %d)", prefix("priority"), k.Priority)
	}

	// Weight must be non-negative
	if k.Weight < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("weight"), k.Weight)
	}

	// Rate limits must be non-negative
	if k.RPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("rpm_limit"), k.RPMLimit)
	}
	if k.TPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("tpm_limit"), k.TPMLimit)
	}
	if k.ITPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("itpm_limit"), k.ITPMLimit)
	}
	if k.OTPMLimit < 0 {
		errs.Addf("%s must be >= 0 (got %d)", prefix("otpm_limit"), k.OTPMLimit)
	}
}

// validateRouting validates the routing configuration section.
func validateRouting(c *Config, errs *ValidationError) {
	// Strategy must be valid if set
	if c.Routing.Strategy != "" && !validRoutingStrategies[c.Routing.Strategy] {
		errs.Addf("routing.strategy is invalid (got %q, valid: failover, round_robin, "+
			"weighted_round_robin, shuffle, model_based, least_loaded, weighted_failover)",
			c.Routing.Strategy)
	}

	// FailoverTimeout must be non-negative
	if c.Routing.FailoverTimeout < 0 {
		errs.Add("routing.failover_timeout must be >= 0")
	}

	// Model-based routing requires model_mapping
	if c.Routing.Strategy == "model_based" && len(c.Routing.ModelMapping) == 0 {
		errs.Add("routing.model_mapping is required when strategy is model_based")
	}
}

// validateLogging validates the logging configuration section.
func validateLogging(c *Config, errs *ValidationError) {
	// Level must be valid if set
	if !validLogLevels[c.Logging.Level] {
		errs.Addf("logging.level is invalid (got %q, valid: debug, info, warn, error)",
			c.Logging.Level)
	}

	// Format must be valid if set
	if !validLogFormats[c.Logging.Format] {
		errs.Addf("logging.format is invalid (got %q, valid: json, console, text, pretty)",
			c.Logging.Format)
	}

	// MaxBodyLogSize must be non-negative
	if c.Logging.DebugOptions.MaxBodyLogSize < 0 {
		errs.Add("logging.debug_options.max_body_log_size must be >= 0")
	}
}
