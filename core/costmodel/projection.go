// Package costmodel defines the normalized cost primitives shared by Manifest
// Mode, the Cluster Cost Explorer and the Optimizations module. Every cost
// figure in the application originates here so the modules can never disagree.
package costmodel

// Time conversion constants. Monthly uses the 730-hour convention
// (365 * 24 / 12) so projections stay stable across calendar months.
const (
	HoursPerDay   = 24.0
	HoursPerMonth = 730.0
	HoursPerYear  = 8760.0
)

// CurrencyUSD is the only currency supported today.
const CurrencyUSD = "USD"

// Projection expresses one hourly rate over the time windows the UI shows.
// It is a run rate extrapolated from a point-in-time rate, not observed spend.
type Projection struct {
	Hourly  float64 `json:"hourly"`
	Daily   float64 `json:"daily"`
	Monthly float64 `json:"monthly"`
	Yearly  float64 `json:"yearly"`
}

// Project expands an hourly USD rate into every supported time window. This is
// the only place in the codebase allowed to perform this multiplication.
func Project(hourlyUSD float64) Projection {
	return Projection{
		Hourly:  hourlyUSD,
		Daily:   hourlyUSD * HoursPerDay,
		Monthly: hourlyUSD * HoursPerMonth,
		Yearly:  hourlyUSD * HoursPerYear,
	}
}

// Add returns the sum of two projections.
func (projection Projection) Add(other Projection) Projection {
	return Projection{
		Hourly:  projection.Hourly + other.Hourly,
		Daily:   projection.Daily + other.Daily,
		Monthly: projection.Monthly + other.Monthly,
		Yearly:  projection.Yearly + other.Yearly,
	}
}

// Sub returns the difference between two projections, clamped at zero so a
// negative gap (allocation above provisioned capacity) never reads as savings.
func (projection Projection) Sub(other Projection) Projection {
	return Project(max(projection.Hourly-other.Hourly, 0))
}
