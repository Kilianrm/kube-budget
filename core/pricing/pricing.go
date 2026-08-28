package pricing

// Resource represents a Kubernetes resource that contributes to cost estimation.
type Resource struct {
	Name      string
	CPU       float64
	MemoryGB  float64
	StorageGB float64
}

// PriceConfig contains the pricing assumptions used by the estimator.
type PriceConfig struct {
	CPUUSDPerCore      float64
	MemoryUSDPerGB     float64
	StorageUSDPerGB    float64
}

// EstimateCost calculates the cost of a set of resources based on a simple pricing model.
func EstimateCost(resources []Resource, cfg PriceConfig) float64 {
	var total float64
	for _, resource := range resources {
		total += resource.CPU * cfg.CPUUSDPerCore
		total += resource.MemoryGB * cfg.MemoryUSDPerGB
		total += resource.StorageGB * cfg.StorageUSDPerGB
	}
	return total
}
