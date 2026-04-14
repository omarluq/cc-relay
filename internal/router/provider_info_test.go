package router_test

import (
	"testing"

	"github.com/omarluq/cc-relay/internal/router"
)

func TestProviderInfoHealthy(t *testing.T) {
	t.Parallel()

	t.Run("nil IsHealthy returns true", func(t *testing.T) {
		t.Parallel()
		info := router.NewTestProviderInfo("p1", 1, 1, nil)
		if !info.Healthy() {
			t.Error("ProviderInfo{} with nil IsHealthy should be healthy")
		}
	})

	t.Run("true IsHealthy returns true", func(t *testing.T) {
		t.Parallel()
		info := router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy())
		if !info.Healthy() {
			t.Error("ProviderInfo{} with true IsHealthy should be healthy")
		}
	})

	t.Run("false IsHealthy returns false", func(t *testing.T) {
		t.Parallel()
		info := router.NewTestProviderInfo("p1", 1, 1, router.NeverHealthy())
		if info.Healthy() {
			t.Error("ProviderInfo{} with false IsHealthy should be unhealthy")
		}
	})
}

func TestFilterHealthyAllHealthy(t *testing.T) {
	t.Parallel()

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p3", 1, 1, nil), // nil = healthy
	}

	healthy := router.FilterHealthy(providers)
	if len(healthy) != 3 {
		t.Errorf("FilterHealthy() = %d, want 3", len(healthy))
	}
}

func TestFilterHealthySomeUnhealthy(t *testing.T) {
	t.Parallel()

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.AlwaysHealthy()),
		router.NewTestProviderInfo("p2", 1, 1, router.NeverHealthy()),
		router.NewTestProviderInfo("p3", 1, 1, router.AlwaysHealthy()),
	}

	healthy := router.FilterHealthy(providers)
	if len(healthy) != 2 {
		t.Errorf("FilterHealthy() = %d, want 2", len(healthy))
	}
	for _, prov := range healthy {
		if prov.Provider.Name() == "p2" {
			t.Error("FilterHealthy() returned unhealthy p2")
		}
	}
}

func TestFilterHealthyAllUnhealthy(t *testing.T) {
	t.Parallel()

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, router.NeverHealthy()),
		router.NewTestProviderInfo("p2", 1, 1, router.NeverHealthy()),
	}

	healthy := router.FilterHealthy(providers)
	if len(healthy) != 0 {
		t.Errorf("FilterHealthy() = %d, want 0", len(healthy))
	}
}

func TestFilterHealthyEmpty(t *testing.T) {
	t.Parallel()

	providers := []router.ProviderInfo{}

	healthy := router.FilterHealthy(providers)
	if len(healthy) != 0 {
		t.Errorf("FilterHealthy() = %d, want 0", len(healthy))
	}
}

func TestFilterHealthyDynamic(t *testing.T) {
	t.Parallel()

	healthyCount := 0
	dynamic := func() bool {
		healthyCount++
		return healthyCount%2 == 1
	}

	providers := []router.ProviderInfo{
		router.NewTestProviderInfo("p1", 1, 1, dynamic),
	}

	// First call - count becomes 1, returns true
	healthy1 := router.FilterHealthy(providers)
	if len(healthy1) != 1 {
		t.Errorf("First FilterHealthy() = %d, want 1", len(healthy1))
	}

	// Second call - count becomes 2, returns false
	healthy2 := router.FilterHealthy(providers)
	if len(healthy2) != 0 {
		t.Errorf("Second FilterHealthy() = %d, want 0", len(healthy2))
	}
}
