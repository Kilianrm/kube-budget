package optimize

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"kube-budget/core/costmodel"
)

const (
	// headroomFraction is the share of node capacity kept free when judging
	// how many nodes the requests need.
	headroomFraction = 0.15
	// highAvailabilityNodes is the fewest nodes the plan ever proposes, unless
	// the cluster already runs on fewer.
	highAvailabilityNodes = 2
	// Rightsizing flags a resource used below rightsizeThreshold of its
	// request and suggests usage plus rightsizeHeadroom. Both are cautious
	// because usage is a single sample, not a peak.
	rightsizeThreshold = 0.5
	rightsizeHeadroom  = 0.5
	// Suggested requests never drop below these per-pod floors, the Vertical
	// Pod Autoscaler's defaults: a sample can read near zero for a pod that
	// still needs room to start and to serve its first requests.
	minimumPodCPUCores = 0.025
	minimumPodMemoryGB = 250.0 / 1024
	// minimumNodeTypeGain is the saving a different machine type must bring
	// before a migration is worth proposing.
	minimumNodeTypeGain = 0.05
)

// scenario is the simulated cluster the steps change one after another. It
// models the usable nodes as one pool of the dominant machine type.
type scenario struct {
	report costmodel.CostReport
	inputs Inputs
	// blocked explains why node-level steps cannot trust the demand.
	blocked string
	// paused explains why node steps wait for a change under way.
	paused     string
	nodes      int
	nodeHourly float64
	capacity   costmodel.Usage
	// demand is the total request, kept for explanations; placement decisions
	// use pods and perNode, because the scheduler places whole pods.
	demand costmodel.Usage
	// pods are the replicas of every priced workload except DaemonSets.
	pods []podDemand
	// perNode is what the DaemonSets request on every node, new ones included.
	perNode     costmodel.Usage
	machineType string
	spotShare   float64
}

// podDemand is one workload's replicas, all of the same size.
type podDemand struct {
	workload string
	perPod   costmodel.Usage
	count    int
}

// planStep is one step of the plan, with the ID and category of the
// recommendation it makes, known even when it makes none.
type planStep struct {
	id       string
	category Category
	run      func(scenario) (*Recommendation, scenario)
}

var planSteps = []planStep{
	{"orphan-volumes", CategoryStorage, stepOrphanVolumes},
	{"unusable-nodes", CategoryNodes, stepUnusableNodes},
	{"consolidate-nodes", CategoryNodes, stepConsolidation},
	{"rightsize-requests", CategoryWorkloads, stepRightsizing},
	{"node-type", CategoryNodes, stepNodeType},
	{"spot-capacity", CategoryNodes, stepSpot},
}

func newScenario(report costmodel.CostReport, inputs Inputs) scenario {
	current := scenario{report: report, inputs: inputs}
	if missing := len(missingRequestItems(report)); missing > 0 {
		current.blocked = fmt.Sprintf("%s without requests leave the cluster's demand incomplete", plural(missing, "workload"))
	}

	var capacity costmodel.Usage
	hourly, spot := 0.0, 0
	types := make(map[string]int)
	for _, item := range usableNodes(report) {
		current.nodes++
		hourly += item.HourlyUSD
		capacity = capacity.Add(item.Usage)
		types[detailString(item, "machineType")]++
		if detailString(item, "purchase") == "spot" {
			spot++
		}
	}
	if current.nodes > 0 {
		count := float64(current.nodes)
		current.nodeHourly = hourly / count
		current.capacity = capacity.Scale(1 / count)
		current.spotShare = float64(spot) / count
		current.machineType = dominant(types)
	}

	for _, item := range report.Items {
		if item.Basis != costmodel.BasisRequested || !item.Priced() {
			continue
		}
		usage := costmodel.Usage{CPUCores: item.Usage.CPUCores, MemoryGB: item.Usage.MemoryGB, GPUUnits: item.Usage.GPUUnits}
		current.demand = current.demand.Add(usage)
		replicas := math.Max(detailFloat(item, "replicas"), 1)
		perPod := usage.Scale(1 / replicas)
		if detailString(item, "kind") == "DaemonSet" {
			current.perNode = current.perNode.Add(perPod)
			continue
		}
		current.pods = append(current.pods, podDemand{workload: podKey(item), perPod: perPod, count: int(math.Round(replicas))})
	}
	return current
}

func podKey(item costmodel.LineItem) string {
	if item.Subject.ID != "" {
		return item.Subject.ID
	}
	return item.Subject.Namespace + "/" + item.Subject.Name
}

