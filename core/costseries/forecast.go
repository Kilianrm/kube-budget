package costseries

import (
	"math"
	"sort"
	"time"

	"kube-budget/core/costmodel"
)

// Forecast projects the billed spend of the current calendar month. It keeps
// what was measured, what was estimated for uncaptured hours and what is
// projected for the rest of the month apart, so a guess never reads as data.
type Forecast struct {
	MonthStart time.Time `json:"monthStart"`
	MonthEnd   time.Time `json:"monthEnd"`
	At         time.Time `json:"at"`
	// RecordedUSD is integrated from captures, like the spend history.
	RecordedUSD float64 `json:"recordedUSD"`
	// EstimatedUSD covers the elapsed hours without captures, at the average
	// recorded rate (or the run rate when nothing was recorded yet).
	EstimatedUSD float64 `json:"estimatedUSD"`
	// ProjectedUSD covers the rest of the month at the current run rate.
	ProjectedUSD   float64       `json:"projectedUSD"`
	TotalUSD       float64       `json:"totalUSD"`
	RunRateHourly  float64       `json:"runRateHourly"`
	ElapsedHours   float64       `json:"elapsedHours"`
	CoveredHours   float64       `json:"coveredHours"`
	RemainingHours float64       `json:"remainingHours"`
	Days           []ForecastDay `json:"days"`
}

// ForecastDay splits one calendar day the same way as the whole forecast.
type ForecastDay struct {
	Start        time.Time `json:"start"`
	End          time.Time `json:"end"`
	RecordedUSD  float64   `json:"recordedUSD"`
	EstimatedUSD float64   `json:"estimatedUSD"`
	ProjectedUSD float64   `json:"projectedUSD"`
	CoveredHours float64   `json:"coveredHours"`
}

// SpentUSD is the month-to-date spend: recorded plus estimated.
func (forecast Forecast) SpentUSD() float64 {
	return forecast.RecordedUSD + forecast.EstimatedUSD
}

// BuildForecast forecasts the calendar month containing now, in now's location.
func BuildForecast(samples []Sample, now time.Time, runRateHourly float64, maxGap time.Duration) Forecast {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)
	series := Build(samples, monthStart, now, BucketDay, maxGap)

	forecast := Forecast{
		MonthStart:     monthStart.UTC(),
		MonthEnd:       monthEnd.UTC(),
		At:             now.UTC(),
		RecordedUSD:    series.TotalProvisionedUSD,
		RunRateHourly:  runRateHourly,
		ElapsedHours:   hoursBetween(monthStart, now),
		CoveredHours:   series.CoveredHours,
		RemainingHours: hoursBetween(now, monthEnd),
	}

	gapRate := runRateHourly
	if series.CoveredHours > 0 {
		gapRate = series.TotalProvisionedUSD / series.CoveredHours
	}

	recorded := make(map[int64]Point, len(series.Points))
	for _, point := range series.Points {
		recorded[point.Start.Unix()] = point
	}
	for start := monthStart; start.Before(monthEnd); start = start.AddDate(0, 0, 1) {
		end := start.AddDate(0, 0, 1)
		point := recorded[start.UTC().Unix()]
		elapsed := hoursBetween(start, earliest(end, now))
		day := ForecastDay{
			Start:        start.UTC(),
			End:          end.UTC(),
			RecordedUSD:  point.ProvisionedUSD,
			CoveredHours: point.CoveredHours,
			EstimatedUSD: math.Max(elapsed-point.CoveredHours, 0) * gapRate,
			ProjectedUSD: hoursBetween(latest(start, now), end) * runRateHourly,
		}
		forecast.EstimatedUSD += day.EstimatedUSD
		forecast.ProjectedUSD += day.ProjectedUSD
		forecast.Days = append(forecast.Days, day)
	}

	forecast.TotalUSD = forecast.RecordedUSD + forecast.EstimatedUSD + forecast.ProjectedUSD
	return forecast
}

// WithRunRateChange returns the forecast if the run rate changed by delta from
// now on. Only the projected rest of the month moves: what was recorded or
// estimated for past hours stays as it was.
func (forecast Forecast) WithRunRateChange(deltaHourly float64) Forecast {
	changed := forecast
	changed.RunRateHourly += deltaHourly
	changed.Days = make([]ForecastDay, len(forecast.Days))
	added := 0.0
	for index, day := range forecast.Days {
		extra := hoursBetween(latest(day.Start, forecast.At), day.End) * deltaHourly
		day.ProjectedUSD += extra
		added += extra
		changed.Days[index] = day
	}
	changed.ProjectedUSD += added
	changed.TotalUSD += added
	return changed
}

