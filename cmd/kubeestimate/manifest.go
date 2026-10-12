package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/internal/application/workspace"
	"kube-cost-workspace/internal/providers"
)

const defaultProvider = "aws"

// manifestOutput is the JSON shape of a manifest estimate.
type manifestOutput struct {
	Workload     string                `json:"workload"`
	Namespace    string                `json:"namespace,omitempty"`
	Replicas     int32                 `json:"replicas"`
	MinReplicas  *int32                `json:"minReplicas,omitempty"`
	MaxReplicas  *int32                `json:"maxReplicas,omitempty"`
	Provider     string                `json:"provider"`
	Region       string                `json:"region"`
	InstanceType string                `json:"instanceType"`
	Currency     string                `json:"currency"`
	Cost         costmodel.Projection  `json:"cost"`
	MinCost      *costmodel.Projection `json:"minCost,omitempty"`
	MaxCost      *costmodel.Projection `json:"maxCost,omitempty"`
	// PerContainer is each container's hourly cost for all replicas.
	PerContainer map[string]float64 `json:"perContainerHourly"`
}

func runManifest(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kubeestimate manifest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var manifestPath string
	flags.StringVar(&manifestPath, "f", "", "Kubernetes manifest file")
	flags.StringVar(&manifestPath, "file", "", "Kubernetes manifest file")
	provider := flags.String("provider", defaultProvider, "registered cloud provider")
	region := flags.String("region", "", "cloud region (for example: eu-west-1 or us-central1)")
	instanceType := flags.String("instance-type", "", "worker node machine type")
	output := outputFlag(flags)
	flags.Usage = func() {
		fmt.Fprintf(stderr, "Usage: kubeestimate manifest -f <manifest.yaml> --provider <%s> --region <region> [--instance-type <type>] [-o table|json]\n", strings.Join(providers.Names(), "|"))
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if manifestPath == "" || flags.NArg() != 0 {
		flags.Usage()
		return exitUsage
	}
	if !validOutput(*output, stderr) {
		return exitUsage
	}
	providerName := strings.ToLower(strings.TrimSpace(*provider))
	selectedProvider, ok := providers.Get(providerName)
	if !ok {
		fmt.Fprintf(stderr, "unsupported provider %q; supported providers: %s\n", *provider, strings.Join(providers.Names(), ", "))
		return exitUsage
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

	manifestInput, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "read manifest: %v\n", err)
		return exitError
	}
	estimate, err := workspace.PriceManifest(manifestInput, selectedProvider, selectedProvider.NormalizeRegion(*region), *instanceType)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}

	if *output == "json" {
		return writeJSON(stdout, manifestJSON(estimate))
	}
	printManifest(stdout, estimate)
	// The estimate is the workload's own cost; the bill can move by more (new
	// nodes) or nothing (idle capacity), which only simulate can tell.
	fmt.Fprintf(stderr, "hint: this is the workload's own cost. To see how it would change a cluster's bill, run: kubeestimate simulate -f %s --context <name>\n", manifestPath)
	return exitOK
}

func manifestJSON(estimate workspace.ManifestEstimate) manifestOutput {
	workload, result := estimate.Workload, estimate.Estimate
	return manifestOutput{
		Workload:     workload.Name,
		Namespace:    workload.Namespace,
		Replicas:     workload.Replicas,
		MinReplicas:  workload.MinReplicas,
		MaxReplicas:  workload.MaxReplicas,
		Provider:     estimate.Provider,
		Region:       estimate.Region,
		InstanceType: estimate.InstanceType,
		Currency:     result.Currency,
		Cost:         costmodel.Project(result.Total),
		MinCost:      projectBound(result.MinTotal),
		MaxCost:      projectBound(result.MaxTotal),
		PerContainer: result.PerResource,
	}
}

func printManifest(stdout io.Writer, estimate workspace.ManifestEstimate) {
	workload, result := estimate.Workload, estimate.Estimate
	cost := costmodel.Project(result.Total)
	fmt.Fprintf(stdout, "Workload: %s\n", workload.Name)
	if workload.Namespace != "" {
		fmt.Fprintf(stdout, "Namespace: %s\n", workload.Namespace)
	}
	fmt.Fprintf(stdout, "Provider: %s (%s)\n", estimate.Provider, estimate.Region)
	fmt.Fprintf(stdout, "Instance type: %s\n", estimate.InstanceType)
	fmt.Fprintf(stdout, "Replicas: %d\n", workload.Replicas)
	fmt.Fprintf(stdout, "Hourly total: %.4f %s\n", cost.Hourly, result.Currency)

	names := make([]string, 0, len(result.PerResource))
	for name := range result.PerResource {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(stdout, "  %s: %.4f %s/hour\n", name, result.PerResource[name], result.Currency)
	}

	fmt.Fprintf(stdout, "Daily total: %.2f %s\n", cost.Daily, result.Currency)
	fmt.Fprintf(stdout, "Monthly total: %.2f %s\n", cost.Monthly, result.Currency)
	if result.MinTotal != nil && result.MaxTotal != nil && workload.MinReplicas != nil && workload.MaxReplicas != nil {
		fmt.Fprintf(stdout, "Autoscaling range: %.2f to %.2f %s/month (%d to %d replicas)\n",
			costmodel.Project(*result.MinTotal).Monthly, costmodel.Project(*result.MaxTotal).Monthly, result.Currency, *workload.MinReplicas, *workload.MaxReplicas)
	}
}

func projectBound(hourlyUSD *float64) *costmodel.Projection {
	if hourlyUSD == nil {
		return nil
	}
	projection := costmodel.Project(*hourlyUSD)
	return &projection
}
