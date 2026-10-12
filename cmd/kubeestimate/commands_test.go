package main

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kube-cost-workspace/core/costmodel"
	clustermode "kube-cost-workspace/internal/application/cluster"
	"kube-cost-workspace/internal/application/workspace"
)

const testKubeconfig = `apiVersion: v1
kind: Config
current-context: staging
clusters:
  - name: staging-cluster
    cluster: {server: "https://staging.example.com"}
contexts:
  - name: staging
    context: {cluster: staging-cluster, user: me}
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

func values(cpu int64, memoryGiB int64) clustermode.ResourceValues {
	return clustermode.ResourceValues{CPUMilli: cpu, MemoryBytes: memoryGiB * gibibyte}
}

// testSnapshot is a two-node m6i.large cluster running one Deployment.
func testSnapshot() clustermode.Snapshot {
	return clustermode.Snapshot{
		Cluster: clustermode.ClusterInfo{Context: "demo", Server: "https://demo.example.com", Version: "v1.30.2"},
		Summary: clustermode.Summary{NodeCount: 2, ReadyNodeCount: 2, WorkloadCount: 1, NamespaceCount: 1, Requests: values(3000, 10), Allocatable: values(3860, 14)},
		Nodes: []clustermode.Node{
			{UID: "a", Name: "a", InstanceType: "m6i.large", NodeGroup: "general", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 7), Requests: values(1500, 5)},
			{UID: "b", Name: "b", InstanceType: "m6i.large", NodeGroup: "general", Region: "us-east-1", Ready: true, Schedulable: true, Allocatable: values(1930, 7), Requests: values(1500, 5)},
		},
		Workloads: []clustermode.Workload{{UID: "w", Kind: "Deployment", Name: "base", Namespace: "prod", DesiredReplicas: 2, ReadyReplicas: 2,
			Containers: []clustermode.Container{{Name: "base", Requests: values(1500, 5)}}, Requests: values(3000, 10)}},
		Namespaces: []clustermode.NamespaceSummary{{Name: "prod", WorkloadCount: 1, PodCount: 2, Requests: values(3000, 10)}},
		Pods: []clustermode.Pod{
			{Name: "base-1", Namespace: "prod", NodeName: "a", OwnerKind: "Deployment", OwnerName: "base", Requests: values(1500, 5)},
			{Name: "base-2", Namespace: "prod", NodeName: "b", OwnerKind: "Deployment", OwnerName: "base", Requests: values(1500, 5)},
		},
		Platform: clustermode.Platform{Provider: "aws", Detected: "aws", Region: "us-east-1", Supported: true},
	}
}

func testPriced(t *testing.T) workspace.PricedCluster {
	t.Helper()
	priced, err := workspace.Price(testSnapshot(), workspace.PriceOptions{})
	if err != nil {
		t.Fatalf("Price() error = %v", err)
	}
	return priced
}

func TestHelpListsEveryCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, command := range commands {
		if !strings.Contains(stdout.String(), command.name) {
			t.Errorf("help = %q, want %q listed", stdout.String(), command.name)
		}
	}
}

func TestUnknownCommandIsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"deploy"}, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), `unknown command "deploy"`) {
		t.Errorf("exit code = %d, stderr = %q; want a usage error", code, stderr.String())
	}
}

func TestManifestCommandWritesJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"manifest", "-f", "../../data/manifests/valid-deployment.yaml", "--provider", "aws", "--region", "us-east-1", "--instance-type", "m6i.large", "-o", "json"}, &stdout, &stderr)
	if code != exitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	var result manifestOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout.String())
	}
	if result.Workload != "api" || result.Replicas != 3 || result.InstanceType != "m6i.large" {
		t.Errorf("result = %+v, want api with 3 replicas on m6i.large", result)
	}
	// the same 730-hour month as the desktop app
	if math.Abs(result.Cost.Monthly-result.Cost.Hourly*costmodel.HoursPerMonth) > 1e-9 {
		t.Errorf("cost = %+v, want the monthly figure at 730 hours", result.Cost)
	}
}

func TestManifestCommandRejectsAnUnknownOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"manifest", "-f", "../../data/manifests/valid-deployment.yaml", "-o", "yaml"}, &stdout, &stderr); code != exitUsage {
		t.Errorf("exit code = %d, want a usage error", code)
	}
}

func TestContextsMarksTheCurrentOne(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"contexts", "--kubeconfig", writeKubeconfig(t)}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "*        staging") || !strings.Contains(stdout.String(), "https://staging.example.com") {
		t.Errorf("contexts = %q, want staging marked current", stdout.String())
	}
}

func TestClusterCommandsReportAnUnknownContext(t *testing.T) {
	kubeconfig := writeKubeconfig(t)
	for _, args := range [][]string{
		{"cluster", "--kubeconfig", kubeconfig, "--context", "prod"},
		{"cost", "--kubeconfig", kubeconfig, "--context", "prod"},
		{"simulate", "-f", "../../data/manifests/valid-deployment.yaml", "--kubeconfig", kubeconfig, "--context", "prod"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != exitError || !strings.Contains(stderr.String(), `context "prod" was not found`) {
			t.Errorf("%s: exit code = %d, stderr = %q; want the missing context", args[0], code, stderr.String())
		}
	}
}

func TestCommandsValidateTheirFlags(t *testing.T) {
	for _, args := range [][]string{
		{"cost", "--by", "pod"},
		{"simulate"},
		{"cluster", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != exitUsage {
			t.Errorf("%v: exit code = %d, want a usage error", args, code)
		}
	}
}

func TestPrintSnapshot(t *testing.T) {
	var stdout bytes.Buffer
	printSnapshot(&stdout, testSnapshot(), true)
	for _, expected := range []string{"demo (https://demo.example.com, Kubernetes v1.30.2)", "aws us-east-1 (can be priced)", "3.00 of 3.86 cores requested (78%)", "general", "prod", "Deployment"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("snapshot output is missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestPrintCostBreaksTheBillDown(t *testing.T) {
	report := testPriced(t).Report
	billed := report.Totals[costmodel.BasisProvisioned]

	for by, expected := range map[string]string{"namespace": "prod", "workload": "base", "nodegroup": "general"} {
		var stdout bytes.Buffer
		printCost(&stdout, report, nil, by, 0)
		if !strings.Contains(stdout.String(), usd(billed.Monthly)) || !strings.Contains(stdout.String(), expected) {
			t.Errorf("--by %s output is missing the bill or %q:\n%s", by, expected, stdout.String())
		}
	}
	// the namespace rows, with idle capacity, add up to the bill
	total := 0.0
	for _, row := range breakdown(report, "namespace") {
		total += row.cost.Hourly
	}
	if math.Abs(total-billed.Hourly) > 1e-9 {
		t.Errorf("rows add up to %v/h, want the billed %v/h", total, billed.Hourly)
	}
}

func TestPrintSimulation(t *testing.T) {
	// Never read the user's real history, budgets or dismissals.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	manifest, err := os.ReadFile("../../data/manifests/valid-deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	priced := testPriced(t)
	simulation, err := workspace.Simulate(priced, manifest)
	if err != nil {
		t.Fatalf("Simulate() error = %v", err)
	}

	var stdout bytes.Buffer
	printSimulation(&stdout, priced.Report.Scope, simulation)
	for _, expected := range []string{"production/api, 3 replicas", "m6i.large rates", "Billed / month", "Placement:", "This month:"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Errorf("simulation output is missing %q:\n%s", expected, stdout.String())
		}
	}
}

func TestUSDGroupsThousands(t *testing.T) {
	for value, want := range map[float64]string{0: "$0.00", 12.5: "$12.50", 1234567.891: "$1,234,567.89", -1500: "-$1,500.00"} {
		if got := usd(value); got != want {
			t.Errorf("usd(%v) = %q, want %q", value, got, want)
		}
	}
	if got := signedUSD(3); got != "+$3.00" {
		t.Errorf("signedUSD(3) = %q", got)
	}
}
