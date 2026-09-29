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
	"math"
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
	// EKS is set for an Amazon EKS cluster whose node groups are known, so a
	// node-count change comes with the exact command.
	EKS *EKSCluster
	// Applying holds the recommendations being applied, frozen as they were
	// when the user started, by ID. They replace the live ones until done.
	Applying map[string]Recommendation
	// NodeTransition explains a node change seen in the cluster itself, such
	// as a node group being created; node steps pause while it lasts.
	NodeTransition string
	// LastKnown holds each recommendation as the report before showed it, so
	// a paused step keeps showing its last value instead of disappearing.
	LastKnown map[string]Recommendation
}

// EKSCluster is what the EKS commands of the plan need.
type EKSCluster struct {
	Name       string
	Region     string
	NodeGroups []EKSNodeGroup
}

// EKSNodeGroup is a managed node group: its size, and what a replacement
// group needs to match it.
type EKSNodeGroup struct {
	Name          string
	Status        string
	Desired       int
	Min           int
	Max           int
	InstanceTypes []string
	CapacityType  string
	AmiType       string
	NodeRole      string
	Subnets       []string
	Labels        map[string]string
	// Taints are spelled for the AWS CLI: key=...,value=...,effect=...
	Taints                []string
	LaunchTemplateID      string
	LaunchTemplateVersion string
}

// Item is one resource a recommendation touches, with the exact change.
type Item struct {
	Subject costmodel.Subject    `json:"subject"`
	Change  string               `json:"change"`
	Savings costmodel.Projection `json:"savings"`
	Check   string               `json:"check,omitempty"`
	Command string               `json:"command,omitempty"`
	// Steps are commands to run in order, for a change one command cannot make.
	Steps []string `json:"steps,omitempty"`
	// Done is set on an item of a recommendation being applied once the
	// cluster no longer needs it.
	Done bool   `json:"done,omitempty"`
	Note string `json:"note,omitempty"`
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
	// SeparateItems is set when each item is a change of its own that can be
	// applied one at a time, so progress is tracked per item.
	SeparateItems bool   `json:"separateItems,omitempty"`
	Items         []Item `json:"items,omitempty"`
	// Target is the end state of a node change, checked while it is applied.
	Target *Target `json:"target,omitempty"`
	// Applying marks a recommendation frozen while the user applies it. Its
	// progress is read from the cluster: Done items, or Milestones.
	Applying bool `json:"applying,omitempty"`
	// Paused marks a node recommendation shown with its last known values
	// while a node change is under way; it cannot be started until then.
	Paused     bool        `json:"paused,omitempty"`
	Milestones []Milestone `json:"milestones,omitempty"`
	// Complete is set when the cluster shows the whole change made.
	Complete bool `json:"complete,omitempty"`
}

// Target is where a node change leaves the cluster.
type Target struct {
	// Group is the node group changed, when the command names one.
	Group    string `json:"group,omitempty"`
	NewGroup string `json:"newGroup,omitempty"`
	// Nodes is the node count the change ends with.
	Nodes           int     `json:"nodes,omitempty"`
	FromMachineType string  `json:"fromMachineType,omitempty"`
	MachineType     string  `json:"machineType,omitempty"`
	SpotShare       float64 `json:"spotShare,omitempty"`
	// SpotNodes is how many nodes the new spot group ends with.
	SpotNodes int `json:"spotNodes,omitempty"`
}

// Milestone is one checkable point of a change being applied.
type Milestone struct {
	Label string `json:"label"`
	Done  bool   `json:"done"`
}

// Step is one bar of the savings waterfall.
type Step struct {
	ID      string               `json:"id"`
	Title   string               `json:"title"`
	Savings costmodel.Projection `json:"savings"`
	// Status is "applying" for a change being applied: Savings is what is
	// left, and Achieved what the cluster already shows done. It is "paused"
	// for a node step waiting on a node change: Savings is its last known
	// value, shown but not counted in the plan.
	Status   string               `json:"status,omitempty"`
	Achieved costmodel.Projection `json:"achieved"`
	Note     string               `json:"note,omitempty"`
	// Change compares the step with the report before: "new", or "changed"
	// from Previous. The plan leaves it to the caller, which keeps reports.
	Change   string               `json:"change,omitempty"`
	Previous costmodel.Projection `json:"previous"`
}

