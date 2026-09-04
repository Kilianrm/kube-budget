package engine

import (
	"testing"

	"kube-budget/core/pricing"
)

func TestEstimateMultipliesCostsByReplicas(t *testing.T) {
	engine := New(pricing.PriceConfig{
		CPUUSDPerCore:  2,
		MemoryUSDPerGB: 1,
	})

	estimate := engine.Estimate(Workload{
		Name:     "api",
		Replicas: 2,
		Resources: []pricing.Resource{
			{Name: "api", CPU: 0.5, MemoryGB: 1},
		},
	})

	if estimate.Total != 4 {
		t.Errorf("Estimate().Total = %v, want 4", estimate.Total)
	}
	if estimate.PerResource["api"] != 4 {
		t.Errorf("Estimate().PerResource[api] = %v, want 4", estimate.PerResource["api"])
	}
}
