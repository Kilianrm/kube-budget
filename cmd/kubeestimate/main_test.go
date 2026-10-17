package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunSelectsManifestModeFromFileFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{
		"-f", "../../data/manifests/valid-deployment.yaml",
		"--provider", "aws",
		"--region", "us-east-1",
		"--instance-type", "m6i.large",
	}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Workload: api") {
		t.Errorf("run() output = %q, want workload name", stdout.String())
	}
}

func TestRunRequiresModeInput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(nil, &stdout, &stderr)

	if exitCode != 2 {
		t.Errorf("run() exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "-f <manifest.yaml>") {
		t.Errorf("run() stderr = %q, want Manifest Mode usage", stderr.String())
	}
}

func TestRunSupportsGCP(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{
		"-f", "../../data/manifests/valid-deployment.yaml",
		"--provider", "gcp",
		"--region", "us-central1",
		"--instance-type", "e2-standard-2",
	}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Provider: gcp (us-central1)") {
		t.Errorf("run() output = %q, want GCP provider", stdout.String())
	}
}
