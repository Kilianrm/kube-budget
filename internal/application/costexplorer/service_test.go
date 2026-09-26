package costexplorer

import (
	"testing"

	"kube-budget/core/costmodel"
	clustermode "kube-budget/internal/application/cluster"
	"kube-budget/internal/providers"
)

func snapshot() clustermode.Snapshot {
	return clustermode.Snapshot{
		Cluster: clustermode.ClusterInfo{Context: "demo"},
		Nodes: []clustermode.Node{
			{UID: "n1", Name: "node-1", InstanceType: "m6i.large", Region: "us-east-1", Ready: true, Schedulable: true,
				Allocatable: clustermode.ResourceValues{CPUMilli: 2000, MemoryBytes: 8 * bytesPerGiB}},
			{UID: "n2", Name: "node-2", InstanceType: "m6i.large", Region: "us-east-1", Ready: true, Schedulable: true,
				Allocatable: clustermode.ResourceValues{CPUMilli: 2000, MemoryBytes: 8 * bytesPerGiB}},
		},
		Workloads: []clustermode.Workload{
			{UID: "w1", Kind: "Deployment", Name: "api", Namespace: "prod", DesiredReplicas: 2,
				Requests: clustermode.ResourceValues{CPUMilli: 1000, MemoryBytes: 2 * bytesPerGiB}},
		},
		Resources: []clustermode.Resource{
			{UID: "pvc1", Kind: "PersistentVolumeClaim", Name: "data", Namespace: "prod",
				Attributes: map[string]string{"Storage class": "gp3"},
				Requests:   clustermode.ResourceValues{StorageBytes: 10 * bytesPerGiB}},
			{UID: "cm1", Kind: "ConfigMap", Name: "settings", Namespace: "prod"},
		},
		Provider: &clustermode.ProviderMetadata{
			Provider: "aws", ClusterName: "demo-eks", Region: "us-east-1",
			NodeGroups: []clustermode.NodeGroup{{Name: "ng-spot", InstanceTypes: []string{"m6i.large"}, CapacityType: "SPOT"}},
		},
		Warnings: []clustermode.Warning{{Resource: "cronjobs", Message: "could not list cronjobs"}},
	}
}

func awsResolver(t *testing.T) providers.Provider {
	t.Helper()
	provider, ok := providers.Get("aws")
	if !ok {
		t.Fatal("providers.Get(aws) not registered")
	}
	return provider
}

func TestReportPricesBothLenses(t *testing.T) {
	report, err := New(awsResolver(t)).Report(snapshot(), Options{})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	if report.Scope.ClusterName != "demo-eks" || report.Scope.Region != "us-east-1" {
		t.Errorf("Scope = %+v, want the EKS cluster metadata", report.Scope)
	}
	if report.Totals[costmodel.BasisProvisioned].Hourly <= 0 {
		t.Errorf("provisioned total = %v, want a positive figure", report.Totals[costmodel.BasisProvisioned].Hourly)
	}
	if report.Totals[costmodel.BasisRequested].Hourly <= 0 {
		t.Errorf("requested total = %v, want a positive figure", report.Totals[costmodel.BasisRequested].Hourly)
	}
	if report.Idle.Hourly <= 0 {
		t.Errorf("idle = %v, want provisioned above requested", report.Idle.Hourly)
	}
	if report.Totals[costmodel.BasisProvisioned].Monthly != report.Totals[costmodel.BasisProvisioned].Hourly*costmodel.HoursPerMonth {
		t.Error("monthly total is not the 730-hour projection of the hourly total")
	}
}

func TestReportAppliesSpotCapacityFromNodeGroup(t *testing.T) {
	report, err := New(awsResolver(t)).Report(snapshot(), Options{})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	for _, item := range report.Items {
		if item.Subject.Kind != costmodel.SubjectNode {
			continue
		}
		if item.Subject.ParentID != "ng-spot" {
			t.Errorf("node %q parent = %q, want ng-spot", item.Subject.Name, item.Subject.ParentID)
		}
		if item.Confidence != costmodel.ConfidenceEstimated {
			t.Errorf("node %q confidence = %q, want estimated for spot", item.Subject.Name, item.Confidence)
		}
		// m6i.large on-demand is 0.096 in us-east-1; spot is a fraction of it.
		if item.HourlyUSD >= 0.096 {
			t.Errorf("node %q hourly = %v, want below the on-demand price", item.Subject.Name, item.HourlyUSD)
		}
	}
}

