package allocation

import (
	"errors"
	"math"
	"testing"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/pricing"
)

// stubResolver prices a single machine type so expectations stay readable.
type stubResolver struct {
	nodeHourly float64
	storage    float64
	gpu        float64
	failFlat   bool
}

func (resolver stubResolver) ResolveNode(request pricing.RateRequest) (pricing.InstanceRate, error) {
	if request.SKU != "big" {
		return pricing.InstanceRate{}, errors.New("unknown instance type")
	}
	hourly := resolver.nodeHourly
	confidence := costmodel.ConfidenceExact
	if request.Purchase == pricing.PurchaseSpot {
		hourly *= 0.5
		confidence = costmodel.ConfidenceEstimated
	}
	return pricing.InstanceRate{
		SKU: "big", VCPU: 4, MemoryGB: 16, HourlyUSD: hourly,
		Purchase: request.Purchase, Confidence: confidence,
	}, nil
}

func (resolver stubResolver) ResolveResources(request pricing.RateRequest) (pricing.RateCard, error) {
	if request.SKU != "big" {
		return pricing.RateCard{}, errors.New("unknown instance type")
	}
	return pricing.RateCard{
		CPUUSDPerCoreHour:   0.02,
		MemoryUSDPerGBHour:  0.01,
		StorageUSDPerGBHour: resolver.storage,
		GPUUSDPerUnitHour:   resolver.gpu,
		CPUCostShare:        0.5,
		Confidence:          costmodel.ConfidenceDerived,
	}, nil
}

func (resolver stubResolver) ResolveFlat(request pricing.RateRequest) (pricing.FlatRate, error) {
	if resolver.failFlat {
		return pricing.FlatRate{}, errors.New("no flat rate")
	}
	return pricing.FlatRate{Description: "control plane", HourlyUSD: 0.10, Confidence: costmodel.ConfidenceExact}, nil
}

func baseCluster() Cluster {
	return Cluster{
		Scope:      costmodel.Scope{Provider: "test", Region: "region-a", ClusterName: "demo"},
		PricingSKU: "big",
		Nodes: []Node{
			{ID: "n1", Name: "node-1", NodeGroup: "ng-1", MachineType: "big", Region: "region-a", Ready: true, Schedulable: true},
			{ID: "n2", Name: "node-2", NodeGroup: "ng-1", MachineType: "big", Region: "region-a", Ready: true, Schedulable: true},
		},
		Workloads: []Workload{
			{ID: "w1", Kind: "Deployment", Name: "api", Namespace: "prod", Replicas: 2, PerReplica: costmodel.Usage{CPUCores: 0.5, MemoryGB: 1}},
		},
		HasControlPlane: true,
	}
}

func TestAllocateSeparatesProvisionedFromRequested(t *testing.T) {
	report := New(stubResolver{nodeHourly: 0.20}).Allocate(baseCluster(), time.Unix(0, 0))

	// two nodes at 0.20 plus a 0.10 control plane
	if got := report.Totals[costmodel.BasisProvisioned].Hourly; got != 0.5 {
		t.Errorf("provisioned hourly = %v, want 0.5", got)
	}
	// two replicas of 0.5 core and 1 GB: 2*(0.5*0.02 + 1*0.01)
	if got := report.Totals[costmodel.BasisRequested].Hourly; got != 0.04 {
		t.Errorf("requested hourly = %v, want 0.04", got)
	}
	if got := report.Idle.Hourly; got != 0.46 {
		t.Errorf("idle hourly = %v, want 0.46", got)
	}
	if got := report.Totals[costmodel.BasisProvisioned].Monthly; got != 0.5*costmodel.HoursPerMonth {
		t.Errorf("provisioned monthly = %v, want %v", got, 0.5*costmodel.HoursPerMonth)
	}
}

func TestAllocateTreatsMissingRequestsAsUnknownNotZero(t *testing.T) {
	cluster := baseCluster()
	cluster.Workloads = append(cluster.Workloads, Workload{
		ID: "w2", Kind: "Deployment", Name: "legacy", Namespace: "prod", Replicas: 3, MissingRequests: true,
	})

	report := New(stubResolver{nodeHourly: 0.20}).Allocate(cluster, time.Unix(0, 0))

	if got := report.Totals[costmodel.BasisRequested].Hourly; got != 0.04 {
		t.Errorf("requested hourly = %v, want the priced workload only", got)
	}
	if !hasWarning(report, "workload-missing-requests") {
		t.Errorf("Warnings = %+v, want workload-missing-requests", report.Warnings)
	}
	if confidence := findItem(t, report, "legacy").Confidence; confidence != costmodel.ConfidenceUnknown {
		t.Errorf("legacy confidence = %q, want unknown", confidence)
	}
}

