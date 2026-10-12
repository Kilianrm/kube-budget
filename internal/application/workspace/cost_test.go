package workspace

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/costseries"
	"kube-cost-workspace/core/optimize"
	clustermode "kube-cost-workspace/internal/application/cluster"
	"kube-cost-workspace/internal/application/costexplorer"
	"kube-cost-workspace/internal/providers"
	"kube-cost-workspace/internal/storage/costhistory"
)

const gibibyte = 1024 * 1024 * 1024

func values(cpu int64, memoryGiB int64) clustermode.ResourceValues {
	return clustermode.ResourceValues{CPUMilli: cpu, MemoryBytes: memoryGiB * gibibyte}
}

// simulationSnapshot is a two-node m6i.large cluster with a tainted c6i.large
// spot node that the simulation must not use.
func simulationSnapshot() clustermode.Snapshot {
	return clustermode.Snapshot{
		Cluster: clustermode.ClusterInfo{Context: "demo"},
		Nodes: []clustermode.Node{
			{UID: "a", Name: "a", ProviderID: "aws:///us-east-1a/i-a", InstanceType: "m6i.large", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 7)},
			{UID: "b", Name: "b", ProviderID: "aws:///us-east-1a/i-b", InstanceType: "m6i.large", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 7)},
			{UID: "s", Name: "s", ProviderID: "aws:///us-east-1a/i-s", InstanceType: "c6i.large", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 3),
				Taints: []string{"workload-tier=batch:NoSchedule"}},
		},
		Workloads: []clustermode.Workload{{UID: "w", Kind: "Deployment", Name: "base", Namespace: "prod", DesiredReplicas: 2,
			Containers: []clustermode.Container{{Name: "base", Requests: values(1500, 5)}}}},
		Pods: []clustermode.Pod{
			{Name: "base-1", Namespace: "prod", NodeName: "a", OwnerKind: "Deployment", OwnerName: "base", Requests: values(1500, 5)},
			{Name: "base-2", Namespace: "prod", NodeName: "b", OwnerKind: "Deployment", OwnerName: "base", Requests: values(1500, 5)},
		},
	}
}

func TestPriceUsesTheDetectedPlatform(t *testing.T) {
	snapshot := simulationSnapshot()
	snapshot.Platform = clusterPlatform(snapshot.Nodes, "", "")

	priced, err := Price(snapshot, PriceOptions{})
	if err != nil {
		t.Fatalf("Price() error = %v", err)
	}
	if priced.Report.Scope.Provider != "aws" || priced.Report.Scope.Region != "us-east-1" || priced.ClusterID == "" {
		t.Errorf("scope = %+v, cluster = %q; want aws us-east-1 with an ID", priced.Report.Scope, priced.ClusterID)
	}
	if priced.Report.Totals[costmodel.BasisProvisioned].Hourly <= 0 || len(priced.Plan.MachineTypes) == 0 {
		t.Errorf("report = %+v, plan = %+v; want a billed total and the catalog's machine types", priced.Report.Totals, priced.Plan)
	}
}

func TestPriceRefusesAClusterWithoutAPriceList(t *testing.T) {
	snapshot := clustermode.Snapshot{Nodes: []clustermode.Node{{Name: "minikube"}}}
	snapshot.Platform = clusterPlatform(snapshot.Nodes, "", "")

	if _, err := Price(snapshot, PriceOptions{}); err == nil || !strings.Contains(err.Error(), "cannot be priced") {
		t.Errorf("Price() error = %v, want cannot be priced", err)
	}
	if _, err := Price(snapshot, PriceOptions{Provider: "ibm"}); err == nil || !strings.Contains(err.Error(), "unsupported pricing provider") {
		t.Errorf("Price() error = %v, want unsupported provider", err)
	}
}

func TestPricingProviderNameDerivesCatalogFromClusterProvider(t *testing.T) {
	for provider, want := range map[string]string{"aws-eks": "aws", "Azure": "azure", "": ""} {
		if got := pricingProviderName(provider); got != want {
			t.Errorf("pricingProviderName(%q) = %q, want %q", provider, got, want)
		}
	}
}

func TestClusterPlatformDecidesWhetherTheCostSectionCanPriceACluster(t *testing.T) {
	eks := []clustermode.Node{{ProviderID: "aws:///us-east-1a/i-0abc", Region: "us-east-1"}}
	for _, test := range []struct {
		name       string
		nodes      []clustermode.Node
		connection string
		region     string
		supported  bool
		reason     string
	}{
		{name: "eks through a kubeconfig context", nodes: eks, supported: true},
		{name: "eks connection without nodes yet", connection: "aws-eks", region: "eu-west-1", supported: true},
		{name: "kind", nodes: []clustermode.Node{{ProviderID: "kind://docker/kind/kind-control-plane"}}, reason: `"kind"`},
		{name: "minikube", nodes: []clustermode.Node{{Name: "minikube"}}, reason: "local or on-premises"},
		{name: "region outside the catalog", nodes: []clustermode.Node{{ProviderID: "aws:///ap-south-1a/i-0abc", Region: "ap-south-1"}}, reason: "ap-south-1"},
		{name: "no region", nodes: []clustermode.Node{{ProviderID: "aws:///zone/i-0abc"}}, reason: "no region label"},
	} {
		t.Run(test.name, func(t *testing.T) {
			platform := clusterPlatform(test.nodes, test.connection, test.region)
			if platform.Supported != test.supported || !strings.Contains(platform.Reason, test.reason) {
				t.Errorf("platform = %+v, want supported %v with a reason mentioning %q", platform, test.supported, test.reason)
			}
		})
	}
}

