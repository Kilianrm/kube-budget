package workspace

import (
	"fmt"
	"slices"
	"strings"

	clustermode "kube-cost-workspace/internal/application/cluster"
	"kube-cost-workspace/internal/providers"
)

// pricingProviderName maps a cluster provider such as "aws-eks" onto the
// pricing catalog of its cloud ("aws").
func pricingProviderName(provider string) string {
	name := strings.ToLower(strings.TrimSpace(provider))
	if index := strings.Index(name, "-"); index > 0 {
		name = name[:index]
	}
	return name
}

var providerTitles = map[string]string{"aws": "AWS", "gcp": "Google Cloud", "azure": "Azure"}

// clusterPlatform settles which price list a cluster uses. The nodes decide;
// an EKS connection stands in for nodes that report nothing (no nodes yet).
// It is supported when that provider's catalog covers the region.
func clusterPlatform(nodes []clustermode.Node, connectionProvider, connectionRegion string) clustermode.Platform {
	platform := clustermode.DetectPlatform(nodes)
	if platform.Detected == "" {
		if name := pricingProviderName(connectionProvider); name != "" {
			platform.Provider, platform.Detected = name, name
		}
	}
	if platform.Region == "" {
		platform.Region = strings.TrimSpace(connectionRegion)
	}

	selected, ok := providers.Get(platform.Provider)
	switch {
	case platform.Detected == "":
		platform.Reason = "its nodes do not report a cloud provider, so it looks like a local or on-premises cluster, and those have no public price list"
	case !ok:
		platform.Reason = fmt.Sprintf("its nodes report %q, which has no public price list; costs can be calculated for clusters on %s", platform.Detected, supportedClouds())
	case platform.Region == "":
		platform.Reason = fmt.Sprintf("it runs on %s but its nodes carry no region label, so the regional prices cannot be chosen", providerTitles[platform.Provider])
	case !slices.Contains(selected.Regions(), selected.NormalizeRegion(platform.Region)):
		platform.Reason = fmt.Sprintf("it runs on %s in %s, a region the price catalog does not cover yet (covered: %s)", providerTitles[platform.Provider], platform.Region, strings.Join(selected.Regions(), ", "))
	default:
		platform.Region = selected.NormalizeRegion(platform.Region)
		platform.Supported = true
	}
	return platform
}

func supportedClouds() string {
	names := make([]string, 0, len(providerTitles))
	for _, name := range providers.Names() {
		if title, ok := providerTitles[name]; ok {
			names = append(names, title)
		}
	}
	return strings.Join(names, ", ")
}
