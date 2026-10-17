package wails

import (
	"strings"
	"testing"
)

func TestGetClusterSnapshotRequiresContext(t *testing.T) {
	_, err := NewClusterAdapter().GetClusterSnapshot(ClusterSnapshotRequest{})
	if err == nil || !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("GetClusterSnapshot() error = %v, want context required error", err)
	}
}

func TestEKSDiscoveryRequiresRegion(t *testing.T) {
	_, err := NewClusterAdapter().ListEKSClusters(EKSClustersRequest{})
	if err == nil || !strings.Contains(err.Error(), "AWS region is required") {
		t.Fatalf("ListEKSClusters() error = %v, want region required error", err)
	}
}
