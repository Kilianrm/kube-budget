package optimize

import (
	"fmt"
	"strings"
	"testing"

	"kube-budget/core/costmodel"
)

func unmountedVolume(name string) costmodel.LineItem {
	no := false
	return costmodel.LineItem{
		Subject: costmodel.Subject{Kind: costmodel.SubjectVolume, ID: name, Name: name, Namespace: "prod"},
		Basis:   costmodel.BasisProvisioned, Usage: costmodel.Usage{StorageGB: 100}, HourlyUSD: 0.01,
		Components: map[costmodel.Component]float64{costmodel.ComponentStorage: 0.01},
		Confidence: costmodel.ConfidenceDerived,
		Detail:     map[string]interface{}{"status": "Bound", "mounted": no},
	}
}

func TestARecommendationBeingAppliedStaysFrozenAndTracksItsItems(t *testing.T) {
	before := append(nodes(2, 0.1), unmountedVolume("a"), unmountedVolume("b"))
	frozen := *find(Build(reportOf(before...), Inputs{}), "orphan-volumes")
	inputs := Inputs{Applying: map[string]Recommendation{"orphan-volumes": frozen}}

	// a deleted: the live plan would now say "Delete 1 volume"
	plan := Build(reportOf(append(nodes(2, 0.1), unmountedVolume("b"))...), inputs)
	shown := find(plan, "orphan-volumes")
	if shown == nil || !shown.Applying || shown.Title != frozen.Title || len(shown.Items) != 2 {
		t.Fatalf("shown = %+v, want the frozen recommendation", shown)
	}
	if !shown.Items[0].Done || shown.Items[1].Done || shown.Complete {
		t.Errorf("items = %+v, want a done and b still to do", shown.Items)
	}
	if !nearly(plan.Savings.Hourly, 0.01) {
		t.Errorf("plan savings = %v, want only b's saving left", plan.Savings.Hourly)
	}

	// both gone: complete, and nothing left to save
	plan = Build(reportOf(nodes(2, 0.1)...), inputs)
	if shown := find(plan, "orphan-volumes"); shown == nil || !shown.Complete || plan.Savings.Hourly != 0 {
		t.Errorf("shown = %+v, savings = %v; want it complete", shown, plan.Savings.Hourly)
	}
}

func TestANodeChangeBeingAppliedPausesTheOtherNodeSteps(t *testing.T) {
	// 4 nodes for 2 cores: consolidation and rightsizing both apply
	items := append(nodes(4, 0.1), workload("api", "Deployment", 2, 4, &costmodel.Usage{CPUCores: 0.2, MemoryGB: 3.5}))
	frozen := *find(Build(reportOf(items...), Inputs{}), "consolidate-nodes")

	plan := Build(reportOf(items...), Inputs{Applying: map[string]Recommendation{"consolidate-nodes": frozen}})
	if !strings.Contains(plan.Paused, frozen.Title) {
		t.Errorf("paused = %q, want the change being applied named", plan.Paused)
	}
	shown := find(plan, "consolidate-nodes")
	if shown == nil || !shown.Applying || len(shown.Milestones) != 1 || shown.Milestones[0].Done {
		t.Fatalf("shown = %+v, want one open milestone", shown)
	}
	rightsize := find(plan, "rightsize-requests")
	if rightsize == nil || rightsize.Savings.Hourly != 0 || !strings.Contains(rightsize.Rationale, "waits for the node change") {
		t.Errorf("rightsize = %+v, want its node saving held back", rightsize)
	}

	// two nodes gone: done
	after := append(nodes(2, 0.1), workload("api", "Deployment", 2, 4, &costmodel.Usage{CPUCores: 0.2, MemoryGB: 3.5}))
	if shown := find(Build(reportOf(after...), Inputs{Applying: map[string]Recommendation{"consolidate-nodes": frozen}}), "consolidate-nodes"); !shown.Complete {
		t.Errorf("shown = %+v, want it complete on 2 nodes", shown)
	}
}

func TestANodeGroupChangingInTheClusterPausesNodeSteps(t *testing.T) {
	items := append(nodes(4, 0.1), workload("api", "Deployment", 2, 4, nil))
	plan := Build(reportOf(items...), Inputs{NodeTransition: "node group general-c is CREATING"})
	if plan.Paused == "" || find(plan, "consolidate-nodes") != nil {
		t.Errorf("paused = %q, consolidate = %+v; want node steps held", plan.Paused, find(plan, "consolidate-nodes"))
	}
}

