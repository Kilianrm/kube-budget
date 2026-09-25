// Package costexplorer projects a live cluster snapshot into the neutral cost
// model and returns the report the Cost Explorer renders.
package costexplorer

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"kube-budget/core/allocation"
	"kube-budget/core/costmodel"
	"kube-budget/core/pricing"
	clustermode "kube-budget/internal/application/cluster"
)

const bytesPerGiB = 1024 * 1024 * 1024

// Options lets the caller override what the snapshot could not tell us.
type Options struct {
	Provider string
	Region   string
	// PricingSKU is the machine type used to derive per-unit rates for
	// workload requests. Defaults to the cluster's most common instance type.
	PricingSKU string
}

// Service builds cost reports from cluster snapshots.
type Service struct {
	resolver pricing.RateResolver
	now      func() time.Time
}

// New creates the Cost Explorer use case.
func New(resolver pricing.RateResolver) *Service {
	return &Service{resolver: resolver, now: time.Now}
}

// Report prices a snapshot. It fails only when no machine type can be used as
// a pricing reference; everything else degrades into warnings.
func (service *Service) Report(snapshot clustermode.Snapshot, options Options) (costmodel.CostReport, error) {
	cluster, err := projectCluster(snapshot, options)
	if err != nil {
		return costmodel.CostReport{}, err
	}

	report := allocation.New(service.resolver).Allocate(cluster, service.now())
	for _, warning := range snapshot.Warnings {
		report.Warnings = append(report.Warnings, costmodel.Warning{
			Code:    "collection-incomplete",
			Subject: warning.Resource,
			Message: warning.Message,
		})
	}
	return report, nil
}

func projectCluster(snapshot clustermode.Snapshot, options Options) (allocation.Cluster, error) {
	scope := costmodel.Scope{
		ClusterName: clusterName(snapshot),
		Provider:    strings.TrimSpace(options.Provider),
		Region:      strings.TrimSpace(options.Region),
	}
	if snapshot.Provider != nil {
		if scope.Provider == "" {
			scope.Provider = snapshot.Provider.Provider
		}
		if scope.Region == "" {
			scope.Region = snapshot.Provider.Region
		}
	}
	if scope.Region == "" {
		scope.Region = firstNodeRegion(snapshot.Nodes)
	}

	nodes := projectNodes(snapshot, scope.Region)
	pricingSKU := strings.TrimSpace(options.PricingSKU)
	if pricingSKU == "" {
		pricingSKU = dominantMachineType(nodes)
	}
	if pricingSKU == "" {
		return allocation.Cluster{}, fmt.Errorf("cost explorer: no instance type available to price cluster %q", scope.ClusterName)
	}

	return allocation.Cluster{
		Scope:           scope,
		PricingSKU:      pricingSKU,
		Nodes:           nodes,
		Workloads:       projectWorkloads(snapshot.Workloads),
		Volumes:         projectVolumes(snapshot.Resources),
		HasControlPlane: snapshot.Provider != nil,
	}, nil
}

func projectNodes(snapshot clustermode.Snapshot, defaultRegion string) []allocation.Node {
	nodes := make([]allocation.Node, 0, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		region := node.Region
		if region == "" {
			region = defaultRegion
		}
		group, purchase := nodeGroupFor(snapshot.Provider, node.InstanceType)
		nodes = append(nodes, allocation.Node{
			ID:          node.UID,
			Name:        node.Name,
			NodeGroup:   group,
			MachineType: node.InstanceType,
			Region:      region,
			Purchase:    purchase,
			Allocatable: usageFrom(node.Allocatable),
			Ready:       node.Ready,
			Schedulable: node.Schedulable,
		})
	}
	return nodes
}

// nodeGroupFor matches a node to its node group by instance type, the only
// link the snapshot exposes today.
func nodeGroupFor(provider *clustermode.ProviderMetadata, instanceType string) (string, pricing.PurchaseOption) {
	if provider == nil || instanceType == "" {
		return "", pricing.PurchaseOnDemand
	}
	for _, group := range provider.NodeGroups {
		for _, candidate := range group.InstanceTypes {
			if candidate == instanceType {
				return group.Name, purchaseOption(group.CapacityType)
			}
		}
	}
	return "", pricing.PurchaseOnDemand
}

func purchaseOption(capacityType string) pricing.PurchaseOption {
	if strings.EqualFold(capacityType, "SPOT") {
		return pricing.PurchaseSpot
	}
	return pricing.PurchaseOnDemand
}

func projectWorkloads(workloads []clustermode.Workload) []allocation.Workload {
	projected := make([]allocation.Workload, 0, len(workloads))
	for _, workload := range workloads {
		replicas := float64(workload.DesiredReplicas)
		projected = append(projected, allocation.Workload{
			ID:              workload.UID,
			Kind:            workload.Kind,
			Name:            workload.Name,
			Namespace:       workload.Namespace,
			Replicas:        replicas,
			PerReplica:      perReplicaUsage(workload.Requests, replicas),
			MissingRequests: workload.MissingRequests,
		})
	}
	return projected
}

// perReplicaUsage undoes the replica multiplication the collector applies, so
// the allocator stays the only place that decides how many replicas are billed.
func perReplicaUsage(requests clustermode.ResourceValues, replicas float64) costmodel.Usage {
	usage := usageFrom(requests)
	if replicas <= 1 {
		return usage
	}
	return usage.Scale(1 / replicas)
}

func projectVolumes(resources []clustermode.Resource) []allocation.Volume {
	volumes := make([]allocation.Volume, 0)
	for _, resource := range resources {
		if resource.Kind != "PersistentVolumeClaim" {
			continue
		}
		volumes = append(volumes, allocation.Volume{
			ID:           resource.UID,
			Name:         resource.Name,
			Namespace:    resource.Namespace,
			StorageClass: resource.Attributes["Storage class"],
			Status:       resource.Status,
			StorageGB:    float64(resource.Requests.StorageBytes) / bytesPerGiB,
		})
	}
	return volumes
}

func usageFrom(values clustermode.ResourceValues) costmodel.Usage {
	return costmodel.Usage{
		CPUCores:  float64(values.CPUMilli) / 1000,
		MemoryGB:  float64(values.MemoryBytes) / bytesPerGiB,
		StorageGB: float64(values.StorageBytes) / bytesPerGiB,
		GPUUnits:  float64(values.GPUUnits),
	}
}

// dominantMachineType picks the instance type that carries the most nodes, so
// workload requests are priced against the cluster's typical hardware.
func dominantMachineType(nodes []allocation.Node) string {
	counts := make(map[string]int)
	for _, node := range nodes {
		if node.MachineType != "" {
			counts[node.MachineType]++
		}
	}
	if len(counts) == 0 {
		return ""
	}

	types := make([]string, 0, len(counts))
	for machineType := range counts {
		types = append(types, machineType)
	}
	sort.Slice(types, func(i, j int) bool {
		if counts[types[i]] == counts[types[j]] {
			return types[i] < types[j]
		}
		return counts[types[i]] > counts[types[j]]
	})
	return types[0]
}

func firstNodeRegion(nodes []clustermode.Node) string {
	for _, node := range nodes {
		if node.Region != "" {
			return node.Region
		}
	}
	return ""
}

func clusterName(snapshot clustermode.Snapshot) string {
	if snapshot.Provider != nil && snapshot.Provider.ClusterName != "" {
		return snapshot.Provider.ClusterName
	}
	return snapshot.Cluster.Context
}
