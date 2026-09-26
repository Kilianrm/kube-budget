// Package optimize turns a cost report into an optimization plan. Build is a
// pure function: it reads the report the kernel already produced plus the
// machine types a caller offers, and never touches a cluster, a price catalog
// or the file system.
//
// Findings are not independent: removing a node, rightsizing the pods on it
// and moving it to spot all spend the same money. The plan therefore applies
// its steps in order to one simulated cluster, and each recommendation claims
// only the saving it adds on top of the steps before it. The total is the
// combined scenario, never a sum of overlapping findings.
package optimize

import (
	"kube-budget/core/costmodel"
)

// Category groups recommendations by where the change is made.
type Category string

const (
	// CategoryData is a problem with the input that makes other figures unreliable.
	CategoryData      Category = "data"
	CategoryWorkloads Category = "workloads"
	CategoryNodes     Category = "nodes"
	CategoryStorage   Category = "storage"
)

// Level grades the effort or the risk of applying a recommendation.
type Level string

const (
	LevelLow    Level = "low"
	LevelMedium Level = "medium"
	LevelHigh   Level = "high"
)

// MachineType is one machine the node-type step may propose.
type MachineType struct {
	Name      string
	VCPU      float64
	MemoryGB  float64
	GPUUnits  float64
	HourlyUSD float64
	// Burstable machines throttle under sustained load and are never proposed.
	Burstable bool
}

// Inputs is what the plan needs beyond the report.
type Inputs struct {
	// MachineTypes are the candidates for the node-type step, on-demand prices.
	MachineTypes []MachineType
	// SpotPriceRatio is the spot price as a fraction of on-demand; zero
	// disables the spot step.
	SpotPriceRatio float64
	// Dismissed recommendation IDs stay listed but leave the scenario.
	Dismissed map[string]bool
}

// Item is one resource a recommendation touches, with the exact change.
type Item struct {
	Subject costmodel.Subject    `json:"subject"`
	Change  string               `json:"change"`
	Savings costmodel.Projection `json:"savings"`
	Command string               `json:"command,omitempty"`
	Note    string               `json:"note,omitempty"`
}

// Recommendation is one step of the plan, or one data issue.
type Recommendation struct {
	ID         string               `json:"id"`
	Category   Category             `json:"category"`
	Title      string               `json:"title"`
	Effort     Level                `json:"effort"`
	Risk       Level                `json:"risk"`
	Confidence costmodel.Confidence `json:"confidence"`
	// Savings is the billed saving this step adds to the plan.
	Savings costmodel.Projection `json:"savings"`
	// FreedRequests is requested cost released without a billed saving yet;
	// it becomes billed savings only when nodes can be removed.
	FreedRequests costmodel.Projection `json:"freedRequests"`
	Rationale     string               `json:"rationale"`
	Action        string               `json:"action"`
	Dismissed     bool                 `json:"dismissed"`
	Items         []Item               `json:"items,omitempty"`
}

// Step is one bar of the savings waterfall.
type Step struct {
	ID      string               `json:"id"`
	Title   string               `json:"title"`
	Savings costmodel.Projection `json:"savings"`
}

// Plan is the combined optimization scenario for one report.
type Plan struct {
	Current         costmodel.Projection   `json:"current"`
	Optimized       costmodel.Projection   `json:"optimized"`
	Savings         costmodel.Projection   `json:"savings"`
	Steps           []Step                 `json:"steps"`
	DataIssues      []Recommendation       `json:"dataIssues"`
	Recommendations []Recommendation       `json:"recommendations"`
	Assumptions     []costmodel.Assumption `json:"assumptions"`
}

// Build simulates every step in order and returns the plan.
func Build(report costmodel.CostReport, inputs Inputs) Plan {
	current := report.Totals[costmodel.BasisProvisioned]
	plan := Plan{
		Current:         current,
		Steps:           make([]Step, 0),
		DataIssues:      dataIssues(report),
		Recommendations: make([]Recommendation, 0),
		Assumptions:     planAssumptions,
	}

	scenario := newScenario(report, inputs)
	saved := 0.0
	for _, step := range planSteps {
		recommendation, next := step(scenario)
		if recommendation == nil {
			continue
		}
		recommendation.Dismissed = inputs.Dismissed[recommendation.ID]
		if !recommendation.Dismissed {
			scenario = next
			if recommendation.Savings.Hourly > 0 {
				saved += recommendation.Savings.Hourly
				plan.Steps = append(plan.Steps, Step{ID: recommendation.ID, Title: recommendation.Title, Savings: recommendation.Savings})
			}
		}
		plan.Recommendations = append(plan.Recommendations, *recommendation)
	}

	plan.Savings = costmodel.Project(saved)
	plan.Optimized = current.Sub(plan.Savings)
	return plan
}

var planAssumptions = []costmodel.Assumption{
	{Key: "plan-order", Detail: "steps are simulated in order and each claims only the saving it adds, so the total never counts the same node twice"},
	{Key: "plan-node-pool", Detail: "node steps model the cluster as one pool of its most common machine type, with 15% headroom and at least two nodes"},
	{Key: "plan-placement", Detail: "affinity, zones, taints and disruption budgets are not modeled; check them before removing nodes"},
}
