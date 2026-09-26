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
	if request.SKU == "small" {
		return pricing.InstanceRate{SKU: "small", VCPU: 2, MemoryGB: 8, HourlyUSD: 0.08, Purchase: request.Purchase, Confidence: costmodel.ConfidenceExact}, nil
	}
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
	if request.SKU == "small" {
		// 0.08/h split 50/50 over 2 cores and 8 GB
		return pricing.RateCard{CPUUSDPerCoreHour: 0.02, MemoryUSDPerGBHour: 0.005, CPUCostShare: 0.5, Confidence: costmodel.ConfidenceDerived}, nil
	}
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
	// idle is node capacity only; the control plane is shared, not idle
	if got := report.Idle.Hourly; !nearly(got, 0.36) {
		t.Errorf("idle hourly = %v, want 0.36", got)
	}
	if got := report.Shared.Hourly; got != 0.1 {
		t.Errorf("shared hourly = %v, want the control plane 0.1", got)
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
	if got := report.ByDimension[costmodel.BasisProvisioned][costmodel.DimensionComponent][string(costmodel.ComponentStorage)].Hourly; got != 0.01 {
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

func placedCluster() Cluster {
	cluster := baseCluster()
	cluster.HasPlacement = true
	cluster.Nodes[1].MachineType = "small"
	cluster.Workloads = []Workload{
		{ID: "w1", Kind: "Deployment", Name: "api", Namespace: "prod", Replicas: 2, Placements: []Placement{
			{Node: "node-1", Pods: 1, Usage: costmodel.Usage{CPUCores: 1, MemoryGB: 2}},
			{Node: "node-2", Pods: 1, Usage: costmodel.Usage{CPUCores: 1, MemoryGB: 2}},
		}},
		{ID: "w2", Kind: "Job", Name: "backup", Namespace: "ops", Replicas: 1, Placements: []Placement{
			{Node: "node-2", Pods: 1, Usage: costmodel.Usage{CPUCores: 0.5, MemoryGB: 1, StorageGB: 20}},
		}},
	}
	cluster.Volumes = []Volume{{ID: "pvc-1", Name: "data", Namespace: "prod", StorageGB: 100}}
	return cluster
}

func TestAllocatePricesPodsAtTheirNodeRate(t *testing.T) {
	report := New(stubResolver{nodeHourly: 0.20, storage: 0.0001}).Allocate(placedCluster(), time.Unix(0, 0))

	// big: 1*0.02 + 2*0.01 = 0.04, small: 1*0.02 + 2*0.005 = 0.03
	api := findItem(t, report, "api")
	if !nearly(api.HourlyUSD, 0.07) {
		t.Errorf("api hourly = %v, want 0.07 priced at each node's rate", api.HourlyUSD)
	}
	if api.Detail["replicas"] != 2.0 || api.Detail["nodes"] != 2 {
		t.Errorf("api detail = %+v, want 2 replicas over 2 nodes", api.Detail)
	}

	// ephemeral storage is part of the node price and is not charged again
	backup := findItem(t, report, "backup")
	if _, ok := backup.Components[costmodel.ComponentStorage]; ok || !nearly(backup.HourlyUSD, 0.015) {
		t.Errorf("backup = %v %+v, want 0.015 with no storage component", backup.HourlyUSD, backup.Components)
	}
}

func TestAllocateReconcilesEveryDollarOfTheBill(t *testing.T) {
	report := New(stubResolver{nodeHourly: 0.20, storage: 0.0001}).Allocate(placedCluster(), time.Unix(0, 0))

	billed := report.Totals[costmodel.BasisProvisioned].Hourly
	// 0.20 big + 0.08 small + 0.10 control plane + 100 GB * 0.0001
	if !nearly(billed, 0.39) {
		t.Fatalf("billed = %v, want 0.39", billed)
	}
	for _, dimension := range []costmodel.Dimension{costmodel.DimensionNamespace, costmodel.DimensionWorkload} {
		total := 0.0
		for _, row := range report.Allocation[dimension] {
			total += row.Cost.Hourly
		}
		if !nearly(total, billed) {
			t.Errorf("%s rows sum to %v, want the billed total %v", dimension, total, billed)
		}
	}

	// small node: cpu 0.04 - (0.02 + 0.01), memory 0.04 - (0.01 + 0.005)
	// big node:   cpu 0.10 - 0.02,          memory 0.10 - 0.02
	if got := report.IdleByComponent[costmodel.ComponentCPU].Hourly; !nearly(got, 0.09) {
		t.Errorf("idle cpu = %v, want 0.09", got)
	}
	if got := report.IdleByComponent[costmodel.ComponentMemory].Hourly; !nearly(got, 0.105) {
		t.Errorf("idle memory = %v, want 0.105", got)
	}
}

func TestAllocateLeavesOutPodsOnUnpricedNodes(t *testing.T) {
	cluster := placedCluster()
	cluster.Nodes = append(cluster.Nodes, Node{ID: "n3", Name: "node-3", MachineType: "mystery"})
	cluster.Workloads = []Workload{
		{ID: "w1", Kind: "Deployment", Name: "api", Namespace: "prod", Placements: []Placement{
			{Node: "node-1", Pods: 1, Usage: costmodel.Usage{CPUCores: 1, MemoryGB: 2}},
			{Node: "node-3", Pods: 2, Usage: costmodel.Usage{CPUCores: 2, MemoryGB: 4}},
		}},
		{ID: "w2", Kind: "Deployment", Name: "stranded", Namespace: "prod", Placements: []Placement{
			{Node: "node-3", Pods: 1, Usage: costmodel.Usage{CPUCores: 1, MemoryGB: 1}},
		}},
	}

	report := New(stubResolver{nodeHourly: 0.20}).Allocate(cluster, time.Unix(0, 0))

	if api := findItem(t, report, "api"); !nearly(api.HourlyUSD, 0.04) || api.Confidence == costmodel.ConfidenceUnknown {
		t.Errorf("api = %v (%s), want only the node-1 pod priced", api.HourlyUSD, api.Confidence)
	}
	if stranded := findItem(t, report, "stranded"); stranded.Confidence != costmodel.ConfidenceUnknown {
		t.Errorf("stranded confidence = %q, want unknown", stranded.Confidence)
	}
	if !hasWarning(report, "workload-on-unpriced-nodes") {
		t.Errorf("Warnings = %+v, want workload-on-unpriced-nodes", report.Warnings)
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

func TestAllocatePricesUsageAtTheSameNodeRates(t *testing.T) {
	cluster := placedCluster()
	cluster.Workloads[0].Placements[0].Used = &costmodel.Usage{CPUCores: 0.25, MemoryGB: 1}
	cluster.Workloads[0].Placements[1].Used = &costmodel.Usage{CPUCores: 0.25, MemoryGB: 1}

	report := New(stubResolver{nodeHourly: 0.20}).Allocate(cluster, time.Unix(0, 0))

	api := findItem(t, report, "api")
	if api.Used == nil || api.Used.CPUCores != 0.5 {
		t.Fatalf("api used = %+v, want 0.5 cores", api.Used)
	}
	// big: 0.25*0.02, small: 0.25*0.02
	if got := api.UsedComponents[costmodel.ComponentCPU]; !nearly(got, 0.01) {
		t.Errorf("api used cpu cost = %v, want 0.01", got)
	}
	if got := report.Usage.Efficiency[costmodel.ComponentCPU]; !nearly(got, 0.25) {
		t.Errorf("cpu efficiency = %v, want 0.25", got)
	}

	// backup has no usage for its pod, so it stays out of the comparison
	if backup := findItem(t, report, "backup"); backup.Used != nil {
		t.Errorf("backup used = %+v, want nil", backup.Used)
	}
	if report.Usage.WithoutUsage != 1 {
		t.Errorf("WithoutUsage = %d, want 1", report.Usage.WithoutUsage)
	}
}
