package workspace

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
)

// ResolveContext turns what a user typed into a kubeconfig context name. It
// accepts, in order: nothing (the current context), the exact name, the
// context's number in Contexts, its short name, or a piece of its name. A
// short name or piece that matches several contexts is refused with the
// candidates, so the wrong cluster is never picked silently.
func ResolveContext(kubeconfigPath, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return CurrentContext(kubeconfigPath)
	}
	contexts, err := Contexts(kubeconfigPath)
	if err != nil {
		return "", err
	}

	for _, context := range contexts {
		if context.Name == query {
			return context.Name, nil
		}
	}
	// A number outside the list may still be part of a name, such as the
	// suffix eksctl gives generated clusters.
	if number, err := strconv.Atoi(query); err == nil && number >= 1 && number <= len(contexts) {
		return contexts[number-1].Name, nil
	}

	for _, match := range []func(KubeconfigContext) bool{
		func(context KubeconfigContext) bool { return strings.EqualFold(context.Short, query) },
		func(context KubeconfigContext) bool {
			return strings.Contains(strings.ToLower(context.Name), strings.ToLower(query))
		},
	} {
		var found []KubeconfigContext
		for _, context := range contexts {
			if match(context) {
				found = append(found, context)
			}
		}
		switch len(found) {
		case 0:
			continue
		case 1:
			return found[0].Name, nil
		default:
			candidates := make([]string, 0, len(found))
			for _, context := range found {
				candidates = append(candidates, fmt.Sprintf("%d) %s", context.Number, context.Name))
			}
			return "", fmt.Errorf("cluster connection: %q matches %d contexts: %s; use the number or the full name", query, len(found), strings.Join(candidates, ", "))
		}
	}
	return "", fmt.Errorf("cluster connection: context %q was not found; list them with 'kubeestimate contexts'", query)
}

// shortContextName finds the cluster name in the context names EKS tools
// write: the ARN from `aws eks update-kubeconfig`, `user@cluster.region.eksctl.io`
// from eksctl, and `eks/cluster/region` from the desktop app. Other names are
// already readable and are kept.
func shortContextName(name string) string {
	if parsed, err := arn.Parse(name); err == nil && parsed.Service == "eks" {
		if cluster, ok := strings.CutPrefix(parsed.Resource, "cluster/"); ok && cluster != "" {
			return cluster
		}
	}
	if host, ok := strings.CutSuffix(name, ".eksctl.io"); ok {
		if _, after, found := strings.Cut(host, "@"); found {
			host = after
		}
		if cluster, _, found := strings.Cut(host, "."); found && cluster != "" {
			return cluster
		}
	}
	if parts := strings.Split(name, "/"); len(parts) == 3 && parts[0] == "eks" && parts[1] != "" {
		return parts[1]
	}
	return name
}
