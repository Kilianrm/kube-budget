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
	// Namespace is set when the snapshot was collected for one namespace only.
	Namespace string
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
	if report.Scope.Namespace != "" {
		report.Warnings = append(report.Warnings, costmodel.Warning{
			Code:    "namespace-scoped",
			Subject: report.Scope.Namespace,
			Message: "only this namespace was collected; idle capacity includes what other namespaces request",
		})
	}
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
		Namespace:   strings.TrimSpace(options.Namespace),
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
		HasPlacement:    len(snapshot.Pods) > 0,
		Nodes:           nodes,
		Workloads:       projectWorkloads(snapshot.Workloads, snapshot.Pods),
		Volumes:         projectVolumes(snapshot.Resources, snapshot.Pods),
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
		group, purchase := nodeGroupFor(snapshot.Provider, node)
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
			Taints:      node.Taints,
		})
	}
	return nodes
}

// nodeGroupFor finds a node's group and purchase option. The node's own EKS
// labels come first; without them, the node is matched to a described node
// group by instance type, which is ambiguous when two groups share a type.
func nodeGroupFor(provider *clustermode.ProviderMetadata, node clustermode.Node) (string, pricing.PurchaseOption) {
	if node.NodeGroup != "" {
		capacityType := node.CapacityType
		if provider != nil {
			for _, group := range provider.NodeGroups {
				if group.Name == node.NodeGroup && group.CapacityType != "" {
					capacityType = group.CapacityType
				}
			}
		}
		return node.NodeGroup, purchaseOption(capacityType)
	}
	if provider == nil || node.InstanceType == "" {
		return "", purchaseOption(node.CapacityType)
	}
	for _, group := range provider.NodeGroups {
		for _, candidate := range group.InstanceTypes {
			if candidate == node.InstanceType {
				return group.Name, purchaseOption(group.CapacityType)
			}
		}
	}
	return "", purchaseOption(node.CapacityType)
}

func purchaseOption(capacityType string) pricing.PurchaseOption {
	if strings.EqualFold(capacityType, "SPOT") {
		return pricing.PurchaseSpot
	}
	return pricing.PurchaseOnDemand
}

// projectWorkloads attaches each scheduled pod to its controller, grouped by
// node. Pods whose controller is not a collected workload, such as Job pods
// or bare pods, become workloads of their own so their cost is not lost.
func projectWorkloads(workloads []clustermode.Workload, pods []clustermode.Pod) []allocation.Workload {
	placements := placementsByOwner(pods)

	projected := make([]allocation.Workload, 0, len(workloads))
	for _, workload := range workloads {
		replicas := float64(workload.DesiredReplicas)
		key := ownerKey(workload.Namespace, workload.Kind, workload.Name)
		projected = append(projected, allocation.Workload{
			ID:              workload.UID,
			Kind:            workload.Kind,
			Name:            workload.Name,
			Namespace:       workload.Namespace,
			Replicas:        replicas,
			PerReplica:      perReplicaUsage(workload.Requests, replicas),
			MissingRequests: declaresNoRequests(workload),
			PartialRequests: partialRequests(workload.MissingResources, declaresNoRequests(workload)),
			Placements:      placements[key].byNode(),
			Containers:      len(workload.Containers),
		})
		delete(placements, key)
	}

	orphanKeys := make([]string, 0, len(placements))
	for key := range placements {
		orphanKeys = append(orphanKeys, key)
	}
	sort.Strings(orphanKeys)
	for _, key := range orphanKeys {
		group := placements[key]
		placed := group.byNode()
		if len(placed) == 0 {
			continue
		}
		projected = append(projected, allocation.Workload{
			ID:              "pods/" + key,
			Kind:            group.kind,
			Name:            group.name,
			Namespace:       group.namespace,
			Replicas:        float64(group.pods),
			MissingRequests: group.noRequests,
			PartialRequests: partialRequests(sortedKeys(group.missingResources), group.noRequests),
			Placements:      placed,
		})
	}
	return projected
}

type ownerPods struct {
	kind      string
	name      string
	namespace string
	pods      int
	// noRequests stays true while every pod declares no CPU and no memory.
	noRequests       bool
	missingResources map[string]bool
	nodes            map[string]*allocation.Placement
	// unsampled marks nodes with at least one pod that has no usage sample.
	unsampled map[string]bool
}

