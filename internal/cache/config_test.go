package cache_test

import (
	"testing"
	"time"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestConfigValidateValidSingleMode(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1000,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestConfigValidateValidHAMode(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode: cache.ModeHA,
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "127.0.0.1:3320",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          true,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 0,
			MaxCost:     0,
			BufferItems: 0,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestConfigValidateValidDisabledMode(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode: cache.ModeDisabled,
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 0,
			MaxCost:     0,
			BufferItems: 0,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestConfigValidateInvalidMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mode    cache.Mode
		wantErr string
	}{
		{
			name:    "empty mode",
			mode:    "",
			wantErr: "mode is required",
		},
		{
			name:    "unknown mode",
			mode:    "invalid-mode",
			wantErr: "invalid-mode",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cfg := cache.Config{
				Mode: testCase.mode,
				Olric: cache.OlricConfig{
					DMapName:          "",
					BindAddr:          "",
					Environment:       "",
					Addresses:         nil,
					Peers:             nil,
					ReplicaCount:      0,
					ReadQuorum:        0,
					WriteQuorum:       0,
					LeaveTimeout:      0,
					MemberCountQuorum: 0,
					Embedded:          false,
				},
				Ristretto: cache.RistrettoConfig{
					NumCounters: 0,
					MaxCost:     0,
					BufferItems: 0,
				},
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
			if !cache.ContainsString(err.Error(), testCase.wantErr) {
				t.Errorf("error %q should contain %q", err.Error(), testCase.wantErr)
			}
		})
	}
}

func TestConfigValidateSingleModeZeroMaxCost(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 1000,
			MaxCost:     0,
			BufferItems: 64,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "max_cost must be positive") {
		t.Errorf("error %q should contain 'max_cost must be positive'", err.Error())
	}
}

func TestConfigValidateSingleModeZeroNumCounters(t *testing.T) {
	t.Parallel()
	cfg := cache.Config{
		Mode: cache.ModeSingle,
		Olric: cache.OlricConfig{
			DMapName:          "",
			BindAddr:          "",
			Environment:       "",
			Addresses:         nil,
			Peers:             nil,
			ReplicaCount:      0,
			ReadQuorum:        0,
			WriteQuorum:       0,
			LeaveTimeout:      0,
			MemberCountQuorum: 0,
			Embedded:          false,
		},
		Ristretto: cache.RistrettoConfig{
			NumCounters: 0,
			MaxCost:     1 << 20,
			BufferItems: 64,
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "num_counters must be positive") {
		t.Errorf("error %q should contain 'num_counters must be positive'", err.Error())
	}
}

func TestOlricConfigValidateEmbeddedNoBindAddr(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "bind_addr required") {
		t.Errorf("error %q should contain 'bind_addr required'", err.Error())
	}
}

func TestOlricConfigValidateClientModeNoAddresses(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          false,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "addresses required") {
		t.Errorf("error %q should contain 'addresses required'", err.Error())
	}
}

func TestOlricConfigValidateInvalidEnvironment(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "127.0.0.1:3320",
		Environment:       "invalid-env",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), `"local", "lan", or "wan"`) {
		t.Errorf("error %q should list valid environments", err.Error())
	}
}

func TestOlricConfigValidateValidEnvironments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		env  string
	}{
		{"empty (default)", ""},
		{"local", cache.EnvLocal},
		{"lan", cache.EnvLAN},
		{"wan", cache.EnvWAN},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cfg := cache.OlricConfig{
				DMapName:          "",
				BindAddr:          "127.0.0.1:3320",
				Environment:       testCase.env,
				Addresses:         nil,
				Peers:             nil,
				ReplicaCount:      0,
				ReadQuorum:        0,
				WriteQuorum:       0,
				LeaveTimeout:      0,
				MemberCountQuorum: 0,
				Embedded:          true,
			}

			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate() error = %v, want nil for env %q", err, testCase.env)
			}
		})
	}
}

func TestOlricConfigValidateWriteQuorumExceedsReplicaCount(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "127.0.0.1:3320",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      2,
		ReadQuorum:        0,
		WriteQuorum:       3,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "write_quorum cannot exceed replica_count") {
		t.Errorf("error %q should mention write_quorum exceeding replica_count", err.Error())
	}
}

func TestOlricConfigValidateReadQuorumExceedsReplicaCount(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "127.0.0.1:3320",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      2,
		ReadQuorum:        3,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "read_quorum cannot exceed replica_count") {
		t.Errorf("error %q should mention read_quorum exceeding replica_count", err.Error())
	}
}

func TestOlricConfigValidateNegativeMemberCountQuorum(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "127.0.0.1:3320",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: -1,
		Embedded:          true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "member_count_quorum cannot be negative") {
		t.Errorf("error %q should mention negative member_count_quorum", err.Error())
	}
}

func TestOlricConfigValidateNegativeLeaveTimeout(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "127.0.0.1:3320",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      -1 * time.Second,
		MemberCountQuorum: 0,
		Embedded:          true,
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error")
	}
	if !cache.ContainsString(err.Error(), "leave_timeout cannot be negative") {
		t.Errorf("error %q should mention negative leave_timeout", err.Error())
	}
}

func TestOlricConfigValidateValidQuorum(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "127.0.0.1:3320",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      3,
		ReadQuorum:        2,
		WriteQuorum:       2,
		LeaveTimeout:      5 * time.Second,
		MemberCountQuorum: 1,
		Embedded:          true,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestDefaultRistrettoConfig(t *testing.T) {
	t.Parallel()
	cfg := cache.DefaultRistrettoConfig()

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
	t.Parallel()
	cfg := cache.DefaultOlricConfig()

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

func TestOlricConfigValidateClientModeWithAddresses(t *testing.T) {
	t.Parallel()
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "",
		Environment:       "",
		Addresses:         []string{"127.0.0.1:3320", "127.0.0.1:3321"},
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          false,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestOlricConfigValidateZeroQuorumValues(t *testing.T) {
	t.Parallel()
	// Zero quorum values should be valid (uses Olric defaults)
	cfg := cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "127.0.0.1:3320",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		LeaveTimeout:      0,
		MemberCountQuorum: 0,
		Embedded:          true,
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil for zero quorum values", err)
	}
}
