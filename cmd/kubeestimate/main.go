package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"kube-budget/core/engine"
	"kube-budget/internal/converter"
	"kube-budget/internal/providers/aws"
)

const (
	defaultProvider     = "aws"
	defaultRegion       = "us-east-1"
	defaultInstanceType = "m6i.large"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kubeestimate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	provider := flags.String("provider", defaultProvider, "cloud provider (currently: aws)")
	region := flags.String("region", defaultRegion, "cloud region (for example: eu-west-1)")
	instanceType := flags.String("instance-type", defaultInstanceType, "AWS worker node instance type")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: kubeestimate <manifest.yaml> --provider aws --region <region> [--instance-type <type>]")
		flags.PrintDefaults()
	}

	manifestPath, flagArgs, ok := splitManifestArgument(args)
	if !ok {
		flags.Usage()
		return 2
	}
	if err := flags.Parse(flagArgs); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if *provider != "aws" {
		fmt.Fprintf(stderr, "unsupported provider %q; supported providers: aws\n", *provider)
		return 2
	}
	if !hasFlag(flagArgs, "provider") {
		fmt.Fprintf(stderr, "advice: --provider was not specified; assuming %s\n", defaultProvider)
	}
	if !hasFlag(flagArgs, "region") {
		fmt.Fprintf(stderr, "advice: --region was not specified; assuming %s\n", defaultRegion)
	}
	if !hasFlag(flagArgs, "instance-type") {
		fmt.Fprintf(stderr, "advice: --instance-type was not specified; assuming %s\n", defaultInstanceType)
	}

	config, err := aws.NewPriceConfigForRegion(*instanceType, normalizeAWSRegion(*region))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "read manifest: %v\n", err)
		return 1
	}
	workload, err := converter.ConvertDeployment(manifest)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	estimate := engine.New(config).Estimate(workload)
	fmt.Fprintf(stdout, "Workload: %s\n", workload.Name)
	if workload.Namespace != "" {
		fmt.Fprintf(stdout, "Namespace: %s\n", workload.Namespace)
	}
	fmt.Fprintf(stdout, "Provider: aws (%s)\n", normalizeAWSRegion(*region))
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

func normalizeAWSRegion(region string) string {
	return strings.ReplaceAll(strings.TrimSpace(region), "west1", "west-1")
}

func splitManifestArgument(args []string) (string, []string, bool) {
	for index, argument := range args {
		if !strings.HasPrefix(argument, "-") {
			return argument, append(args[:index:index], args[index+1:]...), true
		}
	}
	return "", nil, false
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
