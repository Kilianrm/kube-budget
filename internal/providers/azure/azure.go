// Package azure derives core pricing values from Azure VM, disk and AKS pricing.
package azure

import (
	"kube-budget/core/pricing"
	"kube-budget/internal/providers/catalog"
)

// Provider implements the shared pricing-provider contract for Azure.
type Provider struct {
	catalog.Catalog
}

// New creates an Azure pricing provider.
func New() Provider {
	return Provider{Catalog: azureCatalog}
}

// azureCatalog contains illustrative on-demand Azure VM pricing snapshots.
// Values are not live prices and must be updated as Azure pricing changes.
var azureCatalog = catalog.Catalog{
	ProviderName:    "azure",
	Region:          "eastus",
	MachineType:     "Standard_D2s_v5",
	MachineTypeNoun: "VM size",
	Instances: map[string]map[string]catalog.Instance{
		"eastus": {
			"Standard_D2s_v5": {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.0960},
			"Standard_D4s_v5": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.1920},
			"Standard_F2s_v2": {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.0850},
		},
		"westeurope": {
			"Standard_D2s_v5": {VCPU: 2, MemoryGB: 8, HourlyUSD: 0.1090},
			"Standard_D4s_v5": {VCPU: 4, MemoryGB: 16, HourlyUSD: 0.2180},
			"Standard_F2s_v2": {VCPU: 2, MemoryGB: 4, HourlyUSD: 0.0960},
		},
	},
	StorageUSDPerGBMonth: 0.05,
	StorageDescription:   "managed disk",
	// AKS bills the control plane only under the paid uptime SLA tier.
	ControlPlaneUSDPerHour: 0.10,
	CPUCostShare:           0.5,
	SpotPriceRatio:         0.40,
	Source:                 "static-catalog/azure",
}

// NewPriceConfigForRegion derives a resource-price model from an Azure VM size
// in the selected region.
func NewPriceConfigForRegion(vmSizeName, region string) (pricing.PriceConfig, error) {
	return New().NewPriceConfig(vmSizeName, region)
}
