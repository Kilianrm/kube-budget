package wails

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"kube-budget/core/costmodel"
	"kube-budget/core/optimize"
	"kube-budget/core/pricing"
	clustermode "kube-budget/internal/application/cluster"
	"kube-budget/internal/providers"
	"kube-budget/internal/storage/recommendations"
)

// OptimizationResult is the optimization plan plus what the user already did
// with it.
type OptimizationResult struct {
	Plan    optimize.Plan           `json:"plan"`
	Applied []AppliedRecommendation `json:"applied"`
	History []RecommendationEvent   `json:"history"`
	// Progress lists, by recommendation ID, the items already done of
	// recommendations being applied.
	Progress map[string][]DoneResource `json:"progress"`
	// Claimable are the IDs logged as no longer detected that the user can
	// still record as applied outside KubeBudget.
	Claimable []string `json:"claimable"`
}

// DoneResource is one item a report stopped detecting.
type DoneResource struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace,omitempty"`
	At        time.Time `json:"at"`
	// Savings is what the plan claimed for it while it was detected.
	Savings costmodel.Projection `json:"savings"`
	// Item is the item as last detected; absent for items done before it was
	// kept.
	Item *optimize.Item `json:"item,omitempty"`
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
	Pending     bool       `json:"pending"`
	ConfirmedAt *time.Time `json:"confirmedAt,omitempty"`
	// Unmarked when the change was made outside KubeBudget and only noticed
	// because the recommendation disappeared.
	Unmarked bool                 `json:"unmarked,omitempty"`
	Expected costmodel.Projection `json:"expected"`
	// Details is the recommendation as last detected; absent for changes
	// confirmed before it was kept.
	Details *optimize.Recommendation `json:"details,omitempty"`
	Done    []DoneResource           `json:"done,omitempty"`
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

