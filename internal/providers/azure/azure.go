package azure

// Package azure derives core pricing.PriceConfig values from Azure VM and disk pricing.

import (
	"fmt"
	"sort"
	"strings"

	"kube-budget/core/pricing"
)

// Provider implements the shared pricing-provider contract for Azure.
type Provider struct{}

// New creates an Azure pricing provider.
func New() Provider {
	return Provider{}
}

func (Provider) Name() string {
	return "azure"
}

func (Provider) DefaultRegion() string {
	return "eastus"
}

func (Provider) DefaultMachineType() string {
	return "Standard_D2s_v5"
}

func (Provider) NormalizeRegion(region string) string {
	return strings.TrimSpace(region)
}

type vmSize struct {
	VCPU      float64
	MemoryGB  float64
	HourlyUSD float64
}

// vmCatalog contains illustrative on-demand Azure VM pricing snapshots.
// Values are not live prices and must be updated as Azure pricing changes.
var vmCatalog = map[string]map[string]vmSize{
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
}

const (
	managedDiskUSDPerGBMonth = 0.05
	hoursPerMonth            = 730
	cpuCostShare             = 0.5
)

// NewPriceConfigForRegion derives a resource-price model from an Azure VM size
// in the selected region.
func NewPriceConfigForRegion(vmSizeName, region string) (pricing.PriceConfig, error) {
	return New().NewPriceConfig(vmSizeName, region)
}

// NewPriceConfig derives a resource-price model from an Azure VM size in the
// selected region.
func (Provider) NewPriceConfig(vmSizeName, region string) (pricing.PriceConfig, error) {
	regionCatalog, ok := vmCatalog[region]
	if !ok {
		return pricing.PriceConfig{}, fmt.Errorf("azure: unsupported region %q", region)
	}
	vm, ok := regionCatalog[vmSizeName]
	if !ok {
		return pricing.PriceConfig{}, fmt.Errorf("azure: unknown VM size %q for region %q", vmSizeName, region)
	}

	return pricing.PriceConfig{
		CPUUSDPerCore:   (vm.HourlyUSD * cpuCostShare) / vm.VCPU,
		MemoryUSDPerGB:  (vm.HourlyUSD * (1 - cpuCostShare)) / vm.MemoryGB,
		StorageUSDPerGB: managedDiskUSDPerGBMonth / hoursPerMonth,
	}, nil
}

func (Provider) Regions() []string {
	return sortedKeys(vmCatalog)
}

func (Provider) MachineTypes(region string) []string {
	return sortedKeys(vmCatalog[region])
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
