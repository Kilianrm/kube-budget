package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: staging
clusters:
  - name: prod-cluster
    cluster: {server: "https://admin:secret@prod.example.com"}
  - name: staging-cluster
    cluster: {server: "https://staging.example.com"}
contexts:
  - name: staging
    context: {cluster: staging-cluster, user: me}
  - name: prod
    context: {cluster: prod-cluster, user: me}
  - name: dangling
    context: {cluster: missing, user: me}
users:
  - name: me
    user: {token: abc}
`

func writeKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestContextsAreSortedAndMarkTheCurrentOne(t *testing.T) {
	contexts, err := Contexts(writeKubeconfig(t))
	if err != nil {
		t.Fatalf("Contexts() error = %v", err)
	}
	if len(contexts) != 2 || contexts[0].Name != "prod" || contexts[1].Name != "staging" {
		t.Fatalf("contexts = %+v, want prod and staging, without the dangling one", contexts)
	}
	if contexts[0].Current || !contexts[1].Current {
		t.Errorf("contexts = %+v, want staging current", contexts)
	}
	if contexts[0].Server != "https://prod.example.com" {
		t.Errorf("server = %q, want the credentials redacted", contexts[0].Server)
	}
}

func TestCurrentContextIsTheKubeconfigOne(t *testing.T) {
	if got, err := CurrentContext(writeKubeconfig(t)); err != nil || got != "staging" {
		t.Errorf("CurrentContext() = %q, %v; want staging", got, err)
	}
}

func TestSnapshotRequiresContext(t *testing.T) {
	_, err := Snapshot(Connection{})
	if err == nil || !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("Snapshot() error = %v, want context required error", err)
	}
}

func TestSnapshotRejectsAnUnknownContext(t *testing.T) {
	_, err := Snapshot(Connection{KubeconfigPath: writeKubeconfig(t), Context: "missing"})
	if err == nil || !strings.Contains(err.Error(), `context "missing" was not found`) {
		t.Fatalf("Snapshot() error = %v, want context not found", err)
	}
}

func TestRedactServerURLDropsCredentials(t *testing.T) {
	if got := redactServerURL("https://admin:secret@prod.example.com"); got != "https://prod.example.com" {
		t.Errorf("redactServerURL() = %q", got)
	}
}

func TestValidateEKSConnection(t *testing.T) {
	tests := []struct {
		name       string
		connection EKSConnection
		want       string
	}{
		{name: "missing cluster", connection: EKSConnection{Region: "eu-west-1"}, want: "cluster name is required"},
		{name: "missing region", connection: EKSConnection{ClusterName: "production"}, want: "AWS region is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateEKSConnection(test.connection)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateEKSConnection() error = %v, want %q", err, test.want)
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
	_, err := ListEKSClusters("", "")
	if err == nil || !strings.Contains(err.Error(), "AWS region is required") {
		t.Fatalf("ListEKSClusters() error = %v, want region required error", err)
	}
}

func TestWorkloadYAMLReturnsLiveDeploymentYAML(t *testing.T) {
	client := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout", Namespace: "production", Labels: map[string]string{"app": "checkout"}},
	})

	result, err := getWorkloadYAML(context.Background(), client, WorkloadRef{Kind: "Deployment", Namespace: "production", Name: "checkout"})
	if err != nil {
		t.Fatalf("getWorkloadYAML() error = %v", err)
	}
	for _, expected := range []string{"apiVersion: apps/v1", "kind: Deployment", "name: checkout", "namespace: production"} {
		if !strings.Contains(result, expected) {
			t.Errorf("getWorkloadYAML() = %q, want %q", result, expected)
		}
	}
}

func TestWorkloadYAMLRejectsUnsupportedKind(t *testing.T) {
	_, err := getWorkloadYAML(context.Background(), fake.NewSimpleClientset(), WorkloadRef{Kind: "Job"})
	if err == nil || !strings.Contains(err.Error(), "unsupported workload kind") {
		t.Fatalf("getWorkloadYAML() error = %v, want unsupported workload kind error", err)
	}
}
