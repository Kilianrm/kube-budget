package wails

import (
	"fmt"
	"strings"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/optimize"
	"kube-budget/core/pricing"
	"kube-budget/internal/providers"
	"kube-budget/internal/storage/recommendations"
)

// OptimizationResult is the optimization plan plus what the user already did
// with it.
type OptimizationResult struct {
	Plan    optimize.Plan           `json:"plan"`
	Applied []AppliedRecommendation `json:"applied"`
	History []RecommendationEvent   `json:"history"`
}

// RecommendationEvent is one entry of the history log.
type RecommendationEvent struct {
	At      time.Time              `json:"at"`
	ID      string                 `json:"id"`
	Title   string                 `json:"title"`
	Action  recommendations.Action `json:"action"`
	Savings costmodel.Projection   `json:"savings"`
}

// AppliedRecommendation is one journal entry with the change in the billed
// rate since it was applied. The change includes everything else that moved
// the bill in between, so it is evidence, not proof.
type AppliedRecommendation struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	At    time.Time `json:"at"`
	// Pending until a report no longer detects the recommendation.
	Pending     bool                 `json:"pending"`
	ConfirmedAt *time.Time           `json:"confirmedAt,omitempty"`
	Expected    costmodel.Projection `json:"expected"`
	// Realized is the drop in the billed rate since then; negative when it rose.
	Realized costmodel.Projection `json:"realized"`
}

// RecommendationRequest names one recommendation of a cluster's plan.
type RecommendationRequest struct {
	ClusterID string `json:"clusterId"`
	ID        string `json:"id"`
}

// costCache keeps the last report per cluster, so acting on a recommendation
// re-plans instantly instead of collecting the cluster again.
type costCache struct {
	report costmodel.CostReport
	inputs optimize.Inputs
}

// DismissRecommendation takes a recommendation out of the plan's total.
func (adapter *ClusterAdapter) DismissRecommendation(request RecommendationRequest) (OptimizationResult, error) {
	return adapter.changeRecommendation(request, func(store *recommendations.Store, clusterID string, plan optimize.Plan, _ costmodel.CostReport) error {
		return store.Dismiss(clusterID, eventFor(plan, request.ID))
	})
}

// RestoreRecommendation brings a dismissed recommendation back.
func (adapter *ClusterAdapter) RestoreRecommendation(request RecommendationRequest) (OptimizationResult, error) {
	return adapter.changeRecommendation(request, func(store *recommendations.Store, clusterID string, plan optimize.Plan, _ costmodel.CostReport) error {
		return store.Restore(clusterID, eventFor(plan, request.ID))
	})
}

// ReopenRecommendation withdraws an applied mark that is still pending, for a
// change that was not made after all.
func (adapter *ClusterAdapter) ReopenRecommendation(request RecommendationRequest) (OptimizationResult, error) {
	return adapter.changeRecommendation(request, func(store *recommendations.Store, clusterID string, plan optimize.Plan, _ costmodel.CostReport) error {
		return store.Reopen(clusterID, eventFor(plan, request.ID))
	})
}

// eventFor describes a recommendation of the plan for the history log.
func eventFor(plan optimize.Plan, id string) recommendations.Event {
	event := recommendations.Event{At: time.Now(), ID: id, Title: id}
	for _, recommendation := range plan.Recommendations {
		if recommendation.ID == id {
			event.Title = recommendation.Title
			event.SavingsHourly = recommendation.Savings.Hourly
		}
	}
	return event
}

// MarkRecommendationApplied records the billed rate now, so later reports can
// show how the bill moved after the change.
func (adapter *ClusterAdapter) MarkRecommendationApplied(request RecommendationRequest) (OptimizationResult, error) {
	return adapter.changeRecommendation(request, func(store *recommendations.Store, clusterID string, plan optimize.Plan, report costmodel.CostReport) error {
		for _, recommendation := range plan.Recommendations {
			if recommendation.ID != request.ID {
				continue
			}
			return store.MarkApplied(clusterID, recommendations.Applied{
				ID:             recommendation.ID,
				Title:          recommendation.Title,
				At:             time.Now(),
				BaselineHourly: report.Totals[costmodel.BasisProvisioned].Hourly,
				ExpectedHourly: recommendation.Savings.Hourly,
			})
		}
		return fmt.Errorf("optimization: recommendation %q is not in the current plan", request.ID)
	})
}

type recommendationChange func(store *recommendations.Store, clusterID string, plan optimize.Plan, report costmodel.CostReport) error

