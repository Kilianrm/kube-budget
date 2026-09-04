package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	manifestmode "kube-budget/internal/application/manifest"
	"kube-budget/internal/providers"
)

const (
	defaultProvider = "aws"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if hasManifestFileFlag(args) {
		return runManifest(args, stdout, stderr)
	}

	printUsage(stderr)
	return 2
}

func printUsage(stderr io.Writer) {
	fmt.Fprintln(stderr, "Usage: kubeestimate -f <manifest.yaml> [options]")
	fmt.Fprintln(stderr, "  -f, --file  select Manifest Mode and estimate the provided manifest")
}

func runManifest(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kubeestimate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var manifestPath string
	flags.StringVar(&manifestPath, "f", "", "Kubernetes manifest file")
	flags.StringVar(&manifestPath, "file", "", "Kubernetes manifest file")
	provider := flags.String("provider", defaultProvider, "registered cloud provider")
	region := flags.String("region", "", "cloud region (for example: eu-west-1 or us-central1)")
	instanceType := flags.String("instance-type", "", "worker node machine type")
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: kubeestimate -f <manifest.yaml> --provider <%s> --region <region> [--instance-type <type>]\n", strings.Join(providers.Names(), "|"))
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if manifestPath == "" || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	providerName := strings.ToLower(strings.TrimSpace(*provider))
	selectedProvider, ok := providers.Get(providerName)
	if !ok {
		fmt.Fprintf(stderr, "unsupported provider %q; supported providers: %s\n", *provider, strings.Join(providers.Names(), ", "))
		return 2
	}
	defaultRegion, defaultInstanceType := selectedProvider.DefaultRegion(), selectedProvider.DefaultMachineType()
	if *region == "" {
		*region = defaultRegion
	}
	if *instanceType == "" {
		*instanceType = defaultInstanceType
	}
	if !hasFlag(args, "provider") {
		fmt.Fprintf(stderr, "advice: --provider was not specified; assuming %s\n", defaultProvider)
	}
	if !hasFlag(args, "region") {
		fmt.Fprintf(stderr, "advice: --region was not specified; assuming %s\n", defaultRegion)
	}
	if !hasFlag(args, "instance-type") {
		fmt.Fprintf(stderr, "advice: --instance-type was not specified; assuming %s\n", defaultInstanceType)
	}

	regionValue := selectedProvider.NormalizeRegion(*region)
	config, err := selectedProvider.NewPriceConfig(*instanceType, regionValue)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	manifestInput, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "read manifest: %v\n", err)
		return 1
	}
	result, err := manifestmode.New(config).Estimate(manifestInput)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	workload := result.Workload
	estimate := result.Estimate
	fmt.Fprintf(stdout, "Workload: %s\n", workload.Name)
	if workload.Namespace != "" {
		fmt.Fprintf(stdout, "Namespace: %s\n", workload.Namespace)
	}
	fmt.Fprintf(stdout, "Provider: %s (%s)\n", providerName, regionValue)
	fmt.Fprintf(stdout, "Instance type: %s\n", *instanceType)
	fmt.Fprintf(stdout, "Replicas: %d\n", workload.Replicas)
	fmt.Fprintf(stdout, "Hourly total: %.4f %s\n", estimate.Total, estimate.Currency)

	names := make([]string, 0, len(estimate.PerResource))
	for name := range estimate.PerResource {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(stdout, "  %s: %.4f %s/hour\n", name, estimate.PerResource[name], estimate.Currency)
	}

	fmt.Fprintf(stdout, "Daily total: %.4f %s\n", estimate.Total*24, estimate.Currency)
	fmt.Fprintf(stdout, "Monthly total: %.4f %s\n", estimate.Total*24*30, estimate.Currency)

	return 0
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
