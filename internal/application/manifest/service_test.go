package manifest

import (
	"os"
	"testing"

	"kube-budget/core/pricing"
)

func TestEstimateConvertsAndPricesManifest(t *testing.T) {
	input, err := os.ReadFile("../../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	service := New(pricing.PriceConfig{
		CPUUSDPerCore:  2,
		MemoryUSDPerGB: 1,
	})

	result, err := service.Estimate(input)
	if err != nil {
		t.Fatalf("Estimate() error = %v", err)
	}
	if result.Workload.Name != "api" || result.Workload.Namespace != "production" {
		t.Errorf("Estimate().Workload identity = %q/%q, want production/api", result.Workload.Namespace, result.Workload.Name)
	}
	if result.Workload.Replicas != 3 {
		t.Errorf("Estimate().Workload.Replicas = %d, want 3", result.Workload.Replicas)
	}
	if result.Estimate.Total != 4.5 {
		t.Errorf("Estimate().Estimate.Total = %v, want 4.5", result.Estimate.Total)
	}
}

func TestEstimateReturnsConversionError(t *testing.T) {
	input, err := os.ReadFile("../../../data/manifests/missing-name.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	service := New(pricing.PriceConfig{})

	_, err = service.Estimate(input)
	if err == nil {
		t.Fatal("Estimate() error = nil, want missing name error")
	}
}