func TestForecastResultAttachesTheBudgetOnlyWhenSet(t *testing.T) {
	now := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	records := []costhistory.Record{{
		CapturedAt:        now.Add(-30 * time.Minute),
		HourlyByBasis:     map[string]float64{"provisioned": 2},
		HourlyByNodeGroup: map[string]float64{"general": 2},
	}}

	withoutBudget := forecastResult(records, now, 2, 0)
	if withoutBudget.Budget != nil {
		t.Errorf("Budget = %+v, want nil without a budget", withoutBudget.Budget)
	}
	// 30 recorded minutes, 359.5 estimated hours and 360 projected hours, all at 2/h
	if got := withoutBudget.Forecast.TotalUSD; got < 1439.99 || got > 1440.01 {
		t.Errorf("TotalUSD = %v, want 1440", got)
	}

	withBudget := forecastResult(records, now, 2, 1000)
	if withBudget.Budget == nil || withBudget.Budget.State != costseries.BudgetOver {
		t.Errorf("Budget = %+v, want over", withBudget.Budget)
	}
}

func TestPlanInputsFromTheAWSCatalog(t *testing.T) {
	provider, _ := providers.Get("aws")

	inputs := PlanInputs(provider, "us-east-1")

	if len(inputs.MachineTypes) == 0 || math.Abs(inputs.SpotPriceRatio-0.35) > 1e-9 {
		t.Fatalf("inputs = %+v, want the catalog's machine types and spot ratio", inputs)
	}
	for _, machine := range inputs.MachineTypes {
		if machine.Burstable != (machine.Name == "t3.medium") {
			t.Errorf("%s burstable = %v", machine.Name, machine.Burstable)
		}
	}
}

// The showcase relies on the node-type step proposing c6i.large for
// memory-rich m6i.large nodes under CPU-heavy requests. The spot group's
// fallback types sit in the same catalog and must never take its place.
func TestTheAWSCatalogStillProposesC6iForCPUHeavyM6iNodes(t *testing.T) {
	snapshot := clustermode.Snapshot{Cluster: clustermode.ClusterInfo{Context: "showcase"}}
	for _, name := range []string{"a", "b", "c"} {
		snapshot.Nodes = append(snapshot.Nodes, clustermode.Node{UID: name, Name: name, InstanceType: "m6i.large", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 7)})
	}
	snapshot.Workloads = []clustermode.Workload{{UID: "w", Kind: "Deployment", Name: "api", Namespace: "shop", DesiredReplicas: 3,
		Containers: []clustermode.Container{{Name: "api", Requests: values(1000, 1)}}}}
	for index, node := range []string{"a", "b", "c"} {
		snapshot.Pods = append(snapshot.Pods, clustermode.Pod{Name: fmt.Sprintf("api-%d", index), Namespace: "shop", NodeName: node, OwnerKind: "Deployment", OwnerName: "api", Requests: values(1000, 1)})
	}
	provider, _ := providers.Get("aws")
	report, err := costexplorer.New(provider).Report(snapshot, costexplorer.Options{Provider: "aws", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	plan := optimize.Build(report, PlanInputs(provider, "us-east-1"))
	for _, recommendation := range plan.Recommendations {
		if recommendation.ID == "node-type" {
			if recommendation.Title != "Switch to c6i.large nodes" {
				t.Errorf("node-type = %q, want c6i.large", recommendation.Title)
			}
			return
		}
	}
	t.Fatalf("recommendations = %+v, want a node-type step", plan.Recommendations)
}

func TestOnlyEKSConnectionsGetEKSNodeGroups(t *testing.T) {
	metadata := &clustermode.ProviderMetadata{Provider: "aws-eks", ClusterName: "shop", Region: "us-east-1",
		NodeGroups: []clustermode.NodeGroup{{Name: "general", DesiredSize: 3, MinSize: 2}}}

	cluster := eksCluster(metadata)
	if cluster == nil || cluster.Name != "shop" || len(cluster.NodeGroups) != 1 || cluster.NodeGroups[0].Desired != 3 || cluster.NodeGroups[0].Min != 2 {
		t.Errorf("eksCluster() = %+v, want shop with the general group", cluster)
	}
	if eksCluster(nil) != nil || eksCluster(&clustermode.ProviderMetadata{Provider: "gke", ClusterName: "shop", Region: "x"}) != nil {
		t.Error("eksCluster() gave sizes outside EKS")
	}
}

func TestSimulateUsesTheClusterRates(t *testing.T) {
	// Never read the user's real history, budgets or dismissals.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	manifest, err := os.ReadFile("../../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	snapshot := simulationSnapshot()
	snapshot.Platform = clusterPlatform(snapshot.Nodes, "", "")
	priced, err := Price(snapshot, PriceOptions{})
	if err != nil {
		t.Fatal(err)
	}

	result, err := Simulate(priced, manifest)
	if err != nil {
		t.Fatalf("Simulate() error = %v", err)
	}
	// priced with the machine type the untainted nodes run, not the spot one
	if result.Estimate.InstanceType != "m6i.large" || result.Estimate.Provider != "aws" {
		t.Errorf("estimate = %+v, want aws m6i.large", result.Estimate)
	}
	if result.Impact.BilledBefore.Hourly != priced.Report.Totals[costmodel.BasisProvisioned].Hourly || result.ForecastBefore == nil {
		t.Errorf("impact = %+v, want it measured against the report with a forecast", result.Impact)
	}
	if _, err := Simulate(priced, nil); err == nil {
		t.Error("Simulate() without a manifest, want an error")
	}
}