func TestAllocateScalesDaemonSetsWithNodeCount(t *testing.T) {
	cluster := baseCluster()
	cluster.Workloads = []Workload{
		{ID: "d1", Kind: "DaemonSet", Name: "agent", Namespace: "kube-system", PerReplica: costmodel.Usage{CPUCores: 0.1}},
	}

	report := New(stubResolver{nodeHourly: 0.20}).Allocate(cluster, time.Unix(0, 0))

	agent := findItem(t, report, "agent")
	if agent.Usage.CPUCores != 0.2 {
		t.Errorf("agent CPU = %v, want one replica per node", agent.Usage.CPUCores)
	}
	if agent.HourlyUSD != 0.2*0.02 {
		t.Errorf("agent hourly = %v, want %v", agent.HourlyUSD, 0.2*0.02)
	}
}

func TestAllocateExcludesUnpricedNodesAndGPUs(t *testing.T) {
	cluster := baseCluster()
	cluster.Nodes = append(cluster.Nodes, Node{ID: "n3", Name: "node-3", MachineType: "mystery", Region: "region-a"})
	cluster.Workloads = append(cluster.Workloads, Workload{
		ID: "w3", Kind: "Deployment", Name: "trainer", Namespace: "ml", Replicas: 1,
		PerReplica: costmodel.Usage{CPUCores: 1, GPUUnits: 2},
	})

	report := New(stubResolver{nodeHourly: 0.20}).Allocate(cluster, time.Unix(0, 0))

	if got := report.Totals[costmodel.BasisProvisioned].Hourly; got != 0.5 {
		t.Errorf("provisioned hourly = %v, want the unpriced node excluded", got)
	}
	if got := report.Totals[costmodel.BasisRequested].Hourly; got != 0.04 {
		t.Errorf("requested hourly = %v, want the GPU workload excluded", got)
	}
	if !hasWarning(report, "node-rate-unresolved") || !hasWarning(report, "gpu-unpriced") {
		t.Errorf("Warnings = %+v, want node-rate-unresolved and gpu-unpriced", report.Warnings)
	}
}

func TestAllocatePricesSpotNodesAndVolumes(t *testing.T) {
	cluster := baseCluster()
	cluster.Nodes[1].Purchase = pricing.PurchaseSpot
	cluster.Volumes = []Volume{{ID: "pvc-1", Name: "data", Namespace: "prod", StorageClass: "gp3", StorageGB: 100}}

	report := New(stubResolver{nodeHourly: 0.20, storage: 0.0001}).Allocate(cluster, time.Unix(0, 0))

	// 0.20 on-demand + 0.10 spot + 0.10 control plane + 100 GB * 0.0001
	if got := report.Totals[costmodel.BasisProvisioned].Hourly; !nearly(got, 0.41) {
		t.Errorf("provisioned hourly = %v, want 0.41", got)
	}
	if confidence := findItem(t, report, "node-2").Confidence; confidence != costmodel.ConfidenceEstimated {
		t.Errorf("spot node confidence = %q, want estimated", confidence)
	}
	if got := report.ByDimension[costmodel.DimensionComponent][string(costmodel.ComponentStorage)].Hourly; got != 0.01 {
		t.Errorf("storage component = %v, want 0.01", got)
	}
}

func TestAllocateReportsUnresolvableControlPlane(t *testing.T) {
	report := New(stubResolver{nodeHourly: 0.20, failFlat: true}).Allocate(baseCluster(), time.Unix(0, 0))

	if got := report.Totals[costmodel.BasisProvisioned].Hourly; got != 0.4 {
		t.Errorf("provisioned hourly = %v, want nodes only", got)
	}
	if !hasWarning(report, "controlplane-rate-unresolved") {
		t.Errorf("Warnings = %+v, want controlplane-rate-unresolved", report.Warnings)
	}
}

func findItem(t *testing.T, report costmodel.CostReport, name string) costmodel.LineItem {
	t.Helper()
	for _, item := range report.Items {
		if item.Subject.Name == name {
			return item
		}
	}
	t.Fatalf("no line item named %q", name)
	return costmodel.LineItem{}
}

func hasWarning(report costmodel.CostReport, code string) bool {
	for _, warning := range report.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func nearly(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}
