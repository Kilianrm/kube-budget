package whatif

import (
	"math"
	"strings"
	"testing"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/pricing"
)

func nearly(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// card prices 0.02 per core and 0.005 per GB, the split of a 0.08/h node
// with 2 cores and 8 GB.
var card = pricing.RateCard{CPUUSDPerCoreHour: 0.02, MemoryUSDPerGBHour: 0.005, Confidence: costmodel.ConfidenceDerived}

// node is a 2-core, 8 GB node at 0.08/h with some capacity already requested.
func node(name string, requestedCores, requestedMemory float64) costmodel.LineItem {
	return costmodel.LineItem{
		Subject:    costmodel.Subject{Kind: costmodel.SubjectNode, ID: name, Name: name, ParentID: "general"},
		Basis:      costmodel.BasisProvisioned,
		Usage:      costmodel.Usage{CPUCores: 2, MemoryGB: 8},
		HourlyUSD:  0.08,
		Components: map[costmodel.Component]float64{costmodel.ComponentCPU: 0.04, costmodel.ComponentMemory: 0.04},
		Confidence: costmodel.ConfidenceExact,
		Detail: map[string]interface{}{"machineType": "m", "purchase": "on-demand", "ready": true, "schedulable": true,
			"requestedCores": requestedCores, "requestedMemoryGB": requestedMemory},
	}
}

// existing is what already runs, priced with the same card.
func existing(cores, memory float64) costmodel.LineItem {
	return costmodel.LineItem{
		Subject:    costmodel.Subject{Kind: costmodel.SubjectWorkload, ID: "base", Name: "base", Namespace: "prod"},
		Basis:      costmodel.BasisRequested,
		Usage:      costmodel.Usage{CPUCores: cores, MemoryGB: memory},
		HourlyUSD:  cores*0.02 + memory*0.005,
		Components: map[costmodel.Component]float64{costmodel.ComponentCPU: cores * 0.02, costmodel.ComponentMemory: memory * 0.005},
		Confidence: costmodel.ConfidenceDerived,
		Detail:     map[string]interface{}{"kind": "Deployment", "replicas": 1.0, "containers": 1},
	}
}

func reportOf(items ...costmodel.LineItem) costmodel.CostReport {
	return costmodel.NewReport(time.Unix(0, 0), costmodel.Scope{}, items, nil)
}

func api(replicas int, cores, memory float64) Workload {
	return Workload{Name: "api", Namespace: "shop", Kind: "Deployment", Replicas: replicas, PerReplica: costmodel.Usage{CPUCores: cores, MemoryGB: memory}, Containers: 1}
}

func TestAWorkloadThatFitsLeavesTheBillAlone(t *testing.T) {
	report := reportOf(node("a", 0.5, 2), node("b", 0.5, 2), existing(1, 4))

	result := Simulate(report, api(2, 0.5, 1), Inputs{RateCard: card})

	if !result.Fits || result.NewNodes != 0 || result.BilledDelta.Hourly != 0 {
		t.Fatalf("result = fits %v, %d new nodes, +%v billed; want it to fit at no extra cost", result.Fits, result.NewNodes, result.BilledDelta.Hourly)
	}
	// 2 × (0.5 × 0.02 + 1 × 0.005)
	if !nearly(result.Requested.Hourly, 0.03) {
		t.Errorf("requested = %v, want 0.03", result.Requested.Hourly)
	}
	// the capacity it consumes comes out of idle, one for one
	if !nearly(result.IdleBefore.Hourly-result.IdleAfter.Hourly, 0.03) {
		t.Errorf("idle %v → %v, want it to shrink by 0.03", result.IdleBefore.Hourly, result.IdleAfter.Hourly)
	}
	// spread like the scheduler: one replica on each node
	if len(result.Placements) != 2 || result.Placements[0].Replicas != 1 {
		t.Errorf("placements = %+v, want one replica per node", result.Placements)
	}
}

func TestAWorkloadThatDoesNotFitAddsWholeNodes(t *testing.T) {
	report := reportOf(node("a", 1.8, 6), node("b", 1.8, 6), existing(3.6, 12))

	result := Simulate(report, api(1, 1, 2), Inputs{RateCard: card})

	if result.Fits || result.NewNodes != 1 || result.NewNodeType != "m" {
		t.Fatalf("result = fits %v, %d new %q nodes, want one new m node", result.Fits, result.NewNodes, result.NewNodeType)
	}
	// the bill grows by a whole node, not by the workload's own price
	if !nearly(result.BilledDelta.Hourly, 0.08) || !nearly(result.Requested.Hourly, 0.03) {
		t.Errorf("billed +%v, requested %v; want +0.08 billed for 0.03 requested", result.BilledDelta.Hourly, result.Requested.Hourly)
	}
	if !result.Placements[0].New {
		t.Errorf("placements = %+v, want the replica on the new node", result.Placements)
	}
}

func TestTaintedNodesAreNeverUsed(t *testing.T) {
	spot := node("spot", 0, 0)
	spot.Detail["taints"] = []string{"workload-tier=batch:NoSchedule"}
	report := reportOf(node("a", 1.8, 6), spot, existing(1.8, 6))

	result := Simulate(report, api(1, 1, 2), Inputs{RateCard: card})

	if result.NewNodes != 1 {
		t.Errorf("new nodes = %d, want one: the empty spot node is tainted", result.NewNodes)
	}
}

func TestAReplicaLargerThanAnyNodeStaysPending(t *testing.T) {
	report := reportOf(node("a", 0, 0), node("b", 0, 0))

	result := Simulate(report, api(2, 3, 1), Inputs{RateCard: card})

	if result.Unschedulable != 2 || result.NewNodes != 0 || result.Fits {
		t.Fatalf("result = %d pending, %d new nodes, fits %v; want both replicas pending", result.Unschedulable, result.NewNodes, result.Fits)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "Pending") {
		t.Errorf("warnings = %v, want a pending warning", result.Warnings)
	}
	if result.Requested.Hourly != 0 {
		t.Errorf("requested = %v, want 0: pending pods are not billed", result.Requested.Hourly)
	}
}

func TestDeployingCanRemoveASavingFromThePlan(t *testing.T) {
	// four nodes for demand that fits on two: consolidation removes two
	report := reportOf(node("a", 0.5, 1), node("b", 0.5, 1), node("c", 0.2, 1), node("d", 0.2, 1), existing(1.4, 4))

	result := Simulate(report, api(4, 0.6, 1), Inputs{RateCard: card})

	if len(result.PlanChanges) != 1 || result.PlanChanges[0].ID != "consolidate-nodes" {
		t.Fatalf("plan changes = %+v, want consolidation to change", result.PlanChanges)
	}
	change := result.PlanChanges[0]
	if !(change.After.Hourly < change.Before.Hourly) || !(result.PlanSavingsAfter.Hourly < result.PlanSavingsBefore.Hourly) {
		t.Errorf("consolidation %v → %v, want the workload to use capacity the plan wanted to remove", change.Before.Hourly, change.After.Hourly)
	}
}

func TestWithoutPlacementFreeCapacityIsShared(t *testing.T) {
	a, b := node("a", 0, 0), node("b", 0, 0)
	delete(a.Detail, "requestedCores")
	delete(b.Detail, "requestedCores")
	report := reportOf(a, b, existing(2, 8))

	result := Simulate(report, api(2, 0.9, 1), Inputs{RateCard: card})

	// half the cluster is requested, so each node has 1 core free
	if !result.Fits {
		t.Errorf("result = %+v, want two 0.9-core replicas to fit in 1 free core per node", result)
	}
	found := false
	for _, assumption := range result.Assumptions {
		found = found || assumption.Key == "whatif-free-capacity"
	}
	if !found {
		t.Error("assumptions do not mention the estimated free capacity")
	}
}

func TestAManifestWithoutRequestsIsFlagged(t *testing.T) {
	result := Simulate(reportOf(node("a", 0, 0)), api(1, 0, 0), Inputs{RateCard: card})

	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "no CPU or memory requests") {
		t.Errorf("warnings = %v, want the missing requests called out", result.Warnings)
	}
}