func (adapter *ClusterAdapter) changeRecommendation(request RecommendationRequest, change recommendationChange) (OptimizationResult, error) {
	clusterID := strings.TrimSpace(request.ClusterID)
	cached, ok := adapter.cachedCost(clusterID)
	if !ok {
		return OptimizationResult{}, fmt.Errorf("optimization: no report for %q yet; refresh the cost report first", clusterID)
	}
	store, err := defaultRecommendationStore()
	if err != nil {
		return OptimizationResult{}, err
	}
	current, err := optimization(store, clusterID, cached.report, cached.inputs)
	if err != nil {
		return OptimizationResult{}, err
	}
	if err := change(store, clusterID, current.Plan, cached.report); err != nil {
		return OptimizationResult{}, err
	}
	return optimization(store, clusterID, cached.report, cached.inputs)
}

// optimization builds the plan with the cluster's dismissals, confirms the
// applied changes the report no longer detects, and prices the applied
// journal against the report's billed rate.
func optimization(store *recommendations.Store, clusterID string, report costmodel.CostReport, inputs optimize.Inputs) (OptimizationResult, error) {
	state, err := store.Get(clusterID)
	if err != nil {
		return OptimizationResult{}, err
	}
	result := optimizationFrom(state, report, inputs)
	detected := make(map[string]bool, len(result.Plan.Recommendations))
	for _, recommendation := range result.Plan.Recommendations {
		detected[recommendation.ID] = true
	}
	confirmed, err := store.Confirm(clusterID, detected, time.Now())
	if err != nil || !confirmed {
		return result, err
	}
	if state, err = store.Get(clusterID); err != nil {
		return OptimizationResult{}, err
	}
	return optimizationFrom(state, report, inputs), nil
}

func optimizationFrom(state recommendations.State, report costmodel.CostReport, inputs optimize.Inputs) OptimizationResult {
	inputs.Dismissed = make(map[string]bool, len(state.Dismissed))
	for id := range state.Dismissed {
		inputs.Dismissed[id] = true
	}

	result := OptimizationResult{
		Plan:    optimize.Build(report, inputs),
		Applied: make([]AppliedRecommendation, 0, len(state.Applied)),
		History: make([]RecommendationEvent, 0, len(state.History)),
	}
	billed := report.Totals[costmodel.BasisProvisioned].Hourly
	// Journals written before marks were idempotent can hold one recommendation
	// pending several times; the oldest (last) mark is the one that counts.
	oldestPending := make(map[string]int)
	for index, applied := range state.Applied {
		if applied.Pending() {
			oldestPending[applied.ID] = index
		}
	}
	for index, applied := range state.Applied {
		if applied.Pending() && oldestPending[applied.ID] != index {
			continue
		}
		result.Applied = append(result.Applied, AppliedRecommendation{
			ID:          applied.ID,
			Title:       applied.Title,
			At:          applied.At,
			Pending:     applied.Pending(),
			ConfirmedAt: applied.ConfirmedAt,
			Expected:    costmodel.Project(applied.ExpectedHourly),
			Realized:    costmodel.Project(applied.BaselineHourly - billed),
		})
	}
	for _, event := range state.History {
		result.History = append(result.History, RecommendationEvent{
			At:      event.At,
			ID:      event.ID,
			Title:   event.Title,
			Action:  event.Action,
			Savings: costmodel.Project(event.SavingsHourly),
		})
	}
	return result
}

// planInputs offers the region's machine types to the node-type step and
// derives the spot ratio from the catalog.
func planInputs(provider providers.Provider, region string) optimize.Inputs {
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

func (adapter *ClusterAdapter) rememberCost(clusterID string, report costmodel.CostReport, inputs optimize.Inputs) {
	adapter.costMutex.Lock()
	defer adapter.costMutex.Unlock()
	if adapter.lastCosts == nil {
		adapter.lastCosts = make(map[string]costCache)
	}
	adapter.lastCosts[clusterID] = costCache{report: report, inputs: inputs}
}

func (adapter *ClusterAdapter) cachedCost(clusterID string) (costCache, bool) {
	adapter.costMutex.Lock()
	defer adapter.costMutex.Unlock()
	cached, ok := adapter.lastCosts[clusterID]
	return cached, ok
}

func defaultRecommendationStore() (*recommendations.Store, error) {
	path, err := recommendations.DefaultPath()
	if err != nil {
		return nil, err
	}
	return recommendations.New(path), nil
}
