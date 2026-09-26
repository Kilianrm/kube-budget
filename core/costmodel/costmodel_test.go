package costmodel

import (
	"math"
	"testing"
	"time"
)

func TestProjectExpandsHourlyRate(t *testing.T) {
	projection := Project(2)

	if projection.Hourly != 2 {
		t.Errorf("Hourly = %v, want 2", projection.Hourly)
	}
	if projection.Daily != 48 {
		t.Errorf("Daily = %v, want 48", projection.Daily)
	}
	if projection.Monthly != 1460 {
		t.Errorf("Monthly = %v, want 1460", projection.Monthly)
	}
	if projection.Yearly != 17520 {
		t.Errorf("Yearly = %v, want 17520", projection.Yearly)
	}
}

func TestProjectionSubClampsAtZero(t *testing.T) {
	gap := Project(1).Sub(Project(3))

	if gap.Hourly != 0 || gap.Monthly != 0 {
		t.Errorf("Sub() = %+v, want zero projection", gap)
	}
}

func TestNewReportSeparatesBasesAndComputesIdle(t *testing.T) {
	items := []LineItem{
		{
			Subject:    Subject{Kind: SubjectNode, ID: "node-1", Name: "node-1", ParentID: "ng-1"},
			Basis:      BasisProvisioned,
			HourlyUSD:  0.5,
			Components: map[Component]float64{ComponentCPU: 0.25, ComponentMemory: 0.25},
			Confidence: ConfidenceExact,
		},
		{
			Subject:    Subject{Kind: SubjectWorkload, ID: "uid-api", Name: "api", Namespace: "prod"},
			Basis:      BasisRequested,
			HourlyUSD:  0.2,
			Components: map[Component]float64{ComponentCPU: 0.2},
			Confidence: ConfidenceDerived,
		},
	}

	report := NewReport(time.Unix(0, 0), Scope{ClusterName: "demo"}, items, nil)

	if report.Totals[BasisProvisioned].Hourly != 0.5 {
		t.Errorf("provisioned total = %v, want 0.5", report.Totals[BasisProvisioned].Hourly)
	}
	if report.Totals[BasisRequested].Hourly != 0.2 {
		t.Errorf("requested total = %v, want 0.2", report.Totals[BasisRequested].Hourly)
	}
	if !nearly(report.Idle.Hourly, 0.3) {
		t.Errorf("idle = %v, want 0.3", report.Idle.Hourly)
	}
	if got := report.ByDimension[BasisRequested][DimensionNamespace]["prod"].Hourly; got != 0.2 {
		t.Errorf("requested namespace prod = %v, want 0.2", got)
	}
	// Components never sum node cost and workload cost together.
	if got := report.ByDimension[BasisProvisioned][DimensionComponent][string(ComponentCPU)].Hourly; got != 0.25 {
		t.Errorf("provisioned cpu component = %v, want 0.25", got)
	}
	if got := report.ByDimension[BasisRequested][DimensionComponent][string(ComponentCPU)].Hourly; got != 0.2 {
		t.Errorf("requested cpu component = %v, want 0.2", got)
	}
	if _, ok := report.ByDimension[BasisProvisioned][DimensionSubjectKind][string(SubjectWorkload)]; ok {
		t.Error("provisioned breakdown by subject kind contains workloads")
	}
	if got := report.ByDimension[BasisProvisioned][DimensionParent]["ng-1"].Hourly; got != 0.5 {
		t.Errorf("nodegroup ng-1 = %v, want 0.5", got)
	}
	if got := report.IdleByComponent[ComponentCPU].Hourly; !nearly(got, 0.05) {
		t.Errorf("idle cpu = %v, want 0.05", got)
	}
	if got := report.IdleByComponent[ComponentMemory].Hourly; got != 0.25 {
		t.Errorf("idle memory = %v, want 0.25", got)
	}
	if report.Currency != CurrencyUSD {
		t.Errorf("Currency = %q, want %q", report.Currency, CurrencyUSD)
	}
	if got := report.Items[0].Cost.Monthly; got != 0.5*HoursPerMonth {
		t.Errorf("line item monthly cost = %v, want %v", got, 0.5*HoursPerMonth)
	}
}

