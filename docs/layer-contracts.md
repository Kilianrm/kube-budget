# Layer Contracts

This document defines how data moves between the parts of KubeBudget. It is different from [core-contract.md](core-contract.md), which defines the input and output of the core engine itself.

## Data flow

```text
Raw manifest -> Input delivery -> Kubernetes converter -> Normalized Workload -> Core engine -> Estimate
```

## 1. Input delivery contract

The input delivery layer receives raw manifest content from a file, HTTP request, or CI system.

Input:

```text
bytes containing YAML or JSON
```

Responsibilities:

- receive or read the content
- pass the content to the Kubernetes converter
- report file or transport errors

It does not interpret Kubernetes fields or calculate costs.

## 2. Kubernetes converter contract

The converter understands Kubernetes manifest structure.

Input:

```go
[]byte // raw YAML or JSON manifest
```

Output:

```go
core.Workload
```

Responsibilities:

- parse the manifest
- support the Kubernetes kinds defined by the MVP
- extract replicas and resource requests, including extended resources (e.g. `nvidia.com/gpu`)
- include native sidecars (`restartPolicy: Always` init containers) alongside regular containers
- optionally attach `MinReplicas`/`MaxReplicas` from an associated `HorizontalPodAutoscaler` manifest
- convert quantities such as `500m` and `512Mi`
- validate required values
- return errors for malformed or unsupported input

The converter must not contain pricing rules.

Example interface:

```go
type Converter interface {
    Convert(input []byte) (core.Workload, error)
}
```

## 3. Core engine contract

The core receives the normalized `core.Workload` produced by the converter and returns an `Estimate`. The complete data definition is documented in [core-contract.md](core-contract.md).

The core does not know whether the data came from a CLI, GitHub, a webhook, or a direct function call.

## 4. Error ownership

- Input delivery owns file, transport, and request-body errors.
- The converter owns malformed or unsupported manifest errors.
- The core owns invalid normalized models and calculation errors.

Each layer returns errors to its caller instead of silently inventing missing values.

## MVP decision

For the first implementation, input delivery and the Kubernetes converter may live in the same package. They are still separate logical responsibilities, but separate abstractions are unnecessary until another input source is added.
