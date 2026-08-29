package converter

import (
	"encoding/json"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"kube-budget/core/engine"
)

// ConvertDeploymentWithAutoscaler converts a Deployment manifest and attaches the
// min/max replica bounds from an associated autoscaling/v2 HorizontalPodAutoscaler manifest.
func ConvertDeploymentWithAutoscaler(deploymentManifest, hpaManifest []byte) (engine.Workload, error) {
	workload, err := ConvertDeployment(deploymentManifest)
	if err != nil {
		return engine.Workload{}, err
	}

	minReplicas, maxReplicas, err := convertHorizontalPodAutoscaler(hpaManifest, workload.Name)
	if err != nil {
		return engine.Workload{}, err
	}

	workload.MinReplicas = &minReplicas
	workload.MaxReplicas = &maxReplicas
	return workload, nil
}

func convertHorizontalPodAutoscaler(input []byte, deploymentName string) (minReplicas, maxReplicas int32, err error) {
	jsonInput, err := yaml.ToJSON(input)
	if err != nil {
		return 0, 0, fmt.Errorf("kubernetes: parse autoscaler manifest: %w", err)
	}

	var typeMeta metav1.TypeMeta
	if err := json.Unmarshal(jsonInput, &typeMeta); err != nil {
		return 0, 0, fmt.Errorf("kubernetes: decode autoscaler manifest type: %w", err)
	}
	if typeMeta.APIVersion != "autoscaling/v2" || typeMeta.Kind != "HorizontalPodAutoscaler" {
		return 0, 0, fmt.Errorf("kubernetes: unsupported autoscaler kind %q with apiVersion %q", typeMeta.Kind, typeMeta.APIVersion)
	}

	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := json.Unmarshal(jsonInput, &hpa); err != nil {
		return 0, 0, fmt.Errorf("kubernetes: decode HorizontalPodAutoscaler: %w", err)
	}

	if hpa.Spec.ScaleTargetRef.Kind != "Deployment" || hpa.Spec.ScaleTargetRef.Name != deploymentName {
		return 0, 0, fmt.Errorf("kubernetes: HorizontalPodAutoscaler scaleTargetRef %s/%s does not match Deployment %q", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name, deploymentName)
	}
	if hpa.Spec.MaxReplicas <= 0 {
		return 0, 0, fmt.Errorf("kubernetes: HorizontalPodAutoscaler spec.maxReplicas is required")
	}

	minReplicas = int32(1)
	if hpa.Spec.MinReplicas != nil {
		minReplicas = *hpa.Spec.MinReplicas
	}

	return minReplicas, hpa.Spec.MaxReplicas, nil
}
