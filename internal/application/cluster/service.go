// Package cluster implements the live cluster snapshot workflow.
package cluster

import (
	"context"
	"sort"
	"time"
)

const SchemaVersion = 4

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
	// MissingRequests marks incomplete requests: some container leaves CPU or
	// memory unrequested. MissingResources says which.
	MissingRequests  bool     `json:"missingRequests"`
	MissingResources []string `json:"missingResources,omitempty"`
}

// Pod is one scheduled or pending pod and the controller that owns it. Pods are
// what nodes actually run, so they link a workload's requests to the price of
// the node that pays for them.
type Pod struct {
	Name             string         `json:"name"`
	Namespace        string         `json:"namespace"`
	NodeName         string         `json:"nodeName,omitempty"`
	Phase            string         `json:"phase"`
	OwnerKind        string         `json:"ownerKind"`
	OwnerName        string         `json:"ownerName"`
	Requests         ResourceValues `json:"requests"`
	MissingRequests  bool           `json:"missingRequests"`
	MissingResources []string       `json:"missingResources,omitempty"`
	// Usage is the pod's observed CPU and memory from metrics-server; nil when
	// metrics are unavailable or the pod has not been sampled yet.
	Usage *ResourceValues `json:"usage,omitempty"`
	// Claims are the persistent volume claims the pod mounts.
	Claims []string `json:"claims,omitempty"`
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
	Schedulable  bool           `json:"schedulable"`
	Role         string         `json:"role"`
	Zone         string         `json:"zone"`
	Region       string         `json:"region"`
	InstanceType string         `json:"instanceType"`
	ProviderID   string         `json:"providerId"`
	Capacity     ResourceValues `json:"capacity"`
	Allocatable  ResourceValues `json:"allocatable"`
	Requests     ResourceValues `json:"requests"`
	// Taints lists the NoSchedule and NoExecute taints as key=value:Effect.
	Taints []string `json:"taints,omitempty"`
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

type ProviderMetadata struct {
	Provider           string      `json:"provider"`
	ClusterName        string      `json:"clusterName"`
	ClusterARN         string      `json:"clusterArn"`
	AccountID          string      `json:"accountId"`
	Region             string      `json:"region"`
	Status             string      `json:"status"`
	KubernetesVersion  string      `json:"kubernetesVersion"`
	PlatformVersion    string      `json:"platformVersion"`
	CreatedAt          int64       `json:"createdAt"`
	EndpointAccess     string      `json:"endpointAccess"`
	VPCID              string      `json:"vpcId"`
	SubnetIDs          []string    `json:"subnetIds"`
	SecurityGroupIDs   []string    `json:"securityGroupIds"`
	AuthenticationMode string      `json:"authenticationMode"`
	NodeGroups         []NodeGroup `json:"nodeGroups"`
	Addons             []Addon     `json:"addons"`
}

type NodeGroup struct {
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	InstanceTypes []string `json:"instanceTypes"`
	CapacityType  string   `json:"capacityType"`
	DesiredSize   int32    `json:"desiredSize"`
	MinSize       int32    `json:"minSize"`
	MaxSize       int32    `json:"maxSize"`
	AmiType       string   `json:"amiType"`
	NodeRole      string   `json:"nodeRole"`
}

type Addon struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	Status         string `json:"status"`
	Health         string `json:"health"`
	ServiceAccount string `json:"serviceAccount"`
}

type Snapshot struct {
	SchemaVersion int                `json:"schemaVersion"`
	CollectedAt   int64              `json:"collectedAt"`
	Cluster       ClusterInfo        `json:"cluster"`
	Summary       Summary            `json:"summary"`
	Nodes         []Node             `json:"nodes"`
	Workloads     []Workload         `json:"workloads"`
	Pods          []Pod              `json:"pods"`
	Resources     []Resource         `json:"resources"`
	Namespaces    []NamespaceSummary `json:"namespaces"`
	Warnings      []Warning          `json:"warnings"`
	Provider      *ProviderMetadata  `json:"provider,omitempty"`
	Platform      Platform           `json:"platform"`
}

type CollectedData struct {
	Cluster            ClusterInfo
	Nodes              []Node
	Workloads          []Workload
	Pods               []Pod
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
		Pods:          append([]Pod{}, data.Pods...),
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