func TestAMachineTypeChangeIsCheckedGroupByGroup(t *testing.T) {
	frozen := Recommendation{ID: "node-type", Category: CategoryNodes, Title: "Switch to c nodes",
		Target: &Target{Group: "general", NewGroup: "general-c", Nodes: 2, FromMachineType: "m", MachineType: "c"}}
	eks := func(groups ...EKSNodeGroup) *EKSCluster {
		return &EKSCluster{Name: "shop", Region: "us-east-1", NodeGroups: groups}
	}
	newNodes := func() []costmodel.LineItem {
		items := nodes(2, 0.085)
		for index := range items {
			items[index].Subject.Name += "-new"
			items[index].Subject.ParentID = "general-c"
			items[index].Detail["machineType"] = "c"
		}
		return items
	}
	check := func(items []costmodel.LineItem, cluster *EKSCluster) []bool {
		shown := find(Build(reportOf(items...), Inputs{EKS: cluster, Applying: map[string]Recommendation{"node-type": frozen}}), "node-type")
		done := []bool{}
		for _, milestone := range shown.Milestones {
			done = append(done, milestone.Done)
		}
		return done
	}

	// the new group is still being created
	got := check(nodes(2, 0.096), eks(EKSNodeGroup{Name: "general", Status: "ACTIVE"}, EKSNodeGroup{Name: "general-c", Status: "CREATING"}))
	if len(got) != 3 || got[0] || got[1] || got[2] {
		t.Errorf("creating = %v, want nothing done", got)
	}
	// active, and the old nodes cordoned
	old := nodes(2, 0.096)
	for index := range old {
		old[index].Detail["schedulable"] = false
	}
	got = check(append(old, newNodes()...), eks(EKSNodeGroup{Name: "general", Status: "ACTIVE"}, EKSNodeGroup{Name: "general-c", Status: "ACTIVE"}))
	if !got[0] || !got[1] || got[2] {
		t.Errorf("cordoned = %v, want active and cordoned, not deleted", got)
	}
	// the old group deleted
	got = check(newNodes(), eks(EKSNodeGroup{Name: "general-c", Status: "ACTIVE"}))
	if !got[0] || !got[1] || !got[2] {
		t.Errorf("deleted = %v, want every milestone", got)
	}
}

func TestSpotOnEKSMovesTheStatelessShareToANewSpotGroup(t *testing.T) {
	// 75% of the requests are stateless: 3 of 4 nodes move to spot
	items := append(nodes(4, 0.1),
		workload("api", "Deployment", 1.5, 1.5, nil),
		workload("db", "StatefulSet", 0.5, 0.5, nil),
		workload("agent", "DaemonSet", 1, 1, nil),
	)
	general := EKSNodeGroup{Name: "general", Status: "ACTIVE", Desired: 4, Min: 2, Max: 4, InstanceTypes: []string{"m"},
		AmiType: "AL2023_x86_64_STANDARD", NodeRole: "arn:aws:iam::1:role/nodes", Subnets: []string{"subnet-a"},
		Labels: map[string]string{"workload-tier": "general"}}
	inputs := Inputs{
		SpotPriceRatio: 0.35,
		MachineTypes: []MachineType{
			{Name: "m", VCPU: 2, MemoryGB: 8, HourlyUSD: 0.096},
			{Name: "m7", VCPU: 2, MemoryGB: 8, HourlyUSD: 0.1},
			{Name: "m5", VCPU: 2, MemoryGB: 8, HourlyUSD: 0.096},
			{Name: "c", VCPU: 2, MemoryGB: 4, HourlyUSD: 0.085},
		},
		EKS:       &EKSCluster{Name: "shop", Region: "us-east-1", NodeGroups: []EKSNodeGroup{general}},
		Dismissed: map[string]bool{"consolidate-nodes": true, "node-type": true},
	}

	spot := find(Build(reportOf(items...), inputs), "spot-capacity")
	if spot == nil || len(spot.Items) != 1 {
		t.Fatalf("spot = %+v, want one item for the general group", spot)
	}
	item := spot.Items[0]
	want := []string{
		"aws eks create-nodegroup --cluster-name shop --nodegroup-name general-spot --capacity-type SPOT --instance-types m m5 m7" +
			" --scaling-config minSize=0,maxSize=6,desiredSize=3 --subnets subnet-a --node-role arn:aws:iam::1:role/nodes" +
			" --ami-type AL2023_x86_64_STANDARD --labels workload-tier=general --region us-east-1",
		"aws eks wait nodegroup-active --cluster-name shop --nodegroup-name general-spot --region us-east-1",
		`kubectl patch deployment api -n prod --type merge -p '{"spec":{"template":{"spec":{"affinity":{"nodeAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"weight":100,"preference":{"matchExpressions":[{"key":"eks.amazonaws.com/capacityType","operator":"In","values":["SPOT"]}]}}]}}}}}}'`,
		`kubectl patch statefulset db -n prod --type merge -p '{"spec":{"template":{"spec":{"affinity":{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"eks.amazonaws.com/capacityType","operator":"NotIn","values":["SPOT"]}]}]}}}}}}}'`,
		"aws eks update-nodegroup-config --cluster-name shop --nodegroup-name general --scaling-config minSize=1,desiredSize=1 --region us-east-1",
	}
	if item.Change != "4 on-demand → 1 on-demand + 3 spot" || strings.Join(item.Steps, "\n") != strings.Join(want, "\n") {
		t.Errorf("change = %q, steps =\n%s\nwant\n%s", item.Change, strings.Join(item.Steps, "\n"), strings.Join(want, "\n"))
	}
	if spot.Target == nil || spot.Target.NewGroup != "general-spot" || spot.Target.Nodes != 1 {
		t.Errorf("target = %+v, want general-spot with 1 on-demand node left", spot.Target)
	}

	// a group eksctl created: eksctl commands, so the new group owns its stack
	inputs.EKS.NodeGroups[0].Labels["alpha.eksctl.io/nodegroup-name"] = "general"
	item = find(Build(reportOf(items...), inputs), "spot-capacity").Items[0]
	if item.Steps[0] != "eksctl create nodegroup --cluster shop --region us-east-1 --name general-spot --managed --spot --instance-types m,m5,m7 --nodes 3 --nodes-min 0 --nodes-max 6 --node-ami-family AmazonLinux2023 --node-labels workload-tier=general" ||
		item.Steps[len(item.Steps)-1] != "eksctl scale nodegroup --cluster shop --region us-east-1 --name general --nodes 1 --nodes-min 1" {
		t.Errorf("eksctl steps = %q", item.Steps)
	}

	// eksctl cannot copy taints: no commands rather than a group without them
	inputs.EKS.NodeGroups[0].Taints = []string{"key=dedicated,value=web,effect=NO_SCHEDULE"}
	if item := find(Build(reportOf(items...), inputs), "spot-capacity").Items[0]; len(item.Steps) != 0 {
		t.Errorf("steps = %q, want none for a tainted eksctl group", item.Steps)
	}
}

