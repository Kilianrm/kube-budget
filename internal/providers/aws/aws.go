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

// instanceCatalog contains illustrative on-demand EKS worker-node pricing snapshots.
// Values are not live prices and must be updated as AWS pricing changes.
var instanceCatalog = map[string]map[string]instanceType{
	"us-east-1": {
		"t3.medium":  {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.0416},
		"m6i.large":  {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.096},
		"m6i.xlarge": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.192},
		"c6i.large":  {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.085},
	},
	"eu-west-1": {
		"t3.medium":  {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.0464},
		"m6i.large":  {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.113},
		"m6i.xlarge": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.226},
		"c6i.large":  {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.096},
	},
}

// ebsGP3USDPerGBMonth is the on-demand price of EBS gp3 storage, converted to an hourly rate below.
const ebsGP3USDPerGBMonth = 0.08
const hoursPerMonth = 730
const cpuCostShare = 0.5

// NewPriceConfigForRegion derives a resource-price model from an AWS worker
// node type in the selected region.
func NewPriceConfigForRegion(instanceType, region string) (pricing.PriceConfig, error) {
	regionCatalog, ok := instanceCatalog[region]
	if !ok {
		return pricing.PriceConfig{}, fmt.Errorf("aws: unsupported region %q", region)
	}
	instance, ok := regionCatalog[instanceType]
	if !ok {
		return pricing.PriceConfig{}, fmt.Errorf("aws: unknown instance type %q for region %q", instanceType, region)
	}
	return pricing.PriceConfig{
		CPUUSDPerCore:   (instance.HourlyUSD * cpuCostShare) / instance.VCPU,
		MemoryUSDPerGB:  (instance.HourlyUSD * (1 - cpuCostShare)) / instance.MemoryGB,
		StorageUSDPerGB: ebsGP3USDPerGBMonth / hoursPerMonth,
	}, nil
}
