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