func TestNewReportExcludesUnpricedItems(t *testing.T) {
	items := []LineItem{
		{
			Subject:    Subject{Kind: SubjectNode, Name: "mystery-node"},
			Basis:      BasisProvisioned,
			HourlyUSD:  9,
			Confidence: ConfidenceUnknown,
		},
	}

	report := NewReport(time.Unix(0, 0), Scope{}, items, nil)

	if report.Totals[BasisProvisioned].Hourly != 0 {
		t.Errorf("provisioned total = %v, want 0", report.Totals[BasisProvisioned].Hourly)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Subject != "mystery-node" {
		t.Errorf("Warnings = %+v, want one unpriced-subject warning", report.Warnings)
	}
	if len(report.Items) != 1 {
		t.Errorf("Items = %d, want the unpriced item kept for visibility", len(report.Items))
	}
}

func TestNewReportDeduplicatesAssumptions(t *testing.T) {
	assumption := Assumption{Key: "cpu-cost-share", Detail: "50% of node price attributed to CPU"}
	items := []LineItem{
		{Subject: Subject{Name: "a"}, Basis: BasisRequested, HourlyUSD: 1, Confidence: ConfidenceDerived, Assumptions: []Assumption{assumption}},
		{Subject: Subject{Name: "b"}, Basis: BasisRequested, HourlyUSD: 1, Confidence: ConfidenceDerived, Assumptions: []Assumption{assumption}},
	}

	report := NewReport(time.Unix(0, 0), Scope{}, items, nil)

	if len(report.Assumptions) != 1 {
		t.Errorf("Assumptions = %+v, want one deduplicated entry", report.Assumptions)
	}
}

func TestNewReportReconcilesTheBill(t *testing.T) {
	items := []LineItem{
		{
			Subject:    Subject{Kind: SubjectNode, ID: "node-1", Name: "node-1"},
			Basis:      BasisProvisioned,
			HourlyUSD:  0.4,
			Components: map[Component]float64{ComponentCPU: 0.2, ComponentMemory: 0.2},
			Confidence: ConfidenceExact,
		},
		{
			Subject:    Subject{Kind: SubjectControlPlane, ID: "controlplane", Name: "control plane"},
			Basis:      BasisProvisioned,
			HourlyUSD:  0.1,
			Components: map[Component]float64{ComponentFlat: 0.1},
			Confidence: ConfidenceExact,
		},
		{
			Subject:    Subject{Kind: SubjectVolume, ID: "pvc-1", Name: "data", Namespace: "prod"},
			Basis:      BasisProvisioned,
			HourlyUSD:  0.05,
			Components: map[Component]float64{ComponentStorage: 0.05},
			Confidence: ConfidenceDerived,
		},
		{
			Subject:    Subject{Kind: SubjectWorkload, ID: "uid-api", Name: "api", Namespace: "prod"},
			Basis:      BasisRequested,
			HourlyUSD:  0.15,
			Components: map[Component]float64{ComponentCPU: 0.1, ComponentMemory: 0.05},
			Confidence: ConfidenceDerived,
		},
		{
			Subject:    Subject{Kind: SubjectWorkload, ID: "uid-legacy", Name: "legacy", Namespace: "prod"},
			Basis:      BasisRequested,
			Confidence: ConfidenceUnknown,
		},
	}

	report := NewReport(time.Unix(0, 0), Scope{}, items, nil)

	// The control plane and the volume are billed but are not idle capacity.
	if !nearly(report.Idle.Hourly, 0.25) {
		t.Errorf("idle = %v, want the unrequested node cost 0.25", report.Idle.Hourly)
	}
	if report.Shared.Hourly != 0.1 {
		t.Errorf("shared = %v, want the control plane 0.1", report.Shared.Hourly)
	}

	for _, dimension := range []Dimension{DimensionNamespace, DimensionWorkload} {
		rows := report.Allocation[dimension]
		total := 0.0
		for _, row := range rows {
			total += row.Cost.Hourly
		}
		if !nearly(total, report.Totals[BasisProvisioned].Hourly) {
			t.Errorf("%s allocation sums to %v, want the billed total %v", dimension, total, report.Totals[BasisProvisioned].Hourly)
		}
	}

	namespaces := report.Allocation[DimensionNamespace]
	if len(namespaces) != 3 || namespaces[0].Key != "prod" || namespaces[1].Key != AllocationKeyIdle || namespaces[2].Key != AllocationKeyShared {
		t.Fatalf("namespace rows = %+v, want prod, idle, shared", namespaces)
	}
	prod := namespaces[0]
	if !nearly(prod.Cost.Hourly, 0.2) || prod.Items != 3 || prod.Unpriced != 1 {
		t.Errorf("prod row = %+v, want workload plus volume (0.2) and one unpriced workload", prod)
	}
	if prod.Confidence != ConfidenceDerived {
		t.Errorf("prod confidence = %q, want derived", prod.Confidence)
	}
}

func TestNewReportWarnsWhenRequestsExceedCapacity(t *testing.T) {
	items := []LineItem{
		{Subject: Subject{Kind: SubjectNode, Name: "node-1"}, Basis: BasisProvisioned, HourlyUSD: 0.1,
			Components: map[Component]float64{ComponentCPU: 0.05, ComponentMemory: 0.05}, Confidence: ConfidenceExact},
		{Subject: Subject{Kind: SubjectWorkload, Name: "big", Namespace: "prod"}, Basis: BasisRequested, HourlyUSD: 0.2,
			Components: map[Component]float64{ComponentCPU: 0.2}, Confidence: ConfidenceDerived},
	}

	report := NewReport(time.Unix(0, 0), Scope{}, items, nil)

	if got := report.IdleByComponent[ComponentCPU].Hourly; got != 0 {
		t.Errorf("idle cpu = %v, want floored at zero", got)
	}
	found := false
	for _, warning := range report.Warnings {
		found = found || warning.Code == "requests-exceed-capacity"
	}
	if !found {
		t.Errorf("Warnings = %+v, want requests-exceed-capacity", report.Warnings)
	}
}

func nearly(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

func TestNewReportComparesUsageWithRequests(t *testing.T) {
	items := []LineItem{
		{Subject: Subject{Kind: SubjectNode, Name: "node-1"}, Basis: BasisProvisioned, HourlyUSD: 1,
			Components: map[Component]float64{ComponentCPU: 0.5, ComponentMemory: 0.5}, Confidence: ConfidenceExact},
		{Subject: Subject{Kind: SubjectWorkload, ID: "api", Name: "api", Namespace: "prod"}, Basis: BasisRequested, HourlyUSD: 0.3,
			Components:     map[Component]float64{ComponentCPU: 0.2, ComponentMemory: 0.1},
			Used:           &Usage{CPUCores: 0.25, MemoryGB: 1},
			UsedComponents: map[Component]float64{ComponentCPU: 0.05, ComponentMemory: 0.08},
			Confidence:     ConfidenceDerived},
		{Subject: Subject{Kind: SubjectWorkload, ID: "batch", Name: "batch", Namespace: "prod"}, Basis: BasisRequested, HourlyUSD: 0.1,
			Components: map[Component]float64{ComponentCPU: 0.1}, Confidence: ConfidenceDerived},
	}

	report := NewReport(time.Unix(0, 0), Scope{}, items, nil)

	if report.Usage == nil {
		t.Fatal("Usage = nil, want a summary")
	}
	// only api has usage, so batch is left out of the comparison
	if got := report.Usage.Efficiency[ComponentCPU]; !nearly(got, 0.25) {
		t.Errorf("cpu efficiency = %v, want 0.25", got)
	}
	if got := report.Usage.Efficiency[ComponentMemory]; !nearly(got, 0.8) {
		t.Errorf("memory efficiency = %v, want 0.8", got)
	}
	if report.Usage.Workloads != 1 || report.Usage.WithoutUsage != 1 {
		t.Errorf("usage counts = %d/%d, want 1 with and 1 without", report.Usage.Workloads, report.Usage.WithoutUsage)
	}
	prod := report.Allocation[DimensionNamespace][0]
	if prod.Key != "prod" || !nearly(prod.Efficiency[ComponentCPU], 0.25) {
		t.Errorf("prod row = %+v, want cpu efficiency 0.25", prod)
	}
}

func TestNewReportOmitsUsageWithoutMetrics(t *testing.T) {
	items := []LineItem{
		{Subject: Subject{Kind: SubjectWorkload, Name: "api", Namespace: "prod"}, Basis: BasisRequested, HourlyUSD: 0.1,
			Components: map[Component]float64{ComponentCPU: 0.1}, Confidence: ConfidenceDerived},
	}

	if report := NewReport(time.Unix(0, 0), Scope{}, items, nil); report.Usage != nil {
		t.Errorf("Usage = %+v, want nil", report.Usage)
	}
}
