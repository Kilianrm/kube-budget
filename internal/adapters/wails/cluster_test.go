package wails

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetClusterSnapshotRequiresContext(t *testing.T) {
	_, err := NewClusterAdapter().GetClusterSnapshot(ClusterSnapshotRequest{})
	if err == nil || !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("GetClusterSnapshot() error = %v, want context required error", err)
	}
}

func TestValidateEKSConnectionRequest(t *testing.T) {
	tests := []struct {
		name    string
		request EKSConnectionRequest
		want    string
	}{
		{name: "missing cluster", request: EKSConnectionRequest{Region: "eu-west-1"}, want: "cluster name is required"},
		{name: "missing region", request: EKSConnectionRequest{ClusterName: "production"}, want: "AWS region is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateEKSConnectionRequest(test.request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateEKSConnectionRequest() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEKSContextName(t *testing.T) {
	if got := eksContextName("production", "eu-west-1"); got != "eks/production/eu-west-1" {
		t.Fatalf("eksContextName() = %q, want %q", got, "eks/production/eu-west-1")
	}
}

func TestEKSDiscoveryRequiresRegion(t *testing.T) {
	_, err := NewClusterAdapter().ListEKSClusters(EKSClustersRequest{})
	if err == nil || !strings.Contains(err.Error(), "AWS region is required") {
		t.Fatalf("ListEKSClusters() error = %v, want region required error", err)
	}
}

func TestGetWorkloadYAMLReturnsLiveDeploymentYAML(t *testing.T) {
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout", Namespace: "production", Labels: map[string]string{"app": "checkout"}},
	})

	result, err := getWorkloadYAML(context.Background(), client, WorkloadYAMLRequest{
		Kind: "Deployment", Namespace: "production", Name: "checkout",
	})
	if err != nil {
		t.Fatalf("getWorkloadYAML() error = %v", err)
	}
	for _, expected := range []string{"apiVersion: apps/v1", "kind: Deployment", "name: checkout", "namespace: production"} {
		if !strings.Contains(result, expected) {
			t.Errorf("getWorkloadYAML() = %q, want %q", result, expected)
		}
	}
}

func TestGetWorkloadYAMLRejectsUnsupportedKind(t *testing.T) {
	_, err := getWorkloadYAML(context.Background(), fake.NewSimpleClientset(), WorkloadYAMLRequest{Kind: "Job"})
	if err == nil || !strings.Contains(err.Error(), "unsupported workload kind") {
		t.Fatalf("getWorkloadYAML() error = %v, want unsupported workload kind error", err)
	}
}
