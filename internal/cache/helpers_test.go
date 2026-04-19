package cache_test

import (
	"testing"
	"time"

	olricconfig "github.com/olric-data/olric/config"

	"github.com/omarluq/cc-relay/internal/cache"
)

func TestGetEnvironment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		env      string
		expected string
	}{
		{name: "empty returns local", env: "", expected: cache.EnvLocal},
		{name: "local passthrough", env: cache.EnvLocal, expected: cache.EnvLocal},
		{name: "lan passthrough", env: cache.EnvLAN, expected: cache.EnvLAN},
		{name: "wan passthrough", env: cache.EnvWAN, expected: cache.EnvWAN},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got := cache.GetEnvironmentForTest(testCase.env)
			if got != testCase.expected {
				t.Errorf("getEnvironment(%q) = %q, want %q", testCase.env, got, testCase.expected)
			}
		})
	}
}

func TestApplyBindPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		port         int
		expectChange bool
	}{
		{name: "zero port does nothing", port: 0, expectChange: false},
		{name: "negative port does nothing", port: -1, expectChange: false},
		{name: "positive port sets value", port: 3320, expectChange: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			cfg := olricconfig.New("local")
			initialPort := cfg.BindPort

			cache.ApplyBindPortForTest(cfg, testCase.port)

			if testCase.expectChange {
				if cfg.BindPort != testCase.port {
					t.Errorf("applyBindPort() set port to %d, want %d", cfg.BindPort, testCase.port)
				}
			} else {
				if cfg.BindPort != initialPort {
					expectedMsg := "applyBindPort() changed port from %d to %d, expected no change"
					t.Errorf(expectedMsg, initialPort, cfg.BindPort)
				}
			}
		})
	}
}

// TestApplyClusterSettings tests applying cluster settings to olric config.
func TestApplyClusterSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		peers        []string
		replicaCount int
		readQuorum   int
		writeQuorum  int
		memberCount  int32
		leaveTimeout time.Duration
	}{
		{
			name:         "all fields set",
			peers:        []string{"peer1", "peer2"},
			replicaCount: 2,
			readQuorum:   1,
			writeQuorum:  1,
			memberCount:  1,
			leaveTimeout: 5 * time.Second,
		},
		{
			name:         "only peers set",
			peers:        []string{"peer1"},
			replicaCount: 0,
			readQuorum:   0,
			writeQuorum:  0,
			memberCount:  0,
			leaveTimeout: 0,
		},
		{
			name:         "zero values do not override defaults",
			peers:        []string{},
			replicaCount: 0,
			readQuorum:   0,
			writeQuorum:  0,
			memberCount:  0,
			leaveTimeout: 0,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()
			t.Parallel()
			cfg := &cache.OlricConfig{
				DMapName:          "",
				BindAddr:          "",
				Environment:       "",
				Addresses:         nil,
				Peers:             testCase.peers,
				ReplicaCount:      testCase.replicaCount,
				ReadQuorum:        testCase.readQuorum,
				WriteQuorum:       testCase.writeQuorum,
				MemberCountQuorum: testCase.memberCount,
				LeaveTimeout:      testCase.leaveTimeout,
				Embedded:          false,
			}

			olricCfg := olricconfig.New("local")
			defaultPeers := olricCfg.Peers

			cache.ApplyClusterSettingsForTest(olricCfg, cfg)

			verifyClusterPeers(t, olricCfg, testCase.peers, defaultPeers)
			verifyClusterFields(t, olricCfg, &testCase)
		})
	}
}

func verifyClusterPeers(
	t *testing.T,
	olricCfg *olricconfig.Config,
	peers []string,
	defaultPeers []string,
) {
	t.Helper()
	if len(peers) > 0 {
		if len(olricCfg.Peers) != len(peers) {
			t.Errorf("applyClusterSettings() peers = %v, want %v", olricCfg.Peers, peers)
		}
	} else {
		if len(olricCfg.Peers) != len(defaultPeers) {
			t.Errorf("applyClusterSettings() changed default peers from %v to %v", defaultPeers, olricCfg.Peers)
		}
	}
}

