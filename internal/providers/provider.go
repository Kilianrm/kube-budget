package providers

// Package providers defines the pricing-provider contract and registry.

import "kube-budget/core/pricing"

// Provider supplies pricing assumptions for a cloud provider.
type Provider interface {
	Name() string
	DefaultRegion() string
	DefaultMachineType() string
	NormalizeRegion(region string) string
	NewPriceConfig(machineType, region string) (pricing.PriceConfig, error)
	Regions() []string
	MachineTypes(region string) []string
}
