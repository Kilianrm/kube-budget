package wails

import (
	"math"
	"testing"
	"time"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/optimize"
	"kube-cost-workspace/internal/storage/recommendations"
)

func TestGetCostReportRejectsUnknownPricingProvider(t *testing.T) {
	_, err := NewClusterAdapter().GetCostReport(CostReportRequest{Context: "", PricingProvider: "ibm"})
	if err == nil {
		t.Fatal("GetCostReport() error = nil, want an error")
	}
}

func TestOptimizationPricesTheAppliedJournal(t *testing.T) {
	report := costmodel.NewReport(time.Unix(0, 0), costmodel.Scope{}, []costmodel.LineItem{{
		Subject: costmodel.Subject{Kind: costmodel.SubjectNode, Name: "a"}, Basis: costmodel.BasisProvisioned,
		HourlyUSD: 1.0, Confidence: costmodel.ConfidenceExact,
	}}, nil)
	state := recommendations.State{
		Dismissed: map[string]time.Time{"spot-capacity": time.Unix(0, 0)},
		Applied:   []recommendations.Applied{{ID: "consolidate-nodes", BaselineHourly: 1.2, ExpectedHourly: 0.25}},
	}

	result := optimizationFrom(state, report, optimize.Inputs{})

	if len(result.Applied) != 1 || math.Abs(result.Applied[0].Realized.Hourly-0.2) > 1e-9 || result.Applied[0].Expected.Hourly != 0.25 {
		t.Errorf("Applied = %+v, want 0.2/h realized against 0.25/h expected", result.Applied)
	}
}
