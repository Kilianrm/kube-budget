package optimize

import (
	"math"
	"strings"
	"testing"
	"time"

	"kube-budget/core/costmodel"
)

func nearly(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// node is a priced node of machine type "m" (2 vCPU, 8 GB) unless changed.
func node(name string, hourly float64, purchase string) costmodel.LineItem {
	return costmodel.LineItem{
		Subject:    costmodel.Subject{Kind: costmodel.SubjectNode, ID: name, Name: name, ParentID: "general"},
		Basis:      costmodel.BasisProvisioned,
		Usage:      costmodel.Usage{CPUCores: 2, MemoryGB: 8},
		HourlyUSD:  hourly,
		Components: map[costmodel.Component]float64{costmodel.ComponentCPU: hourly / 2, costmodel.ComponentMemory: hourly / 2},
		Confidence: costmodel.ConfidenceExact,
		Detail:     map[string]interface{}{"machineType": "m", "purchase": purchase, "ready": true, "schedulable": true},
	}
}

func nodes(count int, hourly float64) []costmodel.LineItem {
	items := make([]costmodel.LineItem, 0, count)
	for index := 0; index < count; index++ {
		items = append(items, node(string(rune('a'+index)), hourly, "on-demand"))
	}
	return items
}

// workload requests cpu cores and memory GB priced at 0.01 per core and per GB.
func workload(name, kind string, cpu, memory float64, used *costmodel.Usage) costmodel.LineItem {
	return costmodel.LineItem{
		Subject:        costmodel.Subject{Kind: costmodel.SubjectWorkload, ID: name, Name: name, Namespace: "prod"},
		Basis:          costmodel.BasisRequested,
		Usage:          costmodel.Usage{CPUCores: cpu, MemoryGB: memory},
		HourlyUSD:      cpu*0.01 + memory*0.01,
		Components:     map[costmodel.Component]float64{costmodel.ComponentCPU: cpu * 0.01, costmodel.ComponentMemory: memory * 0.01},
		Confidence:     costmodel.ConfidenceDerived,
		Used:           used,
		UsedComponents: map[costmodel.Component]float64{},
		Detail:         map[string]interface{}{"kind": kind, "containers": 1, "replicas": 1.0},
	}
}

func reportOf(items ...costmodel.LineItem) costmodel.CostReport {
	return costmodel.NewReport(time.Unix(0, 0), costmodel.Scope{}, items, nil)
}

func find(plan Plan, id string) *Recommendation {
	for _, list := range [][]Recommendation{plan.Recommendations, plan.DataIssues} {
		for index := range list {
			if list[index].ID == id {
				return &list[index]
			}
		}
	}
	return nil
}

// Consolidation and rightsizing both want to remove nodes. The plan must
// count each node once: rightsizing only claims nodes consolidation left.
func TestPlanNeverCountsTheSameNodeTwice(t *testing.T) {
	items := append(nodes(4, 0.1), workload("api", "Deployment", 2, 4, &costmodel.Usage{CPUCores: 0.2, MemoryGB: 3.5}))

	plan := Build(reportOf(items...), Inputs{})

	consolidate := find(plan, "consolidate-nodes")
	if consolidate == nil || !nearly(consolidate.Savings.Hourly, 0.2) {
		t.Fatalf("consolidate = %+v, want 2 of 4 nodes removed (0.2/h)", consolidate)
	}
	rightsize := find(plan, "rightsize-requests")
	if rightsize == nil {
		t.Fatal("rightsize-requests missing")
	}
	// the two remaining nodes are the high-availability floor, so nothing more is billed less
	if rightsize.Savings.Hourly != 0 || rightsize.FreedRequests.Hourly <= 0 {
		t.Errorf("rightsize = %v billed / %v freed, want 0 billed and some requests freed", rightsize.Savings.Hourly, rightsize.FreedRequests.Hourly)
	}
	if !nearly(plan.Savings.Hourly, 0.2) || !nearly(plan.Optimized.Hourly, plan.Current.Hourly-0.2) {
		t.Errorf("plan = %v saved, %v optimized, want 0.2 saved from %v", plan.Savings.Hourly, plan.Optimized.Hourly, plan.Current.Hourly)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].ID != "consolidate-nodes" {
		t.Errorf("Steps = %+v, want only consolidation in the waterfall", plan.Steps)
	}
}

func TestRightsizingClaimsTheNodesItFrees(t *testing.T) {
	// 6 nodes; requests of 5 cores need 3 of them, usage of 1 core needs the HA floor of 2
	items := append(nodes(6, 0.1), workload("api", "Deployment", 5, 4, &costmodel.Usage{CPUCores: 1, MemoryGB: 3}))

	plan := Build(reportOf(items...), Inputs{})

	if got := find(plan, "consolidate-nodes").Savings.Hourly; !nearly(got, 0.3) {
		t.Errorf("consolidation = %v, want 3 nodes (0.3/h)", got)
	}
	rightsize := find(plan, "rightsize-requests")
	if !nearly(rightsize.Savings.Hourly, 0.1) {
		t.Errorf("rightsizing = %v, want the one extra node (0.1/h)", rightsize.Savings.Hourly)
	}
	if !strings.Contains(rightsize.Items[0].Command, "kubectl set resources deployment/api -n prod --requests=cpu=1500m") {
		t.Errorf("command = %q, want the new cpu request", rightsize.Items[0].Command)
	}
	if strings.Contains(rightsize.Items[0].Command, "memory") {
		t.Errorf("command = %q, memory is used at 75%% and must stay", rightsize.Items[0].Command)
	}
}

func TestDismissedStepsLeaveTheScenario(t *testing.T) {
	items := append(nodes(6, 0.1), workload("api", "Deployment", 5, 4, &costmodel.Usage{CPUCores: 1, MemoryGB: 3}))

	plan := Build(reportOf(items...), Inputs{Dismissed: map[string]bool{"consolidate-nodes": true}})

	if consolidate := find(plan, "consolidate-nodes"); consolidate == nil || !consolidate.Dismissed {
		t.Fatalf("consolidate = %+v, want it listed as dismissed", consolidate)
	}
	// without consolidation, rightsizing is what takes the cluster from 6 to 2 nodes
	if got := find(plan, "rightsize-requests").Savings.Hourly; !nearly(got, 0.4) {
		t.Errorf("rightsizing = %v, want 4 nodes (0.4/h)", got)
	}
	if !nearly(plan.Savings.Hourly, 0.4) {
		t.Errorf("plan savings = %v, want the dismissed step left out", plan.Savings.Hourly)
	}
}

func TestMissingRequestsHoldBackNodeSteps(t *testing.T) {
	legacy := costmodel.LineItem{
		Subject:    costmodel.Subject{Kind: costmodel.SubjectWorkload, Name: "legacy", Namespace: "prod"},
		Basis:      costmodel.BasisRequested,
		Confidence: costmodel.ConfidenceUnknown,
		Detail:     map[string]interface{}{"kind": "Deployment", "missingRequests": true},
	}
	items := append(nodes(6, 0.1), workload("api", "Deployment", 2, 4, &costmodel.Usage{CPUCores: 0.2, MemoryGB: 3}), legacy)

	plan := Build(reportOf(items...), Inputs{SpotPriceRatio: 0.35})

	issue := find(plan, "missing-requests")
	if issue == nil || issue.Category != CategoryData || !strings.Contains(issue.Items[0].Command, "deployment/legacy") {
		t.Fatalf("missing-requests = %+v, want a data issue with a command template", issue)
	}
	for _, id := range []string{"consolidate-nodes", "node-type", "spot-capacity"} {
		if find(plan, id) != nil {
			t.Errorf("%s present, want node steps held back while demand is incomplete", id)
		}
	}
	rightsize := find(plan, "rightsize-requests")
	if rightsize == nil || rightsize.Savings.Hourly != 0 || rightsize.FreedRequests.Hourly <= 0 {
		t.Errorf("rightsize = %+v, want freed requests but no billed saving", rightsize)
	}
}

func TestOrphanVolumesAreBoundClaimsNobodyMounts(t *testing.T) {
	volume := func(name, status string, mounted *bool) costmodel.LineItem {
		item := costmodel.LineItem{
			Subject: costmodel.Subject{Kind: costmodel.SubjectVolume, ID: name, Name: name, Namespace: "prod"},
			Basis:   costmodel.BasisProvisioned, Usage: costmodel.Usage{StorageGB: 100}, HourlyUSD: 0.01,
			Components: map[costmodel.Component]float64{costmodel.ComponentStorage: 0.01},
			Confidence: costmodel.ConfidenceDerived,
			Detail:     map[string]interface{}{"status": status},
		}
		if mounted != nil {
			item.Detail["mounted"] = *mounted
		}
		return item
	}
	yes, no := true, false
	items := append(nodes(2, 0.1),
		volume("unused", "Bound", &no),
		volume("in-use", "Bound", &yes),
		volume("unknown", "Bound", nil),
		volume("lost", "Lost", nil),
	)

	orphans := find(Build(reportOf(items...), Inputs{}), "orphan-volumes")

	if orphans == nil || len(orphans.Items) != 2 || !nearly(orphans.Savings.Hourly, 0.02) {
		t.Fatalf("orphans = %+v, want the unmounted and the lost claim", orphans)
	}
	if orphans.Items[0].Command != "kubectl delete pvc unused -n prod" {
		t.Errorf("command = %q", orphans.Items[0].Command)
	}
	if orphans.Items[0].Check != "kubectl describe pvc unused -n prod" {
		t.Errorf("check = %q", orphans.Items[0].Check)
	}
}

func TestNodeTypeMatchesTheRequestRatioAndSkipsBurstables(t *testing.T) {
	// CPU-heavy demand on memory-heavy machines
	items := append(nodes(4, 0.096), workload("api", "Deployment", 6, 6, nil))
	inputs := Inputs{MachineTypes: []MachineType{
		{Name: "m", VCPU: 2, MemoryGB: 8, HourlyUSD: 0.096},
		{Name: "c", VCPU: 2, MemoryGB: 4, HourlyUSD: 0.085},
		{Name: "t", VCPU: 2, MemoryGB: 4, HourlyUSD: 0.04, Burstable: true},
	}}

	plan := Build(reportOf(items...), inputs)

	nodeType := find(plan, "node-type")
	if nodeType == nil || nodeType.Title != "Switch to c nodes" {
		t.Fatalf("node-type = %+v, want c and never the burstable t", nodeType)
	}
	// 6 cores need 4 nodes either way: 4*0.096 - 4*0.085
	if !nearly(nodeType.Savings.Hourly, 4*0.096-4*0.085) {
		t.Errorf("savings = %v, want %v", nodeType.Savings.Hourly, 4*0.096-4*0.085)
	}
	if nodeType.Items[0].Change != "4 × m → 4 × c" {
		t.Errorf("change = %q", nodeType.Items[0].Change)
	}
}

func TestSpotCoversOnlyStatelessWorkloads(t *testing.T) {
	items := append(nodes(2, 0.1),
		workload("api", "Deployment", 1.5, 1.5, nil),
		workload("db", "StatefulSet", 0.5, 0.5, nil),
		workload("agent", "DaemonSet", 1, 1, nil),
	)

	spot := find(Build(reportOf(items...), Inputs{SpotPriceRatio: 0.35}), "spot-capacity")

	// DaemonSets excluded; 75% of the rest is stateless; 0.2/h of nodes at 65% off
	if spot == nil || !nearly(spot.Savings.Hourly, 0.2*0.75*0.65) {
		t.Fatalf("spot = %+v, want %v", spot, 0.2*0.75*0.65)
	}
	if len(spot.Items) != 1 || spot.Items[0].Subject.Name != "general" {
		t.Errorf("items = %+v, want the on-demand node group", spot.Items)
	}
}

func TestRightsizingCommandNeedsASingleContainer(t *testing.T) {
	sidecar := workload("api", "Deployment", 2, 4, &costmodel.Usage{CPUCores: 0.2, MemoryGB: 3.5})
	sidecar.Detail["containers"] = 2
	job := workload("report", "Job", 2, 4, &costmodel.Usage{CPUCores: 0.2, MemoryGB: 3.5})

	rightsize := find(Build(reportOf(append(nodes(2, 0.1), sidecar, job)...), Inputs{}), "rightsize-requests")

	for _, item := range rightsize.Items {
		if item.Command != "" || item.Note == "" {
			t.Errorf("%s: command %q note %q, want a note instead of an inexact command", item.Subject.Name, item.Command, item.Note)
		}
	}
}

func TestHealthyClusterHasNothingToDo(t *testing.T) {
	items := append(nodes(2, 0.1), workload("api", "Deployment", 3, 12, &costmodel.Usage{CPUCores: 2.5, MemoryGB: 10}))

	plan := Build(reportOf(items...), Inputs{})

	if len(plan.Recommendations) != 0 || len(plan.DataIssues) != 0 || plan.Savings.Hourly != 0 {
		t.Errorf("plan = %+v, want nothing to do", plan)
	}
}

func TestUsageMetricsIssueWhenNothingWasSampled(t *testing.T) {
	plan := Build(reportOf(append(nodes(2, 0.1), workload("api", "Deployment", 3, 12, nil))...), Inputs{})

	issue := find(plan, "usage-metrics")
	if issue == nil || !strings.Contains(issue.Items[0].Command, "metrics-server") {
		t.Errorf("usage-metrics = %+v, want an install hint", issue)
	}
}

func TestRightsizingNeverSuggestsBelowTheFloors(t *testing.T) {
	idle := workload("agent", "DaemonSet", 0.05*4, 1, &costmodel.Usage{CPUCores: 0, MemoryGB: 0.004})
	idle.Detail["replicas"] = 4.0

	rightsize := find(Build(reportOf(append(nodes(2, 0.1), idle)...), Inputs{}), "rightsize-requests")

	if rightsize == nil {
		t.Fatal("rightsize-requests missing")
	}
	// 50m → 25m per pod (the floor, never 0m); 256Mi → 250Mi per pod (the floor)
	change := rightsize.Items[0].Change
	if change != "cpu 50m → 25m per pod · memory 256Mi → 250Mi per pod" {
		t.Errorf("change = %q, want both requests stopped at the floors", change)
	}
}

func TestRightsizingSkipsRequestsAlreadyAtTheFloor(t *testing.T) {
	small := workload("sidecar", "Deployment", 0.02, 0.2, &costmodel.Usage{CPUCores: 0.001, MemoryGB: 0.01})

	if find(Build(reportOf(append(nodes(2, 0.1), small)...), Inputs{}), "rightsize-requests") != nil {
		t.Error("rightsize-requests present, want nothing below the 25m / 250Mi floors")
	}
}

func TestManagedAddOnsAreCostedButNeverRightsized(t *testing.T) {
	coredns := workload("coredns", "Deployment", 0.2, 0.2, &costmodel.Usage{CPUCores: 0.002, MemoryGB: 0.02})
	coredns.Subject.Namespace = "kube-system"
	partial := workload("aws-node", "DaemonSet", 0.2, 0, &costmodel.Usage{CPUCores: 0.01})
	partial.Subject.Namespace = "kube-system"
	partial.Detail["missingResources"] = []string{"memory"}

	plan := Build(reportOf(append(nodes(2, 0.1), coredns, partial)...), Inputs{})

	if find(plan, "rightsize-requests") != nil {
		t.Error("rightsize-requests lists kube-system add-ons")
	}
	if find(plan, "incomplete-requests") != nil {
		t.Error("incomplete-requests lists a managed add-on")
	}
}

// A workload that requests CPU but not memory is priced and reported, but it
// does not hold back node-level steps the way a workload with no requests does.
func TestPartialRequestsDoNotBlockTheNodeSteps(t *testing.T) {
	partial := workload("web", "Deployment", 1, 0, nil)
	partial.Detail["missingResources"] = []string{"memory"}

	plan := Build(reportOf(append(nodes(5, 0.1), partial)...), Inputs{})

	issue := find(plan, "incomplete-requests")
	if issue == nil || !strings.Contains(issue.Items[0].Command, "--requests=memory=<memory>") {
		t.Fatalf("incomplete-requests = %+v, want a notice with a command template", issue)
	}
	if find(plan, "consolidate-nodes") == nil {
		t.Error("consolidate-nodes missing, want node steps to run despite partial requests")
	}
}

// Three 1-core pods on 2-core nodes add up to 3 cores, which would fit on two
// nodes as a fluid. Pods are whole: with headroom only one fits per node, so
// no node can go.
func TestConsolidationPacksWholePods(t *testing.T) {
	api := workload("api", "Deployment", 3, 3, nil)
	api.Detail["replicas"] = 3.0

	if find(Build(reportOf(append(nodes(3, 0.1), api)...), Inputs{}), "consolidate-nodes") != nil {
		t.Error("consolidate-nodes present, want none: one 1-core pod per 1.7 usable cores")
	}
}

func TestDaemonSetsReduceEveryNodesRoom(t *testing.T) {
	// 0.8-core pods fit two per node (1.7 usable) until a 0.2-core DaemonSet
	// takes room on every node.
	api := workload("api", "Deployment", 3.2, 4, nil)
	api.Detail["replicas"] = 4.0
	agent := workload("agent", "DaemonSet", 0.8, 0.4, nil)
	agent.Detail["replicas"] = 4.0

	without := find(Build(reportOf(append(nodes(4, 0.1), api)...), Inputs{}), "consolidate-nodes")
	with := find(Build(reportOf(append(nodes(4, 0.1), api, agent)...), Inputs{}), "consolidate-nodes")

	if without == nil || !nearly(without.Savings.Hourly, 0.2) {
		t.Fatalf("without the DaemonSet = %+v, want 2 nodes removed", without)
	}
	if with != nil {
		t.Errorf("with the DaemonSet = %+v, want no node removed", with)
	}
}

func TestConsolidationOnEKSGivesTheNodeGroupCommand(t *testing.T) {
	items := append(nodes(3, 0.1), workload("api", "Deployment", 1, 2, nil))
	eks := func(desired, min int) Inputs {
		return Inputs{EKS: &EKSCluster{Name: "shop", Region: "us-east-1", NodeGroups: []EKSNodeGroup{{Name: "general", Desired: desired, Min: min}}}}
	}

	consolidate := find(Build(reportOf(items...), eks(3, 2)), "consolidate-nodes")
	if consolidate == nil {
		t.Fatal("consolidate-nodes missing")
	}
	item := consolidate.Items[0]
	if item.Subject.Name != "general" || item.Change != "3 → 2 nodes" || item.Check != "kubectl get nodes -L eks.amazonaws.com/nodegroup" ||
		item.Command != "aws eks update-nodegroup-config --cluster-name shop --nodegroup-name general --scaling-config desiredSize=2 --region us-east-1" {
		t.Errorf("item = %+v, want the general group scaled to 2", item)
	}

	// a minimum above the new size has to come down with it
	item = find(Build(reportOf(items...), eks(3, 3)), "consolidate-nodes").Items[0]
	if !strings.Contains(item.Command, "--scaling-config minSize=2,desiredSize=2 ") {
		t.Errorf("command = %q, want the minimum lowered too", item.Command)
	}

	// without EKS details, only the summary
	item = find(Build(reportOf(items...), Inputs{}), "consolidate-nodes").Items[0]
	if item.Command != "" || item.Subject.Name != "worker nodes" {
		t.Errorf("item = %+v, want no command outside EKS", item)
	}
}

func TestConsolidationGivesNoCommandWhenOneGroupCannotAbsorbTheCut(t *testing.T) {
	// two groups of two nodes, and the plan removes two: that empties a group
	items := nodes(4, 0.1)
	items[2].Subject.ParentID, items[3].Subject.ParentID = "extra", "extra"
	inputs := Inputs{EKS: &EKSCluster{Name: "shop", Region: "us-east-1", NodeGroups: []EKSNodeGroup{{Name: "general", Desired: 2, Min: 1}, {Name: "extra", Desired: 2, Min: 1}}}}

	consolidate := find(Build(reportOf(append(items, workload("api", "Deployment", 1, 2, nil))...), inputs), "consolidate-nodes")
	if consolidate == nil || consolidate.Items[0].Change != "4 → 2 nodes" {
		t.Fatalf("consolidate = %+v, want 2 of 4 nodes removed", consolidate)
	}
	if consolidate.Items[0].Command != "" {
		t.Errorf("command = %q, want none when the cut spans groups", consolidate.Items[0].Command)
	}
}

func TestNodeTypeOnEKSReplacesTheNodeGroup(t *testing.T) {
	items := append(nodes(4, 0.096), workload("api", "Deployment", 6, 6, nil))
	general := EKSNodeGroup{
		Name: "general", Desired: 4, Min: 2, Max: 4, InstanceTypes: []string{"m"}, CapacityType: "ON_DEMAND",
		AmiType: "AL2023_x86_64_STANDARD", NodeRole: "arn:aws:iam::1:role/nodes", Subnets: []string{"subnet-a", "subnet-b"},
		Labels:           map[string]string{"workload-tier": "general", "alpha.eksctl.io/nodegroup-name": "general"},
		Taints:           []string{"key=dedicated,value=web,effect=NO_SCHEDULE"},
		LaunchTemplateID: "lt-1", LaunchTemplateVersion: "3",
	}
	inputs := Inputs{
		MachineTypes: []MachineType{{Name: "m", VCPU: 2, MemoryGB: 8, HourlyUSD: 0.096}, {Name: "c", VCPU: 2, MemoryGB: 4, HourlyUSD: 0.085}},
		EKS:          &EKSCluster{Name: "shop", Region: "us-east-1", NodeGroups: []EKSNodeGroup{general}},
	}

	item := find(Build(reportOf(items...), inputs), "node-type").Items[0]
	want := []string{
		"aws eks create-nodegroup --cluster-name shop --nodegroup-name general-c --instance-types c --capacity-type ON_DEMAND" +
			" --scaling-config minSize=2,maxSize=4,desiredSize=4 --subnets subnet-a subnet-b --node-role arn:aws:iam::1:role/nodes" +
			" --launch-template id=lt-1,version=3 --ami-type AL2023_x86_64_STANDARD --labels workload-tier=general" +
			" --taints key=dedicated,value=web,effect=NO_SCHEDULE --region us-east-1",
		"aws eks wait nodegroup-active --cluster-name shop --nodegroup-name general-c --region us-east-1",
		"kubectl cordon -l eks.amazonaws.com/nodegroup=general",
		"kubectl drain -l eks.amazonaws.com/nodegroup=general --ignore-daemonsets --delete-emptydir-data",
		"aws eks delete-nodegroup --cluster-name shop --nodegroup-name general --region us-east-1",
	}
	if item.Subject.Name != "general" || strings.Join(item.Steps, "\n") != strings.Join(want, "\n") {
		t.Errorf("steps =\n%s\nwant\n%s", strings.Join(item.Steps, "\n"), strings.Join(want, "\n"))
	}

	// a custom AMI lives in the launch template: no AMI type may be given
	inputs.EKS.NodeGroups[0].AmiType = "CUSTOM"
	if item := find(Build(reportOf(items...), inputs), "node-type").Items[0]; strings.Contains(item.Steps[0], "--ami-type") {
		t.Errorf("create = %q, want no AMI type with a custom AMI", item.Steps[0])
	}

	// a group running several types is not a plain swap
	inputs.EKS.NodeGroups[0].InstanceTypes = []string{"m", "r"}
	if item := find(Build(reportOf(items...), inputs), "node-type").Items[0]; len(item.Steps) != 0 || item.Subject.Name != "worker nodes" {
		t.Errorf("item = %+v, want no commands for a mixed group", item)
	}
}
