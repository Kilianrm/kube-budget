// Package whatif answers what deploying a workload does to a cluster's bill.
//
// A workload's own price (its requests times a rate) is the same on any
// cluster. What it adds to the bill is not: if it fits in capacity the cluster
// already pays for, the bill does not move and idle capacity shrinks; if it
// does not, the cluster needs more nodes and pays for whole machines.
//
// Simulate places the new replicas the way the scheduler would, adds nodes of
// the cluster's usual machine type only for what does not fit, then rebuilds
// the report and the optimization plan through the same code as the rest of
// the application and compares them with today's.
package whatif

import (
	"fmt"
	"sort"

	"kube-budget/core/costmodel"
	"kube-budget/core/optimize"
	"kube-budget/core/pricing"
)

// Workload is the deployment being simulated.
type Workload struct {
	Name       string          `json:"name"`
	Namespace  string          `json:"namespace"`
	Kind       string          `json:"kind"`
	Replicas   int             `json:"replicas"`
	PerReplica costmodel.Usage `json:"perReplica"`
	Containers int             `json:"containers"`
}

// Inputs is what the simulation needs beyond the report.
type Inputs struct {
	// RateCard prices the workload's requests: the rates of the machine type
	// the cluster mostly runs on, the same rates its own workloads get.
	RateCard pricing.RateCard
	// Plan is passed through to the optimization plan before and after.
	Plan optimize.Inputs
}

// Placement is where the simulated replicas land.
type Placement struct {
	Node     string `json:"node"`
	Replicas int    `json:"replicas"`
	// New marks a node the cluster would have to add.
	New bool `json:"new"`
}

// PlanChange is one recommendation whose saving the workload changes.
type PlanChange struct {
	ID     string               `json:"id"`
	Title  string               `json:"title"`
	Before costmodel.Projection `json:"before"`
	After  costmodel.Projection `json:"after"`
}

// Result is the impact of deploying the workload.
type Result struct {
	Workload Workload `json:"workload"`
	// Fits is true when every replica lands on nodes the cluster already has.
	Fits          bool        `json:"fits"`
	Placements    []Placement `json:"placements"`
	NewNodes      int         `json:"newNodes"`
	NewNodeType   string      `json:"newNodeType,omitempty"`
	Unschedulable int         `json:"unschedulable"`
	// Requested is the workload's requests at the cluster's rates: the share of
	// capacity it consumes, whether or not the bill moves.
	Requested    costmodel.Projection `json:"requested"`
	BilledBefore costmodel.Projection `json:"billedBefore"`
	BilledAfter  costmodel.Projection `json:"billedAfter"`
	// BilledDelta is what the workload adds to the bill; it can be zero.
	BilledDelta       costmodel.Projection   `json:"billedDelta"`
	IdleBefore        costmodel.Projection   `json:"idleBefore"`
	IdleAfter         costmodel.Projection   `json:"idleAfter"`
	PlanSavingsBefore costmodel.Projection   `json:"planSavingsBefore"`
	PlanSavingsAfter  costmodel.Projection   `json:"planSavingsAfter"`
	PlanChanges       []PlanChange           `json:"planChanges"`
	Warnings          []string               `json:"warnings"`
	Assumptions       []costmodel.Assumption `json:"assumptions"`
}

var assumptions = []costmodel.Assumption{
	{Key: "whatif-scheduler", Detail: "replicas go to the untainted node with the most free capacity, by requests, as the scheduler's default scoring does"},
	{Key: "whatif-taints", Detail: "nodes with NoSchedule or NoExecute taints are skipped, because the manifest's tolerations are not read"},
	{Key: "whatif-new-nodes", Detail: "replicas that fit nowhere add nodes of the cluster's most common untainted machine type at its current price, each already carrying the cluster's DaemonSets"},
	{Key: "whatif-affinity", Detail: "affinity, topology spread and disruption budgets are not modeled"},
}

