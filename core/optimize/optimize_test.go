package optimize

import (
	"math"
	"testing"
	"time"

	"kube-budget/core/costmodel"
)

func nearly(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

func node(name string, hourly float64, ready, schedulable bool, purchase string, cores, memoryGB float64) costmodel.LineItem {
	return costmodel.LineItem{
		Subject:    costmodel.Subject{Kind: costmodel.SubjectNode, ID: name, Name: name},
		Basis:      costmodel.BasisProvisioned,
		Usage:      costmodel.Usage{CPUCores: cores, MemoryGB: memoryGB},
		HourlyUSD:  hourly,
		Confidence: costmodel.ConfidenceExact,
		Detail:     map[string]interface{}{"ready": ready, "schedulable": schedulable, "purchase": purchase},
	}
}

func workload(name string, hourly, cores, memoryGB float64, confidence costmodel.Confidence) costmodel.LineItem {
	return costmodel.LineItem{
		Subject:    costmodel.Subject{Kind: costmodel.SubjectWorkload, ID: name, Name: name, Namespace: "prod"},
		Basis:      costmodel.BasisRequested,
		Usage:      costmodel.Usage{CPUCores: cores, MemoryGB: memoryGB},
		HourlyUSD:  hourly,
		Confidence: confidence,
	}
}

func reportOf(items ...costmodel.LineItem) costmodel.CostReport {
	return costmodel.NewReport(time.Unix(0, 0), costmodel.Scope{}, items, nil)
}

func find(recommendations []Recommendation, id string) *Recommendation {
	for index := range recommendations {
		if recommendations[index].ID == id {
			return &recommendations[index]
		}
	}
	return nil
}

func TestAnalyzeRanksBlockersFirst(t *testing.T) {
	report := reportOf(
		node("node-1", 0.20, false, true, "on-demand", 4, 16),
		workload("legacy", 0, 0, 0, costmodel.ConfidenceUnknown),
	)

	recommendations := Analyze(report)

	if len(recommendations) == 0 {
		t.Fatal("Analyze() returned nothing")
	}
	if recommendations[0].Severity != SeverityBlocker {
		t.Errorf("first severity = %q, want blocker", recommendations[0].Severity)
	}
	if blocker := find(recommendations, "missing-requests"); blocker == nil || blocker.Savings.Hourly != 0 {
		t.Errorf("missing-requests = %+v, want a blocker with no savings", blocker)
	}
}

func TestUnusableNodesClaimTheFullNodePrice(t *testing.T) {
	report := reportOf(
		node("node-1", 0.20, true, true, "on-demand", 4, 16),
		node("node-2", 0.20, false, true, "on-demand", 4, 16),
		node("node-3", 0.20, true, false, "on-demand", 4, 16),
		workload("api", 0.05, 1, 2, costmodel.ConfidenceDerived),
	)

	recommendation := find(Analyze(report), "unusable-nodes")

	if recommendation == nil {
		t.Fatal("no unusable-nodes recommendation")
	}
	if recommendation.Count != 2 {
		t.Errorf("Count = %d, want 2", recommendation.Count)
	}
	if recommendation.Savings.Hourly != 0.4 {
		t.Errorf("Savings = %v, want 0.4", recommendation.Savings.Hourly)
	}
	if recommendation.Savings.Monthly != 0.4*costmodel.HoursPerMonth {
		t.Errorf("Savings.Monthly = %v, want the 730-hour projection", recommendation.Savings.Monthly)
	}
}

func TestConsolidationProposesFewerNodes(t *testing.T) {
	report := reportOf(
		node("node-1", 0.20, true, true, "on-demand", 4, 16),
		node("node-2", 0.20, true, true, "on-demand", 4, 16),
		node("node-3", 0.20, true, true, "on-demand", 4, 16),
		workload("api", 0.05, 2, 8, costmodel.ConfidenceDerived),
	)

	recommendation := find(Analyze(report), "consolidation")

	if recommendation == nil {
		t.Fatal("no consolidation recommendation")
	}
	if recommendation.Count != 2 {
		t.Errorf("removable nodes = %d, want 2", recommendation.Count)
	}
	if !nearly(recommendation.Savings.Hourly, 0.4) {
		t.Errorf("Savings = %v, want 0.4", recommendation.Savings.Hourly)
	}
	if !nearly(recommendation.Proposed.Hourly, 0.2) {
		t.Errorf("Proposed = %v, want 0.2", recommendation.Proposed.Hourly)
	}
}

// Consolidation must stay silent while the demand side is incomplete.
func TestConsolidationSkippedWhenRequestsAreMissing(t *testing.T) {
	report := reportOf(
		node("node-1", 0.20, true, true, "on-demand", 4, 16),
		node("node-2", 0.20, true, true, "on-demand", 4, 16),
		workload("api", 0.05, 1, 2, costmodel.ConfidenceDerived),
		workload("legacy", 0, 0, 0, costmodel.ConfidenceUnknown),
	)

	if recommendation := find(Analyze(report), "consolidation"); recommendation != nil {
		t.Errorf("consolidation = %+v, want none while requests are missing", recommendation)
	}
}

func TestConsolidationSilentWhenCapacityIsNeeded(t *testing.T) {
	report := reportOf(
		node("node-1", 0.20, true, true, "on-demand", 4, 16),
		node("node-2", 0.20, true, true, "on-demand", 4, 16),
		workload("api", 0.05, 7, 28, costmodel.ConfidenceDerived),
	)

	if recommendation := find(Analyze(report), "consolidation"); recommendation != nil {
		t.Errorf("consolidation = %+v, want none when the nodes are needed", recommendation)
	}
}

func TestOrphanVolumesAreFlagged(t *testing.T) {
	volume := func(name, status string, hourly float64) costmodel.LineItem {
		return costmodel.LineItem{
			Subject:    costmodel.Subject{Kind: costmodel.SubjectVolume, ID: name, Name: name},
			Basis:      costmodel.BasisProvisioned,
			HourlyUSD:  hourly,
			Confidence: costmodel.ConfidenceDerived,
			Detail:     map[string]interface{}{"status": status},
		}
	}
	report := reportOf(volume("data", "Bound", 0.01), volume("stale", "Pending", 0.02))

	recommendation := find(Analyze(report), "orphan-volumes")

	if recommendation == nil || recommendation.Count != 1 {
		t.Fatalf("orphan-volumes = %+v, want the single unbound claim", recommendation)
	}
	if recommendation.Savings.Hourly != 0.02 {
		t.Errorf("Savings = %v, want 0.02", recommendation.Savings.Hourly)
	}
}

// Spot capacity has no price in the report, so the rule must not invent one.
func TestSpotCandidatesClaimNoSavings(t *testing.T) {
	report := reportOf(node("node-1", 0.20, true, true, "on-demand", 4, 16))

	recommendation := find(Analyze(report), "spot-candidates")

	if recommendation == nil {
		t.Fatal("no spot-candidates recommendation")
	}
	if recommendation.Savings.Hourly != 0 {
		t.Errorf("Savings = %v, want none", recommendation.Savings.Hourly)
	}
	if recommendation.Confidence != costmodel.ConfidenceUnknown {
		t.Errorf("Confidence = %q, want unknown", recommendation.Confidence)
	}
}

func TestAnalyzeReturnsNothingForAHealthyReport(t *testing.T) {
	report := reportOf(
		node("node-1", 0.20, true, true, "spot", 4, 16),
		workload("api", 0.15, 3, 13, costmodel.ConfidenceDerived),
	)

	if recommendations := Analyze(report); len(recommendations) != 0 {
		t.Errorf("Analyze() = %+v, want no findings", recommendations)
	}
}
