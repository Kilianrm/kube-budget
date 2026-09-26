package wails

import (
	"fmt"
	"strings"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/optimize"
	"kube-budget/core/pricing"
	"kube-budget/core/whatif"
	manifestmode "kube-budget/internal/application/manifest"
	"kube-budget/internal/providers"
)

// SimulationRequest asks what deploying a manifest does to a connected cluster.
type SimulationRequest struct {
	ClusterID string           `json:"clusterId"`
	Document  ManifestDocument `json:"document"`
}

// SimulationResult is the manifest's estimate at the cluster's own rates and
// its impact on the cluster's bill, budget and optimization plan.
type SimulationResult struct {
	Estimate ManifestResult `json:"estimate"`
	Impact   whatif.Result  `json:"impact"`
	// Peak is the impact at the autoscaler's maximum, when the manifest has one.
	Peak *whatif.Result `json:"peak,omitempty"`
	// ForecastBefore and ForecastAfter are this month's forecast without and
	// with the workload deployed now. Nil when history cannot be read.
	ForecastBefore *CostForecastResult `json:"forecastBefore,omitempty"`
	ForecastAfter  *CostForecastResult `json:"forecastAfter,omitempty"`
	// ReportAt is when the cluster report the simulation used was generated.
	ReportAt time.Time `json:"reportAt"`
}

// SimulateManifest prices a manifest with the connected cluster's rates and
// simulates deploying it on the cluster's last report.
func (adapter *ClusterAdapter) SimulateManifest(request SimulationRequest) (SimulationResult, error) {
	clusterID := strings.TrimSpace(request.ClusterID)
	if strings.TrimSpace(request.Document.Content) == "" {
		return SimulationResult{}, fmt.Errorf("simulation: manifest content is required")
	}
	cached, ok := adapter.cachedCost(clusterID)
	if !ok {
		return SimulationResult{}, fmt.Errorf("simulation: no cost report for %q yet; load the cluster's cost report first", clusterID)
	}
	report := cached.report

	selectedProvider, ok := providers.Get(report.Scope.Provider)
	if !ok {
		return SimulationResult{}, fmt.Errorf("simulation: the cluster's provider %q has no price catalog", report.Scope.Provider)
	}
	machineType := referenceMachineType(report)
	if machineType == "" {
		return SimulationResult{}, fmt.Errorf("simulation: the cluster has no priced, schedulable node to take rates from")
	}
	card, err := selectedProvider.ResolveResources(pricing.RateRequest{
		Provider: selectedProvider.Name(),
		Region:   report.Scope.Region,
		SKU:      machineType,
		Purchase: pricing.PurchaseOnDemand,
	})
	if err != nil {
		return SimulationResult{}, err
	}

	estimate, parsed, err := priceManifest(request.Document.Content, selectedProvider, report.Scope.Region, machineType)
	if err != nil {
		return SimulationResult{}, err
	}

	inputs := whatif.Inputs{RateCard: card, Plan: adapter.planInputsWithDismissals(clusterID, cached.inputs)}
	workload := workloadFromManifest(parsed)
	result := SimulationResult{
		Estimate: estimate,
		Impact:   whatif.Simulate(report, workload, inputs),
		ReportAt: report.GeneratedAt,
	}
	if maximum := parsed.Workload.MaxReplicas; maximum != nil && int(*maximum) > workload.Replicas {
		peak := workload
		peak.Replicas = int(*maximum)
		peakImpact := whatif.Simulate(report, peak, inputs)
		result.Peak = &peakImpact
	}

	if forecast, err := loadForecastInputs(clusterID, time.Now()); err == nil {
		runRate := report.Totals[costmodel.BasisProvisioned].Hourly
		before := forecast.forecast(runRate)
		after := forecast.withRunRateChange(before, result.Impact.BilledDelta.Hourly)
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

// planInputsWithDismissals applies the cluster's dismissed recommendations,
// so the plan before and after matches the one the Optimizations view shows.
func (adapter *ClusterAdapter) planInputsWithDismissals(clusterID string, inputs optimize.Inputs) optimize.Inputs {
	store, err := defaultRecommendationStore()
	if err != nil {
		return inputs
	}
	state, err := store.Get(clusterID)
	if err != nil {
		return inputs
	}
	inputs.Dismissed = make(map[string]bool, len(state.Dismissed))
	for id := range state.Dismissed {
		inputs.Dismissed[id] = true
	}
	return inputs
}
