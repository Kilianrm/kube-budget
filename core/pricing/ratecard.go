package pricing

import "kube-budget/core/costmodel"

// PurchaseOption is the commercial term a resource is billed under.
type PurchaseOption string

const (
	PurchaseOnDemand PurchaseOption = "on-demand"
	PurchaseSpot     PurchaseOption = "spot"
	PurchaseReserved PurchaseOption = "reserved"
)

// RateRequest asks a resolver for the price of one SKU.
type RateRequest struct {
	Provider string
	Region   string
	SKU      string
	Purchase PurchaseOption
}

// InstanceRate is the billed price of one machine-hour, used for the
// provisioned basis.
type InstanceRate struct {
	SKU        string
	VCPU       float64
	MemoryGB   float64
	GPUUnits   float64
	HourlyUSD  float64
	Purchase   PurchaseOption
	Confidence costmodel.Confidence
	Source     string
}

// RateCard holds per-unit hourly rates, used for the requested basis.
// CPUCostShare records how much of a machine price was attributed to CPU, so
// the split stops being an invisible assumption.
type RateCard struct {
	CPUUSDPerCoreHour   float64
	MemoryUSDPerGBHour  float64
	StorageUSDPerGBHour float64
	GPUUSDPerUnitHour   float64
	CPUCostShare        float64
	Confidence          costmodel.Confidence
	Source              string
	Assumptions         []costmodel.Assumption
}

// FlatRate is an hourly fee that does not scale with resources, such as a
// managed control plane or a load balancer.
type FlatRate struct {
	Description string
	HourlyUSD   float64
	Confidence  costmodel.Confidence
	Source      string
}

// PriceConfig adapts a rate card to the engine's existing pricing input.
func (card RateCard) PriceConfig() PriceConfig {
	return PriceConfig{
		CPUUSDPerCore:   card.CPUUSDPerCoreHour,
		MemoryUSDPerGB:  card.MemoryUSDPerGBHour,
		StorageUSDPerGB: card.StorageUSDPerGBHour,
		GPUUSDPerUnit:   card.GPUUSDPerUnitHour,
	}
}

// RateResolver turns a SKU into a price. Implementations may read a static
// catalog today and a live pricing API later without any core change.
type RateResolver interface {
	// ResolveNode prices one machine-hour.
	ResolveNode(request RateRequest) (InstanceRate, error)
	// ResolveResources prices individual CPU, memory, storage and GPU units.
	ResolveResources(request RateRequest) (RateCard, error)
	// ResolveFlat prices a non-resource-proportional line item.
	ResolveFlat(request RateRequest) (FlatRate, error)
}
