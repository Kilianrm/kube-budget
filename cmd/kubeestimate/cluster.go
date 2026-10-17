package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	clustermode "kube-cost-workspace/internal/application/cluster"
	"kube-cost-workspace/internal/application/workspace"
)

func runContexts(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kubeestimate contexts", flag.ContinueOnError)
	flags.SetOutput(stderr)
	kubeconfig := flags.String("kubeconfig", "", "path to the kubeconfig file (default: KUBECONFIG or ~/.kube/config)")
	output := outputFlag(flags)
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 || !validOutput(*output, stderr) {
		flags.Usage()
		return exitUsage
	}

	contexts, err := workspace.Contexts(*kubeconfig)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if *output == "json" {
		return writeJSON(stdout, contexts)
	}

	table := newTable(stdout)
	fmt.Fprintln(table, "#\tCURRENT\tSHORT NAME\tCONTEXT\tSERVER")
	for _, context := range contexts {
		current := ""
		if context.Current {
			current = "*"
		}
		fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\n", context.Number, current, context.Short, context.Name, context.Server)
	}
	table.Flush()
	fmt.Fprintln(stdout, "\nPass --context with the number, the short name, the full name or a unique part of it.")
	return exitOK
}

func runCluster(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kubeestimate cluster", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var connection connectionFlags
	connection.register(flags, true)
	workloads := flags.Bool("workloads", false, "also list every workload")
	output := outputFlag(flags)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: kubeestimate cluster [--context <name>] [-n <namespace>] [--workloads] [-o table|json]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 || !validOutput(*output, stderr) {
		flags.Usage()
		return exitUsage
	}

	resolved, err := connection.resolve()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	snapshot, err := workspace.Snapshot(resolved)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	if *output == "json" {
		return writeJSON(stdout, snapshot)
	}
	printSnapshot(stdout, snapshot, *workloads)
	return exitOK
}

func printSnapshot(stdout io.Writer, snapshot clustermode.Snapshot, listWorkloads bool) {
	summary := snapshot.Summary
	fmt.Fprintf(stdout, "Cluster:    %s (%s, Kubernetes %s)\n", snapshot.Cluster.Context, snapshot.Cluster.Server, snapshot.Cluster.Version)
	if snapshot.CollectedAt > 0 {
		fmt.Fprintf(stdout, "Collected:  %s\n", when(time.UnixMilli(snapshot.CollectedAt)))
	}
	fmt.Fprintf(stdout, "Platform:   %s\n", platformLine(snapshot.Platform))
	if provider := snapshot.Provider; provider != nil && provider.ClusterName != "" {
		fmt.Fprintf(stdout, "EKS:        %s in %s, %s, %d node groups\n", provider.ClusterName, provider.Region, provider.Status, len(provider.NodeGroups))
	}
	fmt.Fprintf(stdout, "Nodes:      %d (%d ready)\n", summary.NodeCount, summary.ReadyNodeCount)
	fmt.Fprintf(stdout, "Workloads:  %d (%d missing requests)\n", summary.WorkloadCount, summary.MissingRequestWorkloads)
	fmt.Fprintf(stdout, "Namespaces: %d\n", summary.NamespaceCount)
	fmt.Fprintf(stdout, "CPU:        %s of %s cores requested (%s)\n", cores(summary.Requests.CPUMilli), cores(summary.Allocatable.CPUMilli),
		percent(float64(summary.Requests.CPUMilli), float64(summary.Allocatable.CPUMilli)))
	fmt.Fprintf(stdout, "Memory:     %s of %s requested (%s)\n", gib(summary.Requests.MemoryBytes), gib(summary.Allocatable.MemoryBytes),
		percent(float64(summary.Requests.MemoryBytes), float64(summary.Allocatable.MemoryBytes)))

	if len(snapshot.Nodes) > 0 {
		fmt.Fprintln(stdout)
		table := newTable(stdout)
		fmt.Fprintln(table, "NODE\tTYPE\tGROUP\tZONE\tREADY\tCPU REQ/ALLOC\tMEMORY REQ/ALLOC")
		for _, node := range snapshot.Nodes {
			ready := "yes"
			if !node.Ready {
				ready = "no"
			} else if !node.Schedulable {
				ready = "cordoned"
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s/%s\t%s/%s\n", node.Name, dash(node.InstanceType), dash(node.NodeGroup), dash(node.Zone), ready,
				cores(node.Requests.CPUMilli), cores(node.Allocatable.CPUMilli), gib(node.Requests.MemoryBytes), gib(node.Allocatable.MemoryBytes))
		}
		table.Flush()
	}

	if len(snapshot.Namespaces) > 0 {
		namespaces := append([]clustermode.NamespaceSummary(nil), snapshot.Namespaces...)
		sort.Slice(namespaces, func(i, j int) bool { return namespaces[i].Name < namespaces[j].Name })
		fmt.Fprintln(stdout)
		table := newTable(stdout)
		fmt.Fprintln(table, "NAMESPACE\tWORKLOADS\tPODS\tCPU REQ\tMEMORY REQ\tMISSING REQUESTS")
		for _, namespace := range namespaces {
			fmt.Fprintf(table, "%s\t%d\t%d\t%s\t%s\t%d\n", namespace.Name, namespace.WorkloadCount, namespace.PodCount,
				cores(namespace.Requests.CPUMilli), gib(namespace.Requests.MemoryBytes), namespace.MissingRequests)
		}
		table.Flush()
	}

	if listWorkloads && len(snapshot.Workloads) > 0 {
		workloads := append([]clustermode.Workload(nil), snapshot.Workloads...)
		sort.Slice(workloads, func(i, j int) bool {
			if workloads[i].Namespace != workloads[j].Namespace {
				return workloads[i].Namespace < workloads[j].Namespace
			}
			return workloads[i].Name < workloads[j].Name
		})
		fmt.Fprintln(stdout)
		table := newTable(stdout)
		fmt.Fprintln(table, "NAMESPACE\tKIND\tNAME\tREADY\tCPU REQ\tMEMORY REQ\tMISSING")
		for _, workload := range workloads {
			missing := "-"
			if workload.MissingRequests {
				missing = fmt.Sprint(workload.MissingResources)
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%d/%d\t%s\t%s\t%s\n", workload.Namespace, workload.Kind, workload.Name, workload.ReadyReplicas, workload.DesiredReplicas,
				cores(workload.Requests.CPUMilli), gib(workload.Requests.MemoryBytes), missing)
		}
		table.Flush()
	}

	printWarnings(stdout, snapshot.Warnings)
}

func platformLine(platform clustermode.Platform) string {
	if platform.Supported {
		return fmt.Sprintf("%s %s (can be priced)", platform.Provider, platform.Region)
	}
	detected := platform.Detected
	if detected == "" {
		detected = "unknown"
	}
	return fmt.Sprintf("%s (cannot be priced: %s)", detected, platform.Reason)
}

func dash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
