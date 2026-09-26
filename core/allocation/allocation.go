// Package allocation turns a normalized cluster description into a cost
// report. It knows nothing about Kubernetes APIs: callers project their source
// data into the neutral Cluster model first, exactly as Manifest Mode does.
package allocation

import (
	"fmt"
	"strings"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/pricing"
)

// SKUControlPlane is the reserved SKU used to price a managed control plane.
const SKUControlPlane = "controlplane"

// Node is one billable machine. Nodes are billed whether or not they are ready
// or schedulable, so both flags are diagnostics rather than pricing inputs.
type Node struct {
	ID          string
	Name        string
	NodeGroup   string
	MachineType string
	Region      string
	Purchase    pricing.PurchaseOption
	Allocatable costmodel.Usage
	Ready       bool
	Schedulable bool
	// Taints are the node's NoSchedule and NoExecute taints.
	Taints []string
}

// Workload is one controller and the requests of a single replica.
type Workload struct {
	ID         string
	Kind       string
	Name       string
	Namespace  string
	Replicas   float64
	PerReplica costmodel.Usage
	// MissingRequests means no container declares a CPU or memory request, so
	// the workload's cost is unknown.
	MissingRequests bool
	// PartialRequests lists resources some container leaves unrequested. The
	// workload is still priced on what it declares.
	PartialRequests []string
	Labels          map[string]string
	// Placements are the requests of the pods this workload runs, grouped by
	// node. They are only read when Cluster.HasPlacement is true.
	Placements []Placement
	// Containers is how many containers each pod runs; zero when unknown.
	Containers int
}

// Placement is the summed requests of a workload's pods on one node. Used is
// their observed usage, nil when it was not collected for every pod.
type Placement struct {
	Node  string
	Pods  int
	Usage costmodel.Usage
	Used  *costmodel.Usage
}

// Volume is one provisioned persistent volume claim.
type Volume struct {
	ID           string
	Name         string
	Namespace    string
	StorageClass string
	Status       string
	StorageGB    float64
	// Mounted reports whether a running pod mounts the claim; nil when pods
	// were not collected.
	Mounted *bool
}

// Cluster is the neutral input of the allocator.
type Cluster struct {
	Scope costmodel.Scope
	// PricingSKU is the representative machine type used to derive per-unit
	// rates for workload requests when pod placement is unknown.
	PricingSKU string
	// HasPlacement reports that workload placements were collected, so each
	// pod is priced at the rate of the node it runs on. Idle capacity is then
	// exactly the node cost that no pod requests.
	HasPlacement    bool
	Nodes           []Node
	Workloads       []Workload
	Volumes         []Volume
	HasControlPlane bool
}

// Allocator prices a cluster through a rate resolver.
type Allocator struct {
	resolver pricing.RateResolver
}

// New creates an allocator backed by a rate resolver.
func New(resolver pricing.RateResolver) *Allocator {
	return &Allocator{resolver: resolver}
}

// Allocate builds the cost report holding both lenses: what the provider bills
// for the infrastructure that exists, and what the workloads request.
func (allocator *Allocator) Allocate(cluster Cluster, at time.Time) costmodel.CostReport {
	builder := reportBuilder{allocator: allocator, cluster: cluster}
	builder.addNodes()
	builder.addControlPlane()
	builder.addVolumes()
	builder.addWorkloads()

	return costmodel.NewReport(at, cluster.Scope, builder.items, builder.warnings)
}

type reportBuilder struct {
	allocator *Allocator
	cluster   Cluster
	items     []costmodel.LineItem
	warnings  []costmodel.Warning
	// nodeRates holds the per-unit rates of every priced node, by node name.
	nodeRates map[string]pricing.RateCard
}

func (builder *reportBuilder) addNodes() {
	requested := builder.requestedByNode()
	for _, node := range builder.cluster.Nodes {
		region := node.Region
		if region == "" {
			region = builder.cluster.Scope.Region
		}
		request := pricing.RateRequest{
			Provider: builder.cluster.Scope.Provider,
			Region:   region,
			SKU:      node.MachineType,
			Purchase: node.Purchase,
		}

		item := costmodel.LineItem{
			Subject: costmodel.Subject{
				Kind:     costmodel.SubjectNode,
				ID:       node.ID,
				Name:     node.Name,
				ParentID: node.NodeGroup,
			},
			Basis: costmodel.BasisProvisioned,
			Usage: node.Allocatable,
			Detail: map[string]interface{}{
				"machineType": node.MachineType,
				"purchase":    string(node.Purchase),
				"ready":       node.Ready,
				"schedulable": node.Schedulable,
			},
		}
		if requested != nil {
			usage := requested[node.Name]
			item.Detail["requestedCores"] = usage.CPUCores
			item.Detail["requestedMemoryGB"] = usage.MemoryGB
		}
		if len(node.Taints) > 0 {
			item.Detail["taints"] = node.Taints
		}

		rate, err := builder.allocator.resolver.ResolveNode(request)
		if err != nil {
			item.Confidence = costmodel.ConfidenceUnknown
			builder.warn("node-rate-unresolved", node.Name, err.Error())
			builder.items = append(builder.items, item)
			continue
		}

		item.HourlyUSD = rate.HourlyUSD
		item.Confidence = rate.Confidence
		item.Components = builder.splitNodePrice(node.Name, request, rate)
		if rate.Purchase == pricing.PurchaseSpot {
			item.Assumptions = append(item.Assumptions, costmodel.Assumption{
				Key:    "spot-price",
				Detail: fmt.Sprintf("%s is spot capacity; the billed price varies over time", node.Name),
			})
		}
		builder.items = append(builder.items, item)
	}
}