func verifyClusterFields(t *testing.T, olricCfg *olricconfig.Config, testCase *struct {
	name         string
	peers        []string
	replicaCount int
	readQuorum   int
	writeQuorum  int
	memberCount  int32
	leaveTimeout time.Duration
}) {
	t.Helper()
	verifyReplicaAndQuorums(t, olricCfg, testCase.replicaCount, testCase.readQuorum, testCase.writeQuorum)
	verifyMemberCountAndTimeout(t, olricCfg, testCase.memberCount, testCase.leaveTimeout)
}

func verifyReplicaAndQuorums(
	t *testing.T,
	olricCfg *olricconfig.Config,
	replicaCount int,
	readQuorum int,
	writeQuorum int,
) {
	t.Helper()
	if replicaCount > 0 && olricCfg.ReplicaCount != replicaCount {
		msg := "applyClusterSettings() replicaCount = %d, want %d"
		t.Errorf(msg, olricCfg.ReplicaCount, replicaCount)
	}
	if readQuorum > 0 && olricCfg.ReadQuorum != readQuorum {
		t.Errorf("applyClusterSettings() readQuorum = %d, want %d", olricCfg.ReadQuorum, readQuorum)
	}
	if writeQuorum > 0 && olricCfg.WriteQuorum != writeQuorum {
		t.Errorf("applyClusterSettings() writeQuorum = %d, want %d", olricCfg.WriteQuorum, writeQuorum)
	}
}

func verifyMemberCountAndTimeout(
	t *testing.T,
	olricCfg *olricconfig.Config,
	memberCount int32,
	leaveTimeout time.Duration,
) {
	t.Helper()
	if memberCount > 0 && olricCfg.MemberCountQuorum != memberCount {
		memberCountMsg := "applyClusterSettings() memberCountQuorum = %d, want %d"
		t.Errorf(memberCountMsg, olricCfg.MemberCountQuorum, memberCount)
	}
	if leaveTimeout > 0 && olricCfg.LeaveTimeout != leaveTimeout {
		leaveTimeoutMsg := "applyClusterSettings() leaveTimeout = %v, want %v"
		t.Errorf(leaveTimeoutMsg, olricCfg.LeaveTimeout, leaveTimeout)
	}
}

// TestConfigureMemberlist tests memberlist configuration.
func TestConfigureMemberlist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		bindAddr string
		bindPort int
	}{
		{name: "with port", bindAddr: "127.0.0.1", bindPort: 3320},
		{name: "without port", bindAddr: "127.0.0.1", bindPort: 0},
		{name: "ipv6 with port", bindAddr: "::1", bindPort: 3320},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()
			t.Parallel()
			olricCfg := olricconfig.New("local")

			cache.ConfigureMemberlistForTest(olricCfg, testCase.bindAddr, testCase.bindPort)

			if olricCfg.MemberlistConfig == nil {
				t.Fatal("configureMemberlist() did not create MemberlistConfig")
			}

			if olricCfg.MemberlistConfig.BindAddr != testCase.bindAddr {
				bindAddrMsg := "configureMemberlist() BindAddr = %q, want %q"
				t.Errorf(bindAddrMsg, olricCfg.MemberlistConfig.BindAddr, testCase.bindAddr)
			}

			verifyMemberlistPort(t, olricCfg, testCase.bindPort)
		})
	}
}

func verifyMemberlistPort(t *testing.T, olricCfg *olricconfig.Config, bindPort int) {
	t.Helper()
	if bindPort <= 0 {
		return
	}
	expectedPort := bindPort + 2
	if olricCfg.MemberlistConfig.BindPort != expectedPort {
		bindPortMsg := "configureMemberlist() BindPort = %d, want %d"
		t.Errorf(bindPortMsg, olricCfg.MemberlistConfig.BindPort, expectedPort)
	}
	if olricCfg.MemberlistConfig.AdvertisePort != expectedPort {
		advertisePortMsg := "configureMemberlist() AdvertisePort = %d, want %d"
		t.Errorf(advertisePortMsg, olricCfg.MemberlistConfig.AdvertisePort, expectedPort)
	}
}

