package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	clustermode "kube-cost-workspace/internal/application/cluster"
)

const gibibyte = 1024 * 1024 * 1024

// outputFlag registers -o/--output, which selects a table or JSON.
func outputFlag(flags *flag.FlagSet) *string {
	output := new(string)
	flags.StringVar(output, "o", "table", "output format: table or json")
	flags.StringVar(output, "output", "table", "output format: table or json")
	return output
}

func validOutput(output string, stderr io.Writer) bool {
	if output == "table" || output == "json" {
		return true
	}
	fmt.Fprintf(stderr, "unsupported output %q; use table or json\n", output)
	return false
}

func writeJSON(stdout io.Writer, value any) int {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return 1
	}
	return 0
}

func newTable(stdout io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
}

// usd formats a monthly or larger amount.
func usd(value float64) string {
	sign := ""
	if value < 0 {
		sign, value = "-", -value
	}
	whole := fmt.Sprintf("%.2f", value)
	integer, decimals, _ := strings.Cut(whole, ".")
	for index := len(integer) - 3; index > 0; index -= 3 {
		integer = integer[:index] + "," + integer[index:]
	}
	return sign + "$" + integer + "." + decimals
}

// signedUSD formats a change, always with its sign.
func signedUSD(value float64) string {
	if value >= 0 {
		return "+" + usd(value)
	}
	return usd(value)
}

// hourlyUSD formats an hourly rate, which needs more precision.
func hourlyUSD(value float64) string {
	return fmt.Sprintf("$%.4f", value)
}

func cores(milli int64) string {
	return fmt.Sprintf("%.2f", float64(milli)/1000)
}

func gib(bytes int64) string {
	return fmt.Sprintf("%.1f GiB", float64(bytes)/gibibyte)
}

func percent(part, whole float64) string {
	if whole <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", 100*part/whole)
}

func when(value time.Time) string {
	return value.Local().Format("2006-01-02 15:04")
}

func printWarnings(stdout io.Writer, warnings []clustermode.Warning) {
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintf(stdout, "\nWarnings (%d)\n", len(warnings))
	for _, warning := range warnings {
		fmt.Fprintf(stdout, "  - %s: %s\n", warning.Resource, warning.Message)
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