// requestedByNode sums the requests of the pods placed on each node, whether
// or not their workload could be priced. It is nil without placements.
func (builder *reportBuilder) requestedByNode() map[string]costmodel.Usage {
	if !builder.cluster.HasPlacement {
		return nil
	}
	requested := make(map[string]costmodel.Usage)
	for _, workload := range builder.cluster.Workloads {
		for _, placement := range workload.Placements {
			requested[placement.Node] = requested[placement.Node].Add(placement.Usage)
		}
	}
	return requested
}

// splitNodePrice attributes a machine price to CPU and memory so the explorer
// can show node spend by component, and keeps the node's per-unit rates so
// the pods placed on it are priced from the same figure. It falls back to a
// single flat component when the split is unavailable.
func (builder *reportBuilder) splitNodePrice(name string, request pricing.RateRequest, rate pricing.InstanceRate) map[costmodel.Component]float64 {
	card, err := builder.allocator.resolver.ResolveResources(request)
	if err != nil || card.CPUCostShare <= 0 {
		return map[costmodel.Component]float64{costmodel.ComponentFlat: rate.HourlyUSD}
	}
	if builder.nodeRates == nil {
		builder.nodeRates = make(map[string]pricing.RateCard)
	}
	builder.nodeRates[name] = card
	return map[costmodel.Component]float64{
		costmodel.ComponentCPU:    rate.HourlyUSD * card.CPUCostShare,
		costmodel.ComponentMemory: rate.HourlyUSD * (1 - card.CPUCostShare),
	}
}

func (builder *reportBuilder) addControlPlane() {
	if !builder.cluster.HasControlPlane {
		return
	}

	item := costmodel.LineItem{
		Subject: costmodel.Subject{
			Kind: costmodel.SubjectControlPlane,
			ID:   SKUControlPlane,
			Name: "control plane",
		},
		Basis: costmodel.BasisProvisioned,
	}

	rate, err := builder.allocator.resolver.ResolveFlat(pricing.RateRequest{
		Provider: builder.cluster.Scope.Provider,
		Region:   builder.cluster.Scope.Region,
		SKU:      SKUControlPlane,
	})
	if err != nil {
		item.Confidence = costmodel.ConfidenceUnknown
		builder.warn("controlplane-rate-unresolved", "control plane", err.Error())
		builder.items = append(builder.items, item)
		return
	}

	item.HourlyUSD = rate.HourlyUSD
	item.Confidence = rate.Confidence
	item.Components = map[costmodel.Component]float64{costmodel.ComponentFlat: rate.HourlyUSD}
	builder.items = append(builder.items, item)
}

func (builder *reportBuilder) addVolumes() {
	if len(builder.cluster.Volumes) == 0 {
		return
	}
	card, cardErr := builder.resourceRates()

	for _, volume := range builder.cluster.Volumes {
		item := costmodel.LineItem{
			Subject: costmodel.Subject{
				Kind:      costmodel.SubjectVolume,
				ID:        volume.ID,
				Name:      volume.Name,
				Namespace: volume.Namespace,
			},
			Basis:  costmodel.BasisProvisioned,
			Usage:  costmodel.Usage{StorageGB: volume.StorageGB},
			Detail: map[string]interface{}{"storageClass": volume.StorageClass, "status": volume.Status},
		}
		if volume.Mounted != nil {
			item.Detail["mounted"] = *volume.Mounted
		}

		// A pending claim has no disk behind it yet, so nothing is billed.
		if volume.Status == "Pending" {
			item.Confidence = costmodel.ConfidenceExact
			item.Assumptions = append(item.Assumptions, costmodel.Assumption{
				Key:    "volume-pending",
				Detail: "pending claims have no volume provisioned yet and are not billed",
			})
			builder.items = append(builder.items, item)
			continue
		}

		if cardErr != nil || card.StorageUSDPerGBHour == 0 {
			item.Confidence = costmodel.ConfidenceUnknown
			builder.warn("volume-rate-unresolved", volume.Name, "no storage rate available")
			builder.items = append(builder.items, item)
			continue
		}

		item.HourlyUSD = volume.StorageGB * card.StorageUSDPerGBHour
		item.Components = map[costmodel.Component]float64{costmodel.ComponentStorage: item.HourlyUSD}
		item.Confidence = card.Confidence
		item.Assumptions = append(item.Assumptions, costmodel.Assumption{
			Key:    "volume-storage-class",
			Detail: fmt.Sprintf("%q priced with the provider's default storage rate", volume.StorageClass),
		})
		builder.items = append(builder.items, item)
	}
}

