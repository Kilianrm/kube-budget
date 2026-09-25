// Package catalog turns a static per-provider price snapshot into the shared
// provider and rate-resolver contracts, so each cloud package only declares data.
package catalog

import (
	"fmt"
	"sort"
	"strings"

	"kube-budget/core/costmodel"
	"kube-budget/core/pricing"
)

// SKUControlPlane is the reserved SKU used to price a managed control plane.
const SKUControlPlane = "controlplane"

const hoursPerMonth = costmodel.HoursPerMonth

// Instance is the on-demand shape and price of one machine type.
type Instance struct {
	VCPU      float64
	MemoryGB  float64
	GPUUnits  float64
	HourlyUSD float64
}

// Catalog is a provider's static price snapshot.
type Catalog struct {
	ProviderName string
	Region       string
	MachineType  string
	// MachineTypeNoun is the provider's own wording, used in error messages.
	MachineTypeNoun        string
	Instances              map[string]map[string]Instance
	StorageUSDPerGBMonth   float64
	StorageDescription     string
	ControlPlaneUSDPerHour float64
	// CPUCostShare is the fraction of a machine price attributed to CPU; the
	// remainder goes to memory. Published prices never break this down.
	CPUCostShare float64
	// SpotPriceRatio is the assumed spot price as a fraction of on-demand.
	SpotPriceRatio float64
	// Source identifies the snapshot the prices came from.
	Source string
	// RegionAliases rewrites user-supplied region spellings before lookup.
	RegionAliases map[string]string
}

func (catalog Catalog) Name() string { return catalog.ProviderName }

func (catalog Catalog) DefaultRegion() string { return catalog.Region }

func (catalog Catalog) DefaultMachineType() string { return catalog.MachineType }

func (catalog Catalog) NormalizeRegion(region string) string {
	normalized := strings.TrimSpace(region)
	for from, to := range catalog.RegionAliases {
		normalized = strings.ReplaceAll(normalized, from, to)
	}
	return normalized
}

func (catalog Catalog) Regions() []string { return sortedKeys(catalog.Instances) }

func (catalog Catalog) MachineTypes(region string) []string {
	return sortedKeys(catalog.Instances[catalog.NormalizeRegion(region)])
}

// NewPriceConfig keeps the original per-unit pricing entry point used by
// Manifest Mode.
func (catalog Catalog) NewPriceConfig(machineType, region string) (pricing.PriceConfig, error) {
	card, err := catalog.ResolveResources(pricing.RateRequest{Region: region, SKU: machineType})
	if err != nil {
		return pricing.PriceConfig{}, err
	}
	return card.PriceConfig(), nil
}

// ResolveNode prices one machine-hour.
func (catalog Catalog) ResolveNode(request pricing.RateRequest) (pricing.InstanceRate, error) {
	instance, err := catalog.lookup(request.SKU, request.Region)
	if err != nil {
		return pricing.InstanceRate{}, err
	}

	hourly := instance.HourlyUSD
	confidence := costmodel.ConfidenceExact
	purchase := request.Purchase
	if purchase == "" {
		purchase = pricing.PurchaseOnDemand
	}
	if purchase == pricing.PurchaseSpot && catalog.SpotPriceRatio > 0 {
		hourly *= catalog.SpotPriceRatio
		confidence = costmodel.ConfidenceEstimated
	}

	return pricing.InstanceRate{
		SKU:        request.SKU,
		VCPU:       instance.VCPU,
		MemoryGB:   instance.MemoryGB,
		GPUUnits:   instance.GPUUnits,
		HourlyUSD:  hourly,
		Purchase:   purchase,
		Confidence: confidence,
		Source:     catalog.Source,
	}, nil
}

// ResolveResources splits a machine price into per-unit rates and reports the
// split as an explicit assumption.
func (catalog Catalog) ResolveResources(request pricing.RateRequest) (pricing.RateCard, error) {
	node, err := catalog.ResolveNode(request)
	if err != nil {
		return pricing.RateCard{}, err
	}

	card := pricing.RateCard{
		CPUUSDPerCoreHour:   (node.HourlyUSD * catalog.CPUCostShare) / node.VCPU,
		MemoryUSDPerGBHour:  (node.HourlyUSD * (1 - catalog.CPUCostShare)) / node.MemoryGB,
		StorageUSDPerGBHour: catalog.StorageUSDPerGBMonth / hoursPerMonth,
		CPUCostShare:        catalog.CPUCostShare,
		Confidence:          costmodel.ConfidenceDerived,
		Source:              catalog.Source,
		Assumptions: []costmodel.Assumption{
			{
				Key:    "cpu-cost-share",
				Detail: fmt.Sprintf("%.0f%% of the %s price attributed to CPU, the rest to memory", catalog.CPUCostShare*100, request.SKU),
			},
			{
				Key:    "storage-rate",
				Detail: fmt.Sprintf("%s billed at $%.4f per GB-month", catalog.StorageDescription, catalog.StorageUSDPerGBMonth),
			},
		},
	}
	if node.Purchase == pricing.PurchaseSpot {
		card.Confidence = costmodel.ConfidenceEstimated
		card.Assumptions = append(card.Assumptions, costmodel.Assumption{
			Key:    "spot-price-ratio",
			Detail: fmt.Sprintf("spot capacity priced at %.0f%% of on-demand", catalog.SpotPriceRatio*100),
		})
	}
	return card, nil
}

// ResolveFlat prices line items that do not scale with resources.
func (catalog Catalog) ResolveFlat(request pricing.RateRequest) (pricing.FlatRate, error) {
	if request.SKU != SKUControlPlane {
		return pricing.FlatRate{}, fmt.Errorf("%s: no flat rate for %q", catalog.ProviderName, request.SKU)
	}
	return pricing.FlatRate{
		Description: "managed control plane",
		HourlyUSD:   catalog.ControlPlaneUSDPerHour,
		Confidence:  costmodel.ConfidenceExact,
		Source:      catalog.Source,
	}, nil
}

func (catalog Catalog) lookup(machineType, region string) (Instance, error) {
	normalizedRegion := catalog.NormalizeRegion(region)
	regionCatalog, ok := catalog.Instances[normalizedRegion]
	if !ok {
		return Instance{}, fmt.Errorf("%s: unsupported region %q", catalog.ProviderName, region)
	}
	instance, ok := regionCatalog[strings.TrimSpace(machineType)]
	if !ok {
		return Instance{}, fmt.Errorf("%s: unknown %s %q for region %q", catalog.ProviderName, catalog.machineTypeNoun(), machineType, normalizedRegion)
	}
	return instance, nil
}

func (catalog Catalog) machineTypeNoun() string {
	if catalog.MachineTypeNoun == "" {
		return "instance type"
	}
	return catalog.MachineTypeNoun
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
