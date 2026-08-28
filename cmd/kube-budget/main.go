package main

import (
	"fmt"
	"log"

	"kube-budget/core/engine"
	"kube-budget/core/pricing"
	"kube-budget/core/providers/aws"
)

func main() {
	// Derive pricing configuration for an AWS EC2 instance type (e.g., m6i.large).
	cfg, err := aws.NewPriceConfig("m6i.large")
	if err != nil {
		log.Fatal(err)
	}
	e := engine.New(cfg)

	workload := engine.Workload{
		Name: "example-workload",
		Resources: []pricing.Resource{
			{Name: "app-container", CPU: 0.5, MemoryGB: 1, StorageGB: 10},
			{Name: "sidecar", CPU: 0.1, MemoryGB: 0.25, StorageGB: 1},
		},
	}

	result := e.Estimate(workload)

	fmt.Printf("Workload: %s\n", workload.Name)
	fmt.Printf("Total: %.2f %s\n", result.Total, result.Currency)
	for name, cost := range result.PerResource {
		fmt.Printf("  %s: %.2f %s\n", name, cost, result.Currency)
	}
}
