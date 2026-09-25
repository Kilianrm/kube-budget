// Package wails exposes application workflows as Wails-compatible Go bindings.
package wails

import (
	"fmt"
	"strings"

	"kube-budget/core/costmodel"
	manifestmode "kube-budget/internal/application/manifest"
	"kube-budget/internal/providers"
)

// ManifestAdapter exposes Manifest Mode to the desktop frontend.
type ManifestAdapter struct{}

// ManifestRequest describes manifest documents and the pricing selection made by the UI.
type ManifestRequest struct {
	Documents    []ManifestDocument `json:"documents"`
	Provider     string             `json:"provider"`
	Region       string             `json:"region"`
	InstanceType string             `json:"instanceType"`
}

// ManifestDocument contains one manifest selected by the user.
type ManifestDocument struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// ManifestResult is the presentation-neutral result returned to the frontend.
type ManifestResult struct {
	Workload     WorkloadResult        `json:"workload"`
	Pricing      PricingResult         `json:"pricing"`
	Cost         costmodel.Projection  `json:"cost"`
	HourlyTotal  float64               `json:"hourlyTotal"`
	DailyTotal   float64               `json:"dailyTotal"`
	MonthlyTotal float64               `json:"monthlyTotal"`
	Currency     string                `json:"currency"`
	MinTotal     *float64              `json:"minTotal,omitempty"`
	MaxTotal     *float64              `json:"maxTotal,omitempty"`
	MinCost      *costmodel.Projection `json:"minCost,omitempty"`
	MaxCost      *costmodel.Projection `json:"maxCost,omitempty"`
}

// WorkloadResult contains workload identity and resource requests for display.
type WorkloadResult struct {
	Name        string           `json:"name"`
	Namespace   string           `json:"namespace"`
	Replicas    int32            `json:"replicas"`
	MinReplicas *int32           `json:"minReplicas,omitempty"`
	MaxReplicas *int32           `json:"maxReplicas,omitempty"`
	Resources   []ResourceResult `json:"resources"`
}

// ResourceResult contains one container's requests and hourly estimated cost.
type ResourceResult struct {
	Name       string  `json:"name"`
	CPUCores   float64 `json:"cpuCores"`
	MemoryGB   float64 `json:"memoryGB"`
	StorageGB  float64 `json:"storageGB"`
	GPUUnits   float64 `json:"gpuUnits"`
	HourlyCost float64 `json:"hourlyCost"`
}

// PricingResult identifies the pricing assumptions used for the estimate.
type PricingResult struct {
	Provider     string `json:"provider"`
	Region       string `json:"region"`
	InstanceType string `json:"instanceType"`
}

// NewManifestAdapter creates the Manifest Mode desktop binding.
func NewManifestAdapter() *ManifestAdapter {
	return &ManifestAdapter{}
}

// EstimateManifest estimates the single document supported by the first dashboard version.
func (adapter *ManifestAdapter) EstimateManifest(request ManifestRequest) (ManifestResult, error) {
	if len(request.Documents) != 1 {
		return ManifestResult{}, fmt.Errorf("manifest adapter: expected exactly one document, got %d", len(request.Documents))
	}

	document := request.Documents[0]
	if strings.TrimSpace(document.Content) == "" {
		return ManifestResult{}, fmt.Errorf("manifest adapter: document content is required")
	}

	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	selectedProvider, ok := providers.Get(provider)
	if !ok {
		return ManifestResult{}, fmt.Errorf("manifest adapter: unsupported provider %q; supported providers: %s", request.Provider, strings.Join(providers.Names(), ", "))
	}
	region := selectedProvider.NormalizeRegion(request.Region)
	instanceType := strings.TrimSpace(request.InstanceType)
	if region == "" {
		return ManifestResult{}, fmt.Errorf("manifest adapter: region is required")
	}
	if instanceType == "" {
		return ManifestResult{}, fmt.Errorf("manifest adapter: instance type is required")
	}

	config, err := selectedProvider.NewPriceConfig(instanceType, region)
	if err != nil {
		return ManifestResult{}, err
	}
	result, err := manifestmode.New(config).Estimate([]byte(document.Content))
	if err != nil {
		return ManifestResult{}, err
	}

	resources := make([]ResourceResult, 0, len(result.Workload.Resources))
	for _, resource := range result.Workload.Resources {
		resources = append(resources, ResourceResult{
			Name:       resource.Name,
			CPUCores:   resource.CPU,
			MemoryGB:   resource.MemoryGB,
			StorageGB:  resource.StorageGB,
			GPUUnits:   resource.GPU,
			HourlyCost: result.Estimate.PerResource[resource.Name],
		})
	}

	cost := costmodel.Project(result.Estimate.Total)

	return ManifestResult{
		Workload: WorkloadResult{
			Name:        result.Workload.Name,
			Namespace:   result.Workload.Namespace,
			Replicas:    result.Workload.Replicas,
			MinReplicas: result.Workload.MinReplicas,
			MaxReplicas: result.Workload.MaxReplicas,
			Resources:   resources,
		},
		Pricing: PricingResult{
			Provider:     provider,
			Region:       region,
			InstanceType: instanceType,
		},
		Cost:         cost,
		HourlyTotal:  cost.Hourly,
		DailyTotal:   cost.Daily,
		MonthlyTotal: cost.Monthly,
		Currency:     result.Estimate.Currency,
		MinTotal:     result.Estimate.MinTotal,
		MaxTotal:     result.Estimate.MaxTotal,
		MinCost:      projectBound(result.Estimate.MinTotal),
		MaxCost:      projectBound(result.Estimate.MaxTotal),
	}, nil
}

func projectBound(hourlyUSD *float64) *costmodel.Projection {
	if hourlyUSD == nil {
		return nil
	}
	projection := costmodel.Project(*hourlyUSD)
	return &projection
}