// Simulate deploys the workload on a copy of the cluster and compares.
func Simulate(report costmodel.CostReport, workload Workload, inputs Inputs) Result {
	if workload.Namespace == "" {
		workload.Namespace = "default"
	}
	result := Result{
		Workload:    workload,
		Placements:  make([]Placement, 0),
		PlanChanges: make([]PlanChange, 0),
		Warnings:    make([]string, 0),
		Assumptions: assumptions,
	}

	pool, estimatedFree := schedulableNodes(report)
	if estimatedFree {
		result.Assumptions = append(result.Assumptions, costmodel.Assumption{
			Key:    "whatif-free-capacity",
			Detail: "pod placement was not collected, so each node's free capacity is its share of the cluster's free capacity",
		})
	}
	template, hasTemplate := newNodeTemplate(pool, daemonSetOverhead(report))

	noRequests := workload.PerReplica.CPUCores <= 0 && workload.PerReplica.MemoryGB <= 0
	if noRequests {
		result.Warnings = append(result.Warnings, "The manifest declares no CPU or memory requests: the scheduler can place it anywhere and its cost cannot be attributed.")
	}

	placed := make(map[string]*Placement)
	order := make([]string, 0)
	newNodes := make([]candidate, 0)
	for replica := 0; replica < workload.Replicas; replica++ {
		target := bestFit(pool, workload.PerReplica)
		if target == nil {
			if !hasTemplate || !fitsIn(template.free, workload.PerReplica) {
				result.Unschedulable++
				continue
			}
			node := template
			node.name = fmt.Sprintf("new %s #%d", template.machineType, len(newNodes)+1)
			node.isNew = true
			pool = append(pool, node)
			newNodes = append(newNodes, node)
			target = &pool[len(pool)-1]
		}
		target.free = target.free.Add(workload.PerReplica.Scale(-1))
		if placed[target.name] == nil {
			placed[target.name] = &Placement{Node: target.name, New: target.isNew}
			order = append(order, target.name)
		}
		placed[target.name].Replicas++
	}
	for _, name := range order {
		result.Placements = append(result.Placements, *placed[name])
	}
	result.NewNodes = len(newNodes)
	if len(newNodes) > 0 {
		result.NewNodeType = template.machineType
	}
	result.Fits = result.NewNodes == 0 && result.Unschedulable == 0
	if result.Unschedulable > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d %s larger than any node the cluster can add; %s stay Pending.",
			result.Unschedulable, plural(result.Unschedulable, "replica is", "replicas are"), plural(result.Unschedulable, "it would", "they would")))
	}

	scheduled := workload.Replicas - result.Unschedulable
	workloadItem := workloadLineItem(workload, scheduled, inputs.RateCard, noRequests)
	result.Requested = costmodel.Project(workloadItem.HourlyUSD)

	items := append([]costmodel.LineItem(nil), report.Items...)
	for index, node := range newNodes {
		items = append(items, node.lineItem(index))
	}
	items = append(items, workloadItem)
	after := costmodel.NewReport(report.GeneratedAt, report.Scope, items, nil)

	result.BilledBefore = report.Totals[costmodel.BasisProvisioned]
	result.BilledAfter = after.Totals[costmodel.BasisProvisioned]
	result.BilledDelta = costmodel.Project(result.BilledAfter.Hourly - result.BilledBefore.Hourly)
	result.IdleBefore = report.Idle
	result.IdleAfter = after.Idle

	planBefore := optimize.Build(report, inputs.Plan)
	planAfter := optimize.Build(after, inputs.Plan)
	result.PlanSavingsBefore = planBefore.Savings
	result.PlanSavingsAfter = planAfter.Savings
	result.PlanChanges = planChanges(planBefore, planAfter)
	return result
}

// candidate is a node a new replica may land on.
type candidate struct {
	name        string
	machineType string
	parent      string
	purchase    string
	capacity    costmodel.Usage
	free        costmodel.Usage
	hourly      float64
	components  map[costmodel.Component]float64
	confidence  costmodel.Confidence
	isNew       bool
}

