# KubeBudget Architecture

This document expands on the high-level architecture summarized in the [README](../README.md), detailing the responsibilities, priorities, and scope of each component.

## Responsibilities by component

### 1. Core Engine
The core engine is the main product.

Responsibilities:
- accept a normalized workload model
- estimate CPU, memory, and storage cost
- calculate total cost and per-resource cost
- expose a clean interface for other consumers

The core does not parse Kubernetes YAML. It is implemented in Go and intended to be reused by all other integrations.

### 2. Kubernetes Converter
The converter is the boundary between Kubernetes input and the core.

Responsibilities:
- parse a Kubernetes manifest
- validate the supported Kubernetes fields
- convert CPU, memory, storage, and replicas into the normalized core model
- return clear errors for unsupported or invalid input

The converter knows about Kubernetes objects; the core does not.

### 3. CLI Plugin
The CLI is a developer-facing entry point.

Responsibilities:
- read a manifest or workload definition
- call the core engine
- print cost summaries in a readable format
- help developers understand the cost impact before deployment

This is the primary user experience for the MVP and should behave like a kcost-style tool.

### 4. GitHub Action PR Check
The GitHub Action provides CI-level validation.

Responsibilities:
- run on pull requests or workflow triggers
- compare old and new resource cost
- publish a cost delta summary in the PR
- fail the check if a configured threshold is exceeded

This is valuable because it brings the cost-estimation logic into a real software delivery workflow.

### 5. Admission Webhook
The admission webhook is the Kubernetes-native enforcement path.

Responsibilities:
- intercept workload creation or update requests
- call the core engine
- decide whether to allow or deny a workload based on policy rules

This is the most advanced integration and should be treated as a later step rather than the initial goal.

## Why this architecture works

This design keeps the system modular:

- the core engine owns the pricing logic
- the CLI owns developer UX
- GitHub Actions owns CI policy checks
- the webhook owns cluster enforcement

That separation makes the project easier to build, test, and explain in interviews.

## Priority order

1. Core engine
2. CLI plugin similar to kcost
3. GitHub Action PR check
4. Admission webhook

## MVP scope for this project

For the next 3-4 months, the realistic project scope is:

- compute workload cost from resource requests
- expose it through a CLI plugin
- provide a GitHub Action PR cost summary
- keep Kubernetes admission enforcement as a later enhancement

## Product statement

KubeBudget is a Go-based Kubernetes cost estimation project that provides a reusable cost engine, a kcost-like CLI plugin, and platform-ready integrations for developer workflows and CI policy checks.