func TestSpotBeingAppliedChecksTheNewGroupAndTheShrink(t *testing.T) {
	frozen := Recommendation{ID: "spot-capacity", Category: CategoryNodes, Title: "Run stateless workloads on spot capacity",
		Target: &Target{Group: "general", NewGroup: "general-spot", Nodes: 1, SpotShare: 0.75, SpotNodes: 3}}
	spotNodes := func(count int) []costmodel.LineItem {
		items := make([]costmodel.LineItem, 0, count)
		for index := 0; index < count; index++ {
			item := node(fmt.Sprintf("s%d", index), 0.035, "spot")
			item.Subject.ParentID = "general-spot"
			items = append(items, item)
		}
		return items
	}
	check := func(items []costmodel.LineItem, groups ...EKSNodeGroup) []bool {
		shown := find(Build(reportOf(items...), Inputs{EKS: &EKSCluster{Name: "shop", Region: "us-east-1", NodeGroups: groups},
			Applying: map[string]Recommendation{"spot-capacity": frozen}}), "spot-capacity")
		done := []bool{}
		for _, milestone := range shown.Milestones {
			done = append(done, milestone.Done)
		}
		return done
	}

	// spot group up with its nodes, on-demand not shrunk yet
	got := check(append(nodes(4, 0.1), spotNodes(3)...), EKSNodeGroup{Name: "general", Desired: 4}, EKSNodeGroup{Name: "general-spot", Status: "ACTIVE", Desired: 3})
	if len(got) != 3 || !got[0] || !got[1] || got[2] {
		t.Errorf("halfway = %v, want the spot group done and general not shrunk", got)
	}
	// active, but only one of its three nodes up
	got = check(append(nodes(4, 0.1), spotNodes(1)...), EKSNodeGroup{Name: "general", Desired: 4}, EKSNodeGroup{Name: "general-spot", Status: "ACTIVE", Desired: 3})
	if !got[0] || got[1] {
		t.Errorf("starting = %v, want active with its nodes still coming", got)
	}
	// shrunk: 3 of 4 nodes on spot
	got = check(append(nodes(1, 0.1), spotNodes(3)...), EKSNodeGroup{Name: "general", Desired: 1}, EKSNodeGroup{Name: "general-spot", Status: "ACTIVE", Desired: 3})
	if !got[0] || !got[1] || !got[2] {
		t.Errorf("done = %v, want every milestone", got)
	}
}

