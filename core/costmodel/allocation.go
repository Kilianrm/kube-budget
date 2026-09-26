package costmodel

import (
	"fmt"
	"sort"
)

// AllocationKind tells a consumer row apart from the two rows that close the
// gap to the bill.
type AllocationKind string

const (
	// AllocationConsumer is cost attributed to a namespace, workload or volume.
	AllocationConsumer AllocationKind = "consumer"
	// AllocationIdle is node capacity that is billed but requested by nobody.
	AllocationIdle AllocationKind = "idle"
	// AllocationShared is cluster-wide cost that no workload can request,
	// such as the managed control plane.
	AllocationShared AllocationKind = "shared"
)

// Reserved row keys, chosen so they never collide with a Kubernetes name.
const (
	AllocationKeyIdle   = "__idle__"
	AllocationKeyShared = "__shared__"
)

// clusterScopedNamespace groups consumers that have no namespace.
const clusterScopedNamespace = "(cluster)"

// overcommitTolerance absorbs float rounding when requests fill a node exactly.
const overcommitTolerance = 1e-9

// AllocationRow is one line of the bill: who consumes it and through which
// resource. Unpriced counts consumers whose cost is unknown and left out.
type AllocationRow struct {
	Key         string                   `json:"key"`
	Name        string                   `json:"name"`
	Namespace   string                   `json:"namespace,omitempty"`
	SubjectKind SubjectKind              `json:"subjectKind,omitempty"`
	Kind        AllocationKind           `json:"kind"`
	Components  map[Component]Projection `json:"components,omitempty"`
	Cost        Projection               `json:"cost"`
	Confidence  Confidence               `json:"confidence"`
	Items       int                      `json:"items"`
	Unpriced    int                      `json:"unpriced"`
	// Efficiency is used over requested cost per component, over the items
	// of the row that have usage. Above 1 means usage exceeds requests.
	Efficiency map[Component]float64 `json:"efficiency,omitempty"`
}

// UsageSummary compares what workloads use with what they request, over the
// workloads that have usage. Costs are priced at the same node rates.
type UsageSummary struct {
	Used       map[Component]Projection `json:"used"`
	Requested  map[Component]Projection `json:"requested"`
	Efficiency map[Component]float64    `json:"efficiency"`
	// Workloads counts priced workloads with usage; WithoutUsage counts those
	// without, which the figures above leave out.
	Workloads    int `json:"workloads"`
	WithoutUsage int `json:"withoutUsage"`
}

// usageTally sums used and requested cost over items that have usage.
type usageTally struct {
	used      map[Component]float64
	requested map[Component]float64
}

func (tally *usageTally) add(item LineItem) {
	if tally.used == nil {
		tally.used = make(map[Component]float64)
		tally.requested = make(map[Component]float64)
	}
	for component, hourly := range item.Components {
		tally.requested[component] += hourly
		tally.used[component] += item.UsedComponents[component]
	}
}

func (tally usageTally) efficiency() map[Component]float64 {
	if len(tally.requested) == 0 {
		return nil
	}
	efficiency := make(map[Component]float64, len(tally.requested))
	for component, requested := range tally.requested {
		if requested > 0 {
			efficiency[component] = tally.used[component] / requested
		}
	}
	return efficiency
}

func hasUsage(item LineItem) bool {
	return item.Basis == BasisRequested && item.Priced() && item.Used != nil
}

var confidenceRank = map[Confidence]int{
	ConfidenceExact:     0,
	ConfidenceDerived:   1,
	ConfidenceEstimated: 2,
	ConfidenceUnknown:   3,
}

// WeakerConfidence returns the less trustworthy of two confidences. An empty
// value means nothing has been observed yet.
func WeakerConfidence(current, next Confidence) Confidence {
	if current == "" || confidenceRank[next] > confidenceRank[current] {
		return next
	}
	return current
}

// allocationLedger reconciles the provisioned lens against the requested one:
// node cost is split between the requests it serves and the idle remainder,
// namespaced provisioned items are charged to their namespace, and everything
// else is shared.
type allocationLedger struct {
	capacity  map[Component]float64
	requested map[Component]float64
	shared    float64
	rows      map[Dimension]map[string]*allocationAccumulator
	usage     usageTally
	// withUsage and withoutUsage count priced workloads by whether usage exists.
	withUsage, withoutUsage int
}

type allocationAccumulator struct {
	row        AllocationRow
	hourly     float64
	components map[Component]float64
	priced     bool
	usage      usageTally
}

func newAllocationLedger() *allocationLedger {
	return &allocationLedger{
		capacity:  make(map[Component]float64),
		requested: make(map[Component]float64),
		rows:      make(map[Dimension]map[string]*allocationAccumulator),
	}
}

func (ledger *allocationLedger) add(item LineItem) {
	switch item.Basis {
	case BasisRequested:
		if item.Priced() {
			for component, hourly := range item.Components {
				ledger.requested[component] += hourly
			}
			if hasUsage(item) {
				ledger.usage.add(item)
				ledger.withUsage++
			} else {
				ledger.withoutUsage++
			}
		}
		ledger.consume(item)
	case BasisProvisioned:
		if !item.Priced() {
			return
		}
		switch {
		case item.Subject.Kind == SubjectNode:
			for component, hourly := range item.Components {
				ledger.capacity[component] += hourly
			}
		case item.Subject.Namespace != "":
			ledger.consume(item)
		default:
			ledger.shared += item.HourlyUSD
		}
	}
}

