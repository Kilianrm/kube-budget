// Package gcp derives core pricing values from Google Compute Engine and GKE pricing.
package gcp

import (
	"kube-budget/core/pricing"
	"kube-budget/internal/providers/catalog"
)

// Provider implements the shared pricing-provider contract for GCP.
type Provider struct {
	catalog.Catalog
}

// New creates a GCP pricing provider.
func New() Provider {
	return Provider{Catalog: gcpCatalog}
}

// gcpCatalog contains illustrative on-demand GKE worker-node pricing snapshots.
// Values are not live prices and must be updated as Google Cloud pricing changes.
var gcpCatalog = catalog.Catalog{
	ProviderName:    "gcp",
	Region:          "us-central1",
	MachineType:     "e2-standard-2",
	MachineTypeNoun: "machine type",
	Instances: map[string]map[string]catalog.Instance{
		"us-central1": {
			"e2-standard-2": {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.0670},
			"e2-standard-4": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.1340},
			"n2-standard-2": {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.0971},
		},
		"europe-west1": {
			"e2-standard-2": {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.0750},
			"e2-standard-4": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.1500},
			"n2-standard-2": {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.1090},
		},
	},
	StorageUSDPerGBMonth:   0.04,
	StorageDescription:     "persistent disk",
	ControlPlaneUSDPerHour: 0.10,
	CPUCostShare:           0.5,
	SpotPriceRatio:         0.40,
	Source:                 "static-catalog/gcp",
}

// NewPriceConfigForRegion derives a resource-price model from a GKE worker
// machine type in the selected region.
func NewPriceConfigForRegion(machineTypeName, region string) (pricing.PriceConfig, error) {
	return New().NewPriceConfig(machineTypeName, region)
}