func TestAPausedStepKeepsItsLastValueWithoutCountingIt(t *testing.T) {
	items := append(nodes(4, 0.1), workload("api", "Deployment", 2, 4, nil))
	lastKnown := *find(Build(reportOf(items...), Inputs{}), "consolidate-nodes")

	plan := Build(reportOf(items...), Inputs{NodeTransition: "node group general-c is creating", LastKnown: map[string]Recommendation{"consolidate-nodes": lastKnown}})
	shown := find(plan, "consolidate-nodes")
	if shown == nil || !shown.Paused {
		t.Fatalf("shown = %+v, want the last known recommendation, paused", shown)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Status != "paused" || plan.Steps[0].Savings != lastKnown.Savings || plan.Savings.Hourly != 0 {
		t.Errorf("steps = %+v, savings = %v; want a paused step not counted", plan.Steps, plan.Savings)
	}
}

func TestAStepBeingAppliedSplitsWhatIsDoneFromWhatIsLeft(t *testing.T) {
	frozen := *find(Build(reportOf(append(nodes(2, 0.1), unmountedVolume("a"), unmountedVolume("b"))...), Inputs{}), "orphan-volumes")
	plan := Build(reportOf(append(nodes(2, 0.1), unmountedVolume("b"))...), Inputs{Applying: map[string]Recommendation{"orphan-volumes": frozen}})

	if len(plan.Steps) != 1 || plan.Steps[0].Status != "applying" || !nearly(plan.Steps[0].Savings.Hourly, 0.01) || !nearly(plan.Steps[0].Achieved.Hourly, 0.01) {
		t.Errorf("steps = %+v, want 0.01 done and 0.01 left", plan.Steps)
	}
}

func TestANewGroupBesideTheOneItReplacesIsATemporaryCost(t *testing.T) {
	frozen := Recommendation{ID: "node-type", Category: CategoryNodes, Title: "Switch to c nodes",
		Target: &Target{Group: "general", NewGroup: "general-c", Nodes: 2, FromMachineType: "m", MachineType: "c"}}
	items := nodes(2, 0.096)
	for _, name := range []string{"x", "y"} {
		item := node(name, 0.085, "on-demand")
		item.Subject.ParentID = "general-c"
		items = append(items, item)
	}

	plan := Build(reportOf(items...), Inputs{Applying: map[string]Recommendation{"node-type": frozen}})
	if len(plan.Temporary) != 1 || !nearly(plan.Temporary[0].Cost.Hourly, 0.17) || !strings.Contains(plan.Temporary[0].Label, "general-c running beside general") {
		t.Errorf("temporary = %+v, want the two new nodes while general still runs", plan.Temporary)
	}
	// once general is gone, nothing is temporary
	if plan := Build(reportOf(items[2:]...), Inputs{Applying: map[string]Recommendation{"node-type": frozen}}); len(plan.Temporary) != 0 {
		t.Errorf("temporary = %+v, want none after the switch", plan.Temporary)
	}
}

// A plan share of 95% is a fraction no whole node count reaches once one
// on-demand node stays for the StatefulSets: the change is done when the
// commands' end state is there.
func TestSpotIsDoneWhenTheCommandsEndStateIsReached(t *testing.T) {
	frozen := Recommendation{ID: "spot-capacity", Category: CategoryNodes, Title: "Run stateless workloads on spot capacity",
		Target: &Target{Group: "general", NewGroup: "general-spot", Nodes: 1, SpotShare: 0.95}}
	spot := node("s", 0.035, "spot")
	spot.Subject.ParentID = "general-spot"
	batch := node("b", 0.035, "spot")
	batch.Subject.ParentID = "batch-spot"
	items := append(nodes(1, 0.1), spot, batch)

	shown := find(Build(reportOf(items...), Inputs{
		EKS:      &EKSCluster{Name: "shop", Region: "us-east-1", NodeGroups: []EKSNodeGroup{{Name: "general", Desired: 1}, {Name: "general-spot", Status: "ACTIVE", Desired: 1}}},
		Applying: map[string]Recommendation{"spot-capacity": frozen},
	}), "spot-capacity")
	if !shown.Complete {
		t.Errorf("milestones = %+v, want the change complete", shown.Milestones)
	}

	// without commands, the share check also keeps one node on demand
	frozen.Target = &Target{SpotShare: 0.95}
	shown = find(Build(reportOf(items...), Inputs{Applying: map[string]Recommendation{"spot-capacity": frozen}}), "spot-capacity")
	if !shown.Complete || shown.Milestones[0].Label != "Spot runs 2 nodes of 3" {
		t.Errorf("milestones = %+v, want 2 of 3 nodes on spot to be enough", shown.Milestones)
	}
}