// nodesFor returns how many nodes of a capacity the pods need. It packs whole
// pods first-fit, largest first, into nodes that keep headroom free and carry
// the DaemonSet overhead, as the scheduler would have to. A pod larger than a
// node's usable room cannot be packed; those are counted by volume instead.
func (current scenario) nodesFor(capacity costmodel.Usage) (int, bool) {
	if capacity.CPUCores <= 0 || capacity.MemoryGB <= 0 {
		return 0, false
	}
	usable := 1 - headroomFraction
	room := costmodel.Usage{
		CPUCores: capacity.CPUCores*usable - current.perNode.CPUCores,
		MemoryGB: capacity.MemoryGB*usable - current.perNode.MemoryGB,
	}
	if room.CPUCores <= 0 || room.MemoryGB <= 0 {
		return 0, false
	}

	sizes := make([]costmodel.Usage, 0)
	for _, pods := range current.pods {
		for replica := 0; replica < pods.count; replica++ {
			sizes = append(sizes, pods.perPod)
		}
	}
	share := func(pod costmodel.Usage) float64 {
		return math.Max(pod.CPUCores/room.CPUCores, pod.MemoryGB/room.MemoryGB)
	}
	sort.SliceStable(sizes, func(i, j int) bool { return share(sizes[i]) > share(sizes[j]) })

	const tolerance = 1e-9
	free := make([]costmodel.Usage, 0)
	var oversized costmodel.Usage
	for _, pod := range sizes {
		if pod.CPUCores > room.CPUCores+tolerance || pod.MemoryGB > room.MemoryGB+tolerance {
			oversized = oversized.Add(pod)
			continue
		}
		placed := false
		for index := range free {
			if pod.CPUCores <= free[index].CPUCores+tolerance && pod.MemoryGB <= free[index].MemoryGB+tolerance {
				free[index] = free[index].Add(pod.Scale(-1))
				placed = true
				break
			}
		}
		if !placed {
			free = append(free, room.Add(pod.Scale(-1)))
		}
	}

	needed := len(free) + int(math.Ceil(math.Max(oversized.CPUCores/room.CPUCores, oversized.MemoryGB/room.MemoryGB)-1e-9))
	minimum := min(highAvailabilityNodes, current.nodes)
	return max(needed, minimum, 1), true
}

func (current scenario) nodeCost() float64 {
	return float64(current.nodes) * current.nodeHourly
}

func stepOrphanVolumes(current scenario) (*Recommendation, scenario) {
	items := make([]Item, 0)
	hourly := 0.0
	var confidence costmodel.Confidence
	for _, item := range current.report.Items {
		if item.Subject.Kind != costmodel.SubjectVolume || !item.Priced() || item.HourlyUSD <= 0 {
			continue
		}
		status := detailString(item, "status")
		mounted, known := item.Detail["mounted"].(bool)
		var change string
		switch {
		case status == "Lost":
			change = "claim lost its volume"
		case status == "Bound" && known && !mounted:
			change = "no running pod mounts it"
		default:
			continue
		}
		hourly += item.HourlyUSD
		confidence = costmodel.WeakerConfidence(confidence, item.Confidence)
		items = append(items, Item{
			Subject: item.Subject,
			Change:  fmt.Sprintf("%s · %.0f GiB", change, item.Usage.StorageGB),
			Savings: item.Cost,
			Check:   fmt.Sprintf("kubectl describe pvc %s -n %s", item.Subject.Name, item.Subject.Namespace),
			Command: fmt.Sprintf("kubectl delete pvc %s -n %s", item.Subject.Name, item.Subject.Namespace),
			Note:    "With the Delete reclaim policy this also deletes the data. Snapshot it first if unsure. If \"Used By\" lists a finished pod, the claim is not removed until that pod is gone: remove a one-off Job that is done, but if a CronJob created it, the claim is needed for the next run, so keep it.",
		})
	}
	if len(items) == 0 {
		return nil, current
	}

	return &Recommendation{
		ID:            "orphan-volumes",
		Category:      CategoryStorage,
		Title:         fmt.Sprintf("Delete %s nothing uses", plural(len(items), "volume")),
		Effort:        LevelLow,
		Risk:          LevelMedium,
		Confidence:    confidence,
		Savings:       costmodel.Project(hourly),
		Rationale:     "A claim is billed for its full size whether or not a pod mounts it, and no running pod mounts these.",
		SeparateItems: true,
		Action:        "Check that nothing needs them (a CronJob mounts its claim only while it runs), snapshot what matters, then delete them. A finished Job keeps its pod, and Kubernetes holds the claim until that pod is gone.",
		Items:         items,
	}, current
}

func stepUnusableNodes(current scenario) (*Recommendation, scenario) {
	items := make([]Item, 0)
	hourly := 0.0
	for _, item := range current.report.Items {
		if item.Subject.Kind != costmodel.SubjectNode || !item.Priced() || isUsable(item) {
			continue
		}
		hourly += item.HourlyUSD
		entry := Item{Subject: item.Subject, Savings: item.Cost}
		if isFalse(item.Detail, "ready") {
			entry.Change = "NotReady"
			entry.Command = fmt.Sprintf("kubectl describe node %s", item.Subject.Name)
			entry.Note = "Find out why it is not ready before removing it."
		} else {
			entry.Change = "cordoned (unschedulable)"
			entry.Command = fmt.Sprintf("kubectl drain %s --ignore-daemonsets --delete-emptydir-data", item.Subject.Name)
			entry.Note = "If it was cordoned for maintenance, uncordon it instead; otherwise drain it and remove it from its node group."
		}
		items = append(items, entry)
	}
	if len(items) == 0 {
		return nil, current
	}

	// The scenario never counted these nodes as capacity, so it is unchanged.
	return &Recommendation{
		ID:            "unusable-nodes",
		Category:      CategoryNodes,
		Title:         fmt.Sprintf("Recover or remove %s that cannot run pods", plural(len(items), "node")),
		Effort:        LevelLow,
		Risk:          LevelLow,
		Confidence:    costmodel.ConfidenceExact,
		Savings:       costmodel.Project(hourly),
		Rationale:     "These nodes are billed in full but the scheduler cannot place new pods on them.",
		SeparateItems: true,
		Action:        "Recover them, or drain them and remove them from their node group.",
		Items:         items,
	}, current
}

