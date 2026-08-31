// Package manifest implements the manifest-based cost estimation workflow.
package manifest

import (
	"kube-budget/core/engine"
	"kube-budget/core/pricing"
	"kube-budget/internal/converter"
)

// Result contains the normalized workload and its cost estimate.
type Result struct {
	Workload engine.Workload
	Estimate engine.Estimate
}

// Service coordinates manifest conversion and cost estimation.
type Service struct {
	engine *engine.Engine
}

// New creates a Manifest Mode service with the supplied pricing configuration.
func New(config pricing.PriceConfig) *Service {
	return &Service{engine: engine.New(config)}
}

// Estimate converts a Kubernetes manifest and estimates its requested cost.
func (service *Service) Estimate(input []byte) (Result, error) {
	workload, err := converter.ConvertDeployment(input)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Workload: workload,
		Estimate: service.engine.Estimate(workload),
	}, nil
}