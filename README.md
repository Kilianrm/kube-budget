# KubeBudget

> A cost-aware Kubernetes Admission Controller built in Go.

KubeBudget is a Kubernetes Admission Controller that evaluates the estimated cost of workloads before they are deployed to a cluster.

It allows teams to define budgets for Kubernetes namespaces and automatically reject workloads that would cause the estimated cost to exceed the configured budget.

The project is Kubernetes-native and cloud-agnostic at its core, with **Amazon EKS** as the primary deployment environment.

## How it works

```text
Developer
    │
    │ kubectl apply
    ▼
Kubernetes API Server
    │
    ▼
KubeBudget Admission Controller
    │
    ├── Estimate workload cost
    │
    ├── Check namespace budget
    │
    └── ALLOW / DENY
```

For example:

```text
❌ Deployment rejected

Namespace budget:       $500/month
Current estimated cost: $320/month
New workload:           +$210/month
Projected cost:         $530/month

Reason:
The namespace budget would be exceeded.
```

## Example

Budgets are defined as Kubernetes resources:

```yaml
apiVersion: kubebudget.dev/v1alpha1
kind: Budget
metadata:
  name: production-budget
  namespace: payments
spec:
  monthlyLimit: 500
  currency: USD
```

A workload submitted to the namespace is evaluated before Kubernetes accepts it.

## Tech Stack

* Go
* Kubernetes Admission Webhooks
* Kubernetes API
* Custom Resource Definitions (CRDs)
* Amazon EKS
* Helm
* Terraform
* Docker

## Roadmap

* [ ] Implement Admission Webhook in Go
* [ ] Implement Budget CRD
* [ ] Add basic cost estimation
* [ ] Deploy to local Kubernetes
* [ ] Deploy to Amazon EKS
* [ ] Add OpenCost integration
* [ ] Add `kube-budget plan` CLI
* [ ] Add Prometheus metrics

## Project Status

🚧 Early development

KubeBudget is a personal learning and portfolio project focused on exploring Kubernetes internals, Go-based controllers, cloud infrastructure, and FinOps concepts.
