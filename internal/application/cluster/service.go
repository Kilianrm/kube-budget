// Package cluster implements the live cluster snapshot workflow.
package cluster

import (
	"context"
	"sort"
	"time"
)

const SchemaVersion = 2

type ResourceValues struct {
	CPUMilli     int64 `json:"cpuMilli"`
	MemoryBytes  int64 `json:"memoryBytes"`
	StorageBytes int64 `json:"storageBytes"`
	GPUUnits     int64 `json:"gpuUnits"`
}

type ClusterInfo struct {
	Context string `json:"context"`
	Server  string `json:"server"`
	Version string `json:"version"`
}

type Container struct {
	Name     string         `json:"name"`
	Requests ResourceValues `json:"requests"`
}

type Workload struct {
	UID             string         `json:"uid"`
	Kind            string         `json:"kind"`
	Name            string         `json:"name"`
	Namespace       string         `json:"namespace"`
	DesiredReplicas int32          `json:"desiredReplicas"`
	ReadyReplicas   int32          `json:"readyReplicas"`
	Containers      []Container    `json:"containers"`
	Requests        ResourceValues `json:"requests"`
	MissingRequests bool           `json:"missingRequests"`
}

type Resource struct {
	UID        string            `json:"uid"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Category   string            `json:"category"`
	Status     string            `json:"status"`
	Attributes map[string]string `json:"attributes"`
	Requests   ResourceValues    `json:"requests"`
}

type Node struct {
	UID          string         `json:"uid"`
	Name         string         `json:"name"`
	Ready        bool           `json:"ready"`
	Role         string         `json:"role"`
	Zone         string         `json:"zone"`
	Region       string         `json:"region"`
	InstanceType string         `json:"instanceType"`
	ProviderID   string         `json:"providerId"`
	Capacity     ResourceValues `json:"capacity"`
	Allocatable  ResourceValues `json:"allocatable"`
	Requests     ResourceValues `json:"requests"`
}

type NamespaceSummary struct {
	Name            string         `json:"name"`
	WorkloadCount   int            `json:"workloadCount"`
	PodCount        int            `json:"podCount"`
	MissingRequests int            `json:"missingRequests"`
	Requests        ResourceValues `json:"requests"`
}

type Summary struct {
	NodeCount               int            `json:"nodeCount"`
	ReadyNodeCount          int            `json:"readyNodeCount"`
	WorkloadCount           int            `json:"workloadCount"`
	ResourceCount           int            `json:"resourceCount"`
	NamespaceCount          int            `json:"namespaceCount"`
	MissingRequestWorkloads int            `json:"missingRequestWorkloads"`
	Requests                ResourceValues `json:"requests"`
	Allocatable             ResourceValues `json:"allocatable"`
}

type Warning struct {
	Resource string `json:"resource"`
	Message  string `json:"message"`
}

type Snapshot struct {
	SchemaVersion int                `json:"schemaVersion"`
	CollectedAt   int64              `json:"collectedAt"`
	Cluster       ClusterInfo        `json:"cluster"`
	Summary       Summary            `json:"summary"`
	Nodes         []Node             `json:"nodes"`
	Workloads     []Workload         `json:"workloads"`
	Resources     []Resource         `json:"resources"`
	Namespaces    []NamespaceSummary `json:"namespaces"`
	Warnings      []Warning          `json:"warnings"`
}

type CollectedData struct {
	Cluster            ClusterInfo
	Nodes              []Node
	Workloads          []Workload
	Resources          []Resource
	NamespacePodCounts map[string]int
	Warnings           []Warning
}

type Collector interface {
	Collect(ctx context.Context) (CollectedData, error)
}

type Service struct {
	collector Collector
	now       func() time.Time
}

func New(collector Collector) *Service {
	return &Service{collector: collector, now: time.Now}
}

func (service *Service) Snapshot(ctx context.Context) (Snapshot, error) {
	data, err := service.collector.Collect(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	result := Snapshot{
		SchemaVersion: SchemaVersion,
		CollectedAt:   service.now().UnixMilli(),
		Cluster:       data.Cluster,
		Nodes:         append([]Node{}, data.Nodes...),
		Workloads:     append([]Workload{}, data.Workloads...),
		Resources:     append([]Resource{}, data.Resources...),
		Warnings:      append([]Warning{}, data.Warnings...),
	}

	namespaces := make(map[string]*NamespaceSummary)
	for name, count := range data.NamespacePodCounts {
		namespaces[name] = &NamespaceSummary{Name: name, PodCount: count}
	}
	for _, workload := range data.Workloads {
		namespace := namespaces[workload.Namespace]
		if namespace == nil {
			namespace = &NamespaceSummary{Name: workload.Namespace}
			namespaces[workload.Namespace] = namespace
		}
		namespace.WorkloadCount++
		addResources(&namespace.Requests, workload.Requests)
		addResources(&result.Summary.Requests, workload.Requests)
		if workload.MissingRequests {
			namespace.MissingRequests++
			result.Summary.MissingRequestWorkloads++
		}
	}
	for _, node := range data.Nodes {
		if node.Ready {
			result.Summary.ReadyNodeCount++
		}
		addResources(&result.Summary.Allocatable, node.Allocatable)
	}

	result.Namespaces = make([]NamespaceSummary, 0, len(namespaces))
	for _, namespace := range namespaces {
		result.Namespaces = append(result.Namespaces, *namespace)
	}
	sort.Slice(result.Namespaces, func(left, right int) bool {
		return result.Namespaces[left].Name < result.Namespaces[right].Name
	})
	result.Summary.NodeCount = len(result.Nodes)
	result.Summary.WorkloadCount = len(result.Workloads)
	result.Summary.ResourceCount = len(result.Resources)
	result.Summary.NamespaceCount = len(result.Namespaces)

	return result, nil
}

func addResources(total *ResourceValues, value ResourceValues) {
	total.CPUMilli += value.CPUMilli
	total.MemoryBytes += value.MemoryBytes
	total.StorageBytes += value.StorageBytes
	total.GPUUnits += value.GPUUnits
}
