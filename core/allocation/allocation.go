// Package allocation turns a normalized cluster description into a cost
// report. It knows nothing about Kubernetes APIs: callers project their source
// data into the neutral Cluster model first, exactly as Manifest Mode does.
package allocation

import (
	"fmt"
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
}

// Workload is one controller and the requests of a single replica.
type Workload struct {
	ID              string
	Kind            string
	Name            string
	Namespace       string
	Replicas        float64
	PerReplica      costmodel.Usage
	MissingRequests bool
	Labels          map[string]string
}

// Volume is one provisioned persistent volume claim.
type Volume struct {
	ID           string
	Name         string
	Namespace    string
	StorageClass string
	Status       string
	StorageGB    float64
}

// Cluster is the neutral input of the allocator.
type Cluster struct {
	Scope costmodel.Scope
	// PricingSKU is the representative machine type used to derive per-unit
	// rates for workload requests. Pod placement is not collected, so requests
	// cannot be priced against the node they actually land on.
	PricingSKU      string
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
}

func (builder *reportBuilder) addNodes() {
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

		rate, err := builder.allocator.resolver.ResolveNode(request)
		if err != nil {
			item.Confidence = costmodel.ConfidenceUnknown
			builder.warn("node-rate-unresolved", node.Name, err.Error())
			builder.items = append(builder.items, item)
			continue
		}

		item.HourlyUSD = rate.HourlyUSD
		item.Confidence = rate.Confidence
		item.Components = builder.splitNodePrice(request, rate)
		if rate.Purchase == pricing.PurchaseSpot {
			item.Assumptions = append(item.Assumptions, costmodel.Assumption{
				Key:    "spot-price",
				Detail: fmt.Sprintf("%s is spot capacity; the billed price varies over time", node.Name),
			})
		}
		builder.items = append(builder.items, item)
	}
}

// splitNodePrice attributes a machine price to CPU and memory so the explorer
// can show node spend by component. It falls back to a single flat component
// when the split is unavailable.
func (builder *reportBuilder) splitNodePrice(request pricing.RateRequest, rate pricing.InstanceRate) map[costmodel.Component]float64 {
	card, err := builder.allocator.resolver.ResolveResources(request)
	if err != nil || card.CPUCostShare <= 0 {
		return map[costmodel.Component]float64{costmodel.ComponentFlat: rate.HourlyUSD}
	}
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
	if cardErr != nil {
		builder.warn("resource-rates-unresolved", builder.cluster.PricingSKU, cardErr.Error())
	}

	for _, workload := range builder.cluster.Workloads {
		replicas := builder.replicas(workload)
		usage := workload.PerReplica.Scale(replicas)

		item := costmodel.LineItem{
			Subject: costmodel.Subject{
				Kind:      costmodel.SubjectWorkload,
				ID:        workload.ID,
				Name:      workload.Name,
				Namespace: workload.Namespace,
				Labels:    workload.Labels,
			},
			Basis: costmodel.BasisRequested,
			Usage: usage,
			Detail: map[string]interface{}{
				"kind":     workload.Kind,
				"replicas": replicas,
			},
		}

		// A workload without requests has an unknown cost, never a zero cost:
		// counting it as zero would silently inflate the idle figure.
		if workload.MissingRequests {
			item.Confidence = costmodel.ConfidenceUnknown
			builder.warn("workload-missing-requests", workload.Name, "no resource requests declared; cost cannot be attributed")
			builder.items = append(builder.items, item)
			continue
		}
		if cardErr != nil {
			item.Confidence = costmodel.ConfidenceUnknown
			builder.items = append(builder.items, item)
			continue
		}
		if usage.GPUUnits > 0 && card.GPUUSDPerUnitHour == 0 {
			item.Confidence = costmodel.ConfidenceUnknown
			builder.warn("gpu-unpriced", workload.Name, "GPU requests are not covered by the current price catalog")
			builder.items = append(builder.items, item)
			continue
		}

		components := map[costmodel.Component]float64{
			costmodel.ComponentCPU:     usage.CPUCores * card.CPUUSDPerCoreHour,
			costmodel.ComponentMemory:  usage.MemoryGB * card.MemoryUSDPerGBHour,
			costmodel.ComponentStorage: usage.StorageGB * card.StorageUSDPerGBHour,
			costmodel.ComponentGPU:     usage.GPUUnits * card.GPUUSDPerUnitHour,
		}
		hourly := 0.0
		for component, cost := range components {
			if cost == 0 {
				delete(components, component)
				continue
			}
			hourly += cost
		}

		item.HourlyUSD = hourly
		item.Components = components
		item.Confidence = card.Confidence
		item.Assumptions = append(item.Assumptions, card.Assumptions...)
		item.Assumptions = append(item.Assumptions, costmodel.Assumption{
			Key:    "workload-pricing-sku",
			Detail: fmt.Sprintf("requests priced with %s rates; pod placement is not collected", builder.cluster.PricingSKU),
		})
		builder.items = append(builder.items, item)
	}
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