// schedulableNodes lists priced, usable, untainted nodes with their free
// capacity. Without per-node requests, free capacity is spread in proportion
// to allocatable, and the second result is true.
func schedulableNodes(report costmodel.CostReport) ([]candidate, bool) {
	pool := make([]candidate, 0)
	estimated := false
	for _, item := range report.Items {
		if item.Subject.Kind != costmodel.SubjectNode || !item.Priced() || !usable(item) {
			continue
		}
		if taints, _ := item.Detail["taints"].([]string); len(taints) > 0 {
			continue
		}
		node := candidate{
			name:        item.Subject.Name,
			machineType: detailString(item, "machineType"),
			parent:      item.Subject.ParentID,
			purchase:    detailString(item, "purchase"),
			capacity:    item.Usage,
			hourly:      item.HourlyUSD,
			components:  item.Components,
			confidence:  item.Confidence,
		}
		cores, hasCores := item.Detail["requestedCores"].(float64)
		memory, hasMemory := item.Detail["requestedMemoryGB"].(float64)
		if hasCores && hasMemory {
			node.free = costmodel.Usage{CPUCores: item.Usage.CPUCores - cores, MemoryGB: item.Usage.MemoryGB - memory, GPUUnits: item.Usage.GPUUnits}
		} else {
			estimated = true
		}
		pool = append(pool, node)
	}

	if estimated {
		var capacity, demand costmodel.Usage
		for _, node := range pool {
			capacity = capacity.Add(node.capacity)
		}
		for _, item := range report.Items {
			if item.Basis == costmodel.BasisRequested {
				demand = demand.Add(item.Usage)
			}
		}
		for index := range pool {
			pool[index].free = costmodel.Usage{
				CPUCores: pool[index].capacity.CPUCores * freeShare(demand.CPUCores, capacity.CPUCores),
				MemoryGB: pool[index].capacity.MemoryGB * freeShare(demand.MemoryGB, capacity.MemoryGB),
				GPUUnits: pool[index].capacity.GPUUnits,
			}
		}
	}
	return pool, estimated
}

// newNodeTemplate is an empty node of the pool's most common machine type,
// priced at what the cluster pays for that type on average.
func newNodeTemplate(pool []candidate, overhead costmodel.Usage) (candidate, bool) {
	counts := make(map[string]int)
	for _, node := range pool {
		counts[node.machineType]++
	}
	best, bestCount := "", 0
	for machineType, count := range counts {
		if count > bestCount || (count == bestCount && machineType < best) {
			best, bestCount = machineType, count
		}
	}
	if bestCount == 0 {
		return candidate{}, false
	}

	var template candidate
	var capacity costmodel.Usage
	hourly := 0.0
	for _, node := range pool {
		if node.machineType != best {
			continue
		}
		if template.name == "" {
			template = node
		}
		capacity = capacity.Add(node.capacity)
		hourly += node.hourly
	}
	count := float64(bestCount)
	template.capacity = capacity.Scale(1 / count)
	// A new node starts with the cluster's DaemonSet pods already on it.
	template.free = costmodel.Usage{
		CPUCores: max(template.capacity.CPUCores-overhead.CPUCores, 0),
		MemoryGB: max(template.capacity.MemoryGB-overhead.MemoryGB, 0),
		GPUUnits: template.capacity.GPUUnits,
	}
	template.hourly = hourly / count
	return template, true
}

// daemonSetOverhead is what the DaemonSets request on every node.
func daemonSetOverhead(report costmodel.CostReport) costmodel.Usage {
	var overhead costmodel.Usage
	for _, item := range report.Items {
		if item.Basis != costmodel.BasisRequested || detailString(item, "kind") != "DaemonSet" {
			continue
		}
		replicas, _ := item.Detail["replicas"].(float64)
		if replicas < 1 {
			replicas = 1
		}
		overhead = overhead.Add(item.Usage.Scale(1 / replicas))
	}
	return overhead
}

// bestFit returns the node with the most free capacity that fits the replica,
// measured as the smaller of its free CPU and free memory shares.
func bestFit(pool []candidate, replica costmodel.Usage) *candidate {
	var best *candidate
	bestScore := -1.0
	for index := range pool {
		node := &pool[index]
		if !fitsIn(node.free, replica) {
			continue
		}
		score := minShare(node.free, node.capacity)
		if score > bestScore {
			best, bestScore = node, score
		}
	}
	return best
}

func fitsIn(free, replica costmodel.Usage) bool {
	const tolerance = 1e-9
	return replica.CPUCores <= free.CPUCores+tolerance &&
		replica.MemoryGB <= free.MemoryGB+tolerance &&
		replica.GPUUnits <= free.GPUUnits+tolerance
}

func minShare(free, capacity costmodel.Usage) float64 {
	share := 1.0
	if capacity.CPUCores > 0 {
		share = free.CPUCores / capacity.CPUCores
	}
	if capacity.MemoryGB > 0 && free.MemoryGB/capacity.MemoryGB < share {
		share = free.MemoryGB / capacity.MemoryGB
	}
	return share
}

func freeShare(demand, capacity float64) float64 {
	if capacity <= 0 {
		return 0
	}
	share := 1 - demand/capacity
	if share < 0 {
		return 0
	}
	return share
}

