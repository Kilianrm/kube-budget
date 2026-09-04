package gcp

// Package gcp derives core pricing.PriceConfig values from Google Compute Engine pricing.

import (
	"fmt"
	"sort"
	"strings"

	"kube-budget/core/pricing"
)

// Provider implements the shared pricing-provider contract for GCP.
type Provider struct{}

// New creates a GCP pricing provider.
func New() Provider {
	return Provider{}
}

func (Provider) Name() string {
	return "gcp"
}

func (Provider) DefaultRegion() string {
	return "us-central1"
}

func (Provider) DefaultMachineType() string {
	return "e2-standard-2"
}

func (Provider) NormalizeRegion(region string) string {
	return strings.TrimSpace(region)
}

// machineType describes the on-demand shape of a Compute Engine machine used as
// a GKE worker node.
type machineType struct {
	VCPU      float64
	MemoryGB  float64
	HourlyUSD float64
}

// machineCatalog contains illustrative on-demand GKE worker-node pricing snapshots.
// Values are not live prices and must be updated as Google Cloud pricing changes.
var machineCatalog = map[string]map[string]machineType{
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
}

const (
	persistentDiskUSDPerGBMonth = 0.04
	hoursPerMonth               = 730
	cpuCostShare                = 0.5
)

// NewPriceConfigForRegion derives a resource-price model from a GKE worker
// machine type in the selected region.
func NewPriceConfigForRegion(machineTypeName, region string) (pricing.PriceConfig, error) {
	return New().NewPriceConfig(machineTypeName, region)
}

// NewPriceConfig derives a resource-price model from a GKE worker machine
// type in the selected region.
func (Provider) NewPriceConfig(machineTypeName, region string) (pricing.PriceConfig, error) {
	regionCatalog, ok := machineCatalog[region]
	if !ok {
		return pricing.PriceConfig{}, fmt.Errorf("gcp: unsupported region %q", region)
	}
	machine, ok := regionCatalog[machineTypeName]
	if !ok {
		return pricing.PriceConfig{}, fmt.Errorf("gcp: unknown machine type %q for region %q", machineTypeName, region)
	}

	return pricing.PriceConfig{
		CPUUSDPerCore:   (machine.HourlyUSD * cpuCostShare) / machine.VCPU,
		MemoryUSDPerGB:  (machine.HourlyUSD * (1 - cpuCostShare)) / machine.MemoryGB,
		StorageUSDPerGB: persistentDiskUSDPerGBMonth / hoursPerMonth,
	}, nil
}

func (Provider) Regions() []string {
	return sortedKeys(machineCatalog)
}

func (Provider) MachineTypes(region string) []string {
	return sortedKeys(machineCatalog[region])
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
