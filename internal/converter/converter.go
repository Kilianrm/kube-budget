// Package converter converts Kubernetes manifests into the core workload model.
package converter

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"kube-budget/core/engine"
	"kube-budget/core/pricing"
)

const bytesPerGiB = 1024 * 1024 * 1024

// nvidiaGPUResourceName is the extended resource name used by the NVIDIA device plugin
// to advertise GPUs on a node. It is the de facto standard across Kubernetes clusters.
const nvidiaGPUResourceName = corev1.ResourceName("nvidia.com/gpu")

// ConvertDeployment converts a single apps/v1 Deployment manifest from YAML or JSON.
func ConvertDeployment(input []byte) (engine.Workload, error) {
	jsonInput, err := yaml.ToJSON(input)
	if err != nil {
		return engine.Workload{}, fmt.Errorf("kubernetes: parse manifest: %w", err)
	}

	var typeMeta metav1.TypeMeta
	if err := json.Unmarshal(jsonInput, &typeMeta); err != nil {
		return engine.Workload{}, fmt.Errorf("kubernetes: decode manifest type: %w", err)
	}
	if typeMeta.APIVersion != "apps/v1" || typeMeta.Kind != "Deployment" {
		return engine.Workload{}, fmt.Errorf("kubernetes: unsupported manifest kind %q with apiVersion %q", typeMeta.Kind, typeMeta.APIVersion)
	}

	var deployment appsv1.Deployment
	if err := json.Unmarshal(jsonInput, &deployment); err != nil {
		return engine.Workload{}, fmt.Errorf("kubernetes: decode Deployment: %w", err)
	}
	if deployment.Name == "" {
		return engine.Workload{}, fmt.Errorf("kubernetes: Deployment metadata.name is required")
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return engine.Workload{}, fmt.Errorf("kubernetes: Deployment %q must define at least one container", deployment.Name)
	}

	replicas := int32(1)
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}

	resources := make([]pricing.Resource, 0, len(deployment.Spec.Template.Spec.Containers))
	for _, container := range deployment.Spec.Template.Spec.Containers {
		resource, err := convertContainer(container)
		if err != nil {
			return engine.Workload{}, fmt.Errorf("kubernetes: Deployment %q: %w", deployment.Name, err)
		}
		resources = append(resources, resource)
	}

	// Native sidecars (restartPolicy: Always init containers) run for the pod's entire
	// lifetime and consume resources continuously, so they count toward the estimate too.
	for _, container := range deployment.Spec.Template.Spec.InitContainers {
		if container.RestartPolicy == nil || *container.RestartPolicy != corev1.ContainerRestartPolicyAlways {
			continue
		}
		resource, err := convertContainer(container)
		if err != nil {
			return engine.Workload{}, fmt.Errorf("kubernetes: Deployment %q: %w", deployment.Name, err)
		}
		resources = append(resources, resource)
	}

	return engine.Workload{
		Name:      deployment.Name,
		Namespace: deployment.Namespace,
		Replicas:  replicas,
		Resources: resources,
	}, nil
}

func convertContainer(container corev1.Container) (pricing.Resource, error) {
	cpu, hasCPU := container.Resources.Requests[corev1.ResourceCPU]
	memory, hasMemory := container.Resources.Requests[corev1.ResourceMemory]
	if !hasCPU && !hasMemory {
		return pricing.Resource{}, fmt.Errorf("container %q must define resources.requests.cpu or resources.requests.memory", container.Name)
	}

	var cpuCores float64
	if hasCPU {
		cpuCores = cpu.AsApproximateFloat64()
	}
	var memoryGB float64
	if hasMemory {
		memoryGB = memory.AsApproximateFloat64() / bytesPerGiB
	}

	var storageGB float64
	if storage, ok := container.Resources.Requests[corev1.ResourceEphemeralStorage]; ok {
		storageGB = storage.AsApproximateFloat64() / bytesPerGiB
	}

	var gpu float64
	if quantity, ok := container.Resources.Requests[nvidiaGPUResourceName]; ok {
		gpu = quantity.AsApproximateFloat64()
	}

	return pricing.Resource{
		Name:      container.Name,
		CPU:       cpuCores,
		MemoryGB:  memoryGB,
		StorageGB: storageGB,
		GPU:       gpu,
	}, nil
}
