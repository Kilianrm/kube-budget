// Package optimize derives recommendations from a cost report. Analyze is a
// pure function: it reads the report the kernel already produced and never
// touches a cluster, a price catalog or the file system.
package optimize

import (
	"fmt"
	"math"
	"sort"

	"kube-budget/core/costmodel"
)

// Severity orders recommendations. Blockers come first because they describe
// data problems that make every other figure unreliable.
type Severity string

const (
	SeverityBlocker Severity = "blocker"
	SeverityHigh    Severity = "high"
	SeverityMedium  Severity = "medium"
	SeverityLow     Severity = "low"
	SeverityInfo    Severity = "info"
)

var severityRank = map[Severity]int{
	SeverityBlocker: 0,
	SeverityHigh:    1,
	SeverityMedium:  2,
	SeverityLow:     3,
	SeverityInfo:    4,
}

// headroomFraction is the share of node capacity kept free when judging
// whether the cluster could run on fewer nodes.
const headroomFraction = 0.15

// Recommendation is one actionable finding with the money attached to it.
type Recommendation struct {
	ID         string                 `json:"id"`
	Rule       string                 `json:"rule"`
	Title      string                 `json:"title"`
	Severity   Severity               `json:"severity"`
	Count      int                    `json:"count"`
	Subjects   []costmodel.Subject    `json:"subjects,omitempty"`
	Current    costmodel.Projection   `json:"current"`
	Proposed   costmodel.Projection   `json:"proposed"`
	Savings    costmodel.Projection   `json:"savings"`
	Confidence costmodel.Confidence   `json:"confidence"`
	Rationale  string                 `json:"rationale"`
	Action     string                 `json:"action"`
	Detail     map[string]interface{} `json:"detail,omitempty"`
}

// Analyze applies every rule to a report and returns the findings ordered by
// severity and then by savings.
func Analyze(report costmodel.CostReport) []Recommendation {
	rules := []func(costmodel.CostReport) *Recommendation{
		ruleMissingRequests,
		ruleUnpricedSubjects,
		ruleUnusableNodes,
		ruleOrphanVolumes,
		ruleConsolidation,
		ruleSpotCandidates,
	}

	recommendations := make([]Recommendation, 0, len(rules))
	for _, rule := range rules {
		if recommendation := rule(report); recommendation != nil {
			recommendations = append(recommendations, *recommendation)
		}
	}

	sort.SliceStable(recommendations, func(i, j int) bool {
		if severityRank[recommendations[i].Severity] != severityRank[recommendations[j].Severity] {
			return severityRank[recommendations[i].Severity] < severityRank[recommendations[j].Severity]
		}
		return recommendations[i].Savings.Hourly > recommendations[j].Savings.Hourly
	})
	return recommendations
}

// ruleMissingRequests reports workloads whose cost cannot be attributed. It
// yields no savings: it is the prerequisite for trusting every other rule.
func ruleMissingRequests(report costmodel.CostReport) *Recommendation {
	items := filter(report, func(item costmodel.LineItem) bool {
		return item.Subject.Kind == costmodel.SubjectWorkload && !item.Priced()
	})
	if len(items) == 0 {
		return nil
	}

	return &Recommendation{
		ID:         "missing-requests",
		Rule:       "missing-requests",
		Title:      fmt.Sprintf("%s without resource requests", plural(len(items), "workload")),
		Severity:   SeverityBlocker,
		Count:      len(items),
		Subjects:   subjectsOf(items),
		Confidence: costmodel.ConfidenceExact,
		Rationale:  "Their cost cannot be attributed, so the idle figure is overstated and every saving below is a lower bound.",
		Action:     "Declare resources.requests for these workloads, then re-run the report.",
	}
}

