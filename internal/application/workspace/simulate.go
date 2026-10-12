package workspace

import (
	"fmt"
	"strings"
	"time"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/pricing"
	"kube-cost-workspace/core/whatif"
	manifestmode "kube-cost-workspace/internal/application/manifest"
	"kube-cost-workspace/internal/providers"
)

// ManifestEstimate is a manifest's estimate with the rates it was priced at.
type ManifestEstimate struct {
	manifestmode.Result
	Provider     string
	Region       string
	InstanceType string
}

// PriceManifest estimates a manifest with one provider's rates for one
// machine type. Catalog pricing and connected-cluster pricing both use it, so
// the two can only differ in the rates they pass.
func PriceManifest(content []byte, provider providers.Provider, region, instanceType string) (ManifestEstimate, error) {
	config, err := provider.NewPriceConfig(instanceType, region)
	if err != nil {
		return ManifestEstimate{}, err
	}
	result, err := manifestmode.New(config).Estimate(content)
	if err != nil {
		return ManifestEstimate{}, err
	}
	return ManifestEstimate{Result: result, Provider: provider.Name(), Region: region, InstanceType: instanceType}, nil
}

// Simulation is a manifest's estimate at the cluster's own rates and its
// impact on the cluster's bill, budget and optimization plan.
type Simulation struct {
	Estimate ManifestEstimate
	Impact   whatif.Result
	// Peak is the impact at the autoscaler's maximum, when the manifest has one.
	Peak *whatif.Result
	// ForecastBefore and ForecastAfter are this month's forecast without and
	// with the workload deployed now. Nil when history cannot be read.
	ForecastBefore *Forecast
	ForecastAfter  *Forecast
	// ReportAt is when the cluster report the simulation used was generated.
	ReportAt time.Time
}

// Simulate prices a manifest with a priced cluster's rates and simulates
// deploying it on the cluster's report.
func Simulate(cluster PricedCluster, manifest []byte) (Simulation, error) {
	if strings.TrimSpace(string(manifest)) == "" {
		return Simulation{}, fmt.Errorf("simulation: manifest content is required")
	}
	report := cluster.Report

	selectedProvider, ok := providers.Get(report.Scope.Provider)
	if !ok {
		return Simulation{}, fmt.Errorf("simulation: the cluster's provider %q has no price catalog", report.Scope.Provider)
	}
	machineType := referenceMachineType(report)
	if machineType == "" {
		return Simulation{}, fmt.Errorf("simulation: the cluster has no priced, schedulable node to take rates from")
	}
	card, err := selectedProvider.ResolveResources(pricing.RateRequest{
		Provider: selectedProvider.Name(),
		Region:   report.Scope.Region,
		SKU:      machineType,
		Purchase: pricing.PurchaseOnDemand,
	})
	if err != nil {
		return Simulation{}, err
	}

	estimate, err := PriceManifest(manifest, selectedProvider, report.Scope.Region, machineType)
	if err != nil {
		return Simulation{}, err
	}

	inputs := whatif.Inputs{RateCard: card, Plan: WithDismissals(cluster.ClusterID, cluster.Plan)}
	workload := workloadFromManifest(estimate.Result)
	result := Simulation{
		Estimate: estimate,
		Impact:   whatif.Simulate(report, workload, inputs),
		ReportAt: report.GeneratedAt,
	}
	if maximum := estimate.Workload.MaxReplicas; maximum != nil && int(*maximum) > workload.Replicas {
		peak := workload
		peak.Replicas = int(*maximum)
		peakImpact := whatif.Simulate(report, peak, inputs)
		result.Peak = &peakImpact
	}

	if forecast, err := LoadForecast(cluster.ClusterID, time.Now()); err == nil {
		runRate := report.Totals[costmodel.BasisProvisioned].Hourly
		before := forecast.Forecast(runRate)
		after := forecast.WithRunRateChange(before, result.Impact.BilledDelta.Hourly)
		result.ForecastBefore, result.ForecastAfter = &before, &after
	}
	return result, nil
}

// referenceMachineType is the most common machine type among the priced,
// schedulable, untainted nodes: the rates the cluster's own workloads get.
func referenceMachineType(report costmodel.CostReport) string {
	counts := make(map[string]int)
	for _, item := range report.Items {
		if item.Subject.Kind != costmodel.SubjectNode || !item.Priced() {
			continue
		}
		if ready, ok := item.Detail["ready"].(bool); ok && !ready {
			continue
		}
		if schedulable, ok := item.Detail["schedulable"].(bool); ok && !schedulable {
			continue
		}
		if taints, _ := item.Detail["taints"].([]string); len(taints) > 0 {
			continue
		}
		if machineType, _ := item.Detail["machineType"].(string); machineType != "" {
			counts[machineType]++
		}
	}
	best, bestCount := "", 0
	for machineType, count := range counts {
		if count > bestCount || (count == bestCount && machineType < best) {
			best, bestCount = machineType, count
		}
	}
	return best
}

func workloadFromManifest(parsed manifestmode.Result) whatif.Workload {
	var perReplica costmodel.Usage
	for _, resource := range parsed.Workload.Resources {
		perReplica = perReplica.Add(costmodel.Usage{CPUCores: resource.CPU, MemoryGB: resource.MemoryGB, GPUUnits: resource.GPU})
	}
	return whatif.Workload{
		Name:       parsed.Workload.Name,
		Namespace:  parsed.Workload.Namespace,
		Kind:       "Deployment",
		Replicas:   int(parsed.Workload.Replicas),
		PerReplica: perReplica,
		Containers: len(parsed.Workload.Resources),
	}
}
