package engine

import "kube-budget/core/pricing"

// Workload represents a Kubernetes workload that the estimator can evaluate.
type Workload struct {
	Name      string
	Namespace string
	Replicas  int32
	Resources []pricing.Resource
	// MinReplicas and MaxReplicas are optional autoscaler bounds. Both are nil
	// when the workload has no associated HorizontalPodAutoscaler.
	MinReplicas *int32
	MaxReplicas *int32
}

// Estimate contains the total cost and per-resource breakdown for a workload.
type Estimate struct {
	Total       float64
	PerResource map[string]float64
	Currency    string
	// MinTotal and MaxTotal describe the cost range implied by the workload's
	// autoscaler bounds. Both are nil when the workload has no autoscaler.
	MinTotal *float64
	MaxTotal *float64
}

// Engine is the main cost-estimation component.
type Engine struct {
	Config pricing.PriceConfig
}

// New creates a new engine with a pricing model.
func New(cfg pricing.PriceConfig) *Engine {
	return &Engine{Config: cfg}
}

// Estimate calculates the cost of a workload.
func (e *Engine) Estimate(workload Workload) Estimate {
	breakdown := make(map[string]float64)
	for _, resource := range workload.Resources {
		breakdown[resource.Name] = pricing.EstimateCost([]pricing.Resource{resource}, e.Config) * float64(workload.Replicas)
	}

	perReplicaCost := pricing.EstimateCost(workload.Resources, e.Config)
	total := perReplicaCost * float64(workload.Replicas)

	estimate := Estimate{
		Total:       total,
		PerResource: breakdown,
		Currency:    "USD",
	}

	if workload.MinReplicas != nil && workload.MaxReplicas != nil {
		minTotal := perReplicaCost * float64(*workload.MinReplicas)
		maxTotal := perReplicaCost * float64(*workload.MaxReplicas)
		estimate.MinTotal = &minTotal
		estimate.MaxTotal = &maxTotal
	}

	return estimate
}
