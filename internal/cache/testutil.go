//go:build integration
// +build integration

package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testClusterPortCounter is used to generate unique ports for cluster tests.
// Separate from the regular test port counter to avoid conflicts.
var testClusterPortCounter atomic.Int32

func init() {
	// Start from a high port to avoid conflicts with regular tests.
	testClusterPortCounter.Store(14320)
}

// getClusterTestPort returns a unique port for cluster testing.
// Each call returns a port that is 10 higher than the previous to allow
// room for both Olric client port and memberlist port (port + 2).
func getClusterTestPort() int {
	return int(testClusterPortCounter.Add(10))
}

// testCacheCluster manages a group of embedded Olric nodes for testing.
// It handles node creation, peer discovery, and cleanup.
type testCacheCluster struct {
	t            *testing.T
	dmapName     string
	members      []*olricCache
	memberAddrs  []string // Memberlist addresses (host:memberlistPort) for peer discovery
	mtx          sync.Mutex
}

// newTestCacheCluster creates a new test cluster.
// All nodes are cleaned up when the test completes.
func newTestCacheCluster(t *testing.T) *testCacheCluster {
	t.Helper()

	cl := &testCacheCluster{
		members:  make([]*olricCache, 0),
		t:        t,
		dmapName: fmt.Sprintf("test-cluster-%d", getClusterTestPort()),
	}

	t.Cleanup(func() {
		cl.shutdown()
	})

	return cl
}

// addMember adds a new node to the cluster.
// The new node will attempt to join existing nodes via the Peers list.
func (cl *testCacheCluster) addMember() *olricCache {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()

	port := getClusterTestPort()
	// Memberlist port is Olric port + 2 (see buildOlricConfig)
	memberlistPort := port + 2
	memberlistAddr := fmt.Sprintf("127.0.0.1:%d", memberlistPort)

	cfg := &OlricConfig{
		DMapName:     cl.dmapName,
		Embedded:     true,
		BindAddr:     fmt.Sprintf("127.0.0.1:%d", port),
		Environment:  EnvLocal,
		ReplicaCount: 2, // Enable replication for HA tests
	}

	// Add existing members as peers for discovery.
	// Use the tracked memberlist addresses since MemberlistAddr() doesn't work in embedded mode.
	cfg.Peers = append(cfg.Peers, cl.memberAddrs...)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cache, err := newOlricCache(ctx, cfg)
	if err != nil {
		cl.t.Fatalf("failed to add cluster member: %v", err)
	}

	cl.members = append(cl.members, cache)
	cl.memberAddrs = append(cl.memberAddrs, memberlistAddr)

	// Give the cluster time to converge
	time.Sleep(500 * time.Millisecond)

	return cache
}

// shutdown closes all cluster members gracefully.
func (cl *testCacheCluster) shutdown() {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()

	for _, m := range cl.members {
		if err := m.Close(); err != nil {
			cl.t.Logf("warning: failed to close cluster member: %v", err)
		}
	}
	cl.members = nil
}

// memberCount returns the number of nodes in the cluster.
func (cl *testCacheCluster) memberCount() int {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	return len(cl.members)
}

// member returns the node at the given index.
func (cl *testCacheCluster) member(i int) *olricCache {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()
	if i < 0 || i >= len(cl.members) {
		cl.t.Fatalf("member index %d out of range [0, %d)", i, len(cl.members))
	}
	return cl.members[i]
}

// waitForConvergence waits for all nodes to see the expected member count.
// Returns error if convergence doesn't happen within timeout.
func (cl *testCacheCluster) waitForConvergence(expectedMembers int, timeout time.Duration) error {
	cl.mtx.Lock()
	members := make([]*olricCache, len(cl.members))
	copy(members, cl.members)
	cl.mtx.Unlock()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		allConverged := true
		for _, m := range members {
			if count := m.ClusterMembers(); count != expectedMembers {
				allConverged = false
				break
			}
		}
		if allConverged {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("cluster did not converge to %d members within %v", expectedMembers, timeout)
}

// removeMember removes and closes the node at the given index.
// Returns the removed cache (already closed).
func (cl *testCacheCluster) removeMember(i int) *olricCache {
	cl.mtx.Lock()
	defer cl.mtx.Unlock()

	if i < 0 || i >= len(cl.members) {
		cl.t.Fatalf("member index %d out of range [0, %d)", i, len(cl.members))
	}

	member := cl.members[i]

	// Remove from slices
	cl.members = append(cl.members[:i], cl.members[i+1:]...)
	if i < len(cl.memberAddrs) {
		cl.memberAddrs = append(cl.memberAddrs[:i], cl.memberAddrs[i+1:]...)
	}

	// Close the member
	if err := member.Close(); err != nil {
		cl.t.Logf("warning: failed to close removed member: %v", err)
	}

	return member
}
