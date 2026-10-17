package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/whatif"
	"kube-cost-workspace/internal/application/workspace"
)

// simulationOutput is the JSON shape of a simulation.
type simulationOutput struct {
	ClusterID      string          `json:"clusterId"`
	ReportAt       time.Time       `json:"reportAt"`
	Estimate       manifestOutput  `json:"estimate"`
	Impact         whatif.Result   `json:"impact"`
	Peak           *whatif.Result  `json:"peak,omitempty"`
	ForecastBefore *forecastOutput `json:"forecastBefore,omitempty"`
	ForecastAfter  *forecastOutput `json:"forecastAfter,omitempty"`
}

func runSimulate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kubeestimate simulate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var manifestPath string
	flags.StringVar(&manifestPath, "f", "", "Deployment manifest to simulate")
	flags.StringVar(&manifestPath, "file", "", "Deployment manifest to simulate")
	var connection connectionFlags
	connection.register(flags, false)
	var pricing pricingFlags
	pricing.register(flags)
	output := outputFlag(flags)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: kubeestimate simulate -f <manifest.yaml> [--context <name>] [-o table|json]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if manifestPath == "" || flags.NArg() != 0 || !validOutput(*output, stderr) {
		flags.Usage()
		return exitUsage
	}

	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "read manifest: %v\n", err)
		return exitError
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
	priced, err := workspace.Price(snapshot, pricing.options(""))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}
	simulation, err := workspace.Simulate(priced, manifest)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}

	if *output == "json" {
		result := simulationOutput{
			ClusterID: priced.ClusterID,
			ReportAt:  simulation.ReportAt,
			Estimate:  manifestJSON(simulation.Estimate),
			Impact:    simulation.Impact,
			Peak:      simulation.Peak,
		}
		if simulation.ForecastBefore != nil && simulation.ForecastAfter != nil {
			result.ForecastBefore = &forecastOutput{Forecast: simulation.ForecastBefore.Forecast, Budget: simulation.ForecastBefore.Budget}
			result.ForecastAfter = &forecastOutput{Forecast: simulation.ForecastAfter.Forecast, Budget: simulation.ForecastAfter.Budget}
		}
		return writeJSON(stdout, result)
	}
	printSimulation(stdout, priced.Report.Scope, simulation)
	return exitOK
}

func printSimulation(stdout io.Writer, scope costmodel.Scope, simulation workspace.Simulation) {
	impact := simulation.Impact
	workload := impact.Workload
	fmt.Fprintf(stdout, "Manifest:  %s/%s, %d replicas\n", workload.Namespace, workload.Name, workload.Replicas)
	fmt.Fprintf(stdout, "Cluster:   %s, priced as %s %s at %s rates\n", dash(scope.ClusterName), scope.Provider, scope.Region, simulation.Estimate.InstanceType)
	switch {
	case impact.Operation == "update" && impact.Current != nil:
		fmt.Fprintf(stdout, "Operation: update of the running Deployment (%d replicas today)\n", impact.Current.Replicas)
	default:
		fmt.Fprintln(stdout, "Operation: new workload")
	}
	fmt.Fprintln(stdout)

	table := newTable(stdout)
	fmt.Fprintln(table, "\tBEFORE\tAFTER\tCHANGE")
	fmt.Fprintf(table, "Billed / month\t%s\t%s\t%s\n", usd(impact.BilledBefore.Monthly), usd(impact.BilledAfter.Monthly), signedUSD(impact.BilledDelta.Monthly))
	fmt.Fprintf(table, "Idle / month\t%s\t%s\t%s\n", usd(impact.IdleBefore.Monthly), usd(impact.IdleAfter.Monthly), signedUSD(impact.IdleAfter.Monthly-impact.IdleBefore.Monthly))
	fmt.Fprintf(table, "Plan savings / month\t%s\t%s\t%s\n", usd(impact.PlanSavingsBefore.Monthly), usd(impact.PlanSavingsAfter.Monthly), signedUSD(impact.PlanSavingsAfter.Monthly-impact.PlanSavingsBefore.Monthly))
	table.Flush()
	fmt.Fprintf(stdout, "\nRequested: %s/month of the cluster's capacity at its own rates\n", usd(impact.Requested.Monthly))

	fmt.Fprintf(stdout, "Placement: %s\n", placementLine(impact))
	if impact.Unschedulable > 0 {
		fmt.Fprintf(stdout, "           %d replicas fit on no node\n", impact.Unschedulable)
	}

	if peak := simulation.Peak; peak != nil {
		fmt.Fprintf(stdout, "At the autoscaler's maximum (%d replicas): billed %s/month (%s)\n", peak.Workload.Replicas, usd(peak.BilledAfter.Monthly), signedUSD(peak.BilledDelta.Monthly))
	}

	if len(impact.PlanChanges) > 0 {
		fmt.Fprintln(stdout, "\nOptimization plan changes")
		for _, change := range impact.PlanChanges {
			fmt.Fprintf(stdout, "  - %s: %s -> %s/month\n", change.Title, usd(change.Before.Monthly), usd(change.After.Monthly))
		}
	}

	if before, after := simulation.ForecastBefore, simulation.ForecastAfter; before != nil && after != nil {
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "This month: %s forecast without it, %s if deployed now (%s)\n",
			usd(before.Forecast.TotalUSD), usd(after.Forecast.TotalUSD), signedUSD(after.Forecast.TotalUSD-before.Forecast.TotalUSD))
		if after.Budget != nil && after.Budget.MonthlyUSD > 0 {
			fmt.Fprintf(stdout, "Budget: %s, %s after the change (%s left)\n", usd(after.Budget.MonthlyUSD), after.Budget.State, usd(after.Budget.RemainingUSD))
		}
	}

	if len(impact.Warnings) > 0 {
		fmt.Fprintf(stdout, "\nWarnings (%d)\n", len(impact.Warnings))
		for _, warning := range impact.Warnings {
			fmt.Fprintf(stdout, "  - %s\n", warning)
		}
	}
	fmt.Fprintf(stdout, "\nBased on the cluster as collected at %s.\n", when(simulation.ReportAt))
}

func placementLine(impact whatif.Result) string {
	if len(impact.Placements) == 0 {
		return "nowhere"
	}
	parts := make([]string, 0, len(impact.Placements))
	for _, placement := range impact.Placements {
		parts = append(parts, fmt.Sprintf("%s x%d", placement.Node, placement.Replicas))
	}
	line := strings.Join(parts, ", ")
	switch {
	case impact.NewNodes > 0:
		line += fmt.Sprintf("; needs %d new %s node(s)", impact.NewNodes, dash(impact.NewNodeType))
	case impact.Fits:
		line += "; fits on the existing nodes"
	}
	return line
}
