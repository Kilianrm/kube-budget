package wails

import (
	"os"
	"strings"
	"testing"
)

func TestEstimateManifestReturnsUIResult(t *testing.T) {
	input, err := os.ReadFile("../../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	result, err := NewManifestAdapter().EstimateManifest(ManifestRequest{
		Documents: []ManifestDocument{{
			Name:    "valid-deployment.yaml",
			Content: string(input),
		}},
		Provider:     "aws",
		Region:       "us-east-1",
		InstanceType: "m6i.large",
	})
	if err != nil {
		t.Fatalf("EstimateManifest() error = %v", err)
	}
	if result.Workload.Name != "api" || result.Workload.Namespace != "production" {
		t.Errorf("EstimateManifest().Workload identity = %q/%q, want production/api", result.Workload.Namespace, result.Workload.Name)
	}
	if result.Workload.Replicas != 3 {
		t.Errorf("EstimateManifest().Workload.Replicas = %d, want 3", result.Workload.Replicas)
	}
	if len(result.Workload.Resources) != 1 || result.Workload.Resources[0].Name != "api" {
		t.Errorf("EstimateManifest().Workload.Resources = %#v, want api resource", result.Workload.Resources)
	}
	if result.HourlyTotal <= 0 || result.DailyTotal != result.HourlyTotal*24 || result.MonthlyTotal != result.DailyTotal*30 {
		t.Errorf("EstimateManifest() totals = hourly %v, daily %v, monthly %v", result.HourlyTotal, result.DailyTotal, result.MonthlyTotal)
	}
	if result.Currency != "USD" {
		t.Errorf("EstimateManifest().Currency = %q, want USD", result.Currency)
	}
}

func TestEstimateManifestRequiresOneDocument(t *testing.T) {
	_, err := NewManifestAdapter().EstimateManifest(ManifestRequest{
		Provider:     "aws",
		Region:       "us-east-1",
		InstanceType: "m6i.large",
	})

	if err == nil || !strings.Contains(err.Error(), "exactly one document") {
		t.Fatalf("EstimateManifest() error = %v, want exactly one document error", err)
	}
}

func TestEstimateManifestRejectsUnsupportedProvider(t *testing.T) {
	_, err := NewManifestAdapter().EstimateManifest(ManifestRequest{
		Documents:    []ManifestDocument{{Content: "manifest"}},
		Provider:     "gcp",
		Region:       "us-east-1",
		InstanceType: "m6i.large",
	})

	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("EstimateManifest() error = %v, want unsupported provider error", err)
	}
}
