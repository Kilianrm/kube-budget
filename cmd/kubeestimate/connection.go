package main

import (
	"flag"

	"kube-cost-workspace/internal/application/workspace"
)

// connectionFlags select a cluster the way kubectl does: the kubeconfig's
// current context unless --context names another one.
type connectionFlags struct {
	kubeconfig string
	context    string
	namespace  string
}

func (connection *connectionFlags) register(flags *flag.FlagSet, namespaced bool) {
	flags.StringVar(&connection.kubeconfig, "kubeconfig", "", "path to the kubeconfig file (default: KUBECONFIG or ~/.kube/config)")
	flags.StringVar(&connection.context, "context", "", "context to use: its name, number or short name from 'kubeestimate contexts', or a unique part of the name (default: the current context)")
	if !namespaced {
		return
	}
	flags.StringVar(&connection.namespace, "n", "", "limit to one namespace (default: all namespaces)")
	flags.StringVar(&connection.namespace, "namespace", "", "limit to one namespace (default: all namespaces)")
}

// resolve accepts a context's full name, its number or short name from
// 'kubeestimate contexts', or a unique piece of its name.
func (connection connectionFlags) resolve() (workspace.Connection, error) {
	contextName, err := workspace.ResolveContext(connection.kubeconfig, connection.context)
	if err != nil {
		return workspace.Connection{}, err
	}
	return workspace.Connection{
		KubeconfigPath: connection.kubeconfig,
		Context:        contextName,
		Namespace:      connection.namespace,
	}, nil
}

// pricingFlags override the price list a cluster is priced with.
type pricingFlags struct {
	provider     string
	region       string
	instanceType string
}

func (pricing *pricingFlags) register(flags *flag.FlagSet) {
	flags.StringVar(&pricing.provider, "provider", "", "pricing provider (default: detected from the nodes)")
	flags.StringVar(&pricing.region, "region", "", "pricing region (default: detected from the nodes)")
	flags.StringVar(&pricing.instanceType, "instance-type", "", "machine type whose rates price workload requests when pod placement is unknown (default: the most common node type); nodes keep their own type")
}

func (pricing pricingFlags) options(namespace string) workspace.PriceOptions {
	return workspace.PriceOptions{
		Provider:     pricing.provider,
		Region:       pricing.region,
		InstanceType: pricing.instanceType,
		Namespace:    namespace,
	}
}
