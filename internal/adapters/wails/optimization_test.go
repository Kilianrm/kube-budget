package wails

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/optimize"
	clustermode "kube-budget/internal/application/cluster"
	"kube-budget/internal/application/costexplorer"
	"kube-budget/internal/providers"
	"kube-budget/internal/storage/recommendations"
)

func TestAReportThatNoLongerDetectsAnAppliedChangeConfirmsIt(t *testing.T) {
	store := recommendations.New(filepath.Join(t.TempDir(), "recommendations.json"))
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	// clicked twice: recorded once
	for range 2 {
		if err := store.MarkApplied("demo", recommendations.Applied{ID: "orphan-volumes", Title: "Delete volumes", At: at, BaselineHourly: 1, ExpectedHourly: 0.2}); err != nil {
			t.Fatal(err)
		}
	}
	// an empty cluster: nothing to recommend, so the change took effect
	report := costmodel.NewReport(at, costmodel.Scope{}, nil, nil)

	result, err := optimization(store, "demo", report, optimize.Inputs{})
	if err != nil {
		t.Fatalf("optimization() error = %v", err)
	}
	if len(result.Applied) != 1 || result.Applied[0].Pending || result.Applied[0].ConfirmedAt == nil {
		t.Fatalf("applied = %+v, want one confirmed entry", result.Applied)
	}
	if len(result.History) == 0 || result.History[0].Action != recommendations.ActionConfirmed {
		t.Errorf("history = %+v, want the confirmation logged first", result.History)
	}
}

