package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"time"

	"kube-cost-workspace/core/costmodel"
	"kube-cost-workspace/core/costseries"
	"kube-cost-workspace/internal/application/workspace"
)

// costOutput is the JSON shape of a cluster cost report.
type costOutput struct {
	ClusterID string               `json:"clusterId"`
	Report    costmodel.CostReport `json:"report"`
	Forecast  *forecastOutput      `json:"forecast,omitempty"`
}

type forecastOutput struct {
	Forecast costseries.Forecast      `json:"forecast"`
	Budget   *costseries.BudgetStatus `json:"budget,omitempty"`
}

// costRow is one line of a breakdown.
type costRow struct {
	name       string
	namespace  string
	cost       costmodel.Projection
	confidence costmodel.Confidence
}

func runCost(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("kubeestimate cost", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var connection connectionFlags
	connection.register(flags, true)
	var pricing pricingFlags
	pricing.register(flags)
	by := flags.String("by", "namespace", "break the bill down by namespace, workload or nodegroup")
	top := flags.Int("top", 0, "show only the N most expensive rows (default: all)")
	noHistory := flags.Bool("no-history", false, "do not add this report to the cluster's spend history")
	output := outputFlag(flags)
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: kubeestimate cost [--context <name>] [-n <namespace>] [--by namespace|workload|nodegroup] [--top N] [-o table|json]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 || !validOutput(*output, stderr) {
		flags.Usage()
		return exitUsage
	}
	if *by != "namespace" && *by != "workload" && *by != "nodegroup" {
		fmt.Fprintf(stderr, "unsupported breakdown %q; use namespace, workload or nodegroup\n", *by)
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
	priced, err := workspace.Price(snapshot, pricing.options(resolved.Namespace))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitError
	}

	// The desktop app reads the same history, so each run adds a point to the
	// cluster's spend trend and forecast.
	now := time.Now()
	if !*noHistory {
		if err := workspace.RecordHistory(priced.Report, now); err != nil {
			fmt.Fprintf(stderr, "warning: the report was not added to the spend history: %v\n", err)
		}
	}
	var forecast *forecastOutput
	if inputs, err := workspace.LoadForecast(priced.ClusterID, now); err == nil {
		month := inputs.Forecast(priced.Report.Totals[costmodel.BasisProvisioned].Hourly)
		forecast = &forecastOutput{Forecast: month.Forecast, Budget: month.Budget}
	}

	if *output == "json" {
		return writeJSON(stdout, costOutput{ClusterID: priced.ClusterID, Report: priced.Report, Forecast: forecast})
	}
	printCost(stdout, priced.Report, forecast, *by, *top)
	return exitOK
}

func printCost(stdout io.Writer, report costmodel.CostReport, forecast *forecastOutput, by string, top int) {
	scope := report.Scope
	name := scope.ClusterName
	if name == "" {
		name = "cluster"
	}
	fmt.Fprintf(stdout, "Cluster:   %s, priced as %s %s\n", name, scope.Provider, scope.Region)
	if scope.Namespace != "" {
		fmt.Fprintf(stdout, "Namespace: %s\n", scope.Namespace)
	}
	fmt.Fprintf(stdout, "Generated: %s\n\n", when(report.GeneratedAt))

	billed := report.Totals[costmodel.BasisProvisioned]
	table := newTable(stdout)
	fmt.Fprintln(table, "\tHOURLY\tMONTHLY\tSHARE")
	fmt.Fprintf(table, "Billed\t%s\t%s\t\n", hourlyUSD(billed.Hourly), usd(billed.Monthly))
	requested := report.Totals[costmodel.BasisRequested]
	fmt.Fprintf(table, "  Requested by workloads\t%s\t%s\t%s\n", hourlyUSD(requested.Hourly), usd(requested.Monthly), percent(requested.Hourly, billed.Hourly))
	fmt.Fprintf(table, "  Idle capacity\t%s\t%s\t%s\n", hourlyUSD(report.Idle.Hourly), usd(report.Idle.Monthly), percent(report.Idle.Hourly, billed.Hourly))
	fmt.Fprintf(table, "  Shared costs\t%s\t%s\t%s\n", hourlyUSD(report.Shared.Hourly), usd(report.Shared.Monthly), percent(report.Shared.Hourly, billed.Hourly))
	table.Flush()

	rows := breakdown(report, by)
	if top > 0 && len(rows) > top {
		rows = rows[:top]
	}
	if len(rows) > 0 {
		fmt.Fprintln(stdout)
		table = newTable(stdout)
		switch by {
		case "workload":
			fmt.Fprintln(table, "NAMESPACE\tWORKLOAD\tMONTHLY\tSHARE\tCONFIDENCE")
			for _, row := range rows {
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", dash(row.namespace), row.name, usd(row.cost.Monthly), percent(row.cost.Hourly, billed.Hourly), dash(string(row.confidence)))
			}
		default:
			heading := "NAMESPACE"
			if by == "nodegroup" {
				heading = "NODE GROUP"
			}
			fmt.Fprintf(table, "%s\tMONTHLY\tSHARE\tCONFIDENCE\n", heading)
			for _, row := range rows {
				fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", row.name, usd(row.cost.Monthly), percent(row.cost.Hourly, billed.Hourly), dash(string(row.confidence)))
			}
		}
		table.Flush()
	}

	if forecast != nil {
		fmt.Fprintln(stdout)
		printForecast(stdout, "This month", forecast.Forecast, forecast.Budget)
	}

	if len(report.Warnings) > 0 {
		fmt.Fprintf(stdout, "\nWarnings (%d)\n", len(report.Warnings))
		for _, warning := range report.Warnings {
			subject := ""
			if warning.Subject != "" {
				subject = warning.Subject + ": "
			}
			fmt.Fprintf(stdout, "  - %s%s\n", subject, warning.Message)
		}
	}
}

// breakdown splits the billed total. Namespaces and workloads come from the
// allocation, which includes idle and shared rows so the rows add up to the
// bill; node groups come from the billed nodes.
func breakdown(report costmodel.CostReport, by string) []costRow {
	if by == "nodegroup" {
		groups := report.ByDimension[costmodel.BasisProvisioned][costmodel.DimensionParent]
		rows := make([]costRow, 0, len(groups))
		for _, group := range sortedKeys(groups) {
			rows = append(rows, costRow{name: group, cost: groups[group]})
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].cost.Hourly > rows[j].cost.Hourly })
		return rows
	}

	dimension := costmodel.DimensionNamespace
	if by == "workload" {
		dimension = costmodel.DimensionWorkload
	}
	allocation := report.Allocation[dimension]
	rows := make([]costRow, 0, len(allocation))
	for _, row := range allocation {
		rows = append(rows, costRow{name: row.Name, namespace: row.Namespace, cost: row.Cost, confidence: row.Confidence})
	}
	return rows
}

func printForecast(stdout io.Writer, label string, forecast costseries.Forecast, budget *costseries.BudgetStatus) {
	fmt.Fprintf(stdout, "%s: %s forecast (%s spent so far, %s at %s/h for the remaining %.0f hours)\n", label,
		usd(forecast.TotalUSD), usd(forecast.SpentUSD()), usd(forecast.ProjectedUSD), hourlyUSD(forecast.RunRateHourly), forecast.RemainingHours)
	if budget != nil && budget.MonthlyUSD > 0 {
		fmt.Fprintf(stdout, "Budget: %s, %s (%s of it, %s left)\n", usd(budget.MonthlyUSD), budget.State, percent(budget.ProjectedRatio, 1), usd(budget.RemainingUSD))
	}
}
