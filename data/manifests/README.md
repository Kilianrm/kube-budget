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
