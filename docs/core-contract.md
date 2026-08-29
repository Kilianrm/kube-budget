# Core Engine Contract

This document defines the contract of the cost-estimation core. It describes the data the core accepts and the result it returns.

The core does not receive raw Kubernetes YAML. A converter prepares this normalized input before calling the engine.

## Core input

```go
type Workload struct {
    Name        string
    Namespace   string
    Replicas    int32
    Resources   []Resource
    MinReplicas *int32 // optional, set when an autoscaler is attached
    MaxReplicas *int32 // optional, set when an autoscaler is attached
}

type Resource struct {
    Name      string
    CPU       float64
    MemoryGB  float64
    StorageGB float64
    GPU       float64
}
```

The normalized model contains:

- workload name and namespace
- replica count
- optional min/max replica bounds, when the workload has an associated autoscaler
- CPU requests expressed in cores
- memory requests expressed in GB
- storage requests expressed in GB
- GPU requests expressed in units (whole GPUs)
- a name for each resource entry

The core does not know whether the model came from Kubernetes, a file, a webhook, or another source.

## Core output

```go
type Estimate struct {
    Total       float64
    PerResource map[string]float64
    Currency    string
    MinTotal    *float64 // optional, cost at MinReplicas when an autoscaler is attached
    MaxTotal    *float64 // optional, cost at MaxReplicas when an autoscaler is attached
}
```

Output semantics:

- `Total`: total estimated cost for the workload at `Replicas`
- `PerResource`: cost contribution for each resource entry
- `Currency`: the currency used by the pricing configuration
- `MinTotal`/`MaxTotal`: cost range implied by `MinReplicas`/`MaxReplicas`, nil when the workload has no autoscaler

## Calculation

The MVP applies the configured rates to every resource and then multiplies the result by the workload replica count:

```text
resource cost = (CPU * CPU rate) + (Memory * memory rate) + (Storage * storage rate) + (GPU * GPU rate)
total cost = resource cost * replicas
```

When `MinReplicas`/`MaxReplicas` are set, the same per-replica resource cost is also multiplied by each bound to produce `MinTotal`/`MaxTotal`.

## Core responsibilities

The core engine is responsible for:

- validating the normalized model
- applying pricing configuration
- calculating the total cost
- returning the resource breakdown

The core engine is not responsible for:

- reading files or HTTP requests
- parsing YAML or JSON
- understanding Kubernetes API objects
- converting Kubernetes quantities
- formatting output for users
