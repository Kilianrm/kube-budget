// Command kubeestimate brings the workspace to the terminal: manifest
// estimates, a read-only view of a cluster, its cost report and what
// deploying a manifest would do to its bill.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Exit codes: 0 on success, 1 when the work failed, 2 for a usage error.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

type command struct {
	name    string
	summary string
	run     func(args []string, stdout, stderr io.Writer) int
}

var commands = []command{
	{name: "manifest", summary: "what a Deployment's requests cost at a provider's list prices (no cluster needed)", run: runManifest},
	{name: "contexts", summary: "list the kubeconfig contexts", run: runContexts},
	{name: "cluster", summary: "summarize a cluster: nodes, namespaces, workloads and warnings", run: runCluster},
	{name: "cost", summary: "price a cluster: billed, requested, idle and shared, by namespace, workload or node group", run: runCost},
	{name: "simulate", summary: "how deploying a manifest changes a real cluster's bill: placement, new nodes, idle, budget and plan", run: runSimulate},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return exitUsage
	}

	name := args[0]
	switch {
	case name == "help" || name == "-h" || name == "--help":
		printUsage(stdout)
		return exitOK
	case strings.HasPrefix(name, "-"):
		// kubeestimate -f app.yaml, the form before subcommands existed.
		if hasManifestFileFlag(args) {
			return runManifest(args, stdout, stderr)
		}
		printUsage(stderr)
		return exitUsage
	}

	for _, command := range commands {
		if command.name == name {
			return command.run(args[1:], stdout, stderr)
		}
	}
	fmt.Fprintf(stderr, "unknown command %q\n\n", name)
	printUsage(stderr)
	return exitUsage
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: kubeestimate <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	for _, command := range commands {
		fmt.Fprintf(w, "  %-9s %s\n", command.name, command.summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  kubeestimate manifest -f <manifest.yaml> --provider aws --region eu-west-1")
	fmt.Fprintln(w, "  kubeestimate cost --context prod --by workload")
	fmt.Fprintln(w, "  kubeestimate simulate -f <manifest.yaml> --context prod")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run 'kubeestimate <command> -h' for a command's options. Every command accepts -o json.")
}

func hasFlag(args []string, name string) bool {
	prefix := "--" + name
	for _, argument := range args {
		if argument == prefix || strings.HasPrefix(argument, prefix+"=") {
			return true
		}
	}
	return false
}

func hasManifestFileFlag(args []string) bool {
	for _, argument := range args {
		if argument == "-f" || argument == "--file" || strings.HasPrefix(argument, "-f=") || strings.HasPrefix(argument, "--file=") {
			return true
		}
	}
	return false
}
