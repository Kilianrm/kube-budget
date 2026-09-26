package wails

import (
	"os"
	"strings"
	"testing"

	"kube-budget/core/costmodel"
	clustermode "kube-budget/internal/application/cluster"
	"kube-budget/internal/application/costexplorer"
	"kube-budget/internal/providers"
)

const gibibyte = 1024 * 1024 * 1024

// simulationReport is a two-node m6i.large cluster with a tainted c6i.large
// spot node that the simulation must not use.
func simulationReport(t *testing.T) costmodel.CostReport {
	t.Helper()
	values := func(cpu int64, memoryGiB int64) clustermode.ResourceValues {
		return clustermode.ResourceValues{CPUMilli: cpu, MemoryBytes: memoryGiB * gibibyte}
	}
	snapshot := clustermode.Snapshot{
		Cluster: clustermode.ClusterInfo{Context: "demo"},
		Nodes: []clustermode.Node{
			{UID: "a", Name: "a", InstanceType: "m6i.large", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 7)},
			{UID: "b", Name: "b", InstanceType: "m6i.large", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 7)},
			{UID: "s", Name: "s", InstanceType: "c6i.large", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 3),
				Taints: []string{"workload-tier=batch:NoSchedule"}},
		},
		Workloads: []clustermode.Workload{{UID: "w", Kind: "Deployment", Name: "base", Namespace: "prod", DesiredReplicas: 2,
			Containers: []clustermode.Container{{Name: "base", Requests: values(1500, 5)}}}},
		Pods: []clustermode.Pod{
			{Name: "base-1", Namespace: "prod", NodeName: "a", OwnerKind: "Deployment", OwnerName: "base", Requests: values(1500, 5)},
			{Name: "base-2", Namespace: "prod", NodeName: "b", OwnerKind: "Deployment", OwnerName: "base", Requests: values(1500, 5)},
		},
	}
	provider, _ := providers.Get("aws")
	report, err := costexplorer.New(provider).Report(snapshot, costexplorer.Options{Provider: "aws", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	return report
}

func TestSimulateManifestUsesTheClusterRatesAndItsLastReport(t *testing.T) {
	// Never read the user's real history, budgets or dismissals.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	manifest, err := os.ReadFile("../../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	adapter := NewClusterAdapter()

	if _, err := adapter.SimulateManifest(SimulationRequest{ClusterID: "demo", Document: ManifestDocument{Name: "api.yaml", Content: string(manifest)}}); err == nil ||
		!strings.Contains(err.Error(), "load the cluster's cost report") {
		t.Fatalf("SimulateManifest() without a report error = %v, want a hint to load the report", err)
	}

	report := simulationReport(t)
	provider, _ := providers.Get("aws")
	adapter.rememberCost("demo", report, planInputs(provider, "us-east-1"))

	result, err := adapter.SimulateManifest(SimulationRequest{ClusterID: "demo", Document: ManifestDocument{Name: "api.yaml", Content: string(manifest)}})
	if err != nil {
		t.Fatalf("SimulateManifest() error = %v", err)
	}
	// priced with the machine type the untainted nodes run, not the spot one
	if result.Estimate.Pricing.InstanceType != "m6i.large" || result.Estimate.Pricing.Provider != "aws" {
		t.Errorf("pricing = %+v, want aws m6i.large", result.Estimate.Pricing)
	}
	for _, placement := range result.Impact.Placements {
		if placement.Node == "s" {
			t.Errorf("placements = %+v, want the tainted spot node left alone", result.Impact.Placements)
		}
	}
	if result.Impact.Requested.Hourly <= 0 || result.Impact.BilledBefore.Hourly != report.Totals[costmodel.BasisProvisioned].Hourly {
		t.Errorf("impact = %+v, want it measured against the cached report", result.Impact)
	}
	if result.ForecastBefore == nil || result.ForecastAfter == nil {
		t.Fatal("forecasts missing, want before and after")
	}
	grew := result.ForecastAfter.Forecast.TotalUSD - result.ForecastBefore.Forecast.TotalUSD
	expected := result.Impact.BilledDelta.Hourly * result.ForecastBefore.Forecast.RemainingHours
	if diff := grew - expected; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("forecast grew %v, want the billed change over the rest of the month (%v)", grew, expected)
	}
}