func stepConsolidation(current scenario) (*Recommendation, scenario) {
	if current.blocked != "" {
		return nil, current
	}
	needed, ok := current.nodesFor(current.capacity)
	if !ok || needed >= current.nodes {
		return nil, current
	}

	removable := current.nodes - needed
	next := current
	next.nodes = needed
	item, target := scaleDownItem(current, removable, needed)
	return &Recommendation{
		ID:         "consolidate-nodes",
		Category:   CategoryNodes,
		Title:      fmt.Sprintf("Run on %d fewer %s", removable, noun(removable, "node")),
		Effort:     LevelMedium,
		Risk:       LevelLow,
		Confidence: costmodel.ConfidenceEstimated,
		Savings:    costmodel.Project(float64(removable) * current.nodeHourly),
		Rationale: fmt.Sprintf("Requests total %.1f cores and %.1f GB. Packed pod by pod with %.0f%% headroom and each node's DaemonSets, they fit on %d of the %d usable nodes.",
			current.demand.CPUCores, current.demand.MemoryGB, headroomFraction*100, needed, current.nodes),
		Action: "Lower the node groups' desired and minimum size, or let the Cluster Autoscaler or Karpenter remove the spare nodes. Their pods are rescheduled on the rest.",
		Items:  []Item{item},
		Target: &target,
	}, next
}

// scaleDownItem describes the node-count change and where it ends. On EKS it
// names the largest on-demand node group and the command that shrinks it; the
// pool can span several groups, so it gives up when that one group cannot
// absorb the cut, and the target is then the pool's node count.
func scaleDownItem(current scenario, removable, needed int) (Item, Target) {
	item := Item{
		Subject: costmodel.Subject{Kind: costmodel.SubjectNodeGroup, Name: "worker nodes"},
		Change:  fmt.Sprintf("%d → %d nodes", current.nodes, needed),
		Savings: costmodel.Project(float64(removable) * current.nodeHourly),
		Note:    nodeGroupSummary(current.report),
	}
	target := Target{Nodes: needed}
	eks := current.inputs.EKS
	if eks == nil {
		return item, target
	}
	groups := onDemandGroups(current.report)
	var largest nodeGroupCount
	for _, group := range groups {
		if group.nodes > largest.nodes {
			largest = group
		}
	}
	var size *EKSNodeGroup
	for index := range eks.NodeGroups {
		if eks.NodeGroups[index].Name == largest.name {
			size = &eks.NodeGroups[index]
		}
	}
	if size == nil || largest.nodes-removable < 1 || size.Desired-removable < 1 {
		return item, target
	}

	desired := size.Desired - removable
	scaling := fmt.Sprintf("desiredSize=%d", desired)
	if desired < size.Min {
		scaling = fmt.Sprintf("minSize=%d,desiredSize=%d", desired, desired)
	}
	item.Subject.Name = largest.name
	item.Change = fmt.Sprintf("%d → %d nodes", size.Desired, desired)
	item.Check = "kubectl get nodes -L eks.amazonaws.com/nodegroup"
	item.Command = fmt.Sprintf("aws eks update-nodegroup-config --cluster-name %s --nodegroup-name %s --scaling-config %s --region %s", eks.Name, largest.name, scaling, eks.Region)
	item.Note = "Shrink the node group, not the node: a deleted node is replaced to keep the desired size. EKS picks the node to remove and drains it first. If the Cluster Autoscaler or Karpenter manages this group, lower its limits instead, or it scales back up."
	if len(groups) > 1 {
		item.Note += fmt.Sprintf(" %s is the largest of %d on-demand groups.", largest.name, len(groups))
	}
	return item, Target{Group: largest.name, Nodes: desired}
}

