package cluster

import "testing"

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name     string
		nodes    []Node
		provider string
		detected string
		region   string
	}{
		{"eks", []Node{{ProviderID: "aws:///us-east-1a/i-0abc", Region: "us-east-1"}, {ProviderID: "aws:///us-east-1b/i-0def", Region: "us-east-1"}}, "aws", "aws", "us-east-1"},
		{"gke", []Node{{ProviderID: "gce://project/europe-west1-b/node-1", Region: "europe-west1"}}, "gcp", "gce", "europe-west1"},
		{"aks", []Node{{ProviderID: "azure:///subscriptions/x/resourceGroups/y/providers/Microsoft.Compute/virtualMachines/z", Region: "eastus"}}, "azure", "azure", "eastus"},
		{"kind", []Node{{ProviderID: "kind://docker/kind/kind-control-plane"}}, "", "kind", ""},
		{"minikube", []Node{{Name: "minikube"}}, "", "", ""},
		{"no nodes", nil, "", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			platform := DetectPlatform(test.nodes)
			if platform.Provider != test.provider || platform.Detected != test.detected || platform.Region != test.region {
				t.Errorf("DetectPlatform() = %+v, want provider %q, detected %q, region %q", platform, test.provider, test.detected, test.region)
			}
		})
	}
}
