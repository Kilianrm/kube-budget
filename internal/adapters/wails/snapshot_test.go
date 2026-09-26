package wails

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kube-budget/core/costmodel"
	clustermode "kube-budget/internal/application/cluster"
)

func TestConcurrentSnapshotRequestsShareOneCollection(t *testing.T) {
	adapter := NewClusterAdapter()
	var collections atomic.Int32
	release := make(chan struct{})
	collect := func(ClusterSnapshotRequest) (clustermode.Snapshot, error) {
		collections.Add(1)
		<-release
		return clustermode.Snapshot{CollectedAt: 1}, nil
	}
	request := ClusterSnapshotRequest{Context: "prod"}

	var group sync.WaitGroup
	for range 3 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := adapter.sharedSnapshot(request, collect, time.Now); err != nil {
				t.Error(err)
			}
		}()
	}
	// let every caller join before the collection finishes
	time.Sleep(20 * time.Millisecond)
	close(release)
	group.Wait()

	if got := collections.Load(); got != 1 {
		t.Errorf("collections = %d, want the Cluster section and the cost report to share one", got)
	}
}

func TestASnapshotIsReusedOnlyWhileFresh(t *testing.T) {
	adapter := NewClusterAdapter()
	collections := 0
	collect := func(ClusterSnapshotRequest) (clustermode.Snapshot, error) {
		collections++
		return clustermode.Snapshot{}, nil
	}
	clock := time.Unix(0, 0)
	now := func() time.Time { return clock }
	request := ClusterSnapshotRequest{Context: "prod"}

	_, _ = adapter.sharedSnapshot(request, collect, now)
	clock = clock.Add(snapshotReuseWindow / 2)
	_, _ = adapter.sharedSnapshot(request, collect, now)
	if collections != 1 {
		t.Fatalf("collections = %d, want a fresh snapshot reused", collections)
	}
	clock = clock.Add(snapshotReuseWindow)
	_, _ = adapter.sharedSnapshot(request, collect, now)
	if collections != 2 {
		t.Errorf("collections = %d, want a stale snapshot collected again", collections)
	}
	_, _ = adapter.sharedSnapshot(ClusterSnapshotRequest{Context: "staging"}, collect, now)
	if collections != 3 {
		t.Errorf("collections = %d, want another cluster collected on its own", collections)
	}
}

func TestAFailedSnapshotIsNotReused(t *testing.T) {
	adapter := NewClusterAdapter()
	collections := 0
	collect := func(ClusterSnapshotRequest) (clustermode.Snapshot, error) {
		collections++
		return clustermode.Snapshot{}, errors.New("unreachable")
	}
	request := ClusterSnapshotRequest{Context: "prod"}

	_, _ = adapter.sharedSnapshot(request, collect, time.Now)
	if _, err := adapter.sharedSnapshot(request, collect, time.Now); err == nil || collections != 2 {
		t.Errorf("collections = %d, err = %v; want the failure retried", collections, err)
	}
}

func TestFrequentReportsAreCapturedOnlyWhenTheyAddSomething(t *testing.T) {
	adapter := NewClusterAdapter()
	start := time.Unix(0, 0)
	report := func(hourly float64) costmodel.CostReport {
		return costmodel.CostReport{Totals: map[costmodel.Basis]costmodel.Projection{costmodel.BasisProvisioned: costmodel.Project(hourly)}}
	}

	if !adapter.captureDue("prod", report(1), start) {
		t.Fatal("the first report was not captured")
	}
	if adapter.captureDue("prod", report(1), start.Add(30*time.Second)) {
		t.Error("an unchanged rate 30s later was captured again")
	}
	if !adapter.captureDue("prod", report(1.5), start.Add(time.Minute)) {
		t.Error("a changed rate was not captured")
	}
	if !adapter.captureDue("prod", report(1.5), start.Add(time.Minute+historyCaptureSpacing)) {
		t.Error("an unchanged rate was not captured after the spacing")
	}
	if !adapter.captureDue("staging", report(1), start.Add(time.Minute)) {
		t.Error("another cluster's first report was not captured")
	}
}