func TestReportProjectsVolumesAndPerReplicaRequests(t *testing.T) {
	report, err := New(awsResolver(t)).Report(snapshot(), Options{})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	var volume, workload *costmodel.LineItem
	for index := range report.Items {
		switch report.Items[index].Subject.Kind {
		case costmodel.SubjectVolume:
			volume = &report.Items[index]
		case costmodel.SubjectWorkload:
			workload = &report.Items[index]
		}
	}

	if volume == nil || volume.Usage.StorageGB != 10 {
		t.Fatalf("volume item = %+v, want 10 GB from the PVC", volume)
	}
	if workload == nil {
		t.Fatal("no workload line item")
	}
	// The collector already multiplies requests by replicas; the total must not
	// be squared by the allocator.
	if workload.Usage.CPUCores != 1 || workload.Usage.MemoryGB != 2 {
		t.Errorf("workload usage = %+v, want the snapshot totals", workload.Usage)
	}
}

func TestReportCarriesCollectionWarnings(t *testing.T) {
	report, err := New(awsResolver(t)).Report(snapshot(), Options{})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	found := false
	for _, warning := range report.Warnings {
		if warning.Code == "collection-incomplete" && warning.Subject == "cronjobs" {
			found = true
		}
	}
	if !found {
		t.Errorf("Warnings = %+v, want the collector warning carried over", report.Warnings)
	}
}

func TestReportFailsWithoutAPricingReference(t *testing.T) {
	bare := snapshot()
	bare.Nodes = nil
	bare.Provider = nil

	if _, err := New(awsResolver(t)).Report(bare, Options{}); err == nil {
		t.Error("Report() error = nil, want a missing instance type error")
	}
}