func TestAFixMadeOutsideKubeBudgetIsLoggedAndCanBeClaimed(t *testing.T) {
	store := recommendations.New(filepath.Join(t.TempDir(), "recommendations.json"))
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	// the previous report detected an orphan volume
	if _, err := store.Confirm("demo", map[string]recommendations.Seen{"orphan-volumes": {Title: "Delete 1 volume nothing uses", SavingsHourly: 0.005, BilledHourly: 1}}, at, at); err != nil {
		t.Fatal(err)
	}
	// the volume was deleted by hand: two reports without it
	_, _ = optimization(store, "demo", costmodel.NewReport(at.Add(time.Hour), costmodel.Scope{}, nil, nil), optimize.Inputs{})
	report := costmodel.NewReport(at.Add(2*time.Hour), costmodel.Scope{}, nil, nil)
	result, err := optimization(store, "demo", report, optimize.Inputs{})
	if err != nil {
		t.Fatalf("optimization() error = %v", err)
	}
	if len(result.Applied) != 0 || len(result.Claimable) != 1 || result.History[0].Action != recommendations.ActionGone {
		t.Fatalf("applied = %+v, claimable = %v, history = %+v; want it logged and claimable, not applied", result.Applied, result.Claimable, result.History)
	}

	if err := store.Claim("demo", "orphan-volumes", at.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	result, _ = optimization(store, "demo", report, optimize.Inputs{})
	if len(result.Applied) != 1 || !result.Applied[0].Unmarked || result.Applied[0].Expected.Hourly != 0.005 || len(result.Claimable) != 0 {
		t.Errorf("applied = %+v, claimable = %v; want one claimed entry", result.Applied, result.Claimable)
	}
}

func TestOnlySeparateItemsAreTrackedOneByOne(t *testing.T) {
	item := func(name string) optimize.Item {
		return optimize.Item{Subject: costmodel.Subject{Kind: costmodel.SubjectVolume, Name: name, Namespace: "shop"}, Savings: costmodel.Project(0.01)}
	}
	volumes := optimize.Recommendation{ID: "orphan-volumes", Title: "Delete volumes", SeparateItems: true, Items: []optimize.Item{item("a"), item("b")}}
	nodes := optimize.Recommendation{ID: "consolidate-nodes", Title: "Run on 1 fewer node", Items: []optimize.Item{item("worker nodes")}}

	seen := seenFrom(volumes, 1)
	if len(seen.Items) != 2 || seen.Items[0].Key != "volume/shop/a" || seen.Items[0].SavingsHourly != 0.01 || len(seen.Details) == 0 {
		t.Errorf("seen = %+v, want both volumes keyed and the details kept", seen)
	}
	done := doneResources([]recommendations.DoneItem{{SeenItem: seen.Items[0], At: time.Now()}})
	if done[0].Item == nil || done[0].Item.Subject.Name != "a" {
		t.Errorf("done = %+v, want the item kept with its details", done)
	}
	if seen := seenFrom(nodes, 1); len(seen.Items) != 0 {
		t.Errorf("seen items = %+v, want none for a single group-level change", seen.Items)
	}
}

// The showcase relies on the node-type step proposing c6i.large for
// memory-rich m6i.large nodes under CPU-heavy requests. The spot group's
// fallback types sit in the same catalog and must never take its place.
func TestTheAWSCatalogStillProposesC6iForCPUHeavyM6iNodes(t *testing.T) {
	values := func(cpu int64, memoryGiB int64) clustermode.ResourceValues {
		return clustermode.ResourceValues{CPUMilli: cpu, MemoryBytes: memoryGiB * gibibyte}
	}
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

	plan := optimize.Build(report, planInputs(provider, "us-east-1"))
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

func TestApplyingFreezesARecommendationUntilTheClusterShowsItDone(t *testing.T) {
	// Never touch the user's real recommendations.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	volume := func(name string) costmodel.LineItem {
		return costmodel.LineItem{
			Subject: costmodel.Subject{Kind: costmodel.SubjectVolume, ID: name, Name: name, Namespace: "shop"},
			Basis:   costmodel.BasisProvisioned, Usage: costmodel.Usage{StorageGB: 50}, HourlyUSD: 0.005,
			Components: map[costmodel.Component]float64{costmodel.ComponentStorage: 0.005},
			Confidence: costmodel.ConfidenceDerived,
			Detail:     map[string]interface{}{"status": "Bound", "mounted": false},
		}
	}
	report := func(names ...string) costmodel.CostReport {
		items := []costmodel.LineItem{}
		for _, name := range names {
			items = append(items, volume(name))
		}
		return costmodel.NewReport(at, costmodel.Scope{}, items, nil)
	}
	adapter := NewClusterAdapter()
	store, err := defaultRecommendationStore()
	if err != nil {
		t.Fatal(err)
	}
	find := func(result OptimizationResult) *optimize.Recommendation {
		for index := range result.Plan.Recommendations {
			if result.Plan.Recommendations[index].ID == "orphan-volumes" {
				return &result.Plan.Recommendations[index]
			}
		}
		return nil
	}

	adapter.rememberCost("demo", report("a", "b"), optimize.Inputs{})
	result, err := adapter.MarkRecommendationApplied(RecommendationRequest{ClusterID: "demo", ID: "orphan-volumes"})
	if err != nil {
		t.Fatalf("MarkRecommendationApplied() error = %v", err)
	}
	if shown := find(result); shown == nil || !shown.Applying || len(shown.Items) != 2 {
		t.Fatalf("shown = %+v, want the two volumes frozen", shown)
	}

	// halfway: still pending, one item done
	result, _ = optimization(store, "demo", report("b"), optimize.Inputs{})
	if shown := find(result); shown == nil || shown.Title != "Delete 2 volumes nothing uses" || !shown.Items[0].Done || result.Applied[0].ConfirmedAt != nil {
		t.Fatalf("shown = %+v, applied = %+v; want the frozen title, a done and the change pending", shown, result.Applied)
	}

	// done: confirmed with both volumes
	result, _ = optimization(store, "demo", report(), optimize.Inputs{})
	if len(result.Applied) != 1 || result.Applied[0].Pending || len(result.Applied[0].Done) != 2 || find(result) != nil {
		t.Errorf("applied = %+v, want one confirmed change carrying both volumes", result.Applied)
	}
}

func TestPausedNodeStepsAreNotRecordedAsResolved(t *testing.T) {
	store := recommendations.New(filepath.Join(t.TempDir(), "recommendations.json"))
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	_, _ = store.Confirm("demo", map[string]recommendations.Seen{"consolidate-nodes": {Title: "Run on 1 fewer node", SavingsHourly: 0.1}}, at, at)

	// a node group is being created: node steps pause, so the plan lists none
	report := costmodel.NewReport(at.Add(time.Hour), costmodel.Scope{}, nil, nil)
	result, err := optimization(store, "demo", report, optimize.Inputs{NodeTransition: "node group general-c is creating"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Applied) != 0 {
		t.Errorf("applied = %+v, want nothing resolved while paused", result.Applied)
	}
}

func TestThePlanSaysWhatChangedSinceTheReportBefore(t *testing.T) {
	store := recommendations.New(filepath.Join(t.TempDir(), "recommendations.json"))
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	previous := map[string]recommendations.Seen{
		"consolidate-nodes": {Title: "Run on 1 fewer node", SavingsHourly: 0.1},
		"orphan-volumes":    {Title: "Delete 1 volume nothing uses", SavingsHourly: 0.005},
	}
	_, _ = store.Confirm("demo", previous, at, at)
	_, _ = store.Confirm("demo", previous, at.Add(time.Hour), at.Add(time.Hour))

	plan := optimize.Plan{Steps: []optimize.Step{
		{ID: "consolidate-nodes", Savings: costmodel.Project(0.2)},
		{ID: "spot-capacity", Savings: costmodel.Project(0.05)},
	}}
	state, _ := store.Get("demo")
	compareWithPrevious(&plan, state)

	if plan.Steps[0].Change != "changed" || plan.Steps[0].Previous.Hourly != 0.1 || plan.Steps[1].Change != "new" {
		t.Errorf("steps = %+v, want consolidation changed from 0.1 and spot new", plan.Steps)
	}
	if len(plan.Gone) != 1 || plan.Gone[0].ID != "orphan-volumes" || plan.Gone[0].Change != "gone" {
		t.Errorf("gone = %+v, want the volume step gone", plan.Gone)
	}
}
