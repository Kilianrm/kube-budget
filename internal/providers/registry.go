package providers

import (
	"sort"
	"strings"

	"kube-budget/internal/providers/aws"
	"kube-budget/internal/providers/azure"
	"kube-budget/internal/providers/gcp"
)

var registry = map[string]Provider{
	"aws":   aws.New(),
	"azure": azure.New(),
	"gcp":   gcp.New(),
}

// Get returns the provider registered under name.
func Get(name string) (Provider, bool) {
	provider, ok := registry[strings.ToLower(strings.TrimSpace(name))]
	return provider, ok
}

// Names returns the registered provider names in stable order.
func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
