# Kube Cost Workspace Documentation

This directory documents the architecture and implemented behavior of the
project. Start with the architecture overview, then use the contracts and cost
model guide when working on a specific boundary.

## Reading Order

1. [Architecture overview](architecture.md): implemented entry points, layers,
   workflows, and scope boundaries.
2. [Layer contracts](layer-contracts.md): current data flow and ownership
   between converters, application services, adapters, and core packages.
3. [Core contract](core-contract.md): the single-workload estimator's input,
   pricing, and output.
4. [Cost model and explorer](cost-explorer-design.md): cluster reports,
   pricing assumptions, history, forecasts, optimization, and simulation.

## Supporting Guides

| Guide | Purpose |
|-------|---------|
| [Project README](../README.md) | Desktop setup, usage, features, and project scope |
| [Workspace guide](workspace-guide.md) | Reviewing suggested changes, demo clusters, local data, and cluster access |
| [Core guide](../core/README.md) | Pure Go cost core documentation |
| [CLI guide](CLI.md) | Optional supporting CLI: installation, commands, examples, and troubleshooting |
| [Sample manifests](../data/manifests/README.md) | Example inputs for manifest estimation |
| [Local cluster commands](../k8s-local-cluster-tool/commands.md) | Local kind cluster tooling commands |
| [EKS cluster commands](../k8s-eks-cluster-tool/commands.md) | EKS cluster tooling commands |
| [EKS showcase](../k8s-eks-cluster-tool/showcase/README.md) | Demo scenarios for cost and optimization analysis |