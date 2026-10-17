package cluster

import (
	"context"
	"testing"
	"time"
)

type stubCollector struct {
	data CollectedData
}

func (collector stubCollector) Collect(context.Context) (CollectedData, error) {
	return collector.data, nil
}

func TestSnapshotAggregatesCollectedData(t *testing.T) {
	service := New(stubCollector{data: CollectedData{
		Cluster: ClusterInfo{Context: "kind-budget", Version: "v1.31.0"},
		Nodes: []Node{
			{Name: "worker-1", Ready: true, Allocatable: ResourceValues{CPUMilli: 2000, MemoryBytes: 4 << 30}},
			{Name: "worker-2", Ready: false, Allocatable: ResourceValues{CPUMilli: 1000, MemoryBytes: 2 << 30}},
		},
		Workloads: []Workload{
			{Name: "api", Namespace: "production", Requests: ResourceValues{CPUMilli: 1000, MemoryBytes: 1 << 30}},
			{Name: "worker", Namespace: "production", MissingRequests: true, Requests: ResourceValues{CPUMilli: 250}},
			{Name: "metrics", Namespace: "monitoring", Requests: ResourceValues{CPUMilli: 100}},
		},
		Resources:          []Resource{{Kind: "Service", Name: "api", Namespace: "production"}},
		NamespacePodCounts: map[string]int{"production": 4, "monitoring": 1, "kube-system": 8},
	}})
	service.now = func() time.Time { return time.UnixMilli(1234) }

	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.CollectedAt != 1234 || snapshot.SchemaVersion != SchemaVersion {
		t.Errorf("Snapshot() metadata = %d/%d, want 1234/%d", snapshot.CollectedAt, snapshot.SchemaVersion, SchemaVersion)
	}
	if snapshot.Summary.NodeCount != 2 || snapshot.Summary.ReadyNodeCount != 1 {
		t.Errorf("Snapshot() node summary = %#v", snapshot.Summary)
	}
	if snapshot.Summary.WorkloadCount != 3 || snapshot.Summary.NamespaceCount != 3 {
		t.Errorf("Snapshot() inventory summary = %#v", snapshot.Summary)
	}
	if snapshot.Summary.ResourceCount != 1 || len(snapshot.Resources) != 1 {
		t.Errorf("Snapshot() resource inventory = %#v", snapshot.Resources)
	}
	if snapshot.Summary.Requests.CPUMilli != 1350 || snapshot.Summary.Allocatable.CPUMilli != 3000 {
		t.Errorf("Snapshot() resources = requested %#v, allocatable %#v", snapshot.Summary.Requests, snapshot.Summary.Allocatable)
	}
	if snapshot.Summary.MissingRequestWorkloads != 1 {
		t.Errorf("Snapshot() missing requests = %d, want 1", snapshot.Summary.MissingRequestWorkloads)
	}
	if len(snapshot.Namespaces) != 3 || snapshot.Namespaces[0].Name != "kube-system" || snapshot.Namespaces[2].Name != "production" {
		t.Errorf("Snapshot() namespaces = %#v, want sorted namespace summaries", snapshot.Namespaces)
	}
	if snapshot.Namespaces[2].PodCount != 4 || snapshot.Namespaces[2].WorkloadCount != 2 {
		t.Errorf("Snapshot() production namespace = %#v", snapshot.Namespaces[2])
	}
}
