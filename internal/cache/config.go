package cache

import (
	"errors"
	"fmt"
	"time"
)

// Mode represents the cache operating mode.
type Mode string

const (
	// ModeSingle uses local Ristretto cache (default).
	// Best for single-instance deployments with high performance requirements.
	ModeSingle Mode = "single"

	// ModeHA uses distributed Olric cache for high availability.
	// Best for multi-instance deployments requiring shared cache state.
	ModeHA Mode = "ha"

	// ModeDisabled uses noop cache (caching disabled).
	// All operations return immediately without storing data.
	ModeDisabled Mode = "disabled"
)

// Environment constants for Olric memberlist presets.
const (
	// EnvLocal is for local development (fast failure detection).
	EnvLocal = "local"
	// EnvLAN is for LAN environments (default memberlist settings).
	EnvLAN = "lan"
	// EnvWAN is for WAN environments (longer timeouts for higher latency).
	EnvWAN = "wan"
)

// Config defines cache configuration.
// Use Validate() to check for configuration errors before creating a cache.
type Config struct {
	Mode      Mode            `yaml:"mode" toml:"mode"`
	Olric     OlricConfig     `yaml:"olric" toml:"olric"`
	Ristretto RistrettoConfig `yaml:"ristretto" toml:"ristretto"`
}

// RistrettoConfig configures the Ristretto local cache.
// Ristretto is a high-performance, concurrent cache based on research from
// the Caffeine library.
type RistrettoConfig struct {
	// NumCounters is the number of 4-bit access counters.
	// Recommended: 10x expected max items for optimal admission policy.
	// Example: For 100,000 items, use 1,000,000 counters.
	NumCounters int64 `yaml:"num_counters" toml:"num_counters"`

	// MaxCost is the maximum cost (memory) the cache can hold.
	// Cost is measured in bytes of cached values.
	// Example: 100 << 20 for 100 MB.
	MaxCost int64 `yaml:"max_cost" toml:"max_cost"`

	// BufferItems is the number of keys per Get buffer.
	// This controls the size of the admission buffer.
	// Recommended: 64 (default).
	BufferItems int64 `yaml:"buffer_items" toml:"buffer_items"`
}

// OlricConfig configures the Olric distributed cache.
// Olric provides a distributed in-memory key/value store with clustering support.
type OlricConfig struct {
	DMapName          string        `yaml:"dmap_name" toml:"dmap_name"`
	BindAddr          string        `yaml:"bind_addr" toml:"bind_addr"`
	Environment       string        `yaml:"environment" toml:"environment"`
	Addresses         []string      `yaml:"addresses" toml:"addresses"`
	Peers             []string      `yaml:"peers" toml:"peers"`
	ReplicaCount      int           `yaml:"replica_count" toml:"replica_count"`
	ReadQuorum        int           `yaml:"read_quorum" toml:"read_quorum"`
	WriteQuorum       int           `yaml:"write_quorum" toml:"write_quorum"`
	LeaveTimeout      time.Duration `yaml:"leave_timeout" toml:"leave_timeout"`
	MemberCountQuorum int32         `yaml:"member_count_quorum" toml:"member_count_quorum"`
	Embedded          bool          `yaml:"embedded" toml:"embedded"`
}

// Validate checks OlricConfig for errors.
func (o *OlricConfig) Validate() error {
	// Validate connection mode
	if !o.Embedded && len(o.Addresses) == 0 {
		return errors.New("cache: olric.addresses required when not embedded")
	}
	if o.Embedded && o.BindAddr == "" {
		return errors.New("cache: olric.bind_addr required when embedded")
	}
	// Validate Environment
	if err := o.validateEnvironment(); err != nil {
		return err
	}
	// Validate quorum relationships
	if err := o.validateQuorum(); err != nil {
		return err
	}
	// Validate timeouts
	if o.LeaveTimeout < 0 {
		return errors.New("cache: olric.leave_timeout cannot be negative")
	}
	return nil
}

// validateEnvironment checks Environment field.
func (o *OlricConfig) validateEnvironment() error {
	switch o.Environment {
	case "", EnvLocal, EnvLAN, EnvWAN:
		return nil
	default:
		return errors.New(`cache: olric.environment must be "local", "lan", or "wan"`)
	}
}

// validateQuorum checks quorum field relationships.
func (o *OlricConfig) validateQuorum() error {
	if o.WriteQuorum > 0 && o.ReplicaCount > 0 && o.WriteQuorum > o.ReplicaCount {
		return errors.New("cache: olric.write_quorum cannot exceed replica_count")
	}
	if o.ReadQuorum > 0 && o.ReplicaCount > 0 && o.ReadQuorum > o.ReplicaCount {
		return errors.New("cache: olric.read_quorum cannot exceed replica_count")
	}
	if o.MemberCountQuorum < 0 {
		return errors.New("cache: olric.member_count_quorum cannot be negative")
	}
	return nil
}

// Validate checks the configuration for errors.
// Returns nil if the configuration is valid.
func (c *Config) Validate() error {
	switch c.Mode {
	case ModeSingle:
		if c.Ristretto.MaxCost <= 0 {
			return errors.New("cache: ristretto.max_cost must be positive")
		}
		if c.Ristretto.NumCounters <= 0 {
			return errors.New("cache: ristretto.num_counters must be positive")
		}
	case ModeHA:
		if err := c.Olric.Validate(); err != nil {
			return err
		}
	case ModeDisabled:
		// No validation needed for disabled mode
	case "":
		return errors.New("cache: mode is required")
	default:
		return fmt.Errorf("cache: unknown mode %q", c.Mode)
	}
	return nil
}

// DefaultRistrettoConfig returns a RistrettoConfig with sensible defaults.
// NumCounters: 1,000,000 (for ~100K items).
// MaxCost: 100 MB.
// BufferItems: 64.
func DefaultRistrettoConfig() RistrettoConfig {
	return RistrettoConfig{
		NumCounters: 1_000_000,
		MaxCost:     100 << 20, // 100 MB.
		BufferItems: 64,
	}
}

// DefaultOlricConfig returns an OlricConfig with sensible defaults.
// These defaults match Olric's internal defaults and are suitable for single-node operation.
// Users can override for HA deployments requiring replication and quorum settings.
func DefaultOlricConfig() OlricConfig {
	return OlricConfig{
		DMapName:          "cc-relay",
		Environment:       "local",
		ReplicaCount:      1,
		ReadQuorum:        1,
		WriteQuorum:       1,
		MemberCountQuorum: 1,
		LeaveTimeout:      5 * time.Second,
	}
}
