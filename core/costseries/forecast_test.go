package costseries

import (
	"testing"
	"time"
)

// madrid is not UTC, so the test proves the month follows the caller's calendar.
var madrid = time.FixedZone("CEST", 2*60*60)

func TestBuildForecastSeparatesRecordedEstimatedAndProjected(t *testing.T) {
	// September has 720 hours; "now" is 10 days in.
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, madrid)
	monthStart := time.Date(2026, 9, 1, 0, 0, 0, 0, madrid)
	// 10 recorded hours at 2/h on day one, nothing afterwards.
	samples := make([]Sample, 0, 10)
	for hour := 0; hour < 10; hour++ {
		samples = append(samples, Sample{At: monthStart.Add(time.Duration(hour) * time.Hour), ProvisionedHourly: 2})
	}

	forecast := BuildForecast(samples, now, 3, time.Hour)

	if !nearly(forecast.RecordedUSD, 20) {
		t.Errorf("recorded = %v, want 20", forecast.RecordedUSD)
	}
	// 230 uncaptured elapsed hours at the average recorded rate of 2/h
	if !nearly(forecast.EstimatedUSD, 460) {
		t.Errorf("estimated = %v, want 460", forecast.EstimatedUSD)
	}
	// 480 remaining hours at the run rate of 3/h
	if !nearly(forecast.ProjectedUSD, 1440) || !nearly(forecast.RemainingHours, 480) {
		t.Errorf("projected = %v over %v hours, want 1440 over 480", forecast.ProjectedUSD, forecast.RemainingHours)
	}
	if !nearly(forecast.TotalUSD, 1920) {
		t.Errorf("total = %v, want 1920", forecast.TotalUSD)
	}
	if len(forecast.Days) != 30 || !forecast.Days[0].Start.Equal(monthStart) {
		t.Fatalf("days = %d starting %v, want 30 starting at local midnight", len(forecast.Days), forecast.Days[0].Start)
	}
	if day := forecast.Days[0]; !nearly(day.RecordedUSD, 20) || !nearly(day.EstimatedUSD, 28) || day.ProjectedUSD != 0 {
		t.Errorf("first day = %+v, want 20 recorded and 14 hours estimated", day)
	}
	if day := forecast.Days[29]; !nearly(day.ProjectedUSD, 72) || day.RecordedUSD != 0 {
		t.Errorf("last day = %+v, want a full projected day", day)
	}
}

func TestBuildForecastUsesTheRunRateWhenNothingWasRecorded(t *testing.T) {
	now := time.Date(2026, 9, 2, 0, 0, 0, 0, madrid)

	forecast := BuildForecast(nil, now, 1, time.Hour)

	if !nearly(forecast.EstimatedUSD, 24) || !nearly(forecast.TotalUSD, 720) {
		t.Errorf("forecast = %v estimated, %v total, want 24 and 720", forecast.EstimatedUSD, forecast.TotalUSD)
	}
}

func TestEvaluateBudgetStates(t *testing.T) {
	at := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
	forecast := Forecast{At: at, RecordedUSD: 400, ProjectedUSD: 700, TotalUSD: 1100, RunRateHourly: 2}

	cases := []struct {
		budget float64
		want   BudgetState
	}{
		{2000, BudgetOnTrack},
		{1200, BudgetAtRisk},
		{1000, BudgetOver},
		{300, BudgetExceeded},
	}
	for _, tc := range cases {
		if got := EvaluateBudget(forecast, tc.budget).State; got != tc.want {
			t.Errorf("budget %v: state = %q, want %q", tc.budget, got, tc.want)
		}
	}

	// 600 of budget left at 2/h runs out in 300 hours
	over := EvaluateBudget(forecast, 1000)
	if over.ExhaustedAt == nil || !over.ExhaustedAt.Equal(at.Add(300*time.Hour)) {
		t.Errorf("ExhaustedAt = %v, want %v", over.ExhaustedAt, at.Add(300*time.Hour))
	}
	if !nearly(over.RemainingUSD, -100) {
		t.Errorf("RemainingUSD = %v, want -100", over.RemainingUSD)
	}
}

func TestDriversCompareRatesNotTotals(t *testing.T) {
	// Previous window: 10 covered hours. Current window: 2 covered hours.
	previous := Series{CoveredHours: 10, TotalIdleUSD: 10,
		TotalByNamespace: map[string]float64{"prod": 20, "old": 5},
		TotalByNodeGroup: map[string]float64{"general": 30}}
	current := Series{CoveredHours: 2, TotalIdleUSD: 2,
		TotalByNamespace: map[string]float64{"prod": 6, "new": 1},
		TotalByNodeGroup: map[string]float64{"general": 6, "gpu": 4}}

	drivers := Drivers(current, previous)

	byKey := make(map[string]Driver)
	for _, driver := range drivers {
		byKey[driver.Dimension+"/"+driver.Key] = driver
	}
	// idle stays at 1/h and is not a driver, even though its total fell from 10 to 2
	if _, ok := byKey["idle/Idle capacity"]; ok {
		t.Error("idle listed as a driver, want it unchanged at 1/h")
	}
	if got := byKey["namespace/prod"].Delta.Hourly; !nearly(got, 1) {
		t.Errorf("prod delta = %v, want 3/h - 2/h = 1", got)
	}
	if got := byKey["namespace/old"].Delta.Hourly; !nearly(got, -0.5) {
		t.Errorf("old delta = %v, want -0.5", got)
	}
	if drivers[0].Key != "gpu" {
		t.Errorf("first driver = %s, want the new gpu node group (+2/h)", drivers[0].Key)
	}
	if Drivers(current, Series{CoveredHours: 0.5}) != nil {
		t.Error("Drivers() with a barely covered previous window, want nil")
	}
}

func TestRunRateChangesOnlyMoveTheFuture(t *testing.T) {
	now := time.Date(2026, 9, 11, 0, 0, 0, 0, madrid)
	// nothing recorded: the past is estimated at the run rate
	forecast := BuildForecast(nil, now, 1, time.Hour)

	changed := forecast.WithRunRateChange(0.5)

	if changed.EstimatedUSD != forecast.EstimatedUSD || changed.RecordedUSD != forecast.RecordedUSD {
		t.Errorf("past = %v estimated, want it unchanged at %v", changed.EstimatedUSD, forecast.EstimatedUSD)
	}
	// 480 hours left at +0.5/h
	if !nearly(changed.TotalUSD-forecast.TotalUSD, 240) || !nearly(changed.RunRateHourly, 1.5) {
		t.Errorf("total grew %v at %v/h, want +240 at 1.5/h", changed.TotalUSD-forecast.TotalUSD, changed.RunRateHourly)
	}
	if changed.Days[0].ProjectedUSD != 0 || !nearly(changed.Days[29].ProjectedUSD, 36) {
		t.Errorf("days = first %v, last %v projected, want 0 and 36", changed.Days[0].ProjectedUSD, changed.Days[29].ProjectedUSD)
	}
	if forecast.Days[29].ProjectedUSD != 24 {
		t.Errorf("original forecast changed: last day %v, want 24", forecast.Days[29].ProjectedUSD)
	}
}