// lineItem prices an added node like the nodes of its type already are, with
// the same split between CPU and memory.
func (node candidate) lineItem(index int) costmodel.LineItem {
	components := make(map[costmodel.Component]float64, len(node.components))
	total := 0.0
	for _, value := range node.components {
		total += value
	}
	for component, value := range node.components {
		if total > 0 {
			components[component] = node.hourly * value / total
		}
	}
	return costmodel.LineItem{
		Subject:    costmodel.Subject{Kind: costmodel.SubjectNode, ID: fmt.Sprintf("whatif-node-%d", index+1), Name: node.name, ParentID: node.parent},
		Basis:      costmodel.BasisProvisioned,
		Usage:      node.capacity,
		HourlyUSD:  node.hourly,
		Components: components,
		Confidence: costmodel.WeakerConfidence(node.confidence, costmodel.ConfidenceEstimated),
		Detail: map[string]interface{}{
			"machineType": node.machineType,
			"purchase":    node.purchase,
			"ready":       true,
			"schedulable": true,
		},
	}
}

func workloadLineItem(workload Workload, replicas int, card pricing.RateCard, noRequests bool) costmodel.LineItem {
	usage := workload.PerReplica.Scale(float64(replicas))
	item := costmodel.LineItem{
		Subject: costmodel.Subject{Kind: costmodel.SubjectWorkload, ID: "whatif-" + workload.Namespace + "/" + workload.Name, Name: workload.Name, Namespace: workload.Namespace},
		Basis:   costmodel.BasisRequested,
		Usage:   usage,
		Detail: map[string]interface{}{
			"kind":       workload.Kind,
			"replicas":   float64(replicas),
			"containers": workload.Containers,
		},
	}
	if noRequests {
		item.Confidence = costmodel.ConfidenceUnknown
		item.Detail["missingRequests"] = true
		return item
	}
	if usage.GPUUnits > 0 && card.GPUUSDPerUnitHour == 0 {
		item.Confidence = costmodel.ConfidenceUnknown
		return item
	}

	item.Components = map[costmodel.Component]float64{}
	add := func(component costmodel.Component, cost float64) {
		if cost > 0 {
			item.Components[component] = cost
			item.HourlyUSD += cost
		}
	}
	add(costmodel.ComponentCPU, usage.CPUCores*card.CPUUSDPerCoreHour)
	add(costmodel.ComponentMemory, usage.MemoryGB*card.MemoryUSDPerGBHour)
	add(costmodel.ComponentGPU, usage.GPUUnits*card.GPUUSDPerUnitHour)
	item.Confidence = card.Confidence
	if item.Confidence == "" {
		item.Confidence = costmodel.ConfidenceDerived
	}
	return item
}

// planChanges lists the active recommendations whose saving moved.
func planChanges(before, after optimize.Plan) []PlanChange {
	savings := func(plan optimize.Plan) map[string]optimize.Recommendation {
		active := make(map[string]optimize.Recommendation)
		for _, recommendation := range plan.Recommendations {
			if !recommendation.Dismissed {
				active[recommendation.ID] = recommendation
			}
		}
		return active
	}
	previous, next := savings(before), savings(after)

	ids := make([]string, 0, len(previous)+len(next))
	for id := range previous {
		ids = append(ids, id)
	}
	for id := range next {
		if _, ok := previous[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	changes := make([]PlanChange, 0)
	for _, id := range ids {
		was, had := previous[id]
		now, has := next[id]
		if had && has && abs(was.Savings.Hourly-now.Savings.Hourly) < 1e-9 {
			continue
		}
		change := PlanChange{ID: id, Before: was.Savings, After: now.Savings}
		change.Title = now.Title
		if !has {
			change.Title = was.Title
		}
		changes = append(changes, change)
	}
	return changes
}

func usable(item costmodel.LineItem) bool {
	ready, hasReady := item.Detail["ready"].(bool)
	schedulable, hasSchedulable := item.Detail["schedulable"].(bool)
	return (!hasReady || ready) && (!hasSchedulable || schedulable)
}

func detailString(item costmodel.LineItem, key string) string {
	value, _ := item.Detail[key].(string)
	return value
}

func plural(count int, singular, pluralForm string) string {
	if count == 1 {
		return singular
	}
	return pluralForm
}

func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