func ruleUnpricedSubjects(report costmodel.CostReport) *Recommendation {
	items := filter(report, func(item costmodel.LineItem) bool {
		return item.Subject.Kind != costmodel.SubjectWorkload && !item.Priced()
	})
	if len(items) == 0 {
		return nil
	}

	return &Recommendation{
		ID:         "unpriced-subjects",
		Rule:       "unpriced-subjects",
		Title:      fmt.Sprintf("%s missing a price", plural(len(items), "resource")),
		Severity:   SeverityBlocker,
		Count:      len(items),
		Subjects:   subjectsOf(items),
		Confidence: costmodel.ConfidenceExact,
		Rationale:  "These resources are excluded from every total, so the billed figure is understated.",
		Action:     "Add the missing SKUs to the price catalog for this region.",
	}
}

func ruleUnusableNodes(report costmodel.CostReport) *Recommendation {
	items := filter(report, func(item costmodel.LineItem) bool {
		if item.Subject.Kind != costmodel.SubjectNode || !item.Priced() {
			return false
		}
		return isFalse(item.Detail, "ready") || isFalse(item.Detail, "schedulable")
	})
	if len(items) == 0 {
		return nil
	}

	current := sum(items)
	return &Recommendation{
		ID:         "unusable-nodes",
		Rule:       "idle-node",
		Title:      fmt.Sprintf("%s billed but not usable", plural(len(items), "node")),
		Severity:   SeverityHigh,
		Count:      len(items),
		Subjects:   subjectsOf(items),
		Current:    current,
		Savings:    current,
		Confidence: lowestConfidence(items),
		Rationale:  "Nodes that are not ready or are cordoned still bill at the full instance rate while running nothing schedulable.",
		Action:     "Recover or terminate these nodes.",
	}
}

func ruleOrphanVolumes(report costmodel.CostReport) *Recommendation {
	items := filter(report, func(item costmodel.LineItem) bool {
		if item.Subject.Kind != costmodel.SubjectVolume || !item.Priced() {
			return false
		}
		status, _ := item.Detail["status"].(string)
		return status != "" && status != "Bound"
	})
	if len(items) == 0 {
		return nil
	}

	current := sum(items)
	return &Recommendation{
		ID:         "orphan-volumes",
		Rule:       "orphan-volume",
		Title:      fmt.Sprintf("%s not bound to a workload", plural(len(items), "volume")),
		Severity:   SeverityMedium,
		Count:      len(items),
		Subjects:   subjectsOf(items),
		Current:    current,
		Savings:    current,
		Confidence: lowestConfidence(items),
		Rationale:  "Provisioned storage is billed whether or not a pod ever mounts it.",
		Action:     "Delete the claims that are no longer needed.",
	}
}

// ruleConsolidation checks whether the requested capacity would fit on fewer
// nodes. It stays silent when any workload lacks requests, because the demand
// side would be incomplete and the answer would be wrong.
func ruleConsolidation(report costmodel.CostReport) *Recommendation {
	if ruleMissingRequests(report) != nil {
		return nil
	}

	nodes := filter(report, func(item costmodel.LineItem) bool {
		return item.Subject.Kind == costmodel.SubjectNode && item.Priced()
	})
	if len(nodes) < 2 || len(nodes) != countNodes(report) {
		return nil
	}

	var capacity costmodel.Usage
	for _, node := range nodes {
		capacity = capacity.Add(node.Usage)
	}
	perNodeCPU := capacity.CPUCores / float64(len(nodes))
	perNodeMemory := capacity.MemoryGB / float64(len(nodes))
	if perNodeCPU <= 0 || perNodeMemory <= 0 {
		return nil
	}

	var requested costmodel.Usage
	for _, item := range filter(report, func(item costmodel.LineItem) bool {
		return item.Basis == costmodel.BasisRequested && item.Priced()
	}) {
		requested = requested.Add(item.Usage)
	}

	usable := 1 - headroomFraction
	needed := math.Ceil(math.Max(requested.CPUCores/(perNodeCPU*usable), requested.MemoryGB/(perNodeMemory*usable)))
	removable := len(nodes) - int(math.Max(needed, 1))
	if removable <= 0 {
		return nil
	}

	current := sum(nodes)
	averageHourly := current.Hourly / float64(len(nodes))
	savings := costmodel.Project(averageHourly * float64(removable))

	return &Recommendation{
		ID:         "consolidation",
		Rule:       "consolidation",
		Title:      fmt.Sprintf("Requested capacity fits on %d fewer %s", removable, noun(removable, "node")),
		Severity:   SeverityHigh,
		Count:      removable,
		Current:    current,
		Proposed:   current.Sub(savings),
		Savings:    savings,
		Confidence: costmodel.ConfidenceEstimated,
		Rationale:  fmt.Sprintf("Requests total %.1f cores and %.1f GB, which fits on %d of %d nodes with %.0f%% headroom.", requested.CPUCores, requested.MemoryGB, int(needed), len(nodes), headroomFraction*100),
		Action:     "Reduce the node group's desired size, or let the cluster autoscaler reclaim the spare nodes.",
		Detail:     map[string]interface{}{"neededNodes": int(needed), "currentNodes": len(nodes)},
	}
}

