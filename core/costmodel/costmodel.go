package costmodel

import (
	"sort"
	"time"
)

// Basis records what a cost was computed from. Costs with different bases
// describe the same cluster from different angles and must never be summed
// together into a single figure.
type Basis string

const (
	// BasisRequested is demand-side cost derived from pod resource requests.
	BasisRequested Basis = "requested"
	// BasisLimits is demand-side cost derived from pod resource limits.
	BasisLimits Basis = "limits"
	// BasisProvisioned is supply-side cost: what the provider bills for the
	// infrastructure that exists, whether or not workloads use it.
	BasisProvisioned Basis = "provisioned"
	// BasisUsed is cost derived from observed utilization. Not collected yet.
	BasisUsed Basis = "used"
)

// SubjectKind identifies what a line item is charging for.
type SubjectKind string

const (
	SubjectWorkload     SubjectKind = "workload"
	SubjectNamespace    SubjectKind = "namespace"
	SubjectNode         SubjectKind = "node"
	SubjectNodeGroup    SubjectKind = "nodegroup"
	SubjectControlPlane SubjectKind = "controlplane"
	SubjectVolume       SubjectKind = "volume"
	SubjectAddon        SubjectKind = "addon"
	SubjectCluster      SubjectKind = "cluster"
)

// Subject is the entity a cost is attributed to.
type Subject struct {
	Kind      SubjectKind       `json:"kind"`
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	ParentID  string            `json:"parentId,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// Component is a priced dimension of a line item.
type Component string

const (
	ComponentCPU     Component = "cpu"
	ComponentMemory  Component = "memory"
	ComponentStorage Component = "storage"
	ComponentGPU     Component = "gpu"
	// ComponentFlat covers per-hour fees that are not resource-proportional,
	// such as the managed control plane or a load balancer.
	ComponentFlat Component = "flat"
)

// Confidence qualifies how trustworthy a cost figure is. The UI must present
// anything below ConfidenceExact as an estimate.
type Confidence string

const (
	// ConfidenceExact comes from a published price for the exact SKU.
	ConfidenceExact Confidence = "exact"
	// ConfidenceDerived is split out of a known node price using a cost share.
	ConfidenceDerived Confidence = "derived"
	// ConfidenceEstimated uses a fallback catalog entry.
	ConfidenceEstimated Confidence = "estimated"
	// ConfidenceUnknown means no rate was found; the cost is excluded from totals.
	ConfidenceUnknown Confidence = "unknown"
)

// Assumption is a pricing decision that must stay visible to the caller.
type Assumption struct {
	Key    string `json:"key"`
	Detail string `json:"detail"`
}

// Warning reports data that could not be priced or collected.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Subject string `json:"subject,omitempty"`
}

// Usage holds normalized resource quantities in the units the core works with.
type Usage struct {
	CPUCores  float64 `json:"cpuCores"`
	MemoryGB  float64 `json:"memoryGB"`
	StorageGB float64 `json:"storageGB"`
	GPUUnits  float64 `json:"gpuUnits"`
}

// Add returns the sum of two usage values.
func (usage Usage) Add(other Usage) Usage {
	return Usage{
		CPUCores:  usage.CPUCores + other.CPUCores,
		MemoryGB:  usage.MemoryGB + other.MemoryGB,
		StorageGB: usage.StorageGB + other.StorageGB,
		GPUUnits:  usage.GPUUnits + other.GPUUnits,
	}
}

// Scale multiplies every quantity, typically by a replica count.
func (usage Usage) Scale(factor float64) Usage {
	return Usage{
		CPUCores:  usage.CPUCores * factor,
		MemoryGB:  usage.MemoryGB * factor,
		StorageGB: usage.StorageGB * factor,
		GPUUnits:  usage.GPUUnits * factor,
	}
}

// LineItem is one priced entity: the atom every module reads and writes.
type LineItem struct {
	Subject     Subject                `json:"subject"`
	Basis       Basis                  `json:"basis"`
	Usage       Usage                  `json:"usage"`
	Components  map[Component]float64  `json:"components,omitempty"`
	HourlyUSD   float64                `json:"hourlyUSD"`
	Cost        Projection             `json:"cost"`
	Confidence  Confidence             `json:"confidence"`
	Assumptions []Assumption           `json:"assumptions,omitempty"`
	Detail      map[string]interface{} `json:"detail,omitempty"`
	// Used is the observed usage of a requested item, when metrics exist.
	// UsedComponents prices it at the same rates as the requests, so the two
	// can be compared component by component.
	Used           *Usage                `json:"used,omitempty"`
	UsedComponents map[Component]float64 `json:"usedComponents,omitempty"`
}

// Priced reports whether the item contributes to totals. Items without a
// resolved rate are kept for visibility but excluded from every sum.
func (item LineItem) Priced() bool {
	return item.Confidence != ConfidenceUnknown
}

// Dimension is a grouping axis offered by a report.
type Dimension string

const (
	DimensionNamespace   Dimension = "namespace"
	DimensionSubjectKind Dimension = "subjectKind"
	DimensionParent      Dimension = "parent"
	DimensionComponent   Dimension = "component"
	// DimensionWorkload keeps one row per consumer: a workload or a volume.
	DimensionWorkload Dimension = "workload"
)

// Scope describes what a report covers.
type Scope struct {
	ClusterName string `json:"clusterName,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Region      string `json:"region,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

// CostReport is the single artifact consumed by the Cost Explorer UI and by
// the Optimizations rules.
type CostReport struct {
	GeneratedAt time.Time            `json:"generatedAt"`
	Scope       Scope                `json:"scope"`
	Currency    string               `json:"currency"`
	Items       []LineItem           `json:"items"`
	Totals      map[Basis]Projection `json:"totals"`
	// Idle is node capacity that is billed but not requested. Flat fees and
	// volumes are not idle: they are reported as Shared and as consumers.
	Idle            Projection               `json:"idle"`
	IdleByComponent map[Component]Projection `json:"idleByComponent"`
	// Shared is billed cost that no workload requests by design, such as the
	// managed control plane.
	Shared Projection `json:"shared"`
	// ByDimension is keyed by basis first, so a breakdown never mixes what the
	// provider bills with what the workloads request.
	ByDimension map[Basis]map[Dimension]map[string]Projection `json:"byDimension"`
	// Allocation splits the billed total between its consumers, idle capacity
	// and shared costs. The rows of each dimension add up to the provisioned total.
	Allocation map[Dimension][]AllocationRow `json:"allocation"`
	// Usage compares observed usage with requests. It is nil when no usage
	// was collected.
	Usage       *UsageSummary `json:"usage,omitempty"`
	Assumptions []Assumption  `json:"assumptions,omitempty"`
	Warnings    []Warning     `json:"warnings,omitempty"`
}

// NewReport aggregates line items into a report. Totals and breakdowns are
// kept separate per basis, and the billed total is reconciled into consumers,
// idle node capacity and shared costs.
func NewReport(generatedAt time.Time, scope Scope, items []LineItem, warnings []Warning) CostReport {
	report := CostReport{
		GeneratedAt: generatedAt,
		Scope:       scope,
		Currency:    CurrencyUSD,
		Items:       items,
		Totals:      make(map[Basis]Projection),
		ByDimension: make(map[Basis]map[Dimension]map[string]Projection),
		Warnings:    warnings,
	}

	hourlyByBasis := make(map[Basis]float64)
	hourlyByDimension := make(map[Basis]map[Dimension]map[string]float64)
	seenAssumptions := make(map[Assumption]bool)
	ledger := newAllocationLedger()

	for index := range items {
		item := &items[index]
		item.Cost = Project(item.HourlyUSD)
		for _, assumption := range item.Assumptions {
			if !seenAssumptions[assumption] {
				seenAssumptions[assumption] = true
				report.Assumptions = append(report.Assumptions, assumption)
			}
		}
		ledger.add(*item)
		if !item.Priced() {
			report.Warnings = append(report.Warnings, Warning{
				Code:    "unpriced-subject",
				Message: "no rate resolved; excluded from totals",
				Subject: item.Subject.Name,
			})
			continue
		}

		hourlyByBasis[item.Basis] += item.HourlyUSD
		buckets := hourlyByDimension[item.Basis]
		if buckets == nil {
			buckets = make(map[Dimension]map[string]float64)
			hourlyByDimension[item.Basis] = buckets
		}
		addDimension(buckets, DimensionSubjectKind, string(item.Subject.Kind), item.HourlyUSD)
		if item.Subject.Namespace != "" {
			addDimension(buckets, DimensionNamespace, item.Subject.Namespace, item.HourlyUSD)
		}
		if item.Subject.ParentID != "" {
			addDimension(buckets, DimensionParent, item.Subject.ParentID, item.HourlyUSD)
		}
		for component, hourly := range item.Components {
			addDimension(buckets, DimensionComponent, string(component), hourly)
		}
	}

	for basis, hourly := range hourlyByBasis {
		report.Totals[basis] = Project(hourly)
	}
	for basis, dimensions := range hourlyByDimension {
		report.ByDimension[basis] = make(map[Dimension]map[string]Projection, len(dimensions))
		for dimension, buckets := range dimensions {
			projected := make(map[string]Projection, len(buckets))
			for key, hourly := range buckets {
				projected[key] = Project(hourly)
			}
			report.ByDimension[basis][dimension] = projected
		}
	}

	ledger.finish(&report)
	sortAssumptions(report.Assumptions)

	return report
}

func addDimension(buckets map[Dimension]map[string]float64, dimension Dimension, key string, hourly float64) {
	if buckets[dimension] == nil {
		buckets[dimension] = make(map[string]float64)
	}
	buckets[dimension][key] += hourly
}

func sortAssumptions(assumptions []Assumption) {
	sort.Slice(assumptions, func(i, j int) bool {
		if assumptions[i].Key == assumptions[j].Key {
			return assumptions[i].Detail < assumptions[j].Detail
		}
		return assumptions[i].Key < assumptions[j].Key
	})
}
