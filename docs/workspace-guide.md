# Workspace Guide

This guide covers reviewing suggested changes, trying demo clusters, and
understanding local data and cluster access. For desktop setup and startup
commands, see the [project README](../README.md#getting-started).

## Review Suggested Changes

Manifest mode generates a context-pinned `kubectl apply` command to copy, but
never runs it. Connected-cluster simulations distinguish new workloads from
updates and show current versus proposed replicas, requests, and costs.

The optimization view presents a waterfall from today's bill to the optimized
bill, with each step claiming only its incremental savings. Review each risk
note and its `kubectl`, `aws`, or `eksctl` commands; the app never runs them.
Start applying a recommendation to track your change. Its proposed details
stay fixed while it is in progress, and later cluster reports determine when
the change is complete. Dismissing a recommendation recalculates the plan.

## Demo Clusters

- [Local cluster tooling](../k8s-local-cluster-tool/): a kind cluster for inventory
  and capacity inspection; local nodes cannot be priced
- [EKS showcase](../k8s-eks-cluster-tool/showcase/README.md): a cluster with scenarios
  for the cost and optimization views, costing approximately $0.45/hour;
  destroy it after use
- [Sample manifests](../data/manifests/): valid and invalid inputs for estimation

## Local Data and Cluster Access

The application stores budgets, cost history, and recommendation state in
your user configuration directory.

Normal workspace usage does not provision cloud infrastructure or modify
cluster resources. Cluster inspection uses your existing access, while cloud
costs come from bundled AWS, Google Cloud, and Azure public list-price catalogs
matched to machine type, region, and purchase option (on-demand or spot).

Creating a demo cluster is a separate operation using the linked cluster
tooling and may incur cloud charges.