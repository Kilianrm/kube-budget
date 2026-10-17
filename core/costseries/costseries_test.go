package costseries

import (
	"math"
	"testing"
	"time"
)

var base = time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC)

func nearly(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

// A constant rate over a full window must integrate to rate x hours.
func TestBuildIntegratesAConstantRate(t *testing.T) {
	samples := []Sample{
		{At: base, ProvisionedHourly: 2, RequestedHourly: 1, IdleHourly: 1},
		{At: base.Add(time.Hour), ProvisionedHourly: 2, RequestedHourly: 1, IdleHourly: 1},
		{At: base.Add(2 * time.Hour), ProvisionedHourly: 2, RequestedHourly: 1, IdleHourly: 1},
	}

	series := Build(samples, base, base.Add(3*time.Hour), BucketHour, time.Hour)

	if len(series.Points) != 3 {
		t.Fatalf("points = %d, want 3", len(series.Points))
	}
	if !nearly(series.TotalProvisionedUSD, 6) {
		t.Errorf("provisioned total = %v, want 6", series.TotalProvisionedUSD)
	}
	if !nearly(series.TotalIdleUSD, 3) {
		t.Errorf("idle total = %v, want 3", series.TotalIdleUSD)
	}
	if !nearly(series.Coverage(), 1) {
		t.Errorf("coverage = %v, want full", series.Coverage())
	}
}

// Scaling down halfway through must cost less than the run rate implies.
func TestBuildWeightsEachRateByItsDuration(t *testing.T) {
	samples := []Sample{
		{At: base, ProvisionedHourly: 10},
		{At: base.Add(2 * time.Hour), ProvisionedHourly: 4},
	}

	series := Build(samples, base, base.Add(4*time.Hour), BucketHour, 2*time.Hour)

	// 10/h for two hours, then 4/h for two hours.
	if !nearly(series.TotalProvisionedUSD, 28) {
		t.Errorf("provisioned total = %v, want 28", series.TotalProvisionedUSD)
	}
}

// The app only records while it is open: a gap must stay a gap.
func TestBuildLeavesGapsUncoveredInsteadOfInterpolating(t *testing.T) {
	samples := []Sample{
		{At: base, ProvisionedHourly: 10},
		{At: base.Add(6 * time.Hour), ProvisionedHourly: 10},
	}

	series := Build(samples, base, base.Add(8*time.Hour), BucketHour, time.Hour)

	if !nearly(series.TotalProvisionedUSD, 20) {
		t.Errorf("provisioned total = %v, want only the two covered hours", series.TotalProvisionedUSD)
	}
	if !nearly(series.CoveredHours, 2) {
		t.Errorf("covered hours = %v, want 2", series.CoveredHours)
	}
	if !nearly(series.Coverage(), 0.25) {
		t.Errorf("coverage = %v, want 0.25", series.Coverage())
	}

	gap := series.Points[3]
	if gap.CoveredHours != 0 || gap.ProvisionedUSD != 0 {
		t.Errorf("gap bucket = %+v, want no data", gap)
	}
	if gap.Coverage() != 0 {
		t.Errorf("gap coverage = %v, want 0", gap.Coverage())
	}
}

func TestBuildAggregatesIntoDailyBuckets(t *testing.T) {
	samples := []Sample{
		{At: base.Add(1 * time.Hour), ProvisionedHourly: 1},
		{At: base.Add(25 * time.Hour), ProvisionedHourly: 3},
	}

	series := Build(samples, base, base.Add(48*time.Hour), BucketDay, time.Hour)

	if len(series.Points) != 2 {
		t.Fatalf("points = %d, want 2 days", len(series.Points))
	}
	if !nearly(series.Points[0].ProvisionedUSD, 1) {
		t.Errorf("day one = %v, want 1", series.Points[0].ProvisionedUSD)
	}
	if !nearly(series.Points[1].ProvisionedUSD, 3) {
		t.Errorf("day two = %v, want 3", series.Points[1].ProvisionedUSD)
	}
}

func TestBuildClipsSamplesToTheWindow(t *testing.T) {
	samples := []Sample{{At: base.Add(-2 * time.Hour), ProvisionedHourly: 10}}

	series := Build(samples, base, base.Add(2*time.Hour), BucketHour, 3*time.Hour)

	// The sample stays valid for one hour inside the window.
	if !nearly(series.TotalProvisionedUSD, 10) {
		t.Errorf("provisioned total = %v, want 10", series.TotalProvisionedUSD)
	}
}

func TestBuildWithoutSamplesReportsNoCoverage(t *testing.T) {
	series := Build(nil, base, base.Add(3*time.Hour), BucketHour, time.Hour)

	if len(series.Points) != 3 {
		t.Fatalf("points = %d, want the empty buckets", len(series.Points))
	}
	if series.CoveredHours != 0 || series.Coverage() != 0 {
		t.Errorf("coverage = %v, want none", series.Coverage())
	}
}

func TestBuildIntegratesSharedAndNamespaceSpend(t *testing.T) {
	samples := []Sample{
		{At: base, ProvisionedHourly: 3, SharedHourly: 0.5, HourlyByNamespace: map[string]float64{"prod": 1, "ops": 0.5}},
		{At: base.Add(time.Hour), ProvisionedHourly: 3, SharedHourly: 0.5, HourlyByNamespace: map[string]float64{"prod": 2}},
	}

	series := Build(samples, base, base.Add(2*time.Hour), BucketHour, time.Hour)

	if !nearly(series.TotalSharedUSD, 1) {
		t.Errorf("shared total = %v, want 1", series.TotalSharedUSD)
	}
	if !nearly(series.TotalByNamespace["prod"], 3) || !nearly(series.TotalByNamespace["ops"], 0.5) {
		t.Errorf("namespace totals = %+v, want prod 3 and ops 0.5", series.TotalByNamespace)
	}
	if !nearly(series.Points[1].ByNamespace["prod"], 2) {
		t.Errorf("second bucket = %+v, want prod 2", series.Points[1].ByNamespace)
	}
}

// Two windows with different coverage compare through their average rate.
func TestSummaryAverageRateIgnoresUncoveredTime(t *testing.T) {
	series := Build([]Sample{{At: base, ProvisionedHourly: 4}}, base, base.Add(10*time.Hour), BucketHour, 2*time.Hour)

	summary := series.Summary()
	if !nearly(summary.ProvisionedUSD, 8) || !nearly(summary.CoveredHours, 2) {
		t.Fatalf("summary = %+v, want 8 USD over 2 covered hours", summary)
	}
	if !nearly(summary.AverageProvisionedHourly(), 4) {
		t.Errorf("average rate = %v, want 4", summary.AverageProvisionedHourly())
	}
}
