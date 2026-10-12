package workspace

import (
	"fmt"
	"strings"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/optimize"
	"kube-cost-workspace/core/pricing"
	clustermode "kube-cost-workspace/internal/application/cluster"
	costexplorermode "kube-cost-workspace/internal/application/costexplorer"
	"kube-cost-workspace/internal/providers"
	"kube-cost-workspace/internal/storage/costhistory"
)

// PriceOptions overrides the price list a cluster is priced with. Empty
// fields fall back to the platform detected from the cluster's nodes.
type PriceOptions struct {
	Provider     string
	Region       string
	InstanceType string
	// Namespace is the namespace the snapshot was collected for, if any.
	Namespace string
}

// PricedCluster is a cluster's cost report with what the optimization plan
// and the simulation need to reason about it.
type PricedCluster struct {
	Report costmodel.CostReport
	// ClusterID keys the cluster's history, budget and recommendations.
	ClusterID string
	// Plan offers the region's machine types, the spot ratio and, for EKS,
	// the node groups to the optimization plan.
	Plan optimize.Inputs
}

// Price builds the cost report of a snapshot. An explicit pricing provider
// wins; otherwise the cluster's platform, detected from its nodes, picks the
// price list.
func Price(snapshot clustermode.Snapshot, options PriceOptions) (PricedCluster, error) {
	providerName := strings.ToLower(strings.TrimSpace(options.Provider))
	region := strings.TrimSpace(options.Region)
	if providerName == "" {
		if !snapshot.Platform.Supported {
			return PricedCluster{}, fmt.Errorf("cost report: this cluster cannot be priced: %s", snapshot.Platform.Reason)
		}
		providerName = snapshot.Platform.Provider
	}
	if region == "" {
		region = snapshot.Platform.Region
	}
	selectedProvider, ok := providers.Get(providerName)
	if !ok {
		return PricedCluster{}, fmt.Errorf("cost report: unsupported pricing provider %q; supported providers: %s", providerName, strings.Join(providers.Names(), ", "))
	}

	report, err := costexplorermode.New(selectedProvider).Report(snapshot, costexplorermode.Options{
		Provider:   selectedProvider.Name(),
		Region:     selectedProvider.NormalizeRegion(region),
		PricingSKU: strings.TrimSpace(options.InstanceType),
		Namespace:  CollectionNamespace(options.Namespace),
	})
	if err != nil {
		return PricedCluster{}, err
	}

	inputs := PlanInputs(selectedProvider, report.Scope.Region)
	inputs.EKS = eksCluster(snapshot.Provider)
	inputs.NodeTransition = nodeTransition(snapshot.Provider)
	return PricedCluster{Report: report, ClusterID: costhistory.ClusterID(report.Scope), Plan: inputs}, nil
}

// PlanInputs offers the region's machine types to the node-type step and
// derives the spot ratio from the catalog.
func PlanInputs(provider providers.Provider, region string) optimize.Inputs {
	inputs := optimize.Inputs{}
	for _, name := range provider.MachineTypes(region) {
		request := pricing.RateRequest{Provider: provider.Name(), Region: region, SKU: name, Purchase: pricing.PurchaseOnDemand}
		rate, err := provider.ResolveNode(request)
		if err != nil || rate.HourlyUSD <= 0 {
			continue
		}
		inputs.MachineTypes = append(inputs.MachineTypes, optimize.MachineType{
			Name:      name,
			VCPU:      rate.VCPU,
			MemoryGB:  rate.MemoryGB,
			GPUUnits:  rate.GPUUnits,
			HourlyUSD: rate.HourlyUSD,
			Burstable: isBurstable(provider.Name(), name),
		})
		if inputs.SpotPriceRatio == 0 {
			request.Purchase = pricing.PurchaseSpot
			if spot, err := provider.ResolveNode(request); err == nil && spot.HourlyUSD < rate.HourlyUSD {
				inputs.SpotPriceRatio = spot.HourlyUSD / rate.HourlyUSD
			}
		}
	}
	return inputs
}

// eksCluster gives the plan what an EKS scaling command needs, for clusters
// connected through EKS; other connections have no node group sizes.
func eksCluster(metadata *clustermode.ProviderMetadata) *optimize.EKSCluster {
	if metadata == nil || metadata.Provider != "aws-eks" || metadata.ClusterName == "" || metadata.Region == "" {
		return nil
	}
	cluster := &optimize.EKSCluster{Name: metadata.ClusterName, Region: metadata.Region}
	for _, group := range metadata.NodeGroups {
		taints := make([]string, 0, len(group.Taints))
		for _, taint := range group.Taints {
			spelled := "key=" + taint.Key
			if taint.Value != "" {
				spelled += ",value=" + taint.Value
			}
			taints = append(taints, spelled+",effect="+taint.Effect)
		}
		cluster.NodeGroups = append(cluster.NodeGroups, optimize.EKSNodeGroup{
			Name:                  group.Name,
			Status:                group.Status,
			Desired:               int(group.DesiredSize),
			Min:                   int(group.MinSize),
			Max:                   int(group.MaxSize),
			InstanceTypes:         group.InstanceTypes,
			CapacityType:          group.CapacityType,
			AmiType:               group.AmiType,
			NodeRole:              group.NodeRole,
			Subnets:               group.Subnets,
			Labels:                group.Labels,
			Taints:                taints,
			LaunchTemplateID:      group.LaunchTemplateID,
			LaunchTemplateVersion: group.LaunchTemplateVersion,
		})
	}
	return cluster
}

// nodeTransition names an EKS node group in the middle of a change, if any.
func nodeTransition(metadata *clustermode.ProviderMetadata) string {
	if metadata == nil {
		return ""
	}
	for _, group := range metadata.NodeGroups {
		switch group.Status {
		case "CREATING", "UPDATING", "DELETING":
			return fmt.Sprintf("node group %s is %s", group.Name, strings.ToLower(group.Status))
		}
	}
	return ""
}

// isBurstable recognizes CPU-credit machine families, which throttle under
// sustained load and are never proposed as a node type.
func isBurstable(provider, machineType string) bool {
	switch provider {
	case "aws":
		return len(machineType) > 1 && machineType[0] == 't' && machineType[1] >= '0' && machineType[1] <= '9'
	case "azure":
		return strings.HasPrefix(machineType, "Standard_B")
	case "gcp":
		for _, prefix := range []string{"e2-micro", "e2-small", "e2-medium", "f1-", "g1-"} {
			if strings.HasPrefix(machineType, prefix) {
				return true
			}
		}
	}
	return false
}