// ClaimRecommendation records a recommendation logged as no longer detected
// as applied outside KubeBudget, when the user says the change was theirs.
func (adapter *ClusterAdapter) ClaimRecommendation(request RecommendationRequest) (OptimizationResult, error) {
	return adapter.changeRecommendation(request, func(store *recommendations.Store, clusterID string, _ optimize.Plan, _ costmodel.CostReport) error {
		return store.Claim(clusterID, request.ID, time.Now())
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

// MarkRecommendationApplied starts applying a recommendation: it is frozen as
// it is now, and the billed rate is recorded so later reports can show how
// the bill moved after the change.
func (adapter *ClusterAdapter) MarkRecommendationApplied(request RecommendationRequest) (OptimizationResult, error) {
	return adapter.changeRecommendation(request, func(store *recommendations.Store, clusterID string, plan optimize.Plan, report costmodel.CostReport) error {
		for _, recommendation := range plan.Recommendations {
			if recommendation.ID != request.ID {
				continue
			}
			// The recommendation is frozen as it is now: the plan shows this
			// copy, with its progress, until the cluster shows it done.
			details, err := json.Marshal(recommendation)
			if err != nil {
				return fmt.Errorf("optimization: freeze recommendation %q: %w", recommendation.ID, err)
			}
			return store.MarkApplied(clusterID, recommendations.Applied{
				ID:             recommendation.ID,
				Title:          recommendation.Title,
				At:             time.Now(),
				BaselineHourly: report.Totals[costmodel.BasisProvisioned].Hourly,
				ExpectedHourly: recommendation.Savings.Hourly,
				Details:        details,
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
// changes the report no longer detects, marked or not, and prices the applied
// journal against the report's billed rate.
func optimization(store *recommendations.Store, clusterID string, report costmodel.CostReport, inputs optimize.Inputs) (OptimizationResult, error) {
	state, err := store.Get(clusterID)
	if err != nil {
		return OptimizationResult{}, err
	}
	result := optimizationFrom(state, report, inputs)
	billed := report.Totals[costmodel.BasisProvisioned].Hourly
	detected := make(map[string]recommendations.Seen, len(result.Plan.Recommendations))
	for _, recommendation := range result.Plan.Recommendations {
		// A change being applied is confirmed once the cluster shows it
		// complete, so it counts as detected until then.
		if recommendation.Applying && recommendation.Complete {
			continue
		}
		detected[recommendation.ID] = seenFrom(recommendation, billed)
	}
	// Paused node steps were not evaluated: they are neither gone nor done.
	if result.Plan.Paused != "" {
		for id, seen := range state.Detected {
			if _, listed := detected[id]; !listed && optimize.IsNodeStep(id) {
				if _, applying := pendingDetails(state)[id]; !applying {
					detected[id] = seen
				}
			}
		}
	}
	if _, err := store.Confirm(clusterID, detected, report.GeneratedAt, time.Now()); err != nil {
		return result, err
	}
	// Rebuilt from the state as this report left it, which also knows the
	// report before, for what changed since.
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
	inputs.Applying = pendingDetails(state)
	inputs.LastKnown = make(map[string]optimize.Recommendation, len(state.Detected))
	for id, seen := range state.Detected {
		var recommendation optimize.Recommendation
		if len(seen.Details) > 0 && json.Unmarshal(seen.Details, &recommendation) == nil {
			inputs.LastKnown[id] = recommendation
		}
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
		var details *optimize.Recommendation
		if len(applied.Details) > 0 {
			details = new(optimize.Recommendation)
			if json.Unmarshal(applied.Details, details) != nil {
				details = nil
			}
		}
		result.Applied = append(result.Applied, AppliedRecommendation{
			ID:          applied.ID,
			Title:       applied.Title,
			At:          applied.At,
			Pending:     applied.Pending(),
			ConfirmedAt: applied.ConfirmedAt,
			Unmarked:    applied.Unmarked,
			Expected:    costmodel.Project(applied.ExpectedHourly),
			Realized:    costmodel.Project(applied.BaselineHourly - billed),
			Details:     details,
			Done:        doneResources(applied.Done),
		})
	}
	result.Progress = make(map[string][]DoneResource, len(state.Progress))
	for id, done := range state.Progress {
		result.Progress[id] = doneResources(done)
	}
	compareWithPrevious(&result.Plan, state)
	result.Claimable = make([]string, 0)
	for id, absence := range state.Absent {
		if absence.Logged {
			result.Claimable = append(result.Claimable, id)
		}
	}
	sort.Strings(result.Claimable)
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

// compareWithPrevious marks what moved in the plan since the report before:
// steps that are new or whose saving changed, and steps that left it, either
// done or gone. Steps being applied or paused carry their own explanation.
func compareWithPrevious(plan *optimize.Plan, state recommendations.State) {
	if state.PreviousAt.IsZero() {
		return
	}
	listed := make(map[string]bool, len(plan.Steps))
	for index := range plan.Steps {
		step := &plan.Steps[index]
		listed[step.ID] = true
		if step.Status != "" {
			continue
		}
		previous, known := state.Previous[step.ID]
		switch {
		case !known:
			step.Change = "new"
		case math.Abs(step.Savings.Hourly-previous.SavingsHourly) > math.Max(0.02*previous.SavingsHourly, 0.5/costmodel.HoursPerMonth):
			step.Change, step.Previous = "changed", costmodel.Project(previous.SavingsHourly)
		}
	}
	confirmed := make(map[string]bool)
	for _, applied := range state.Applied {
		if applied.ConfirmedAt != nil && applied.ConfirmedAt.After(state.PreviousAt) {
			confirmed[applied.ID] = true
		}
	}
	for _, id := range sortedIDs(state.Previous) {
		previous := state.Previous[id]
		if _, dismissed := state.Dismissed[id]; listed[id] || dismissed || previous.SavingsHourly <= 0 {
			continue
		}
		step := optimize.Step{ID: id, Title: previous.Title, Previous: costmodel.Project(previous.SavingsHourly), Change: "gone"}
		if confirmed[id] {
			step.Change = "done"
		}
		plan.Gone = append(plan.Gone, step)
	}
}

func sortedIDs(values map[string]recommendations.Seen) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// pendingDetails are the recommendations being applied, frozen when they
// were marked. Marks made before recommendations were frozen have none and
// fall back to the live recommendation.
func pendingDetails(state recommendations.State) map[string]optimize.Recommendation {
	frozen := make(map[string]optimize.Recommendation)
	for _, applied := range state.Applied {
		if !applied.Pending() || len(applied.Details) == 0 {
			continue
		}
		var recommendation optimize.Recommendation
		if json.Unmarshal(applied.Details, &recommendation) == nil {
			// the oldest mark comes last and wins
			frozen[applied.ID] = recommendation
		}
	}
	return frozen
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

// seenFrom describes a detected recommendation for the store. Items are
// tracked one by one only when each is a change of its own.
func seenFrom(recommendation optimize.Recommendation, billed float64) recommendations.Seen {
	seen := recommendations.Seen{Title: recommendation.Title, SavingsHourly: recommendation.Savings.Hourly, BilledHourly: billed}
	if details, err := json.Marshal(recommendation); err == nil {
		seen.Details = details
	}
	if !recommendation.SeparateItems {
		return seen
	}
	for _, item := range recommendation.Items {
		// items of a change being applied that are done count as gone
		if item.Done {
			continue
		}
		subject := item.Subject
		entry := recommendations.SeenItem{
			Key:           string(subject.Kind) + "/" + subject.Namespace + "/" + subject.Name,
			Name:          subject.Name,
			Namespace:     subject.Namespace,
			SavingsHourly: item.Savings.Hourly,
		}
		if details, err := json.Marshal(item); err == nil {
			entry.Details = details
		}
		seen.Items = append(seen.Items, entry)
	}
	return seen
}

func doneResources(done []recommendations.DoneItem) []DoneResource {
	resources := make([]DoneResource, 0, len(done))
	for _, item := range done {
		resource := DoneResource{Name: item.Name, Namespace: item.Namespace, At: item.At, Savings: costmodel.Project(item.SavingsHourly)}
		if len(item.Details) > 0 {
			resource.Item = new(optimize.Item)
			if json.Unmarshal(item.Details, resource.Item) != nil {
				resource.Item = nil
			}
		}
		resources = append(resources, resource)
	}
	return resources
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
