package costmodel

import (
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
	if report.Idle.Hourly != 0.3 {
		t.Errorf("idle = %v, want 0.3", report.Idle.Hourly)
	}
	if got := report.ByDimension[DimensionNamespace]["prod"].Hourly; got != 0.2 {
		t.Errorf("namespace prod = %v, want 0.2", got)
	}
	if got := report.ByDimension[DimensionComponent][string(ComponentCPU)].Hourly; got != 0.45 {
		t.Errorf("cpu component = %v, want 0.45", got)
	}
	if got := report.ByDimension[DimensionParent]["ng-1"].Hourly; got != 0.5 {
		t.Errorf("nodegroup ng-1 = %v, want 0.5", got)
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
