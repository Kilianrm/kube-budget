# Core Engine Contract

This document defines the contract of the cost-estimation core. It describes the data the core accepts and the result it returns.

The core does not receive raw Kubernetes YAML. A converter prepares this normalized input before calling the engine.

## Core input

```go
type Workload struct {
    Name      string
    Namespace string
    Replicas  int32
    Resources []Resource
}

type Resource struct {
    Name      string
    CPU       float64
    MemoryGB  float64
    StorageGB float64
}
```

The normalized model contains:

- workload name and namespace
- replica count
- CPU requests expressed in cores
- memory requests expressed in GB
- storage requests expressed in GB
- a name for each resource entry

The core does not know whether the model came from Kubernetes, a file, a webhook, or another source.

## Core output

```go
type Estimate struct {
    Total       float64
    PerResource map[string]float64
    Currency    string
}
```

Output semantics:

- `Total`: total estimated cost for the workload
- `PerResource`: cost contribution for each resource entry
- `Currency`: the currency used by the pricing configuration

## Calculation

The MVP applies the configured rates to every resource and then multiplies the result by the workload replica count:

```text
resource cost = (CPU * CPU rate) + (Memory * memory rate) + (Storage * storage rate)
total cost = resource cost * replicas
```

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