func stepRightsizing(current scenario) (*Recommendation, scenario) {
	items := make([]Item, 0)
	var released costmodel.Usage
	freed := 0.0
	touchesMemory := false
	// resized holds the new per-pod requests of each rightsized workload.
	resized := make(map[string]costmodel.Usage)

	for _, item := range current.report.Items {
		// Managed add-ons are reconfigured through their add-on, not by hand.
		if item.Basis != costmodel.BasisRequested || !item.Priced() || item.Used == nil || IsSystemNamespace(item.Subject.Namespace) {
			continue
		}
		replicas := math.Max(detailFloat(item, "replicas"), 1)
		cpuTarget, cpuFreed := rightsize(item.Usage.CPUCores, item.Used.CPUCores, minimumPodCPUCores*replicas, item.Components[costmodel.ComponentCPU])
		memoryTarget, memoryFreed := rightsize(item.Usage.MemoryGB, item.Used.MemoryGB, minimumPodMemoryGB*replicas, item.Components[costmodel.ComponentMemory])
		if cpuFreed <= 0 && memoryFreed <= 0 {
			continue
		}

		changes := make([]string, 0, 2)
		cpuPerPod, memoryPerPod := 0.0, 0.0
		if cpuFreed > 0 {
			cpuTarget = roundUp(cpuTarget, 1000)
			released.CPUCores += item.Usage.CPUCores - cpuTarget
			cpuPerPod = roundUp(cpuTarget/replicas, 1000)
			changes = append(changes, fmt.Sprintf("cpu %s → %s per pod", millicores(item.Usage.CPUCores/replicas), millicores(cpuPerPod)))
		}
		if memoryFreed > 0 {
			memoryTarget = roundUp(memoryTarget, 1024)
			released.MemoryGB += item.Usage.MemoryGB - memoryTarget
			memoryPerPod = roundUp(memoryTarget/replicas, 1024)
			changes = append(changes, fmt.Sprintf("memory %s → %s per pod", mebibytes(item.Usage.MemoryGB/replicas), mebibytes(memoryPerPod)))
			touchesMemory = true
		}
		freed += cpuFreed + memoryFreed
		newPerPod := item.Usage.Scale(1 / replicas)
		if cpuPerPod > 0 {
			newPerPod.CPUCores = cpuPerPod
		}
		if memoryPerPod > 0 {
			newPerPod.MemoryGB = memoryPerPod
		}
		resized[podKey(item)] = newPerPod

		command, note := setResourcesCommand(item, cpuPerPod, memoryPerPod)
		items = append(items, Item{
			Subject: item.Subject,
			Change:  strings.Join(changes, " · "),
			Savings: costmodel.Project(cpuFreed + memoryFreed),
			Command: command,
			Note:    note,
		})
	}
	if len(items) == 0 {
		return nil, current
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Savings.Hourly > items[j].Savings.Hourly })

	risk := LevelLow
	if touchesMemory {
		risk = LevelMedium
	}
	recommendation := &Recommendation{
		ID:            "rightsize-requests",
		Category:      CategoryWorkloads,
		Title:         fmt.Sprintf("Rightsize %s", plural(len(items), "workload")),
		Effort:        LevelLow,
		Risk:          risk,
		Confidence:    costmodel.ConfidenceEstimated,
		FreedRequests: costmodel.Project(freed),
		Action:        "Apply the new requests, then watch usage over a full day. Memory set below real peaks gets pods OOM-killed.",
		SeparateItems: true,
		Items:         items,
	}

	rationale := fmt.Sprintf("Usage is below %.0f%% of the request; suggestions keep %.0f%% headroom over one metrics-server sample, so peaks between captures are not seen, and never go below %s CPU or %s memory per pod.",
		rightsizeThreshold*100, rightsizeHeadroom*100, millicores(minimumPodCPUCores), mebibytes(minimumPodMemoryGB))
	if current.blocked != "" {
		recommendation.Rationale = rationale + " The billed saving is not computed until the data issues are fixed: " + current.blocked + "."
		return recommendation, current
	}
	if current.paused != "" {
		recommendation.Rationale = rationale + " The billed saving waits for the node change under way: " + current.paused + "."
		return recommendation, current
	}

	next := current.resize(resized, released)
	needed, ok := next.nodesFor(current.capacity)
	removable := 0
	if ok && needed < current.nodes {
		removable = current.nodes - needed
		next.nodes = needed
	}

	recommendation.Savings = costmodel.Project(float64(removable) * current.nodeHourly)
	if removable > 0 {
		recommendation.Rationale = rationale + fmt.Sprintf(" The released requests let the cluster run on %d fewer %s on top of the steps above.", removable, noun(removable, "node"))
	} else {
		recommendation.Rationale = rationale + " The released requests do not free a whole node yet, so they leave room to grow rather than cut the bill."
	}
	return recommendation, next
}

// resize applies rightsized per-pod requests to a copy of the scenario.
func (current scenario) resize(resized map[string]costmodel.Usage, released costmodel.Usage) scenario {
	next := current
	next.demand.CPUCores -= released.CPUCores
	next.demand.MemoryGB -= released.MemoryGB
	next.pods = make([]podDemand, len(current.pods))
	for index, pods := range current.pods {
		if perPod, ok := resized[pods.workload]; ok {
			pods.perPod = perPod
		}
		next.pods[index] = pods
	}
	// A DaemonSet's saving applies on every node.
	for _, item := range current.report.Items {
		perPod, ok := resized[podKey(item)]
		if !ok || detailString(item, "kind") != "DaemonSet" {
			continue
		}
		old := item.Usage.Scale(1 / math.Max(detailFloat(item, "replicas"), 1))
		next.perNode = next.perNode.Add(old.Scale(-1)).Add(perPod)
	}
	return next
}

func stepNodeType(current scenario) (*Recommendation, scenario) {
	if current.blocked != "" || len(current.inputs.MachineTypes) == 0 || current.demand.GPUUnits > 0 || current.nodes == 0 {
		return nil, current
	}
	var reference *MachineType
	for index := range current.inputs.MachineTypes {
		if current.inputs.MachineTypes[index].Name == current.machineType {
			reference = &current.inputs.MachineTypes[index]
		}
	}
	if reference == nil || reference.VCPU <= 0 || reference.MemoryGB <= 0 {
		return nil, current
	}

	// Allocatable is below the machine's size (system reservations); assume
	// every candidate loses the same share.
	cpuRatio := math.Min(current.capacity.CPUCores/reference.VCPU, 1)
	memoryRatio := math.Min(current.capacity.MemoryGB/reference.MemoryGB, 1)
	purchaseFactor := 1.0
	if ratio := current.inputs.SpotPriceRatio; ratio > 0 {
		purchaseFactor = 1 - current.spotShare*(1-ratio)
	}

	currentCost := current.nodeCost()
	bestCost := currentCost
	var best *MachineType
	var bestNodes int
	var bestCapacity costmodel.Usage
	for index := range current.inputs.MachineTypes {
		candidate := current.inputs.MachineTypes[index]
		if candidate.Name == reference.Name || candidate.Burstable || candidate.GPUUnits > 0 || candidate.VCPU <= 0 || candidate.MemoryGB <= 0 {
			continue
		}
		capacity := costmodel.Usage{CPUCores: candidate.VCPU * cpuRatio, MemoryGB: candidate.MemoryGB * memoryRatio}
		nodes, ok := current.nodesFor(capacity)
		if !ok {
			continue
		}
		cost := float64(nodes) * candidate.HourlyUSD * purchaseFactor
		if cost < bestCost {
			bestCost, best, bestNodes, bestCapacity = cost, &current.inputs.MachineTypes[index], nodes, capacity
		}
	}
	if best == nil || bestCost > currentCost*(1-minimumNodeTypeGain) {
		return nil, current
	}

	next := current
	next.nodes = bestNodes
	next.nodeHourly = bestCost / float64(bestNodes)
	next.capacity = bestCapacity
	next.machineType = best.Name
	savings := costmodel.Project(currentCost - bestCost)
	item, target := migrationItem(current, reference.Name, best.Name, bestNodes, savings)
	return &Recommendation{
		ID:         "node-type",
		Category:   CategoryNodes,
		Title:      fmt.Sprintf("Switch to %s nodes", best.Name),
		Effort:     LevelMedium,
		Risk:       LevelLow,
		Confidence: costmodel.ConfidenceEstimated,
		Savings:    savings,
		Rationale: fmt.Sprintf("After the steps above, requests need %.1f cores and %.1f GB. %s (%.0f vCPU, %.0f GB) fits that CPU-to-memory ratio better than %s (%.0f vCPU, %.0f GB).",
			current.demand.CPUCores, current.demand.MemoryGB, best.Name, best.VCPU, best.MemoryGB, reference.Name, reference.VCPU, reference.MemoryGB),
		Action: fmt.Sprintf("Create a node group of %s, cordon and drain the %s nodes, then remove the old group.", best.Name, reference.Name),
		Items:  []Item{item},
		Target: &target,
	}, next
}

