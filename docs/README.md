# Kube Cost Workspace Documentation

This index is the entry point to the project's architecture, cost model,
design documentation, and supporting guides. Architecture and design documents
live in this directory; implementation and tool guides remain alongside their
respective components.

## Reading Order

1. [Architecture overview](architecture.md): understand the system's components
   and responsibilities.
2. [Layer contracts](layer-contracts.md): follow the boundaries between entry
   points, application workflows, adapters, and the core.
3. [Core contract](core-contract.md): review the contracts for the cost logic.
4. [Cost explorer design](cost-explorer-design.md): explore the design of the
   cost analysis workspace.

## Supporting Guides

| Guide | Purpose |
|-------|---------|
| [Project README](../README.md) | Desktop setup, usage, features, and project scope |
| [Workspace guide](workspace-guide.md) | Reviewing suggested changes, demo clusters, local data, and cluster access |
| [Core guide](../core/README.md) | Pure Go cost core documentation |
| [CLI guide](../CLI.md) | Optional supporting CLI: installation, commands, examples, and troubleshooting |
| [Sample manifests](../data/manifests/README.md) | Example inputs for manifest estimation |
| [Local cluster commands](../k8s-local-cluster-tool/commands.md) | Local kind cluster tooling commands |
| [EKS cluster commands](../k8s-eks-cluster-tool/commands.md) | EKS cluster tooling commands |
| [EKS showcase](../k8s-eks-cluster-tool/showcase/README.md) | Demo scenarios for cost and optimization analysis |