package workspace

import (
	"strings"
	"time"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/costseries"
	"kube-cost-workspace/core/optimize"
	"kube-cost-workspace/internal/storage/budget"
	"kube-cost-workspace/internal/storage/costhistory"
	"kube-cost-workspace/internal/storage/recommendations"
)

// The stores live in the user's config directory, so the desktop app and the
// CLI read and write the same history, budgets and recommendations.

// RecordHistory appends a report to its cluster's spend history.
func RecordHistory(report costmodel.CostReport, now time.Time) error {
	store, err := CostHistoryStore()
	if err != nil {
		return err
	}
	return store.Append(costhistory.FromReport(report, now))
}

// Trend is the recorded spend over a window plus the totals of the window of
// equal length just before it, so a change can be shown.
type Trend struct {
	Series   costseries.Series
	Previous costseries.Summary
	// Drivers explains the change in rate against the previous window.
	Drivers []costseries.Driver
}

// SpendTrend integrates the recorded captures into spend over the last days.
// Periods without captures are returned as gaps, never interpolated.
func SpendTrend(clusterID string, days int, bucket costseries.Bucket, now time.Time) (Trend, error) {
	if days <= 0 {
		days = 7
	}
	store, err := CostHistoryStore()
	if err != nil {
		return Trend{}, err
	}

	from := now.AddDate(0, 0, -days)
	previousFrom := from.AddDate(0, 0, -days)
	records, err := store.Range(strings.TrimSpace(clusterID), previousFrom, now)
	if err != nil {
		return Trend{}, err
	}

	samples := make([]costseries.Sample, 0, len(records))
	currency := costmodel.CurrencyUSD
	for _, record := range records {
		if record.Currency != "" {
			currency = record.Currency
		}
		samples = append(samples, sampleFromRecord(record))
	}

	series := costseries.Build(samples, from, now, bucket, costseries.DefaultMaxGap)
	series.Currency = currency
	previous := costseries.Build(samples, previousFrom, from, bucket, costseries.DefaultMaxGap)
	return Trend{Series: series, Previous: previous.Summary(), Drivers: costseries.Drivers(series, previous)}, nil
}

// Forecast is a month's forecast and, when one is set, its budget.
type Forecast struct {
	Forecast costseries.Forecast
	Budget   *costseries.BudgetStatus
}

// ForecastInputs is the recorded month and the budget of one cluster, loaded
// once so several run rates can be forecast against them.
type ForecastInputs struct {
	records       []costhistory.Record
	now           time.Time
	monthlyBudget float64
}

// LoadForecast reads what the current month's forecast of a cluster needs.
func LoadForecast(clusterID string, now time.Time) (ForecastInputs, error) {
	history, err := CostHistoryStore()
	if err != nil {
		return ForecastInputs{}, err
	}
	budgets, err := BudgetStore()
	if err != nil {
		return ForecastInputs{}, err
	}

	clusterID = strings.TrimSpace(clusterID)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	// A capture just before the month still covers its first hour.
	records, err := history.Range(clusterID, monthStart.Add(-costseries.DefaultMaxGap), now)
	if err != nil {
		return ForecastInputs{}, err
	}
	stored, ok, err := budgets.Get(clusterID)
	if err != nil {
		return ForecastInputs{}, err
	}
	inputs := ForecastInputs{records: records, now: now}
	if ok {
		inputs.monthlyBudget = stored.MonthlyUSD
	}
	return inputs, nil
}

// Forecast projects the month at a billed run rate.
func (inputs ForecastInputs) Forecast(runRateHourly float64) Forecast {
	return forecastResult(inputs.records, inputs.now, runRateHourly, inputs.monthlyBudget)
}

// WithRunRateChange re-forecasts after a change that starts now, with the
// budget evaluated against the new total.
func (inputs ForecastInputs) WithRunRateChange(before Forecast, deltaHourly float64) Forecast {
	after := Forecast{Forecast: before.Forecast.WithRunRateChange(deltaHourly)}
	if inputs.monthlyBudget > 0 {
		status := costseries.EvaluateBudget(after.Forecast, inputs.monthlyBudget)
		after.Budget = &status
	}
	return after
}

// SetBudget stores a cluster's monthly budget; zero removes it.
func SetBudget(clusterID string, monthlyUSD float64, now time.Time) error {
	budgets, err := BudgetStore()
	if err != nil {
		return err
	}
	return budgets.Set(strings.TrimSpace(clusterID), monthlyUSD, now)
}

// WithDismissals applies the cluster's dismissed recommendations to the plan
// inputs, so every view of the plan matches the Optimizations view.
func WithDismissals(clusterID string, inputs optimize.Inputs) optimize.Inputs {
	store, err := RecommendationStore()
	if err != nil {
		return inputs
	}
	state, err := store.Get(clusterID)
	if err != nil {
		return inputs
	}
	inputs.Dismissed = make(map[string]bool, len(state.Dismissed))
	for id := range state.Dismissed {
		inputs.Dismissed[id] = true
	}
	return inputs
}

func forecastResult(records []costhistory.Record, now time.Time, runRateHourly, monthlyBudget float64) Forecast {
	samples := make([]costseries.Sample, 0, len(records))
	for _, record := range records {
		samples = append(samples, sampleFromRecord(record))
	}
	result := Forecast{Forecast: costseries.BuildForecast(samples, now, runRateHourly, costseries.DefaultMaxGap)}
	if monthlyBudget > 0 {
		status := costseries.EvaluateBudget(result.Forecast, monthlyBudget)
		result.Budget = &status
	}
	return result
}

func sampleFromRecord(record costhistory.Record) costseries.Sample {
	return costseries.Sample{
		At:                record.CapturedAt,
		ProvisionedHourly: record.HourlyByBasis[string(costmodel.BasisProvisioned)],
		RequestedHourly:   record.HourlyByBasis[string(costmodel.BasisRequested)],
		IdleHourly:        record.IdleHourly,
		SharedHourly:      record.SharedHourly,
		HourlyByNamespace: record.HourlyByNamespace,
		HourlyByNodeGroup: record.HourlyByNodeGroup,
	}
}

// BudgetStore opens the user's budgets.
func BudgetStore() (*budget.Store, error) {
	path, err := budget.DefaultPath()
	if err != nil {
		return nil, err
	}
	return budget.New(path), nil
}

// CostHistoryStore opens the user's spend history.
func CostHistoryStore() (*costhistory.Store, error) {
	path, err := costhistory.DefaultPath()
	if err != nil {
		return nil, err
	}
	return costhistory.New(path), nil
}

// RecommendationStore opens the user's recommendation journal.
func RecommendationStore() (*recommendations.Store, error) {
	path, err := recommendations.DefaultPath()
	if err != nil {
		return nil, err
	}
	return recommendations.New(path), nil
}