func (builder *reportBuilder) addWorkloads() {
	if len(builder.cluster.Workloads) == 0 {
		return
	}
	card, cardErr := builder.resourceRates()
	if cardErr != nil && !builder.cluster.HasPlacement {
		builder.warn("resource-rates-unresolved", builder.cluster.PricingSKU, cardErr.Error())
	}

	for _, workload := range builder.cluster.Workloads {
		item := costmodel.LineItem{
			Subject: costmodel.Subject{
				Kind:      costmodel.SubjectWorkload,
				ID:        workload.ID,
				Name:      workload.Name,
				Namespace: workload.Namespace,
				Labels:    workload.Labels,
			},
			Basis:  costmodel.BasisRequested,
			Detail: map[string]interface{}{"kind": workload.Kind, "containers": workload.Containers},
		}

		// A workload without requests has an unknown cost, never a zero cost:
		// counting it as zero would silently inflate the idle figure.
		if workload.MissingRequests {
			item.Detail["missingRequests"] = true
			item.Usage = builder.placedUsage(workload)
			item.Confidence = costmodel.ConfidenceUnknown
			builder.warn("workload-missing-requests", workload.Name, "no resource requests declared; cost cannot be attributed")
			builder.items = append(builder.items, item)
			continue
		}

		if builder.cluster.HasPlacement {
			builder.priceByPlacement(workload, &item)
		} else {
			builder.priceByReference(workload, &item, card, cardErr)
		}
		if len(workload.PartialRequests) > 0 && item.Priced() {
			item.Detail["missingResources"] = workload.PartialRequests
			item.Assumptions = append(item.Assumptions, costmodel.Assumption{
				Key:    "partial-requests",
				Detail: fmt.Sprintf("%s/%s declares no %s request; it is priced on what it declares", workload.Namespace, workload.Name, strings.Join(workload.PartialRequests, " or ")),
			})
		}
		if item.Usage.StorageGB > 0 && item.Priced() {
			item.Assumptions = append(item.Assumptions, costmodel.Assumption{
				Key:    "ephemeral-storage",
				Detail: "ephemeral storage requests use the node's disk, which is already in the node price",
			})
		}
		builder.items = append(builder.items, item)
	}
}

// priceByPlacement prices every pod at the rates of the node it runs on, so
// requested cost is a true share of each node's bill.
func (builder *reportBuilder) priceByPlacement(workload Workload, item *costmodel.LineItem) {
	var usage, used costmodel.Usage
	components := make(map[costmodel.Component]float64)
	usedComponents := make(map[costmodel.Component]float64)
	pods, unpricedPods := 0, 0
	var confidence costmodel.Confidence
	// Usage is only comparable with requests when every priced pod has it.
	complete := len(workload.Placements) > 0

	for _, placement := range workload.Placements {
		usage = usage.Add(placement.Usage)
		card, ok := builder.nodeRates[placement.Node]
		if !ok {
			unpricedPods += placement.Pods
			continue
		}
		if placement.Usage.GPUUnits > 0 && card.GPUUSDPerUnitHour == 0 {
			item.Usage = usage
			item.Confidence = costmodel.ConfidenceUnknown
			builder.warn("gpu-unpriced", workload.Name, "GPU requests are not covered by the current price catalog")
			return
		}
		pods += placement.Pods
		addRequestComponents(components, placement.Usage, card)
		if placement.Used == nil {
			complete = false
		} else {
			used = used.Add(*placement.Used)
			addRequestComponents(usedComponents, *placement.Used, card)
		}
		confidence = costmodel.WeakerConfidence(confidence, card.Confidence)
		item.Assumptions = appendUnique(item.Assumptions, card.Assumptions...)
	}

	item.Usage = usage
	item.Detail["replicas"] = float64(pods + unpricedPods)
	item.Detail["desiredReplicas"] = workload.Replicas
	item.Detail["nodes"] = len(workload.Placements)

	if pods == 0 && unpricedPods > 0 {
		item.Confidence = costmodel.ConfidenceUnknown
		builder.warn("workload-on-unpriced-nodes", workload.Name, "every pod runs on a node without a price")
		return
	}
	if unpricedPods > 0 {
		item.Assumptions = append(item.Assumptions, costmodel.Assumption{
			Key:    "pods-on-unpriced-nodes",
			Detail: fmt.Sprintf("%s: %d pod(s) on nodes without a price are left out", workload.Name, unpricedPods),
		})
	}
	if confidence == "" {
		// Nothing is scheduled, so nothing is billed for this workload.
		confidence = costmodel.ConfidenceExact
	}

	item.Components = components
	item.HourlyUSD = sumComponents(components)
	item.Confidence = confidence
	if complete && pods > 0 {
		item.Used = &used
		item.UsedComponents = usedComponents
	}
	item.Assumptions = append(item.Assumptions, costmodel.Assumption{
		Key:    "workload-node-rate",
		Detail: "requests priced at the rate of the node each pod runs on",
	})
}