func TestReportPricesPodsOnTheirNodes(t *testing.T) {
	placed := snapshot()
	placed.Provider = nil
	placed.Nodes[1].InstanceType = "m6i.xlarge"
	placed.Pods = []clustermode.Pod{
		{Name: "api-1", Namespace: "prod", NodeName: "node-1", Phase: "Running", OwnerKind: "Deployment", OwnerName: "api",
			Requests: clustermode.ResourceValues{CPUMilli: 500, MemoryBytes: bytesPerGiB}},
		{Name: "api-2", Namespace: "prod", NodeName: "node-2", Phase: "Running", OwnerKind: "Deployment", OwnerName: "api",
			Requests: clustermode.ResourceValues{CPUMilli: 500, MemoryBytes: bytesPerGiB}},
		{Name: "backup-1", Namespace: "ops", NodeName: "node-2", Phase: "Running", OwnerKind: "Job", OwnerName: "backup",
			Requests: clustermode.ResourceValues{CPUMilli: 250, MemoryBytes: bytesPerGiB}},
		{Name: "backup-2", Namespace: "ops", Phase: "Pending", OwnerKind: "Job", OwnerName: "backup",
			Requests: clustermode.ResourceValues{CPUMilli: 250, MemoryBytes: bytesPerGiB}},
	}

	report, err := New(awsResolver(t)).Report(placed, Options{Provider: "aws", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	var api, backup *costmodel.LineItem
	for index := range report.Items {
		switch report.Items[index].Subject.Name {
		case "api":
			api = &report.Items[index]
		case "backup":
			backup = &report.Items[index]
		}
	}
	if api == nil || backup == nil {
		t.Fatalf("Items = %+v, want api and the Job pods as their own workload", report.Items)
	}
	if api.Detail["nodes"] != 2 {
		t.Errorf("api placed on %v nodes, want 2", api.Detail["nodes"])
	}
	// only the scheduled Job pod is billed
	if backup.Usage.CPUCores != 0.25 || backup.Detail["replicas"] != 1.0 {
		t.Errorf("backup usage = %+v detail = %+v, want the scheduled pod only", backup.Usage, backup.Detail)
	}

	billed := report.Totals[costmodel.BasisProvisioned].Hourly
	total := 0.0
	for _, row := range report.Allocation[costmodel.DimensionNamespace] {
		total += row.Cost.Hourly
	}
	if diff := total - billed; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("namespace allocation sums to %v, want the billed total %v", total, billed)
	}
}

func TestReportWarnsWhenCollectedForOneNamespace(t *testing.T) {
	report, err := New(awsResolver(t)).Report(snapshot(), Options{Namespace: "prod"})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	for _, warning := range report.Warnings {
		if warning.Code == "namespace-scoped" {
			return
		}
	}
	t.Errorf("Warnings = %+v, want namespace-scoped", report.Warnings)
}

func TestReportComparesUsageOnlyForFullySampledWorkloads(t *testing.T) {
	sampled := snapshot()
	sampled.Provider = nil
	sampled.Pods = []clustermode.Pod{
		{Name: "api-1", Namespace: "prod", NodeName: "node-1", OwnerKind: "Deployment", OwnerName: "api",
			Requests: clustermode.ResourceValues{CPUMilli: 500, MemoryBytes: bytesPerGiB}, Usage: &clustermode.ResourceValues{CPUMilli: 50, MemoryBytes: bytesPerGiB / 2}},
		{Name: "api-2", Namespace: "prod", NodeName: "node-2", OwnerKind: "Deployment", OwnerName: "api",
			Requests: clustermode.ResourceValues{CPUMilli: 500, MemoryBytes: bytesPerGiB}, Usage: &clustermode.ResourceValues{CPUMilli: 50, MemoryBytes: bytesPerGiB / 2}},
		{Name: "backup-1", Namespace: "ops", NodeName: "node-2", OwnerKind: "Job", OwnerName: "backup",
			Requests: clustermode.ResourceValues{CPUMilli: 250, MemoryBytes: bytesPerGiB}},
	}

	report, err := New(awsResolver(t)).Report(sampled, Options{Provider: "aws", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Usage == nil {
		t.Fatal("Usage = nil, want a usage summary")
	}
	// same node type everywhere, so cost efficiency equals quantity efficiency
	if got := report.Usage.Efficiency[costmodel.ComponentCPU]; got < 0.0999 || got > 0.1001 {
		t.Errorf("cpu efficiency = %v, want 0.1", got)
	}
	if report.Usage.Workloads != 1 || report.Usage.WithoutUsage != 1 {
		t.Errorf("usage counts = %d with / %d without, want api measured and backup not", report.Usage.Workloads, report.Usage.WithoutUsage)
	}
}

func TestReportMarksMountedClaimsAndLeavesPendingOnesUnbilled(t *testing.T) {
	claims := snapshot()
	claims.Provider = nil
	claims.Resources = append(claims.Resources,
		clustermode.Resource{UID: "pvc2", Kind: "PersistentVolumeClaim", Name: "unused", Namespace: "prod", Status: "Bound",
			Requests: clustermode.ResourceValues{StorageBytes: 10 * bytesPerGiB}},
		clustermode.Resource{UID: "pvc3", Kind: "PersistentVolumeClaim", Name: "waiting", Namespace: "prod", Status: "Pending",
			Requests: clustermode.ResourceValues{StorageBytes: 10 * bytesPerGiB}},
	)
	claims.Resources[0].Status = "Bound"
	claims.Pods = []clustermode.Pod{{Name: "api-1", Namespace: "prod", NodeName: "node-1", OwnerKind: "Deployment", OwnerName: "api",
		Requests: clustermode.ResourceValues{CPUMilli: 500, MemoryBytes: bytesPerGiB}, Claims: []string{"data"}}}

	report, err := New(awsResolver(t)).Report(claims, Options{Provider: "aws", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	volumes := map[string]costmodel.LineItem{}
	for _, item := range report.Items {
		if item.Subject.Kind == costmodel.SubjectVolume {
			volumes[item.Subject.Name] = item
		}
	}
	if volumes["data"].Detail["mounted"] != true || volumes["unused"].Detail["mounted"] != false {
		t.Errorf("mounted = %v / %v, want data mounted and unused not", volumes["data"].Detail["mounted"], volumes["unused"].Detail["mounted"])
	}
	if pending := volumes["waiting"]; pending.HourlyUSD != 0 || !pending.Priced() {
		t.Errorf("pending claim = %v (%s), want priced at zero", pending.HourlyUSD, pending.Confidence)
	}
}

// aws-node and kube-proxy request CPU but no memory. They are priced on what
// they declare; only a workload that declares nothing has an unknown cost.
func TestReportPricesPartialRequestsAndLeavesOnlyEmptyOnesUnknown(t *testing.T) {
	partial := snapshot()
	partial.Provider = nil
	partial.Workloads = append(partial.Workloads,
		clustermode.Workload{UID: "kp", Kind: "DaemonSet", Name: "kube-proxy", Namespace: "kube-system", DesiredReplicas: 2,
			Containers:      []clustermode.Container{{Name: "kube-proxy", Requests: clustermode.ResourceValues{CPUMilli: 100}}},
			Requests:        clustermode.ResourceValues{CPUMilli: 200},
			MissingRequests: true, MissingResources: []string{"memory"}},
		clustermode.Workload{UID: "lg", Kind: "Deployment", Name: "legacy", Namespace: "prod", DesiredReplicas: 1,
			Containers:      []clustermode.Container{{Name: "legacy"}},
			MissingRequests: true, MissingResources: []string{"cpu", "memory"}},
	)

	report, err := New(awsResolver(t)).Report(partial, Options{Provider: "aws", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	items := map[string]costmodel.LineItem{}
	for _, item := range report.Items {
		items[item.Subject.Name] = item
	}
	if proxy := items["kube-proxy"]; !proxy.Priced() || proxy.HourlyUSD <= 0 {
		t.Errorf("kube-proxy = %v (%s), want priced on its CPU request", proxy.HourlyUSD, proxy.Confidence)
	}
	if missing, _ := items["kube-proxy"].Detail["missingResources"].([]string); len(missing) != 1 || missing[0] != "memory" {
		t.Errorf("kube-proxy missingResources = %v, want [memory]", items["kube-proxy"].Detail["missingResources"])
	}
	if items["legacy"].Priced() {
		t.Error("legacy priced, want an unknown cost when nothing is requested")
	}
}
