# Layer Contracts

This document describes the data boundaries implemented by the application.
The workload estimator's normalized input and output are defined in
[Core contract](core-contract.md).

## Workflow map

```text
Manifest bytes -> converter -> engine.Workload -> core.Engine -> engine.Estimate

Kubernetes API -> collector -> cluster.Snapshot -> cost explorer -> CostReport
                                                         |             |
                                                         |             +-> optimization plan
                                                         +-> allocation core
                                                              -> history / forecast
```

The desktop and CLI adapt user input and output around these workflows. They do
not own separate pricing or cost-allocation rules.

## Manifest input

`internal/converter.ConvertDeployment` accepts YAML or JSON bytes for one
`apps/v1` Deployment and returns an `engine.Workload`.

The converter:

- checks the API version, kind, name, and presence of containers
- converts each regular container's CPU, memory, ephemeral-storage, and
  `nvidia.com/gpu` requests when present
- includes native sidecars declared as init containers with
  `restartPolicy: Always`
- requires each container to declare at least a CPU or memory request
- returns parsing, unsupported-kind, and incomplete-request errors

Autoscaler bounds are supported by the separate
`ConvertDeploymentWithAutoscaler` helper, which takes the Deployment and HPA
documents separately. The basic converter does not discover or resolve
associated documents. The converter contains no pricing rules.

## Workload estimation

The converter's `engine.Workload` is priced by `core/engine`. Each entry in
`Resources` represents a container's normalized requests. Rates come from a
`pricing.PriceConfig`; totals are hourly USD, multiplied by replicas. Optional
minimum and maximum replica bounds produce additional totals. See
[Core contract](core-contract.md) for the calculation and units.

The engine is a calculation primitive, not a Kubernetes validator: conversion
errors are returned by the converter, while `Engine.Estimate` returns an
estimate directly.

## Cluster collection and cost reporting

The cluster application returns a `cluster.Snapshot` (schema version 4). It
retains Kubernetes-specific identity and context, including nodes, workloads,
pods and their placements, resources, provider metadata, and collection
warnings. Optional metrics-server samples are attached as observed pod usage;
they do not replace resource requests.

`internal/application/costexplorer` projects the snapshot into the neutral
`core/allocation.Cluster` input. The allocator resolves rates through
`pricing.RateResolver` and produces a `costmodel.CostReport`. Unknown rates and
partial collection are represented as warnings or unpriced line items rather
than silently assigned a price.

The report keeps requested and provisioned bases separate. It also carries
reconciled idle and shared costs, assumptions, warnings, optional usage
summaries, and dimension breakdowns. See
[Cost model and explorer](cost-explorer-design.md) for the pricing rules and
their assumptions.

## Pricing provider boundary

`pricing.RateResolver` exposes three operations:

- `ResolveNode` for a machine's billed hourly price
- `ResolveResources` for per-unit CPU, memory, storage, and GPU rates
- `ResolveFlat` for non-resource fees such as a managed control plane

Provider catalogs implement this contract. Callers select a provider and
region through application options; the core does not perform provider
detection or network calls.

## Persistence and errors

Local stores own budgets, captured cost history, and recommendation state. The
application workflows combine those stores with core operations; calculation
packages do not read or write files.

- Entry-point adapters report flag, file-selection, binding, and presentation
  errors.
- The converter reports malformed, unsupported, or incomplete Deployment
  input.
- Cluster collection returns connection and API errors, and may also return a
  snapshot with warnings for incomplete data.
- Pricing resolution failures remain visible in the report; unpriced items do
  not contribute to cost totals.
- Core planning and estimation operate on normalized values and do not access
  Kubernetes or local storage.