// Package costseries turns point-in-time cost samples into spend over a
// window. It integrates each sampled rate over the time it was in effect, and
// reports the periods it has no data for instead of interpolating across them.
package costseries

import (
	"sort"
	"time"
)

// Bucket is the granularity of a series.
type Bucket string

const (
	BucketHour Bucket = "hour"
	BucketDay  Bucket = "day"
)

// DefaultMaxGap is how long a sample is assumed to stay valid. Beyond it the
// window counts as uncovered: the app was probably closed.
const DefaultMaxGap = 90 * time.Minute

// Sample is one observed set of hourly rates.
type Sample struct {
	At                time.Time
	ProvisionedHourly float64
	RequestedHourly   float64
	IdleHourly        float64
}

// Point is the spend inside one bucket. CoveredHours against BucketHours tells
// the caller how much of the bucket is actually backed by data.
type Point struct {
	Start          time.Time `json:"start"`
	End            time.Time `json:"end"`
	ProvisionedUSD float64   `json:"provisionedUSD"`
	RequestedUSD   float64   `json:"requestedUSD"`
	IdleUSD        float64   `json:"idleUSD"`
	CoveredHours   float64   `json:"coveredHours"`
	BucketHours    float64   `json:"bucketHours"`
}

// Coverage is the share of the bucket backed by samples, from 0 to 1.
func (point Point) Coverage() float64 {
	if point.BucketHours <= 0 {
		return 0
	}
	return point.CoveredHours / point.BucketHours
}

// Series is the result of integrating samples over a window.
type Series struct {
	Bucket              Bucket    `json:"bucket"`
	Currency            string    `json:"currency"`
	From                time.Time `json:"from"`
	To                  time.Time `json:"to"`
	Points              []Point   `json:"points"`
	TotalProvisionedUSD float64   `json:"totalProvisionedUSD"`
	TotalRequestedUSD   float64   `json:"totalRequestedUSD"`
	TotalIdleUSD        float64   `json:"totalIdleUSD"`
	CoveredHours        float64   `json:"coveredHours"`
	WindowHours         float64   `json:"windowHours"`
}

// Coverage is the share of the whole window backed by samples.
func (series Series) Coverage() float64 {
	if series.WindowHours <= 0 {
		return 0
	}
	return series.CoveredHours / series.WindowHours
}

// Build integrates the samples over [from, to). A sample's rates apply from
// its timestamp until the next sample, capped at maxGap.
func Build(samples []Sample, from, to time.Time, bucket Bucket, maxGap time.Duration) Series {
	if maxGap <= 0 {
		maxGap = DefaultMaxGap
	}
	from, to = from.UTC(), to.UTC()

	series := Series{
		Bucket:      bucket,
		From:        from,
		To:          to,
		Points:      buildBuckets(from, to, bucket),
		WindowHours: hoursBetween(from, to),
	}
	if !to.After(from) || len(series.Points) == 0 {
		return series
	}

	ordered := append([]Sample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })

	for index, sample := range ordered {
		start := sample.At.UTC()
		end := start.Add(maxGap)
		if index+1 < len(ordered) {
			if next := ordered[index+1].At.UTC(); next.Before(end) {
				end = next
			}
		}
		applyInterval(&series, sample, start, end)
	}

	for _, point := range series.Points {
		series.TotalProvisionedUSD += point.ProvisionedUSD
		series.TotalRequestedUSD += point.RequestedUSD
		series.TotalIdleUSD += point.IdleUSD
		series.CoveredHours += point.CoveredHours
	}
	return series
}

// applyInterval spreads one sample's rates across the buckets it overlaps.
func applyInterval(series *Series, sample Sample, start, end time.Time) {
	for index := range series.Points {
		point := &series.Points[index]
		overlapStart := latest(start, point.Start)
		overlapEnd := earliest(end, point.End)
		if !overlapEnd.After(overlapStart) {
			continue
		}

		hours := hoursBetween(overlapStart, overlapEnd)
		point.ProvisionedUSD += sample.ProvisionedHourly * hours
		point.RequestedUSD += sample.RequestedHourly * hours
		point.IdleUSD += sample.IdleHourly * hours
		point.CoveredHours += hours
	}
}

func buildBuckets(from, to time.Time, bucket Bucket) []Point {
	if !to.After(from) {
		return nil
	}

	step := time.Hour
	start := from.Truncate(time.Hour)
	if bucket == BucketDay {
		step = 24 * time.Hour
		start = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	}

	points := make([]Point, 0)
	for cursor := start; cursor.Before(to); cursor = cursor.Add(step) {
		bucketEnd := cursor.Add(step)
		windowStart := latest(cursor, from)
		windowEnd := earliest(bucketEnd, to)
		points = append(points, Point{
			Start:       cursor,
			End:         bucketEnd,
			BucketHours: hoursBetween(windowStart, windowEnd),
		})
	}
	return points
}

func hoursBetween(start, end time.Time) float64 {
	if !end.After(start) {
		return 0
	}
	return end.Sub(start).Hours()
}

func latest(left, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}

func earliest(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
