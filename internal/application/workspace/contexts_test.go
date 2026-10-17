package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// eksKubeconfig has the context names EKS tools write: two for the same
// cluster (aws eks update-kubeconfig and eksctl), another cluster, and kind.
const eksKubeconfig = `apiVersion: v1
kind: Config
current-context: kind-dev
clusters:
  - name: c
    cluster: {server: "https://example.com"}
contexts:
  - name: arn:aws:eks:us-east-1:111122223333:cluster/floral-painting-1784466883
    context: {cluster: c, user: me}
  - name: terraform@floral-painting-1784466883.us-east-1.eksctl.io
    context: {cluster: c, user: me}
  - name: arn:aws:eks:us-east-1:111122223333:cluster/beautiful-creature-1786788647
    context: {cluster: c, user: me}
  - name: eks/shop/eu-west-1
    context: {cluster: c, user: me}
  - name: kind-dev
    context: {cluster: c, user: me}
users:
  - name: me
    user: {token: abc}
`

func writeEKSKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(eksKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestShortContextNamesNameTheEKSCluster(t *testing.T) {
	for name, want := range map[string]string{
		"arn:aws:eks:us-east-1:111122223333:cluster/floral-painting-1784466883": "floral-painting-1784466883",
		"terraform@floral-painting-1784466883.us-east-1.eksctl.io":              "floral-painting-1784466883",
		"shop.eu-west-1.eksctl.io":                                              "shop",
		"eks/shop/eu-west-1":                                                    "shop",
		"kind-dev":                                                              "kind-dev",
		"arn:aws:iam::111122223333:role/viewer":                                 "arn:aws:iam::111122223333:role/viewer",
	} {
		if got := shortContextName(name); got != want {
			t.Errorf("shortContextName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestContextsAreNumberedInOrder(t *testing.T) {
	contexts, err := Contexts(writeEKSKubeconfig(t))
	if err != nil {
		t.Fatal(err)
	}
	for index, context := range contexts {
		if context.Number != index+1 {
			t.Errorf("%s is number %d, want %d", context.Name, context.Number, index+1)
		}
	}
}

func TestResolveContext(t *testing.T) {
	path := writeEKSKubeconfig(t)
	contexts, err := Contexts(path)
	if err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]string{
		"":                   "kind-dev",
		"kind-dev":           "kind-dev",
		"1":                  contexts[0].Name,
		"5":                  contexts[4].Name,
		"beautiful-creature": "arn:aws:eks:us-east-1:111122223333:cluster/beautiful-creature-1786788647",
		"BEAUTIFUL":          "arn:aws:eks:us-east-1:111122223333:cluster/beautiful-creature-1786788647",
		"shop":               "eks/shop/eu-west-1",
		"1786788647":         "arn:aws:eks:us-east-1:111122223333:cluster/beautiful-creature-1786788647",
	} {
		if got, err := ResolveContext(path, query); err != nil || got != want {
			t.Errorf("ResolveContext(%q) = %q, %v; want %q", query, got, err, want)
		}
	}
}

func TestResolveContextRefusesToGuess(t *testing.T) {
	path := writeEKSKubeconfig(t)

	// both floral-painting contexts share the short name
	_, err := ResolveContext(path, "floral-painting-1784466883")
	if err == nil || !strings.Contains(err.Error(), "matches 2 contexts") || !strings.Contains(err.Error(), "eksctl.io") {
		t.Errorf("ResolveContext() error = %v, want both candidates listed", err)
	}
	if _, err := ResolveContext(path, "staging"); err == nil || !strings.Contains(err.Error(), "kubeestimate contexts") {
		t.Errorf("ResolveContext() error = %v, want a hint to list the contexts", err)
	}
}
