package optimize

import (
	"fmt"
	"math"

	"kube-budget/core/costmodel"
)

// nodePause explains why node steps wait: a node recommendation being
// applied, or a node change the cluster shows under way.
func nodePause(inputs Inputs) string {
	for _, step := range planSteps {
		if frozen, ok := inputs.Applying[step.id]; ok && step.category == CategoryNodes {
			return fmt.Sprintf("%q is being applied", frozen.Title)
		}
	}
	return inputs.NodeTransition
}

// applied shows a recommendation being applied: frozen as it was when the
// user started, with its progress read from the cluster. Items that are
// changes of their own are done once the live plan no longer lists them;
// node changes are checked against their target.
func applied(frozen Recommendation, live *Recommendation, report costmodel.CostReport, inputs Inputs) Recommendation {
	shown := frozen
	shown.Applying = true
	shown.Dismissed = false
	shown.Items = append([]Item(nil), frozen.Items...)
	shown.Milestones = nil

	switch {
	case frozen.SeparateItems:
		listed := make(map[string]bool)
		if live != nil {
			for _, item := range live.Items {
				listed[itemKey(item)] = true
			}
		}
		shown.Complete = len(shown.Items) > 0
		for index := range shown.Items {
			shown.Items[index].Done = !listed[itemKey(shown.Items[index])]
			if !shown.Items[index].Done {
				shown.Complete = false
			}
		}
	case frozen.Target != nil:
		shown.Milestones = milestones(frozen, report, inputs)
		shown.Complete = len(shown.Milestones) > 0
		for _, milestone := range shown.Milestones {
			if !milestone.Done {
				shown.Complete = false
			}
		}
	default:
		shown.Complete = live == nil
	}
	return shown
}

// remainingSavings is the part of a frozen recommendation's saving not made
// yet: the items still to do, or all of it until a node change is complete.
func remainingSavings(shown Recommendation) costmodel.Projection {
	if !shown.SeparateItems {
		return shown.Savings
	}
	total, remaining := 0.0, 0.0
	for _, item := range shown.Items {
		total += item.Savings.Hourly
		if !item.Done {
			remaining += item.Savings.Hourly
		}
	}
	if total <= 0 {
		return shown.Savings
	}
	return costmodel.Project(shown.Savings.Hourly * remaining / total)
}

func itemKey(item Item) string {
	return string(item.Subject.Kind) + "/" + item.Subject.Namespace + "/" + item.Subject.Name
}

