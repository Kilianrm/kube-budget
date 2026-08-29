// Command try_converter is a dev-only manual harness for exercising the converter package; it is not a product entry point.
package main

import (
	"fmt"
	"os"

	"kube-budget/internal/converter"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: try_converter <manifest-path>")
		os.Exit(1)
	}

	manifest, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	workload, err := converter.ConvertDeployment(manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Converted Kubernetes Workload")
	fmt.Println("================================")
	fmt.Printf("Name:      %s\n", workload.Name)
	fmt.Printf("Namespace: %s\n", workload.Namespace)
	fmt.Printf("Replicas:  %d\n", workload.Replicas)
	if workload.MinReplicas != nil && workload.MaxReplicas != nil {
		fmt.Printf("Autoscaling: %d-%d replicas\n", *workload.MinReplicas, *workload.MaxReplicas)
	}
	fmt.Println("Resources:")

	if len(workload.Resources) == 0 {
		fmt.Println("  - none")
		return
	}

	for i, resource := range workload.Resources {
		fmt.Printf("  [%d] %s\n", i+1, resource.Name)
		fmt.Printf("      CPU:      %.2f\n", resource.CPU)
		fmt.Printf("      MemoryGB: %.2f\n", resource.MemoryGB)
		fmt.Printf("      StorageGB: %.2f\n", resource.StorageGB)
		fmt.Printf("      GPU:      %.2f\n", resource.GPU)
	}
}