// priceByReference is the fallback when pod placement is unknown: requests
// are priced with the rates of one representative machine type.
func (builder *reportBuilder) priceByReference(workload Workload, item *costmodel.LineItem, card pricing.RateCard, cardErr error) {
	replicas := builder.replicas(workload)
	item.Usage = workload.PerReplica.Scale(replicas)
	item.Detail["replicas"] = replicas

	if cardErr != nil {
		item.Confidence = costmodel.ConfidenceUnknown
		return
	}
	if item.Usage.GPUUnits > 0 && card.GPUUSDPerUnitHour == 0 {
		item.Confidence = costmodel.ConfidenceUnknown
		builder.warn("gpu-unpriced", workload.Name, "GPU requests are not covered by the current price catalog")
		return
	}

	components := make(map[costmodel.Component]float64)
	addRequestComponents(components, item.Usage, card)
	item.Components = components
	item.HourlyUSD = sumComponents(components)
	item.Confidence = card.Confidence
	item.Assumptions = append(item.Assumptions, card.Assumptions...)
	item.Assumptions = append(item.Assumptions, costmodel.Assumption{
		Key:    "workload-pricing-sku",
		Detail: fmt.Sprintf("requests priced with %s rates; pod placement is not collected", builder.cluster.PricingSKU),
	})
}

// placedUsage reports what a workload's pods request even when its cost is
// unknown, so the UI can still show the quantities.
func (builder *reportBuilder) placedUsage(workload Workload) costmodel.Usage {
	if !builder.cluster.HasPlacement {
		return workload.PerReplica.Scale(builder.replicas(workload))
	}
	var usage costmodel.Usage
	for _, placement := range workload.Placements {
		usage = usage.Add(placement.Usage)
	}
	return usage
}

// addRequestComponents prices CPU, memory and GPU requests. Ephemeral storage
// is deliberately left out: it is the node's own disk and is already billed
// in the node price.
func addRequestComponents(components map[costmodel.Component]float64, usage costmodel.Usage, card pricing.RateCard) {
	add := func(component costmodel.Component, cost float64) {
		if cost != 0 {
			components[component] += cost
		}
	}
	add(costmodel.ComponentCPU, usage.CPUCores*card.CPUUSDPerCoreHour)
	add(costmodel.ComponentMemory, usage.MemoryGB*card.MemoryUSDPerGBHour)
	add(costmodel.ComponentGPU, usage.GPUUnits*card.GPUUSDPerUnitHour)
}

func appendUnique(assumptions []costmodel.Assumption, more ...costmodel.Assumption) []costmodel.Assumption {
	for _, candidate := range more {
		seen := false
		for _, existing := range assumptions {
			if existing == candidate {
				seen = true
				break
			}
		}
		if !seen {
			assumptions = append(assumptions, candidate)
		}
	}
	return assumptions
}

func sumComponents(components map[costmodel.Component]float64) float64 {
	total := 0.0
	for _, cost := range components {
		total += cost
	}
	return total
}

// replicas resolves how many copies of a workload are billed. A DaemonSet runs
// one pod per node, so an unreported replica count follows the node count.
func (builder *reportBuilder) replicas(workload Workload) float64 {
	if workload.Kind == "DaemonSet" && workload.Replicas <= 0 {
		return float64(len(builder.cluster.Nodes))
	}
	return workload.Replicas
}

func (builder *reportBuilder) resourceRates() (pricing.RateCard, error) {
	return builder.allocator.resolver.ResolveResources(pricing.RateRequest{
		Provider: builder.cluster.Scope.Provider,
		Region:   builder.cluster.Scope.Region,
		SKU:      builder.cluster.PricingSKU,
	})
}

func (builder *reportBuilder) warn(code, subject, message string) {
	builder.warnings = append(builder.warnings, costmodel.Warning{Code: code, Subject: subject, Message: message})
}
