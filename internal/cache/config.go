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

// Config defines cache configuration.
// Use Validate() to check for configuration errors before creating a cache.
type Config struct {
	Mode      Mode            `yaml:"mode"`
	Olric     OlricConfig     `yaml:"olric"`
	Ristretto RistrettoConfig `yaml:"ristretto"`
}

// RistrettoConfig configures the Ristretto local cache.
// Ristretto is a high-performance, concurrent cache based on research from
// the Caffeine library.
type RistrettoConfig struct {
	// NumCounters is the number of 4-bit access counters.
	// Recommended: 10x expected max items for optimal admission policy.
	// Example: For 100,000 items, use 1,000,000 counters.
	NumCounters int64 `yaml:"num_counters"`

	// MaxCost is the maximum cost (memory) the cache can hold.
	// Cost is measured in bytes of cached values.
	// Example: 100 << 20 for 100 MB.
	MaxCost int64 `yaml:"max_cost"`

	// BufferItems is the number of keys per Get buffer.
	// This controls the size of the admission buffer.
	// Recommended: 64 (default).
	BufferItems int64 `yaml:"buffer_items"`
}

// OlricConfig configures the Olric distributed cache.
// Olric provides a distributed in-memory key/value store with clustering support.
type OlricConfig struct {
	DMapName          string        `yaml:"dmap_name"`
	BindAddr          string        `yaml:"bind_addr"`
	Environment       string        `yaml:"environment"`
	Addresses         []string      `yaml:"addresses"`
	Peers             []string      `yaml:"peers"`
	ReplicaCount      int           `yaml:"replica_count"`
	ReadQuorum        int           `yaml:"read_quorum"`
	WriteQuorum       int           `yaml:"write_quorum"`
	LeaveTimeout      time.Duration `yaml:"leave_timeout"`
	MemberCountQuorum int32         `yaml:"member_count_quorum"`
	Embedded          bool          `yaml:"embedded"`
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
		if !c.Olric.Embedded && len(c.Olric.Addresses) == 0 {
			return errors.New("cache: olric.addresses required when not embedded")
		}
		if c.Olric.Embedded && c.Olric.BindAddr == "" {
			return errors.New("cache: olric.bind_addr required when embedded")
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
// DMapName: "cc-relay".
func DefaultOlricConfig() OlricConfig {
	return OlricConfig{
		DMapName: "cc-relay",
	}
}
