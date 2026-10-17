# Core Contracts

The `core/` directory contains several calculation packages. This page maps
their responsibilities and defines the detailed contract for the single-
workload estimator. Cluster reporting, history, optimization, and rollout
simulation are described in [Cost model and explorer](cost-explorer-design.md);
the provider pricing boundary is described in [Layer contracts](layer-contracts.md).

| Package | Responsibility | Contract details |
|---|---|---|
| `engine` | Estimate a normalized workload from resource requests and rates | This page |
| `pricing` | Define estimator rates and provider rate-card contracts | This page and [Layer contracts](layer-contracts.md) |
| `allocation` | Price normalized cluster infrastructure and workload requests | [Cost model and explorer](cost-explorer-design.md) |
| `costmodel` | Represent, project, group, and reconcile cost reports | [Cost model and explorer](cost-explorer-design.md) |
| `costseries` | Integrate captured rates into history and build forecasts | [Cost model and explorer](cost-explorer-design.md) |
| `optimize` | Build an ordered plan with incremental, non-overlapping savings | [Cost model and explorer](cost-explorer-design.md) |
| `whatif` | Simulate workload placement and its impact on report costs | [Cost model and explorer](cost-explorer-design.md) |

The sections below specify only the `engine` and single-workload `pricing`
contract; they are not a complete field-by-field API reference for every core
package.

## Input

`core/engine.Workload` contains the workload identity, replica count, a list of
resource requests, and optional autoscaler bounds. Each `core/pricing.Resource`
represents one container:

```go
type Workload struct {
    Name        string
    Namespace   string
    Replicas    int32
    Resources   []pricing.Resource
    MinReplicas *int32
    MaxReplicas *int32
}

type Resource struct {
    Name      string
    CPU       float64 // cores
    MemoryGB  float64 // GiB, despite the field name
    StorageGB float64 // GiB
    GPU       float64 // device units
}
```

The converter supplies this model for supported Deployment manifests. The
engine itself does not parse Kubernetes objects or validate the source
manifest.

## Pricing and output

`pricing.PriceConfig` supplies hourly USD rates per CPU core, memory GB, storage
GB, and GPU unit. The engine returns an `engine.Estimate`:


```go
type Estimate struct {
    Total       float64
    PerResource map[string]float64
    Currency    string
    MinTotal    *float64
    MaxTotal    *float64
}
```

`Total` and `PerResource` are hourly USD at `Replicas`. `PerResource` is keyed
by `Resource.Name`; names should be unique. If names repeat, the map retains
only the last entry for that name, while `Total` still includes every entry.
`MinTotal` and `MaxTotal` are set together when both autoscaler bounds are
present; if either bound is nil, both outputs are nil. The bounds are used as
provided, without checking that they are nonnegative or ordered. The currency
is USD.

For each resource, the per-replica hourly cost is:

```text
CPU * CPU rate
+ MemoryGB * memory rate
+ StorageGB * storage rate
+ GPU * GPU rate
```

The total is the sum of per-replica resource costs multiplied by replicas.
Autoscaler totals use the same per-replica cost multiplied by each bound.
Rates are assumed to be hourly USD per unit. Daily and monthly figures are
projections made by the calling workflow, not additional engine outputs.
Monthly projections use 730 hours.

## Responsibilities

The estimator applies rates and returns a total and resource breakdown. It
does not validate workloads, resource names, replica counts, autoscaler bounds,
or pricing values; negative values are not rejected. Callers are responsible
for supplying meaningful normalized values. It does not read files, parse
manifests, resolve provider catalogs, access a cluster, or format user-facing
output. Manifest parsing and basic required-field checks belong to
`internal/converter`, which does not perform full Kubernetes API validation;
provider lookup belongs to the application and pricing resolver.