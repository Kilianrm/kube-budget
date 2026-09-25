package wails

import (
	"os"
	"strings"
	"testing"

	"kube-budget/core/costmodel"
	"kube-budget/internal/providers"
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
	if result.HourlyTotal <= 0 || result.Cost != costmodel.Project(result.HourlyTotal) {
		t.Errorf("EstimateManifest() cost = %+v, want projection of hourly %v", result.Cost, result.HourlyTotal)
	}
	if result.DailyTotal != result.Cost.Daily || result.MonthlyTotal != result.Cost.Monthly {
		t.Errorf("EstimateManifest() totals = daily %v, monthly %v, want %v/%v", result.DailyTotal, result.MonthlyTotal, result.Cost.Daily, result.Cost.Monthly)
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
		Provider:     "oracle",
		Region:       "us-east-1",
		InstanceType: "m6i.large",
	})

	if err == nil || !strings.Contains(err.Error(), "unsupported provider") {
		t.Fatalf("EstimateManifest() error = %v, want unsupported provider error", err)
	}
}

func TestEstimateManifestSupportsGCP(t *testing.T) {
	input, err := os.ReadFile("../../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	result, err := NewManifestAdapter().EstimateManifest(ManifestRequest{
		Documents:    []ManifestDocument{{Content: string(input)}},
		Provider:     "gcp",
		Region:       "us-central1",
		InstanceType: "e2-standard-2",
	})
	if err != nil {
		t.Fatalf("EstimateManifest() error = %v", err)
	}
	if result.Pricing.Provider != "gcp" || result.Pricing.Region != "us-central1" {
		t.Errorf("EstimateManifest().Pricing = %#v, want GCP us-central1", result.Pricing)
	}
}

func TestEstimateManifestSupportsAzure(t *testing.T) {
	if names := providers.Names(); len(names) != 3 || names[1] != "azure" {
		t.Fatalf("providers.Names() = %#v, want AWS, Azure, and GCP", names)
	}
	if provider, ok := providers.Get("azure"); !ok || provider.Name() != "azure" {
		t.Fatalf("providers.Get(azure) = %#v, %v; want Azure provider", provider, ok)
	}
	input, err := os.ReadFile("../../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	result, err := NewManifestAdapter().EstimateManifest(ManifestRequest{
		Documents:    []ManifestDocument{{Content: string(input)}},
		Provider:     "azure",
		Region:       "eastus",
		InstanceType: "Standard_D2s_v5",
	})
	if err != nil {
		t.Fatalf("EstimateManifest() error = %v", err)
	}
	if result.Pricing.Provider != "azure" || result.Pricing.Region != "eastus" {
		t.Errorf("EstimateManifest().Pricing = %#v, want Azure eastus", result.Pricing)
	}
}