func placementsByOwner(pods []clustermode.Pod) map[string]*ownerPods {
	owners := make(map[string]*ownerPods)
	for _, pod := range pods {
		key := ownerKey(pod.Namespace, pod.OwnerKind, pod.OwnerName)
		owner := owners[key]
		if owner == nil {
			owner = &ownerPods{kind: pod.OwnerKind, name: pod.OwnerName, namespace: pod.Namespace, noRequests: true,
				missingResources: make(map[string]bool), nodes: make(map[string]*allocation.Placement), unsampled: make(map[string]bool)}
			owners[key] = owner
		}
		owner.pods++
		owner.noRequests = owner.noRequests && pod.Requests.CPUMilli == 0 && pod.Requests.MemoryBytes == 0
		for _, resource := range pod.MissingResources {
			owner.missingResources[resource] = true
		}
		// A pending pod is not running anywhere, so no node bills for it.
		if pod.NodeName == "" {
			continue
		}
		placement := owner.nodes[pod.NodeName]
		if placement == nil {
			placement = &allocation.Placement{Node: pod.NodeName}
			owner.nodes[pod.NodeName] = placement
		}
		placement.Pods++
		placement.Usage = placement.Usage.Add(usageFrom(pod.Requests))
		if pod.Usage == nil {
			owner.unsampled[pod.NodeName] = true
			continue
		}
		used := usageFrom(*pod.Usage)
		if placement.Used != nil {
			used = used.Add(*placement.Used)
		}
		placement.Used = &used
	}
	return owners
}

func (owner *ownerPods) byNode() []allocation.Placement {
	if owner == nil {
		return nil
	}
	placements := make([]allocation.Placement, 0, len(owner.nodes))
	for node, placement := range owner.nodes {
		// Partial usage would understate the node's share, so drop it.
		if owner.unsampled[node] {
			placement.Used = nil
		}
		placements = append(placements, *placement)
	}
	sort.Slice(placements, func(i, j int) bool { return placements[i].Node < placements[j].Node })
	return placements
}

// declaresNoRequests reports a workload whose containers request neither CPU
// nor memory. Its demand is unknown, unlike one that only omits a resource.
// Without a container list it falls back to the summed requests.
func declaresNoRequests(workload clustermode.Workload) bool {
	if len(workload.Containers) == 0 {
		return workload.MissingRequests && workload.Requests.CPUMilli == 0 && workload.Requests.MemoryBytes == 0
	}
	for _, container := range workload.Containers {
		if container.Requests.CPUMilli > 0 || container.Requests.MemoryBytes > 0 {
			return false
		}
	}
	return true
}

func partialRequests(missing []string, noRequests bool) []string {
	if noRequests || len(missing) == 0 {
		return nil
	}
	return missing
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func ownerKey(namespace, kind, name string) string {
	return namespace + "/" + kind + "/" + name
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

// projectVolumes maps claims onto volumes. Whether a claim is mounted is only
// known when pods were collected; otherwise it is left unset.
func projectVolumes(resources []clustermode.Resource, pods []clustermode.Pod) []allocation.Volume {
	var mounted map[string]bool
	if len(pods) > 0 {
		mounted = make(map[string]bool)
		for _, pod := range pods {
			for _, claim := range pod.Claims {
				mounted[pod.Namespace+"/"+claim] = true
			}
		}
	}

	volumes := make([]allocation.Volume, 0)
	for _, resource := range resources {
		if resource.Kind != "PersistentVolumeClaim" {
			continue
		}
		volume := allocation.Volume{
			ID:           resource.UID,
			Name:         resource.Name,
			Namespace:    resource.Namespace,
			StorageClass: resource.Attributes["Storage class"],
			Status:       resource.Status,
			StorageGB:    float64(resource.Requests.StorageBytes) / bytesPerGiB,
		}
		if mounted != nil {
			isMounted := mounted[resource.Namespace+"/"+resource.Name]
			volume.Mounted = &isMounted
		}
		volumes = append(volumes, volume)
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
