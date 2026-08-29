// Package aws derives core pricing.PriceConfig values from AWS EC2/EBS pricing.
package aws

import (
	"fmt"

	"kube-budget/core/pricing"
)

// instanceType describes the on-demand shape of an EC2 instance used as an EKS worker node.
type instanceType struct {
	VCPU      float64
	MemoryGB  float64
	HourlyUSD float64
}

// instanceCatalog holds on-demand us-east-1 pricing for common EKS worker node types.
// Values are illustrative snapshots, not live pricing.
var instanceCatalog = map[string]instanceType{
	"t3.medium":  {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.0416},
	"m6i.large":  {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.096},
	"m6i.xlarge": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.192},
	"c6i.large":  {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.085},
}

// ebsGP3USDPerGBMonth is the on-demand price of EBS gp3 storage, converted to an hourly rate below.
const ebsGP3USDPerGBMonth = 0.08
const hoursPerMonth = 730

// cpuCostShare is the assumed fraction of instance cost attributed to CPU vs. memory.
const cpuCostShare = 0.5

// NewPriceConfig derives a pricing.PriceConfig from an EC2 instance type's on-demand price,
// splitting the hourly instance cost between CPU and memory using cpuCostShare.
func NewPriceConfig(instanceType string) (pricing.PriceConfig, error) {
	inst, ok := instanceCatalog[instanceType]
	if !ok {
		return pricing.PriceConfig{}, fmt.Errorf("aws: unknown instance type %q", instanceType)
	}

	return pricing.PriceConfig{
		CPUUSDPerCore:   (inst.HourlyUSD * cpuCostShare) / inst.VCPU,
		MemoryUSDPerGB:  (inst.HourlyUSD * (1 - cpuCostShare)) / inst.MemoryGB,
		StorageUSDPerGB: ebsGP3USDPerGBMonth / hoursPerMonth,
	}, nil
}