// migrationItem describes the machine-type change. On EKS, when one
// on-demand node group of the old type holds the pool, it gives the commands
// that replace it with a group of the new type, copied from the old one and
// sized for the steps above. A group's instance type cannot be changed in
// place.
func migrationItem(current scenario, from, to string, nodes int, savings costmodel.Projection) (Item, Target) {
	item := Item{
		Subject: costmodel.Subject{Kind: costmodel.SubjectNodeGroup, Name: "worker nodes"},
		Change:  fmt.Sprintf("%d × %s → %d × %s", current.nodes, from, nodes, to),
		Savings: savings,
	}
	target := Target{Nodes: nodes, FromMachineType: from, MachineType: to}
	eks := current.inputs.EKS
	groups := onDemandGroups(current.report)
	if eks == nil || len(groups) != 1 {
		return item, target
	}
	var old *EKSNodeGroup
	for index := range eks.NodeGroups {
		if eks.NodeGroups[index].Name == groups[0].name {
			old = &eks.NodeGroups[index]
		}
	}
	if old == nil || len(old.InstanceTypes) != 1 || old.InstanceTypes[0] != from || old.NodeRole == "" || len(old.Subnets) == 0 {
		return item, target
	}

	name := old.Name + "-" + strings.ReplaceAll(to, ".", "-")
	if len(name) > 63 {
		name = name[:63]
	}
	create := []string{
		"aws eks create-nodegroup",
		"--cluster-name " + eks.Name,
		"--nodegroup-name " + name,
		"--instance-types " + to,
		"--capacity-type ON_DEMAND",
		fmt.Sprintf("--scaling-config minSize=%d,maxSize=%d,desiredSize=%d", min(old.Min, nodes), max(old.Max, nodes), nodes),
		"--subnets " + strings.Join(old.Subnets, " "),
		"--node-role " + old.NodeRole,
	}
	// The launch template carries the disks; a CUSTOM AMI type means it
	// carries the image too, and then no AMI type may be given.
	if old.LaunchTemplateID != "" {
		create = append(create, fmt.Sprintf("--launch-template id=%s,version=%s", old.LaunchTemplateID, old.LaunchTemplateVersion))
	}
	if old.AmiType != "" && old.AmiType != "CUSTOM" {
		create = append(create, "--ami-type "+old.AmiType)
	}
	if labels := copiedLabels(old.Labels); labels != "" {
		create = append(create, "--labels "+labels)
	}
	if len(old.Taints) > 0 {
		create = append(create, "--taints "+strings.Join(old.Taints, " "))
	}
	create = append(create, "--region "+eks.Region)

	selector := "-l eks.amazonaws.com/nodegroup=" + old.Name
	item.Subject.Name = old.Name
	item.Check = "kubectl get nodes -L eks.amazonaws.com/nodegroup,node.kubernetes.io/instance-type"
	item.Steps = []string{
		strings.Join(create, " "),
		fmt.Sprintf("aws eks wait nodegroup-active --cluster-name %s --nodegroup-name %s --region %s", eks.Name, name, eks.Region),
		"kubectl cordon " + selector,
		"kubectl drain " + selector + " --ignore-daemonsets --delete-emptydir-data",
		fmt.Sprintf("aws eks delete-nodegroup --cluster-name %s --nodegroup-name %s --region %s", eks.Name, old.Name, eks.Region),
	}
	item.Note = fmt.Sprintf("The new group copies %s's launch template, AMI type, subnets, role, labels and taints, and is sized for the steps above. Workloads that select the old group by name (eks.amazonaws.com/nodegroup=%s) must be changed first, or they stay Pending.", old.Name, old.Name)
	target.Group, target.NewGroup = old.Name, name
	return item, target
}

// copiedLabels are the labels a replacement group keeps, sorted, as the AWS
// CLI takes them. Labels that name the old group are its own.
func copiedLabels(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		if strings.HasPrefix(key, "alpha.eksctl.io/") || strings.HasPrefix(key, "eks.amazonaws.com/") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key+"="+labels[key])
	}
	return strings.Join(pairs, ",")
}

