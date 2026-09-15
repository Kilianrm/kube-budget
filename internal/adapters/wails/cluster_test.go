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
