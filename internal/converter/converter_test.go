package converter

import (
	"os"
	"testing"

	"kube-budget/core/engine"
	"kube-budget/core/pricing"
)

func TestConvertDeployment(t *testing.T) {
	tests := []struct {
		name         string
		manifestFile string
		wantErr      bool
		wantWorkload engine.Workload
	}{
		{
			name:         "valid deployment with one container",
			manifestFile: "../../data/manifests/valid-deployment.yaml",
			wantWorkload: engine.Workload{
				Name:      "api",
				Namespace: "production",
				Replicas:  3,
				Resources: []pricing.Resource{
					{Name: "api", CPU: 0.5, MemoryGB: 0.5},
				},
			},
		},
		{
			name:         "missing memory request defaults to zero",
			manifestFile: "../../data/manifests/missing-memory-request.yaml",
			wantWorkload: engine.Workload{
				Name:     "api",
				Replicas: 1,
				Resources: []pricing.Resource{
					{Name: "api", CPU: 0.5, MemoryGB: 0},
				},
			},
		},
		{
			name:         "missing both cpu and memory requests",
			manifestFile: "../../data/manifests/missing-cpu-and-memory.yaml",
			wantErr:      true,
		},
		{
			name:         "missing name",
			manifestFile: "../../data/manifests/missing-name.yaml",
			wantErr:      true,
		},
		{
			name:         "valid deployment with ephemeral storage request",
			manifestFile: "../../data/manifests/valid-deployment-with-storage.yaml",
			wantWorkload: engine.Workload{
				Name:      "api",
				Namespace: "production",
				Replicas:  2,
				Resources: []pricing.Resource{
					{Name: "api", CPU: 0.25, MemoryGB: 0.25, StorageGB: 1},
				},
			},
		},
		{
			name:         "ingress-nginx controller deployment",
			manifestFile: "../../data/manifests/ingress-nginx-controller.yaml",
			wantWorkload: engine.Workload{
				Name:      "ingress-nginx-controller",
				Namespace: "ingress-nginx",
				Replicas:  1,
				Resources: []pricing.Resource{
					{Name: "controller", CPU: 0.1, MemoryGB: 90.0 / 1024},
				},
			},
		},
		{
			name:         "prometheus-operator deployment",
			manifestFile: "../../data/manifests/prometheus-operator.yaml",
			wantWorkload: engine.Workload{
				Name:      "prometheus-operator",
				Namespace: "default",
				Replicas:  1,
				Resources: []pricing.Resource{
					{Name: "prometheus-operator", CPU: 0.1, MemoryGB: 100.0 / 1024},
				},
			},
		},
		{
			name:         "argocd-server deployment has no resource requests",
			manifestFile: "../../data/manifests/argocd-server.yaml",
			wantErr:      true,
		},
		{
			name:         "cert-manager controller deployment has no resource requests",
			manifestFile: "../../data/manifests/cert-manager-controller.yaml",
			wantErr:      true,
		},
		{
			name:         "kube-state-metrics deployment has no resource requests",
			manifestFile: "../../data/manifests/kube-state-metrics.yaml",
			wantErr:      true,
		},
		{
			name:         "valid deployment with gpu request",
			manifestFile: "../../data/manifests/valid-deployment-with-gpu.yaml",
			wantWorkload: engine.Workload{
				Name:      "api",
				Namespace: "production",
				Replicas:  1,
				Resources: []pricing.Resource{
					{Name: "api", CPU: 0.5, MemoryGB: 0.5, GPU: 1},
				},
			},
		},
		{
			name:         "valid deployment with native sidecar init container",
			manifestFile: "../../data/manifests/valid-deployment-with-sidecar.yaml",
			wantWorkload: engine.Workload{
				Name:      "api",
				Namespace: "production",
				Replicas:  1,
				Resources: []pricing.Resource{
					{Name: "api", CPU: 0.5, MemoryGB: 0.5},
					{Name: "envoy-sidecar", CPU: 0.1, MemoryGB: 0.125},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := os.ReadFile(tt.manifestFile)
			if err != nil {
				t.Fatalf("failed to read manifest fixture: %v", err)
			}

			workload, err := ConvertDeployment(manifest)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ConvertDeployment() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ConvertDeployment() error = %v", err)
			}

			if workload.Name != tt.wantWorkload.Name || workload.Namespace != tt.wantWorkload.Namespace || workload.Replicas != tt.wantWorkload.Replicas {
				t.Errorf("ConvertDeployment() workload identity = %#v, want %#v", workload, tt.wantWorkload)
			}
			if len(workload.Resources) != len(tt.wantWorkload.Resources) {
				t.Fatalf("ConvertDeployment() resource count = %d, want %d", len(workload.Resources), len(tt.wantWorkload.Resources))
			}
			for i, resource := range workload.Resources {
				want := tt.wantWorkload.Resources[i]
				if resource.Name != want.Name || resource.CPU != want.CPU || resource.MemoryGB != want.MemoryGB || resource.StorageGB != want.StorageGB || resource.GPU != want.GPU {
					t.Errorf("ConvertDeployment() resource = %#v, want %#v", resource, want)
				}
			}
		})
	}
}

func TestConvertDeploymentWithAutoscaler(t *testing.T) {
	deploymentManifest, err := os.ReadFile("../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatalf("failed to read deployment fixture: %v", err)
	}
	hpaManifest, err := os.ReadFile("../../data/manifests/hpa-api.yaml")
	if err != nil {
		t.Fatalf("failed to read autoscaler fixture: %v", err)
	}

	workload, err := ConvertDeploymentWithAutoscaler(deploymentManifest, hpaManifest)
	if err != nil {
		t.Fatalf("ConvertDeploymentWithAutoscaler() error = %v", err)
	}

	if workload.MinReplicas == nil || workload.MaxReplicas == nil {
		t.Fatalf("ConvertDeploymentWithAutoscaler() MinReplicas/MaxReplicas = nil, want set")
	}
	if *workload.MinReplicas != 2 || *workload.MaxReplicas != 10 {
		t.Errorf("ConvertDeploymentWithAutoscaler() MinReplicas/MaxReplicas = %d/%d, want 2/10", *workload.MinReplicas, *workload.MaxReplicas)
	}
}

func TestConvertDeploymentWithAutoscalerRejectsMismatchedTarget(t *testing.T) {
	deploymentManifest, err := os.ReadFile("../../data/manifests/ingress-nginx-controller.yaml")
	if err != nil {
		t.Fatalf("failed to read deployment fixture: %v", err)
	}
	hpaManifest, err := os.ReadFile("../../data/manifests/hpa-api.yaml")
	if err != nil {
		t.Fatalf("failed to read autoscaler fixture: %v", err)
	}

	_, err = ConvertDeploymentWithAutoscaler(deploymentManifest, hpaManifest)
	if err == nil {
		t.Fatal("ConvertDeploymentWithAutoscaler() error = nil, want scaleTargetRef mismatch error")
	}
}