func stepSpot(current scenario) (*Recommendation, scenario) {
	ratio := current.inputs.SpotPriceRatio
	onDemand := 1 - current.spotShare
	if ratio <= 0 || current.blocked != "" || onDemand <= 0 || current.nodes == 0 {
		return nil, current
	}

	// DaemonSets run on every node, spot or not, so they are left out.
	stateless, total := 0.0, 0.0
	for _, item := range current.report.Items {
		if item.Basis != costmodel.BasisRequested || !item.Priced() {
			continue
		}
		kind := detailString(item, "kind")
		if kind == "DaemonSet" {
			continue
		}
		total += item.HourlyUSD
		if kind == "Deployment" || kind == "Job" || kind == "CronJob" || kind == "ReplicaSet" {
			stateless += item.HourlyUSD
		}
	}
	if total <= 0 || stateless <= 0 {
		return nil, current
	}
	share := stateless / total
	moved := onDemand * share
	savings := costmodel.Project(current.nodeCost() * moved * (1 - ratio))

	next := current
	next.spotShare = current.spotShare + moved
	next.nodeHourly = current.nodeHourly * (1 - moved*(1-ratio))

	items := make([]Item, 0)
	item, target, ok := spotItem(current, share)
	if ok {
		items = append(items, item)
	} else {
		for _, group := range onDemandGroups(current.report) {
			items = append(items, Item{
				Subject: costmodel.Subject{Kind: costmodel.SubjectNodeGroup, Name: group.name},
				Change:  fmt.Sprintf("on-demand · %s", plural(group.nodes, "node")),
			})
		}
	}
	target.SpotShare = next.spotShare
	return &Recommendation{
		ID:         "spot-capacity",
		Category:   CategoryNodes,
		Title:      "Run stateless workloads on spot capacity",
		Effort:     LevelMedium,
		Risk:       LevelMedium,
		Confidence: costmodel.ConfidenceEstimated,
		Savings:    savings,
		Rationale: fmt.Sprintf("%.0f%% of requests belong to Deployments and Jobs, which tolerate interruptions. Spot is assumed at %.0f%% of the on-demand price, which varies over time.",
			share*100, ratio*100),
		Action: "Add a spot node group with several instance types, steer stateless workloads to it with a node affinity or a taint and toleration, and keep StatefulSets and critical services on on-demand.",
		Items:  items,
		Target: &target,
	}, next
}

// spotItem gives the commands that move the stateless share of the one
// on-demand EKS node group to a new spot group: create the spot group with
// several instance types of the same size, prefer spot for Deployments and
// CronJobs, keep StatefulSets off it, then shrink the on-demand group. Groups
// eksctl created get eksctl commands, so the new group has its own stack,
// role and launch template instead of borrowing ones that eksctl deletes with
// the old group.
func spotItem(current scenario, share float64) (Item, Target, bool) {
	eks := current.inputs.EKS
	groups := onDemandGroups(current.report)
	if eks == nil || len(groups) != 1 {
		return Item{}, Target{}, false
	}
	old := eksGroup(current.inputs, groups[0].name)
	if old == nil || len(old.InstanceTypes) != 1 || old.Desired < 2 {
		return Item{}, Target{}, false
	}
	spotNodes := int(math.Round(share * float64(old.Desired)))
	// StatefulSets need somewhere to stay: keep one on-demand node at least.
	spotNodes = min(max(spotNodes, 1), old.Desired-1)
	onDemand := old.Desired - spotNodes

	types := spotTypes(old.InstanceTypes[0], current.inputs.MachineTypes)
	name := old.Name + "-spot"
	byEksctl := old.Labels["alpha.eksctl.io/nodegroup-name"] != ""
	var steps []string
	notes := []string{}
	if byEksctl {
		if len(old.Taints) > 0 {
			// eksctl takes no taints on the command line
			return Item{}, Target{}, false
		}
		create := []string{"eksctl create nodegroup", "--cluster " + eks.Name, "--region " + eks.Region, "--name " + name,
			"--managed --spot", "--instance-types " + strings.Join(types, ","),
			fmt.Sprintf("--nodes %d --nodes-min 0 --nodes-max %d", spotNodes, 2*spotNodes)}
		if family := eksctlAMIFamily(old.AmiType); family != "" {
			create = append(create, "--node-ami-family "+family)
		}
		if labels := copiedLabels(old.Labels); labels != "" {
			create = append(create, "--node-labels "+labels)
		}
		steps = append(steps, strings.Join(create, " "))
		notes = append(notes, "eksctl gives the new group its own stack and waits until it is ready. It defaults to an 80 GB root volume; add --node-volume-size to match the old group.")
	} else {
		if old.NodeRole == "" || len(old.Subnets) == 0 {
			return Item{}, Target{}, false
		}
		create := []string{"aws eks create-nodegroup", "--cluster-name " + eks.Name, "--nodegroup-name " + name,
			"--capacity-type SPOT", "--instance-types " + strings.Join(types, " "),
			fmt.Sprintf("--scaling-config minSize=0,maxSize=%d,desiredSize=%d", 2*spotNodes, spotNodes),
			"--subnets " + strings.Join(old.Subnets, " "), "--node-role " + old.NodeRole}
		if old.AmiType != "" && old.AmiType != "CUSTOM" {
			create = append(create, "--ami-type "+old.AmiType)
		}
		if labels := copiedLabels(old.Labels); labels != "" {
			create = append(create, "--labels "+labels)
		}
		if len(old.Taints) > 0 {
			create = append(create, "--taints "+strings.Join(old.Taints, " "))
		}
		create = append(create, "--region "+eks.Region)
		steps = append(steps, strings.Join(create, " "),
			fmt.Sprintf("aws eks wait nodegroup-active --cluster-name %s --nodegroup-name %s --region %s", eks.Name, name, eks.Region))
		notes = append(notes, "The spot group uses the default root volume, not the old group's launch template.")
	}

	patches := steeringPatches(current.report)
	steps = append(steps, patches...)
	if byEksctl {
		scale := fmt.Sprintf("eksctl scale nodegroup --cluster %s --region %s --name %s --nodes %d", eks.Name, eks.Region, old.Name, onDemand)
		if onDemand < old.Min {
			scale += fmt.Sprintf(" --nodes-min %d", onDemand)
		}
		steps = append(steps, scale)
	} else {
		scaling := fmt.Sprintf("desiredSize=%d", onDemand)
		if onDemand < old.Min {
			scaling = fmt.Sprintf("minSize=%d,desiredSize=%d", onDemand, onDemand)
		}
		steps = append(steps, fmt.Sprintf("aws eks update-nodegroup-config --cluster-name %s --nodegroup-name %s --scaling-config %s --region %s", eks.Name, old.Name, scaling, eks.Region))
	}

	if len(types) == 1 {
		notes = append(notes, fmt.Sprintf("Only %s has this size in the price catalog: add more types of the same size, since a spot group with one type is often left without capacity.", types[0]))
	}
	if len(patches) > 0 {
		notes = append(notes, "The patches replace a workload's existing node affinity of the same kind and restart its pods; merge them by hand if it has one. Deployments and CronJobs prefer spot; StatefulSets are kept off it.")
	}
	notes = append(notes, "Shrinking the on-demand group last lets EKS drain those nodes once spot capacity is there.")

	item := Item{
		Subject: costmodel.Subject{Kind: costmodel.SubjectNodeGroup, Name: old.Name},
		Change:  fmt.Sprintf("%d on-demand → %d on-demand + %d spot", old.Desired, onDemand, spotNodes),
		Check:   "kubectl get nodes -L eks.amazonaws.com/nodegroup,eks.amazonaws.com/capacityType",
		Steps:   steps,
		Note:    strings.Join(notes, " "),
	}
	return item, Target{Group: old.Name, NewGroup: name, Nodes: onDemand, SpotNodes: spotNodes}, true
}

