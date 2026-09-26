package wails

import (
	"path/filepath"
	"testing"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/optimize"
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
