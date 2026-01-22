package cache

import (
	"testing"
	"time"
)

func TestConfig_Validate_ValidSingleMode(t *testing.T) {
	cfg := Config{
		Mode: ModeSingle,
		Ristretto: RistrettoConfig{
			NumCounters: 1000,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestConfig_Validate_ValidHAMode(t *testing.T) {
	cfg := Config{
		Mode: ModeHA,
		Olric: OlricConfig{
			Embedded: true,
			BindAddr: "127.0.0.1:3320",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestConfig_Validate_ValidDisabledMode(t *testing.T) {
	cfg := Config{
		Mode: ModeDisabled,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestConfig_Validate_EmptyMode(t *testing.T) {
	cfg := Config{
		Mode: "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "mode is required") {
		t.Errorf("error %q should contain 'mode is required'", err.Error())
	}
}

func TestConfig_Validate_UnknownMode(t *testing.T) {
	cfg := Config{
		Mode: "invalid-mode",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "invalid-mode") {
		t.Errorf("error %q should contain 'invalid-mode'", err.Error())
	}
}

func TestConfig_Validate_SingleModeZeroMaxCost(t *testing.T) {
	cfg := Config{
		Mode: ModeSingle,
		Ristretto: RistrettoConfig{
			NumCounters: 1000,
			MaxCost:     0,
			BufferItems: 64,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "max_cost must be positive") {
		t.Errorf("error %q should contain 'max_cost must be positive'", err.Error())
	}
}

func TestConfig_Validate_SingleModeZeroNumCounters(t *testing.T) {
	cfg := Config{
		Mode: ModeSingle,
		Ristretto: RistrettoConfig{
			NumCounters: 0,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "num_counters must be positive") {
		t.Errorf("error %q should contain 'num_counters must be positive'", err.Error())
	}
}

func TestOlricConfig_Validate_EmbeddedNoBindAddr(t *testing.T) {
	cfg := OlricConfig{
		Embedded: true,
		BindAddr: "",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "bind_addr required") {
		t.Errorf("error %q should contain 'bind_addr required'", err.Error())
	}
}

func TestOlricConfig_Validate_ClientModeNoAddresses(t *testing.T) {
	cfg := OlricConfig{
		Embedded:  false,
		Addresses: nil,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "addresses required") {
		t.Errorf("error %q should contain 'addresses required'", err.Error())
	}
}

func TestOlricConfig_Validate_InvalidEnvironment(t *testing.T) {
	cfg := OlricConfig{
		Embedded:    true,
		BindAddr:    "127.0.0.1:3320",
		Environment: "invalid-env",
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), `"local", "lan", or "wan"`) {
		t.Errorf("error %q should list valid environments", err.Error())
	}
}

func TestOlricConfig_Validate_ValidEnvironments(t *testing.T) {
	tests := []struct {
		name string
		env  string
	}{
		{"empty (default)", ""},
		{"local", EnvLocal},
		{"lan", EnvLAN},
		{"wan", EnvWAN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := OlricConfig{
				Embedded:    true,
				BindAddr:    "127.0.0.1:3320",
				Environment: tt.env,
			}

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() error = %v, want nil for env %q", err, tt.env)
			}
		})
	}
}

func TestOlricConfig_Validate_WriteQuorumExceedsReplicaCount(t *testing.T) {
	cfg := OlricConfig{
		Embedded:     true,
		BindAddr:     "127.0.0.1:3320",
		ReplicaCount: 2,
		WriteQuorum:  3,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "write_quorum cannot exceed replica_count") {
		t.Errorf("error %q should mention write_quorum exceeding replica_count", err.Error())
	}
}

func TestOlricConfig_Validate_ReadQuorumExceedsReplicaCount(t *testing.T) {
	cfg := OlricConfig{
		Embedded:     true,
		BindAddr:     "127.0.0.1:3320",
		ReplicaCount: 2,
		ReadQuorum:   3,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "read_quorum cannot exceed replica_count") {
		t.Errorf("error %q should mention read_quorum exceeding replica_count", err.Error())
	}
}

func TestOlricConfig_Validate_NegativeMemberCountQuorum(t *testing.T) {
	cfg := OlricConfig{
		Embedded:          true,
		BindAddr:          "127.0.0.1:3320",
		MemberCountQuorum: -1,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "member_count_quorum cannot be negative") {
		t.Errorf("error %q should mention negative member_count_quorum", err.Error())
	}
}

func TestOlricConfig_Validate_NegativeLeaveTimeout(t *testing.T) {
	cfg := OlricConfig{
		Embedded:     true,
		BindAddr:     "127.0.0.1:3320",
		LeaveTimeout: -1 * time.Second,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !containsString(err.Error(), "leave_timeout cannot be negative") {
		t.Errorf("error %q should mention negative leave_timeout", err.Error())
	}
}

func TestOlricConfig_Validate_ValidQuorum(t *testing.T) {
	cfg := OlricConfig{
		Embedded:          true,
		BindAddr:          "127.0.0.1:3320",
		ReplicaCount:      3,
		ReadQuorum:        2,
		WriteQuorum:       2,
		MemberCountQuorum: 1,
		LeaveTimeout:      5 * time.Second,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestDefaultRistrettoConfig(t *testing.T) {
	cfg := DefaultRistrettoConfig()

	if cfg.NumCounters != 1_000_000 {
		t.Errorf("NumCounters = %d, want 1000000", cfg.NumCounters)
	}
	if cfg.MaxCost != 100<<20 {
		t.Errorf("MaxCost = %d, want %d", cfg.MaxCost, 100<<20)
	}
	if cfg.BufferItems != 64 {
		t.Errorf("BufferItems = %d, want 64", cfg.BufferItems)
	}
}

func TestDefaultOlricConfig(t *testing.T) {
	cfg := DefaultOlricConfig()

	if cfg.DMapName != "cc-relay" {
		t.Errorf("DMapName = %q, want 'cc-relay'", cfg.DMapName)
	}
	if cfg.Environment != "local" {
		t.Errorf("Environment = %q, want 'local'", cfg.Environment)
	}
	if cfg.ReplicaCount != 1 {
		t.Errorf("ReplicaCount = %d, want 1", cfg.ReplicaCount)
	}
	if cfg.ReadQuorum != 1 {
		t.Errorf("ReadQuorum = %d, want 1", cfg.ReadQuorum)
	}
	if cfg.WriteQuorum != 1 {
		t.Errorf("WriteQuorum = %d, want 1", cfg.WriteQuorum)
	}
	if cfg.MemberCountQuorum != 1 {
		t.Errorf("MemberCountQuorum = %d, want 1", cfg.MemberCountQuorum)
	}
	if cfg.LeaveTimeout != 5*time.Second {
		t.Errorf("LeaveTimeout = %v, want 5s", cfg.LeaveTimeout)
	}
}

func TestOlricConfig_Validate_ClientModeWithAddresses(t *testing.T) {
	cfg := OlricConfig{
		Embedded:  false,
		Addresses: []string{"127.0.0.1:3320", "127.0.0.1:3321"},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestOlricConfig_Validate_ZeroQuorumValues(t *testing.T) {
	// Zero quorum values should be valid (uses Olric defaults)
	cfg := OlricConfig{
		Embedded:     true,
		BindAddr:     "127.0.0.1:3320",
		ReplicaCount: 0,
		ReadQuorum:   0,
		WriteQuorum:  0,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil for zero quorum values", err)
	}
}