// spotTypes are the instance type and up to three priced ones of the same
// size, cheapest first, so spot can fall back when one type runs out.
func spotTypes(base string, machineTypes []MachineType) []string {
	var reference *MachineType
	for index := range machineTypes {
		if machineTypes[index].Name == base {
			reference = &machineTypes[index]
		}
	}
	types := []string{base}
	if reference == nil {
		return types
	}
	similar := make([]MachineType, 0)
	for _, candidate := range machineTypes {
		if candidate.Name != base && !candidate.Burstable && candidate.VCPU == reference.VCPU && candidate.MemoryGB == reference.MemoryGB && candidate.GPUUnits == reference.GPUUnits {
			similar = append(similar, candidate)
		}
	}
	sort.Slice(similar, func(i, j int) bool {
		if similar[i].HourlyUSD != similar[j].HourlyUSD {
			return similar[i].HourlyUSD < similar[j].HourlyUSD
		}
		return similar[i].Name < similar[j].Name
	})
	for index := 0; index < len(similar) && index < 3; index++ {
		types = append(types, similar[index].Name)
	}
	return types
}

// steeringPatches send the stateless workloads to spot and keep the stateful
// ones off it. Managed node groups label every node with its capacity type.
func steeringPatches(report costmodel.CostReport) []string {
	const (
		preferSpot = `"affinity":{"nodeAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"weight":100,"preference":{"matchExpressions":[{"key":"eks.amazonaws.com/capacityType","operator":"In","values":["SPOT"]}]}}]}}`
		avoidSpot  = `"affinity":{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"eks.amazonaws.com/capacityType","operator":"NotIn","values":["SPOT"]}]}]}}}`
	)
	type patch struct{ kind, namespace, name, body string }
	patches := make([]patch, 0)
	for _, item := range report.Items {
		if item.Basis != costmodel.BasisRequested || IsSystemNamespace(item.Subject.Namespace) {
			continue
		}
		switch detailString(item, "kind") {
		case "Deployment":
			patches = append(patches, patch{"deployment", item.Subject.Namespace, item.Subject.Name, `{"spec":{"template":{"spec":{` + preferSpot + `}}}}`})
		case "CronJob":
			patches = append(patches, patch{"cronjob", item.Subject.Namespace, item.Subject.Name, `{"spec":{"jobTemplate":{"spec":{"template":{"spec":{` + preferSpot + `}}}}}}`})
		case "StatefulSet":
			patches = append(patches, patch{"statefulset", item.Subject.Namespace, item.Subject.Name, `{"spec":{"template":{"spec":{` + avoidSpot + `}}}}`})
		}
	}
	sort.Slice(patches, func(i, j int) bool {
		if patches[i].kind != patches[j].kind {
			return patches[i].kind < patches[j].kind
		}
		return patches[i].namespace+"/"+patches[i].name < patches[j].namespace+"/"+patches[j].name
	})
	commands := make([]string, 0, len(patches))
	for _, patch := range patches {
		commands = append(commands, fmt.Sprintf("kubectl patch %s %s -n %s --type merge -p '%s'", patch.kind, patch.name, patch.namespace, patch.body))
	}
	return commands
}

