package wails

import (
	"fmt"
	"strings"
	"time"

	"kube-cost-workspace/core/whatif"
	"kube-cost-workspace/internal/application/workspace"
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

	simulation, err := workspace.Simulate(workspace.PricedCluster{Report: cached.report, ClusterID: clusterID, Plan: cached.inputs}, []byte(request.Document.Content))
	if err != nil {
		return SimulationResult{}, err
	}
	result := SimulationResult{
		Estimate: manifestResult(simulation.Estimate),
		Impact:   simulation.Impact,
		Peak:     simulation.Peak,
		ReportAt: simulation.ReportAt,
	}
	if simulation.ForecastBefore != nil && simulation.ForecastAfter != nil {
		before, after := forecastResult(*simulation.ForecastBefore), forecastResult(*simulation.ForecastAfter)
		result.ForecastBefore, result.ForecastAfter = &before, &after
	}
	return result, nil
}
