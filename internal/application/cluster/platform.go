package cluster

import (
	"sort"
	"strings"
)

// Platform is where the cluster runs, as far as its nodes tell. The Cost
// section needs it to pick a price list; the Cluster section works anywhere.
type Platform struct {
	// Provider is the pricing provider name ("aws", "gcp", "azure"), or empty
	// when the nodes do not identify a public cloud.
	Provider string `json:"provider"`
	// Detected is what the nodes report, such as "aws", "kind" or "k3s", even
	// when it is not a cloud; empty when they report nothing.
	Detected string `json:"detected"`
	// Region is the most common node region label.
	Region string `json:"region,omitempty"`
	// Supported is set by the caller once it knows it can price Provider in Region.
	Supported bool `json:"supported"`
	// Reason explains why the cluster cannot be priced; empty when it can.
	Reason string `json:"reason,omitempty"`
}

// providerSchemes maps node providerID schemes to pricing providers.
var providerSchemes = map[string]string{
	"aws":   "aws",
	"gce":   "gcp",
	"azure": "azure",
}

// DetectPlatform identifies the cloud from the nodes' providerID schemes
// (aws:///zone/i-…, gce://project/zone/name, azure:///subscriptions/…) and
// the region from their topology labels. The most common value wins.
func DetectPlatform(nodes []Node) Platform {
	schemes := make(map[string]int)
	regions := make(map[string]int)
	for _, node := range nodes {
		if scheme, _, found := strings.Cut(node.ProviderID, "://"); found && scheme != "" {
			schemes[strings.ToLower(scheme)]++
		}
		if node.Region != "" {
			regions[node.Region]++
		}
	}
	platform := Platform{Detected: mostCommon(schemes), Region: mostCommon(regions)}
	platform.Provider = providerSchemes[platform.Detected]
	return platform
}

func mostCommon(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) == 0 {
		return ""
	}
	return keys[0]
}