// eksctlAMIFamily maps an EKS AMI type to the family eksctl takes.
func eksctlAMIFamily(amiType string) string {
	switch {
	case strings.HasPrefix(amiType, "AL2023_"):
		return "AmazonLinux2023"
	case strings.HasPrefix(amiType, "AL2_"):
		return "AmazonLinux2"
	case strings.HasPrefix(amiType, "BOTTLEROCKET_"):
		return "Bottlerocket"
	}
	return ""
}

// rightsize returns the suggested request and the hourly cost it would free
// for one resource, or a zero saving when the request is already close to use
// or already at the floor.
func rightsize(requested, used, floor, requestedCost float64) (float64, float64) {
	if requested <= 0 || requestedCost <= 0 || used/requested >= rightsizeThreshold {
		return requested, 0
	}
	target := math.Max(used*(1+rightsizeHeadroom), floor)
	if target >= requested {
		return requested, 0
	}
	return target, requestedCost * (1 - target/requested)
}

// IsSystemNamespace recognizes namespaces owned by Kubernetes or the cloud
// provider. Their workloads are costed like any other, but they are managed
// add-ons: the plan never asks the user to edit them by hand.
func IsSystemNamespace(namespace string) bool {
	switch namespace {
	case "kube-system", "kube-public", "kube-node-lease":
		return true
	}
	for _, prefix := range []string{"amazon-", "aws-", "gke-", "gmp-", "azure-"} {
		if strings.HasPrefix(namespace, prefix) {
			return true
		}
	}
	return false
}

// setResourcesCommand builds a kubectl command when one command is exact: a
// controller kubectl can patch, with a single container.
func setResourcesCommand(item costmodel.LineItem, cpuCores, memoryGB float64) (string, string) {
	kind := strings.ToLower(detailString(item, "kind"))
	switch kind {
	case "deployment", "statefulset", "daemonset":
	default:
		return "", fmt.Sprintf("Change the requests in the %s spec that creates these pods.", detailString(item, "kind"))
	}
	if containers := int(detailFloat(item, "containers")); containers != 1 {
		return "", fmt.Sprintf("%d containers: split the change between them in the %s spec.", containers, detailString(item, "kind"))
	}

	requests := make([]string, 0, 2)
	if cpuCores > 0 {
		requests = append(requests, "cpu="+millicores(cpuCores))
	}
	if memoryGB > 0 {
		requests = append(requests, "memory="+mebibytes(memoryGB))
	}
	return fmt.Sprintf("kubectl set resources %s/%s -n %s --requests=%s", kind, item.Subject.Name, item.Subject.Namespace, strings.Join(requests, ",")), ""
}

type nodeGroupCount struct {
	name  string
	nodes int
}

func onDemandGroups(report costmodel.CostReport) []nodeGroupCount {
	counts := make(map[string]int)
	for _, item := range usableNodes(report) {
		if detailString(item, "purchase") == "spot" {
			continue
		}
		counts[groupName(item)]++
	}
	return sortedGroups(counts)
}

func nodeGroupSummary(report costmodel.CostReport) string {
	counts := make(map[string]int)
	for _, item := range usableNodes(report) {
		counts[groupName(item)]++
	}
	parts := make([]string, 0, len(counts))
	for _, group := range sortedGroups(counts) {
		parts = append(parts, fmt.Sprintf("%s: %d", group.name, group.nodes))
	}
	return "Nodes today by group, " + strings.Join(parts, ", ") + "."
}

func sortedGroups(counts map[string]int) []nodeGroupCount {
	groups := make([]nodeGroupCount, 0, len(counts))
	for name, nodes := range counts {
		groups = append(groups, nodeGroupCount{name: name, nodes: nodes})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].name < groups[j].name })
	return groups
}

func groupName(item costmodel.LineItem) string {
	if item.Subject.ParentID == "" {
		return "(no node group)"
	}
	return item.Subject.ParentID
}

func usableNodes(report costmodel.CostReport) []costmodel.LineItem {
	nodes := make([]costmodel.LineItem, 0)
	for _, item := range report.Items {
		if item.Subject.Kind == costmodel.SubjectNode && item.Priced() && isUsable(item) {
			nodes = append(nodes, item)
		}
	}
	return nodes
}

func isUsable(item costmodel.LineItem) bool {
	return !isFalse(item.Detail, "ready") && !isFalse(item.Detail, "schedulable")
}

func isFalse(detail map[string]interface{}, key string) bool {
	value, ok := detail[key].(bool)
	return ok && !value
}

func detailString(item costmodel.LineItem, key string) string {
	value, _ := item.Detail[key].(string)
	return value
}

// detailFloat reads a numeric detail whether it was stored as an int or a float.
func detailFloat(item costmodel.LineItem, key string) float64 {
	switch value := item.Detail[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	}
	return 0
}

func dominant(counts map[string]int) string {
	best, bestCount := "", 0
	for name, count := range counts {
		if count > bestCount || (count == bestCount && name < best) {
			best, bestCount = name, count
		}
	}
	return best
}

// roundUp rounds to whole millicores or MiB, ignoring float noise such as
// 0.1*1.5 = 0.15000000000000002.
func roundUp(value, units float64) float64 {
	return math.Ceil(value*units-1e-6) / units
}

func millicores(cores float64) string {
	return fmt.Sprintf("%.0fm", math.Round(cores*1000))
}

func mebibytes(gigabytes float64) string {
	return fmt.Sprintf("%.0fMi", math.Round(gigabytes*1024))
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
