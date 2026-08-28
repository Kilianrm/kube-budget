package engine

import "kube-budget/core/pricing"

// Workload represents a Kubernetes workload that the estimator can evaluate.
type Workload struct {
	Name      string
	Resources []pricing.Resource
}

// Estimate contains the total cost and per-resource breakdown for a workload.
type Estimate struct {
	Total       float64
	PerResource map[string]float64
	Currency    string
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
		breakdown[resource.Name] = pricing.EstimateCost([]pricing.Resource{resource}, e.Config)
	}

	total := pricing.EstimateCost(workload.Resources, e.Config)
	return Estimate{
		Total:       total,
		PerResource: breakdown,
		Currency:    "USD",
	}
}