// Temporary is a cost in today's bill that a change in progress adds only
// until it finishes, such as a new node group running beside the old one.
type Temporary struct {
	Label string               `json:"label"`
	Cost  costmodel.Projection `json:"cost"`
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
	// Paused explains why node recommendations wait: a node change is being
	// applied, or the cluster shows one under way.
	Paused string `json:"paused,omitempty"`
	// Temporary lists costs in today's bill that changes in progress add
	// until they finish.
	Temporary []Temporary `json:"temporary,omitempty"`
	// Gone lists steps the report before counted that this one has not,
	// with their saving then in Previous. The caller fills it.
	Gone []Step `json:"gone,omitempty"`
}

// Build simulates every step in order and returns the plan. A recommendation
// being applied is shown frozen instead of recomputed, and while a node change
// is under way the other node steps pause, since a cluster halfway through a
// change would make them propose conflicting ones.
func Build(report costmodel.CostReport, inputs Inputs) Plan {
	current := report.Totals[costmodel.BasisProvisioned]
	plan := Plan{
		Current:         current,
		Steps:           make([]Step, 0),
		DataIssues:      dataIssues(report),
		Recommendations: make([]Recommendation, 0),
		Assumptions:     planAssumptions,
		Paused:          nodePause(inputs),
	}

	scenario := newScenario(report, inputs)
	scenario.paused = plan.Paused
	saved := 0.0
	for _, step := range planSteps {
		frozen, applying := inputs.Applying[step.id]
		paused := plan.Paused != "" && step.category == CategoryNodes
		if paused && !applying {
			last, known := inputs.LastKnown[step.id]
			if !known {
				continue
			}
			last.Paused, last.Applying, last.Complete, last.Milestones = true, false, false, nil
			last.Dismissed = inputs.Dismissed[step.id]
			if !last.Dismissed && last.Savings.Hourly > 0 {
				plan.Steps = append(plan.Steps, Step{ID: last.ID, Title: last.Title, Savings: last.Savings, Status: "paused", Note: plan.Paused})
			}
			plan.Recommendations = append(plan.Recommendations, last)
			continue
		}
		var live *Recommendation
		next := scenario
		if !paused {
			live, next = step.run(scenario)
		}

		if applying {
			shown := applied(frozen, live, report, inputs)
			remaining := remainingSavings(shown)
			if shown.Complete {
				remaining = costmodel.Projection{}
			}
			achieved := costmodel.Project(math.Max(shown.Savings.Hourly-remaining.Hourly, 0))
			if remaining.Hourly > 0 || achieved.Hourly > 0 {
				saved += remaining.Hourly
				plan.Steps = append(plan.Steps, Step{ID: shown.ID, Title: shown.Title, Savings: remaining, Status: "applying", Achieved: achieved})
			}
			scenario = next
			plan.Recommendations = append(plan.Recommendations, shown)
			continue
		}
		if live == nil {
			continue
		}
		live.Dismissed = inputs.Dismissed[live.ID]
		if !live.Dismissed {
			scenario = next
			if live.Savings.Hourly > 0 {
				saved += live.Savings.Hourly
				plan.Steps = append(plan.Steps, Step{ID: live.ID, Title: live.Title, Savings: live.Savings})
			}
		}
		plan.Recommendations = append(plan.Recommendations, *live)
	}

	plan.Savings = costmodel.Project(saved)
	plan.Optimized = current.Sub(plan.Savings)
	plan.Temporary = temporaryCosts(report, inputs)
	return plan
}

var planAssumptions = []costmodel.Assumption{
	{Key: "plan-order", Detail: "steps are simulated in order and each claims only the saving it adds, so the total never counts the same node twice"},
	{Key: "plan-node-pool", Detail: "node steps model the cluster as one pool of its most common machine type, with 15% headroom and at least two nodes"},
	{Key: "plan-placement", Detail: "affinity, zones, taints and disruption budgets are not modeled; check them before removing nodes"},
}
