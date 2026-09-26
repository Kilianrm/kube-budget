package optimize

import (
	"fmt"
	"strings"

	"kube-budget/core/costmodel"
)

// metricsServerManifest is the upstream install manifest of metrics-server.
const metricsServerManifest = "https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml"

// dataIssues lists what makes the report incomplete. They carry no saving:
// they decide how far every saving in the plan can be trusted.
func dataIssues(report costmodel.CostReport) []Recommendation {
	issues := make([]Recommendation, 0)

	if missing := missingRequestItems(report); len(missing) > 0 {
		items := make([]Item, 0, len(missing))
		for _, item := range missing {
			entry := Item{Subject: item.Subject, Change: "no CPU or memory request"}
			kind := strings.ToLower(detailString(item, "kind"))
			if kind == "deployment" || kind == "statefulset" || kind == "daemonset" {
				entry.Command = fmt.Sprintf("kubectl set resources %s/%s -n %s --requests=cpu=<cpu>,memory=<memory>", kind, item.Subject.Name, item.Subject.Namespace)
			}
			entry.Note = fmt.Sprintf("Base the values on real usage: kubectl top pods -n %s", item.Subject.Namespace)
			items = append(items, entry)
		}
		issues = append(issues, Recommendation{
			ID:         "missing-requests",
			Category:   CategoryData,
			Title:      fmt.Sprintf("%s %s no resource requests", plural(len(missing), "workload"), verb(len(missing), "has", "have")),
			Effort:     LevelLow,
			Risk:       LevelLow,
			Confidence: costmodel.ConfidenceExact,
			Rationale:  "Their cost cannot be attributed, idle capacity is overstated, and node-level savings (consolidation, node type, spot) are held back until the cluster's demand is complete.",
			Action:     "Declare resources.requests for each of them, then refresh the report.",
			Items:      items,
		})
	}

	if partial := partialRequestItems(report); len(partial) > 0 {
		items := make([]Item, 0, len(partial))
		for _, item := range partial {
			missing, _ := item.Detail["missingResources"].([]string)
			entry := Item{Subject: item.Subject, Change: "no " + strings.Join(missing, " or ") + " request"}
			kind := strings.ToLower(detailString(item, "kind"))
			if kind == "deployment" || kind == "statefulset" || kind == "daemonset" {
				placeholders := make([]string, 0, len(missing))
				for _, resource := range missing {
					placeholders = append(placeholders, fmt.Sprintf("%s=<%s>", resource, resource))
				}
				entry.Command = fmt.Sprintf("kubectl set resources %s/%s -n %s --requests=%s", kind, item.Subject.Name, item.Subject.Namespace, strings.Join(placeholders, ","))
			}
			items = append(items, entry)
		}
		issues = append(issues, Recommendation{
			ID:         "incomplete-requests",
			Category:   CategoryData,
			Title:      fmt.Sprintf("%s %s CPU or memory unrequested", plural(len(partial), "workload"), verb(len(partial), "leaves", "leave")),
			Effort:     LevelLow,
			Risk:       LevelLow,
			Confidence: costmodel.ConfidenceExact,
			Rationale:  "They are priced on what they declare, so their cost reads low and idle capacity reads high. The plan still runs.",
			Action:     "Add the missing request to each container, based on its usage.",
			Items:      items,
		})
	}

	unpriced := make([]Item, 0)
	for _, item := range report.Items {
		if item.Priced() || isMissingRequests(item) {
			continue
		}
		unpriced = append(unpriced, Item{Subject: item.Subject, Change: "no rate in the price catalog"})
	}
	if len(unpriced) > 0 {
		issues = append(issues, Recommendation{
			ID:         "unpriced-resources",
			Category:   CategoryData,
			Title:      fmt.Sprintf("%s %s no price", plural(len(unpriced), "resource"), verb(len(unpriced), "has", "have")),
			Effort:     LevelLow,
			Risk:       LevelLow,
			Confidence: costmodel.ConfidenceExact,
			Rationale:  "They are left out of every total, so the billed figure and the plan are understated.",
			Action:     "Add the missing machine types or storage rates to the price catalog for this region.",
			Items:      unpriced,
		})
	}

	if report.Usage == nil && hasPricedWorkloads(report) {
		issues = append(issues, Recommendation{
			ID:         "usage-metrics",
			Category:   CategoryData,
			Title:      "Usage metrics are not available",
			Effort:     LevelLow,
			Risk:       LevelLow,
			Confidence: costmodel.ConfidenceExact,
			Rationale:  "Without metrics-server the plan cannot compare requests with real usage, so rightsizing is off.",
			Action:     "Install metrics-server (most managed clusters offer it as an add-on), then refresh the report.",
			Items: []Item{{
				Subject: costmodel.Subject{Kind: costmodel.SubjectAddon, Name: "metrics-server"},
				Change:  "not reachable",
				Command: "kubectl apply -f " + metricsServerManifest,
			}},
		})
	}
	return issues
}

func missingRequestItems(report costmodel.CostReport) []costmodel.LineItem {
	items := make([]costmodel.LineItem, 0)
	for _, item := range report.Items {
		if isMissingRequests(item) {
			items = append(items, item)
		}
	}
	return items
}

// partialRequestItems lists workloads outside system namespaces that omit a
// resource request. Managed add-ons are left out: their requests are set by
// the provider.
func partialRequestItems(report costmodel.CostReport) []costmodel.LineItem {
	items := make([]costmodel.LineItem, 0)
	for _, item := range report.Items {
		missing, _ := item.Detail["missingResources"].([]string)
		if item.Basis == costmodel.BasisRequested && len(missing) > 0 && !IsSystemNamespace(item.Subject.Namespace) {
			items = append(items, item)
		}
	}
	return items
}

func isMissingRequests(item costmodel.LineItem) bool {
	missing, _ := item.Detail["missingRequests"].(bool)
	return item.Basis == costmodel.BasisRequested && missing
}

func hasPricedWorkloads(report costmodel.CostReport) bool {
	for _, item := range report.Items {
		if item.Basis == costmodel.BasisRequested && item.Priced() {
			return true
		}
	}
	return false
}

func verb(count int, singular, pluralForm string) string {
	if count == 1 {
		return singular
	}
	return pluralForm
}