func (ledger *allocationLedger) consume(item LineItem) {
	namespace := item.Subject.Namespace
	if namespace == "" {
		namespace = clusterScopedNamespace
	}
	ledger.accumulate(DimensionNamespace, namespace, AllocationRow{Name: namespace}, item)

	key := item.Subject.ID
	if key == "" {
		key = fmt.Sprintf("%s/%s/%s", item.Subject.Kind, item.Subject.Namespace, item.Subject.Name)
	}
	ledger.accumulate(DimensionWorkload, key, AllocationRow{
		Name:        item.Subject.Name,
		Namespace:   item.Subject.Namespace,
		SubjectKind: item.Subject.Kind,
	}, item)
}

func (ledger *allocationLedger) accumulate(dimension Dimension, key string, template AllocationRow, item LineItem) {
	if ledger.rows[dimension] == nil {
		ledger.rows[dimension] = make(map[string]*allocationAccumulator)
	}
	accumulator := ledger.rows[dimension][key]
	if accumulator == nil {
		template.Key = key
		template.Kind = AllocationConsumer
		accumulator = &allocationAccumulator{row: template, components: make(map[Component]float64)}
		ledger.rows[dimension][key] = accumulator
	}

	accumulator.row.Items++
	if !item.Priced() {
		accumulator.row.Unpriced++
		return
	}
	accumulator.priced = true
	accumulator.hourly += item.HourlyUSD
	accumulator.row.Confidence = WeakerConfidence(accumulator.row.Confidence, item.Confidence)
	for component, hourly := range item.Components {
		accumulator.components[component] += hourly
	}
	if hasUsage(item) {
		accumulator.usage.add(item)
	}
}

// finish writes idle, shared and the allocation rows into the report.
func (ledger *allocationLedger) finish(report *CostReport) {
	idle, overcommitted := ledger.idle()
	if overcommitted > overcommitTolerance {
		report.Warnings = append(report.Warnings, Warning{
			Code:    "requests-exceed-capacity",
			Message: fmt.Sprintf("requests are priced $%.4f/h above the billed node capacity; idle is floored at zero", overcommitted),
		})
	}

	idleHourly := 0.0
	report.IdleByComponent = make(map[Component]Projection, len(idle))
	for component, hourly := range idle {
		idleHourly += hourly
		report.IdleByComponent[component] = Project(hourly)
	}
	report.Idle = Project(idleHourly)
	report.Shared = Project(ledger.shared)
	if ledger.withUsage > 0 {
		report.Usage = &UsageSummary{
			Used:         projectComponents(ledger.usage.used),
			Requested:    projectComponents(ledger.usage.requested),
			Efficiency:   ledger.usage.efficiency(),
			Workloads:    ledger.withUsage,
			WithoutUsage: ledger.withoutUsage,
		}
	}

	report.Allocation = make(map[Dimension][]AllocationRow, len(ledger.rows))
	for _, dimension := range []Dimension{DimensionNamespace, DimensionWorkload} {
		rows := make([]AllocationRow, 0, len(ledger.rows[dimension])+2)
		for _, accumulator := range ledger.rows[dimension] {
			rows = append(rows, accumulator.build())
		}
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Cost.Hourly != rows[j].Cost.Hourly {
				return rows[i].Cost.Hourly > rows[j].Cost.Hourly
			}
			if rows[i].Namespace != rows[j].Namespace {
				return rows[i].Namespace < rows[j].Namespace
			}
			return rows[i].Name < rows[j].Name
		})

		if idleHourly > 0 {
			rows = append(rows, AllocationRow{
				Key:        AllocationKeyIdle,
				Name:       "Idle capacity",
				Kind:       AllocationIdle,
				Components: report.IdleByComponent,
				Cost:       report.Idle,
				Confidence: ConfidenceDerived,
			})
		}
		if ledger.shared > 0 {
			rows = append(rows, AllocationRow{
				Key:        AllocationKeyShared,
				Name:       "Shared cluster costs",
				Kind:       AllocationShared,
				Components: map[Component]Projection{ComponentFlat: report.Shared},
				Cost:       report.Shared,
				Confidence: ConfidenceExact,
			})
		}
		if len(rows) > 0 {
			report.Allocation[dimension] = rows
		}
	}
}

// idle subtracts requested cost from node cost component by component.
// Requested cost in a component the nodes were not split into (for example a
// node priced as a single flat figure) is taken from the flat remainder.
func (ledger *allocationLedger) idle() (map[Component]float64, float64) {
	idle := make(map[Component]float64, len(ledger.capacity))
	for component, hourly := range ledger.capacity {
		idle[component] = hourly
	}

	unmatched := 0.0
	for component, hourly := range ledger.requested {
		if _, ok := ledger.capacity[component]; ok && component != ComponentFlat {
			idle[component] -= hourly
			continue
		}
		unmatched += hourly
	}
	if unmatched > 0 {
		idle[ComponentFlat] -= unmatched
	}

	overcommitted := 0.0
	for component, hourly := range idle {
		if hourly < 0 {
			overcommitted -= hourly
		}
		if hourly <= overcommitTolerance {
			delete(idle, component)
		}
	}
	return idle, overcommitted
}

func (accumulator *allocationAccumulator) build() AllocationRow {
	row := accumulator.row
	row.Cost = Project(accumulator.hourly)
	if !accumulator.priced {
		row.Confidence = ConfidenceUnknown
	}
	if len(accumulator.components) > 0 {
		row.Components = projectComponents(accumulator.components)
	}
	row.Efficiency = accumulator.usage.efficiency()
	return row
}

func projectComponents(hourly map[Component]float64) map[Component]Projection {
	projected := make(map[Component]Projection, len(hourly))
	for component, value := range hourly {
		projected[component] = Project(value)
	}
	return projected
}