// milestones checks a node change against the cluster. A change that names
// its node groups is checked group by group, with the EKS status of the new
// one; otherwise it is checked by machine type and purchase option.
func milestones(frozen Recommendation, report costmodel.CostReport, inputs Inputs) []Milestone {
	target := *frozen.Target
	nodes := make([]costmodel.LineItem, 0)
	for _, item := range report.Items {
		if item.Subject.Kind == costmodel.SubjectNode {
			nodes = append(nodes, item)
		}
	}
	count := func(match func(costmodel.LineItem) bool) int {
		total := 0
		for _, node := range nodes {
			if match(node) {
				total++
			}
		}
		return total
	}
	inGroup := func(name string) func(costmodel.LineItem) bool {
		return func(node costmodel.LineItem) bool { return node.Subject.ParentID == name }
	}
	onDemandOf := func(machineType string, usableOnly bool) func(costmodel.LineItem) bool {
		return func(node costmodel.LineItem) bool {
			return detailString(node, "machineType") == machineType && detailString(node, "purchase") != "spot" && (!usableOnly || isUsable(node))
		}
	}

	switch frozen.ID {
	case "consolidate-nodes":
		if target.Group != "" {
			group := eksGroup(inputs, target.Group)
			done := count(inGroup(target.Group)) <= target.Nodes && (group == nil || group.Desired <= target.Nodes)
			return []Milestone{{Label: fmt.Sprintf("%s runs %s", target.Group, plural(target.Nodes, "node")), Done: done}}
		}
		usable := count(func(node costmodel.LineItem) bool {
			return isUsable(node) && detailString(node, "purchase") != "spot"
		})
		return []Milestone{{Label: fmt.Sprintf("The cluster runs %s on demand", plural(target.Nodes, "node")), Done: usable <= target.Nodes}}

	case "node-type":
		if target.NewGroup != "" {
			created := eksGroup(inputs, target.NewGroup)
			usable := count(func(node costmodel.LineItem) bool { return inGroup(target.Group)(node) && isUsable(node) })
			return []Milestone{
				{Label: fmt.Sprintf("%s is active", target.NewGroup), Done: created != nil && created.Status == "ACTIVE"},
				{Label: fmt.Sprintf("%s's nodes are cordoned", target.Group), Done: usable == 0},
				{Label: fmt.Sprintf("%s is deleted", target.Group), Done: count(inGroup(target.Group)) == 0 && eksGroup(inputs, target.Group) == nil},
			}
		}
		return []Milestone{
			{Label: fmt.Sprintf("%d %s nodes run", target.Nodes, target.MachineType), Done: count(onDemandOf(target.MachineType, true)) >= target.Nodes},
			{Label: fmt.Sprintf("No %s nodes are left", target.FromMachineType), Done: count(onDemandOf(target.FromMachineType, false)) == 0},
		}

	case "spot-capacity":
		// With commands, the end state is what they do, in whole nodes and
		// keeping on-demand room for StatefulSets. The plan's spot share is a
		// fraction no node count may reach.
		if target.NewGroup != "" {
			created := eksGroup(inputs, target.NewGroup)
			shrunk := eksGroup(inputs, target.Group)
			// changes frozen before the spot node count was kept need one
			spotNodes := max(target.SpotNodes, 1)
			usableIn := func(name string) int {
				return count(func(node costmodel.LineItem) bool { return inGroup(name)(node) && isUsable(node) })
			}
			return []Milestone{
				{Label: fmt.Sprintf("%s is active", target.NewGroup), Done: created != nil && created.Status == "ACTIVE"},
				{Label: fmt.Sprintf("%s runs %s", target.NewGroup, plural(spotNodes, "spot node")), Done: usableIn(target.NewGroup) >= spotNodes},
				{Label: fmt.Sprintf("%s runs %s", target.Group, plural(target.Nodes, "on-demand node")),
					Done: count(inGroup(target.Group)) <= target.Nodes && (shrunk == nil || shrunk.Desired <= target.Nodes)},
			}
		}
		usable := usableNodes(report)
		spot := 0
		for _, node := range usable {
			if detailString(node, "purchase") == "spot" {
				spot++
			}
		}
		needed := int(math.Round(target.SpotShare * float64(len(usable))))
		// one node stays on demand, for the StatefulSets
		if len(usable) > 1 {
			needed = min(needed, len(usable)-1)
		}
		label := fmt.Sprintf("Spot runs %s of %d", plural(needed, "node"), len(usable))
		return []Milestone{{Label: label, Done: len(usable) > 0 && spot >= needed}}
	}
	return nil
}

func eksGroup(inputs Inputs, name string) *EKSNodeGroup {
	if inputs.EKS == nil {
		return nil
	}
	for index := range inputs.EKS.NodeGroups {
		if inputs.EKS.NodeGroups[index].Name == name {
			return &inputs.EKS.NodeGroups[index]
		}
	}
	return nil
}

// IsNodeStep reports whether a recommendation ID belongs to a node step, the
// ones that pause while a node change is under way.
func IsNodeStep(id string) bool {
	for _, step := range planSteps {
		if step.id == id {
			return step.category == CategoryNodes
		}
	}
	return false
}

// temporaryCosts are the nodes a node change in progress added beside the
// ones it replaces or shrinks: billed today, gone once it finishes.
func temporaryCosts(report costmodel.CostReport, inputs Inputs) []Temporary {
	temporary := make([]Temporary, 0)
	for _, step := range planSteps {
		frozen, ok := inputs.Applying[step.id]
		if !ok || frozen.Target == nil || frozen.Target.NewGroup == "" {
			continue
		}
		target := frozen.Target
		added, old, hourly := 0, 0, 0.0
		for _, item := range report.Items {
			if item.Subject.Kind != costmodel.SubjectNode {
				continue
			}
			switch item.Subject.ParentID {
			case target.NewGroup:
				added++
				hourly += item.HourlyUSD
			case target.Group:
				old++
			}
		}
		// A replaced group overlaps until it is deleted; a shrunk one until it
		// is down to its target.
		overlapping := old > target.Nodes
		if frozen.ID == "node-type" {
			overlapping = old > 0
		}
		if added == 0 || !overlapping {
			continue
		}
		temporary = append(temporary, Temporary{
			Label: fmt.Sprintf("%s of %s running beside %s until %q finishes", plural(added, "node"), target.NewGroup, target.Group, frozen.Title),
			Cost:  costmodel.Project(hourly),
		})
	}
	return temporary
}
