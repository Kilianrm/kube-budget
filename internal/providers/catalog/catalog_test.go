package catalog

import (
	"testing"

	"kube-budget/core/costmodel"
	"kube-budget/core/pricing"
)

var testCatalog = Catalog{
	ProviderName:    "test",
	Region:          "region-a",
	MachineType:     "big",
	MachineTypeNoun: "instance type",
	Instances: map[string]map[string]Instance{
		"region-a": {"big": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.20}},
	},
	StorageUSDPerGBMonth:   0.08,
	StorageDescription:     "test disk",
	ControlPlaneUSDPerHour: 0.10,
	CPUCostShare:           0.5,
	SpotPriceRatio:         0.35,
	Source:                 "unit-test",
	RegionAliases:          map[string]string{"west1": "west-1"},
}

func TestResolveNodePricesOnDemandAndSpot(t *testing.T) {
	onDemand, err := testCatalog.ResolveNode(pricing.RateRequest{Region: "region-a", SKU: "big"})
	if err != nil {
		t.Fatalf("ResolveNode() error = %v", err)
	}
	if onDemand.HourlyUSD != 0.20 || onDemand.Confidence != costmodel.ConfidenceExact {
		t.Errorf("on-demand rate = %v (%s), want 0.20 (exact)", onDemand.HourlyUSD, onDemand.Confidence)
	}
	if onDemand.Purchase != pricing.PurchaseOnDemand {
		t.Errorf("Purchase = %q, want on-demand default", onDemand.Purchase)
	}

	spot, err := testCatalog.ResolveNode(pricing.RateRequest{Region: "region-a", SKU: "big", Purchase: pricing.PurchaseSpot})
	if err != nil {
		t.Fatalf("ResolveNode(spot) error = %v", err)
	}
	if wantSpot := float64(0.20) * float64(0.35); spot.HourlyUSD != wantSpot {
		t.Errorf("spot rate = %v, want %v", spot.HourlyUSD, wantSpot)
	}
	if spot.Confidence != costmodel.ConfidenceEstimated {
		t.Errorf("spot confidence = %q, want estimated", spot.Confidence)
	}
}

func TestResolveResourcesSplitsPriceAndRecordsAssumptions(t *testing.T) {
	card, err := testCatalog.ResolveResources(pricing.RateRequest{Region: "region-a", SKU: "big"})
	if err != nil {
		t.Fatalf("ResolveResources() error = %v", err)
	}

	if card.CPUUSDPerCoreHour != 0.025 {
		t.Errorf("CPUUSDPerCoreHour = %v, want 0.025", card.CPUUSDPerCoreHour)
	}
	if card.MemoryUSDPerGBHour != 0.00625 {
		t.Errorf("MemoryUSDPerGBHour = %v, want 0.00625", card.MemoryUSDPerGBHour)
	}
	if card.Confidence != costmodel.ConfidenceDerived {
		t.Errorf("Confidence = %q, want derived", card.Confidence)
	}
	if card.CPUCostShare != 0.5 {
		t.Errorf("CPUCostShare = %v, want 0.5", card.CPUCostShare)
	}
	if len(card.Assumptions) != 2 {
		t.Errorf("Assumptions = %+v, want the cpu split and storage rate", card.Assumptions)
	}
}

func TestResolveFlatPricesControlPlane(t *testing.T) {
	rate, err := testCatalog.ResolveFlat(pricing.RateRequest{Region: "region-a", SKU: SKUControlPlane})
	if err != nil {
		t.Fatalf("ResolveFlat() error = %v", err)
	}
	if rate.HourlyUSD != 0.10 {
		t.Errorf("control plane rate = %v, want 0.10", rate.HourlyUSD)
	}

	if _, err := testCatalog.ResolveFlat(pricing.RateRequest{SKU: "load-balancer"}); err == nil {
		t.Error("ResolveFlat(load-balancer) error = nil, want unsupported SKU error")
	}
}

func TestNormalizeRegionAppliesAliases(t *testing.T) {
	if got := testCatalog.NormalizeRegion(" eu-west1 "); got != "eu-west-1" {
		t.Errorf("NormalizeRegion() = %q, want eu-west-1", got)
	}
}

func TestLookupErrorsUseProviderWording(t *testing.T) {
	if _, err := testCatalog.ResolveNode(pricing.RateRequest{Region: "nowhere", SKU: "big"}); err == nil {
		t.Error("ResolveNode() error = nil, want unsupported region")
	}
	_, err := testCatalog.ResolveNode(pricing.RateRequest{Region: "region-a", SKU: "small"})
	if err == nil || !contains(err.Error(), "unknown instance type") {
		t.Errorf("ResolveNode() error = %v, want unknown instance type", err)
	}
}

func contains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}