// BudgetState summarizes a forecast against a budget.
type BudgetState string

const (
	BudgetOnTrack  BudgetState = "on-track"
	BudgetAtRisk   BudgetState = "at-risk"
	BudgetOver     BudgetState = "over"
	BudgetExceeded BudgetState = "exceeded"
)

// budgetRiskRatio is the projected share of the budget that counts as at risk.
const budgetRiskRatio = 0.9

// BudgetStatus compares one month's forecast with its budget.
type BudgetStatus struct {
	MonthlyUSD float64     `json:"monthlyUSD"`
	State      BudgetState `json:"state"`
	// ProjectedRatio is the forecast total over the budget.
	ProjectedRatio float64 `json:"projectedRatio"`
	// RemainingUSD is the budget left after the forecast; negative when over.
	RemainingUSD float64 `json:"remainingUSD"`
	// ExhaustedAt is when the run rate uses up the budget, if it does this month.
	ExhaustedAt *time.Time `json:"exhaustedAt,omitempty"`
}

// EvaluateBudget places a forecast against a monthly budget.
func EvaluateBudget(forecast Forecast, monthlyUSD float64) BudgetStatus {
	status := BudgetStatus{MonthlyUSD: monthlyUSD, RemainingUSD: monthlyUSD - forecast.TotalUSD}
	if monthlyUSD <= 0 {
		return status
	}
	status.ProjectedRatio = forecast.TotalUSD / monthlyUSD

	spent := forecast.SpentUSD()
	switch {
	case spent >= monthlyUSD:
		status.State = BudgetExceeded
	case forecast.TotalUSD > monthlyUSD:
		status.State = BudgetOver
		if forecast.RunRateHourly > 0 {
			hours := (monthlyUSD - spent) / forecast.RunRateHourly
			exhausted := forecast.At.Add(time.Duration(hours * float64(time.Hour)))
			status.ExhaustedAt = &exhausted
		}
	case status.ProjectedRatio >= budgetRiskRatio:
		status.State = BudgetAtRisk
	default:
		status.State = BudgetOnTrack
	}
	return status
}

// Driver is one contributor to the change between two windows, expressed as
// average rates so windows with different coverage stay comparable.
type Driver struct {
	// Dimension is "namespace", "nodeGroup", "idle" or "shared".
	Dimension string               `json:"dimension"`
	Key       string               `json:"key"`
	Current   costmodel.Projection `json:"current"`
	Previous  costmodel.Projection `json:"previous"`
	// Delta may be negative: the rate went down.
	Delta costmodel.Projection `json:"delta"`
}

// minimumDriverCoverage is the captured time each window needs before its
// average rate is trusted.
const minimumDriverCoverage = 1.0

// Drivers explains the change in rate between two windows. Namespaces show
// who consumes more; node groups show what is billed more; idle and shared
// close the gap. It returns nil when either window has too little data.
func Drivers(current, previous Series) []Driver {
	if current.CoveredHours < minimumDriverCoverage || previous.CoveredHours < minimumDriverCoverage {
		return nil
	}

	drivers := make([]Driver, 0)
	add := func(dimension, key string, currentUSD, previousUSD float64) {
		now := currentUSD / current.CoveredHours
		before := previousUSD / previous.CoveredHours
		if math.Abs(now-before) < 1e-9 {
			return
		}
		drivers = append(drivers, Driver{
			Dimension: dimension,
			Key:       key,
			Current:   costmodel.Project(now),
			Previous:  costmodel.Project(before),
			Delta:     costmodel.Project(now - before),
		})
	}
	addGroup := func(dimension string, now, before map[string]float64) {
		for _, key := range unionKeys(now, before) {
			add(dimension, key, now[key], before[key])
		}
	}

	addGroup("namespace", current.TotalByNamespace, previous.TotalByNamespace)
	addGroup("nodeGroup", current.TotalByNodeGroup, previous.TotalByNodeGroup)
	add("idle", "Idle capacity", current.TotalIdleUSD, previous.TotalIdleUSD)
	add("shared", "Shared cluster costs", current.TotalSharedUSD, previous.TotalSharedUSD)

	sort.SliceStable(drivers, func(i, j int) bool {
		return math.Abs(drivers[i].Delta.Hourly) > math.Abs(drivers[j].Delta.Hourly)
	})
	return drivers
}

func unionKeys(left, right map[string]float64) []string {
	keys := make([]string, 0, len(left)+len(right))
	for key := range left {
		keys = append(keys, key)
	}
	for key := range right {
		if _, ok := left[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
