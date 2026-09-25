package providers

// Package providers defines the pricing-provider contract and registry.

import "kube-budget/core/pricing"

// Provider supplies pricing assumptions for a cloud provider. It is also a
// pricing.RateResolver, so cluster-wide cost reports resolve rates through the
// same catalogs Manifest Mode uses.
type Provider interface {
	pricing.RateResolver

	Name() string
	DefaultRegion() string
	DefaultMachineType() string
	NormalizeRegion(region string) string
	NewPriceConfig(machineType, region string) (pricing.PriceConfig, error)
	Regions() []string
	MachineTypes(region string) []string
}