// ruleSpotCandidates flags on-demand capacity without claiming a saving: the
// spot price is not part of the report and varies over time.
func ruleSpotCandidates(report costmodel.CostReport) *Recommendation {
	items := filter(report, func(item costmodel.LineItem) bool {
		if item.Subject.Kind != costmodel.SubjectNode || !item.Priced() {
			return false
		}
		purchase, _ := item.Detail["purchase"].(string)
		return purchase == "on-demand"
	})
	if len(items) == 0 {
		return nil
	}

	return &Recommendation{
		ID:         "spot-candidates",
		Rule:       "spot-candidate",
		Title:      fmt.Sprintf("%s running on on-demand capacity", plural(len(items), "node")),
		Severity:   SeverityLow,
		Count:      len(items),
		Subjects:   subjectsOf(items),
		Current:    sum(items),
		Confidence: costmodel.ConfidenceUnknown,
		Rationale:  "Fault-tolerant workloads can run on spot capacity, but the discount varies by instance type and region and is not part of this report.",
		Action:     "Move interruption-tolerant workloads to a spot node group and compare the two reports.",
	}
}

func filter(report costmodel.CostReport, keep func(costmodel.LineItem) bool) []costmodel.LineItem {
	matching := make([]costmodel.LineItem, 0)
	for _, item := range report.Items {
		if keep(item) {
			matching = append(matching, item)
		}
	}
	return matching
}

func countNodes(report costmodel.CostReport) int {
	count := 0
	for _, item := range report.Items {
		if item.Subject.Kind == costmodel.SubjectNode {
			count++
		}
	}
	return count
}

func sum(items []costmodel.LineItem) costmodel.Projection {
	total := 0.0
	for _, item := range items {
		total += item.HourlyUSD
	}
	return costmodel.Project(total)
}

func subjectsOf(items []costmodel.LineItem) []costmodel.Subject {
	subjects := make([]costmodel.Subject, 0, len(items))
	for _, item := range items {
		subjects = append(subjects, item.Subject)
	}
	return subjects
}

func lowestConfidence(items []costmodel.LineItem) costmodel.Confidence {
	lowest := costmodel.ConfidenceExact
	ranking := map[costmodel.Confidence]int{
		costmodel.ConfidenceExact:     0,
		costmodel.ConfidenceDerived:   1,
		costmodel.ConfidenceEstimated: 2,
		costmodel.ConfidenceUnknown:   3,
	}
	for _, item := range items {
		if ranking[item.Confidence] > ranking[lowest] {
			lowest = item.Confidence
		}
	}
	return lowest
}

func isFalse(detail map[string]interface{}, key string) bool {
	value, ok := detail[key].(bool)
	return ok && !value
}

func plural(count int, word string) string {
	return fmt.Sprintf("%d %s", count, noun(count, word))
}

func noun(count int, word string) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
