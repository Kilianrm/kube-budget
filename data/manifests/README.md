# Manifest fixtures

YAML fixtures used by [converter_test.go](../../internal/converter/converter_test.go) via `os.ReadFile`.

- `valid-deployment.yaml` ? happy path, one container with cpu/memory requests.
- `missing-memory-request.yaml` ? container has cpu but no memory request; converts successfully with `MemoryGB: 0`.
- `missing-cpu-and-memory.yaml` ? container defines neither cpu nor memory; converter error (at least one is required).
- `missing-name.yaml` ? deployment missing `metadata.name`.
- `valid-deployment-with-storage.yaml` ? happy path, container also requests `ephemeral-storage`.
- `valid-deployment-with-gpu.yaml` ? happy path, container also requests `nvidia.com/gpu`.
- `valid-deployment-with-sidecar.yaml` ? happy path, includes a native sidecar (`restartPolicy: Always` init container, counted) and a regular init container (not counted).
- `hpa-api.yaml` ? `autoscaling/v2` HorizontalPodAutoscaler targeting `valid-deployment.yaml`'s `api` Deployment, used by `ConvertDeploymentWithAutoscaler`.

Real-world manifests, taken as-is from upstream public releases (unmodified) to check converter coverage against production-grade YAML:

- `ingress-nginx-controller.yaml` ? [ingress-nginx](https://github.com/kubernetes/ingress-nginx) controller Deployment. Valid, has cpu/memory requests.
- `prometheus-operator.yaml` ? [prometheus-operator](https://github.com/prometheus-operator/prometheus-operator) Deployment. Valid, has cpu/memory requests and limits.
- `argocd-server.yaml` ? [Argo CD](https://github.com/argoproj/argo-cd) server Deployment. No resource requests defined upstream ? converter error.
- `cert-manager-controller.yaml` ? [cert-manager](https://github.com/cert-manager/cert-manager) controller Deployment. No resource requests defined upstream ? converter error.
- `kube-state-metrics.yaml` ? [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) Deployment. No resource requests defined upstream ? converter error.

Note: most upstream projects ship manifests *without* resource requests (left for users/Helm values to set), so 3 of the 5 real-world fixtures intentionally exercise the "missing cpu/memory request" error path rather than the happy path.
Add a case by dropping a new `.yaml` file here and a matching row in the test table.

## Capacity growth demo

`oversized-deployment.yaml` defines a new `production/capacity-growth-demo`
Deployment with 40 replicas, each requesting 500m CPU and 512Mi memory.
Its total requests are 20 CPU cores and 20 GiB memory, exceeding the two
`t3.medium` workers in the repository's EKS configuration. Each pod is small
enough to fit a worker, so the simulation can model additional nodes and a
higher infrastructure bill rather than only oversized, unschedulable pods.

Load this file in Manifest estimator, select Connected cluster, and analyze it.
The exact added-node count and cost depend on the collected cluster report.
This is an analysis fixture: do not apply it just to preview costs. Applying it
can create real workloads and costs. The configured EKS node group has a maximum
of three nodes; the simulation's added nodes are estimates, not a guarantee that
the real cluster can scale to that size.