// newEmptyOlricConfig creates an empty OlricConfig with all fields set to zero values.
func newEmptyOlricConfig() cache.OlricConfig {
	return cache.OlricConfig{
		DMapName:          "",
		BindAddr:          "",
		Environment:       "",
		Addresses:         nil,
		Peers:             nil,
		ReplicaCount:      0,
		ReadQuorum:        0,
		WriteQuorum:       0,
		MemberCountQuorum: 0,
		LeaveTimeout:      0,
		Embedded:          false,
	}
}

// newOlricConfigWithAddr creates an OlricConfig with specified bind address.
func newOlricConfigWithAddr(bindAddr string) cache.OlricConfig {
	cfg := newEmptyOlricConfig()
	cfg.BindAddr = bindAddr
	return cfg
}

// TestBuildOlricConfig tests building olric config from cache config.
func TestBuildOlricConfig(t *testing.T) {
	t.Parallel()

	t.Run("basic config", func(t *testing.T) {
		t.Parallel()
		cfg := newOlricConfigWithAddr("127.0.0.1:3320")
		cfg.Environment = cache.EnvLocal

		olricCfg := cache.BuildOlricConfigForTest(&cfg)
		if olricCfg == nil {
			t.Fatal("buildOlricConfig() returned nil")
		}

		if olricCfg.BindAddr != "127.0.0.1" {
			t.Errorf("BindAddr = %q, want 127.0.0.1", olricCfg.BindAddr)
		}
		if olricCfg.BindPort != 3320 {
			t.Errorf("BindPort = %d, want 3320", olricCfg.BindPort)
		}
	})

	t.Run("with cluster settings", func(t *testing.T) {
		t.Parallel()
		cfg := newOlricConfigWithAddr("127.0.0.1:3320")
		cfg.Peers = []string{"peer1"}
		cfg.ReplicaCount = 2
		cfg.ReadQuorum = 1
		cfg.WriteQuorum = 1
		cfg.MemberCountQuorum = 1

		olricCfg := cache.BuildOlricConfigForTest(&cfg)
		if olricCfg == nil {
			t.Fatal("buildOlricConfig() returned nil")
		}

		if len(olricCfg.Peers) != 1 {
			t.Errorf("Peers length = %d, want 1", len(olricCfg.Peers))
		}
		if olricCfg.ReplicaCount != 2 {
			t.Errorf("ReplicaCount = %d, want 2", olricCfg.ReplicaCount)
		}
	})

	t.Run("empty environment defaults to local", func(t *testing.T) {
		t.Parallel()
		cfg := newOlricConfigWithAddr("127.0.0.1:3320")

		olricCfg := cache.BuildOlricConfigForTest(&cfg)
		if olricCfg == nil {
			t.Fatal("buildOlricConfig() returned nil")
		}
	})
}

func TestParseBindAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     string
		wantHost string
		wantPort int
	}{
		{name: "host only", addr: "127.0.0.1", wantHost: "127.0.0.1", wantPort: 0},
		{name: "host and port", addr: "127.0.0.1:3320", wantHost: "127.0.0.1", wantPort: 3320},
		{name: "ipv6 loopback", addr: "::1", wantHost: "::1", wantPort: 0},
		{name: "ipv6 with port", addr: "[::1]:3320", wantHost: "::1", wantPort: 3320},
		{name: "hostname only", addr: "localhost", wantHost: "localhost", wantPort: 0},
		{name: "hostname and port", addr: "localhost:3320", wantHost: "localhost", wantPort: 3320},
		{name: "empty string", addr: "", wantHost: "", wantPort: 0},
		{name: "invalid port returns host only", addr: "127.0.0.1:abc", wantHost: "127.0.0.1", wantPort: 0},
		{name: "just colon", addr: ":", wantHost: "", wantPort: 0},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Helper()
			t.Parallel()
			host, port := cache.ParseBindAddrForTest(testCase.addr)
			if host != testCase.wantHost {
				hostMsg := "parseBindAddr(%q) host = %q, want %q"
				t.Errorf(hostMsg, testCase.addr, host, testCase.wantHost)
			}
			if port != testCase.wantPort {
				portMsg := "parseBindAddr(%q) port = %d, want %d"
				t.Errorf(portMsg, testCase.addr, port, testCase.wantPort)
			}
		})
	}
}
