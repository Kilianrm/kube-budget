// Package aws derives core pricing values from AWS EC2/EBS/EKS pricing.
package aws

import (
	"kube-budget/core/pricing"
	"kube-budget/internal/providers/catalog"
)

// Provider implements the shared pricing-provider contract for AWS.
type Provider struct {
	catalog.Catalog
}

// New creates an AWS pricing provider.
func New() Provider {
	return Provider{Catalog: awsCatalog}
}

// awsCatalog contains illustrative on-demand EKS worker-node pricing snapshots.
// Values are not live prices and must be updated as AWS pricing changes.
var awsCatalog = catalog.Catalog{
	ProviderName:    "aws",
	Region:          "us-east-1",
	MachineType:     "m6i.large",
	MachineTypeNoun: "instance type",
	Instances: map[string]map[string]catalog.Instance{
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
	},
	StorageUSDPerGBMonth:   0.08,
	StorageDescription:     "EBS gp3",
	ControlPlaneUSDPerHour: 0.10,
	CPUCostShare:           0.5,
	SpotPriceRatio:         0.35,
	Source:                 "static-catalog/aws",
	RegionAliases:          map[string]string{"west1": "west-1"},
}

// NewPriceConfigForRegion derives a resource-price model from an AWS worker
// node type in the selected region.
func NewPriceConfigForRegion(instanceType, region string) (pricing.PriceConfig, error) {
	return New().NewPriceConfig(instanceType, region)
}
