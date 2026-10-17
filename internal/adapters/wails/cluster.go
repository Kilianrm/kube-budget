package wails

import (
	"sync"
	"time"

	clustermode "kube-cost-workspace/internal/application/cluster"
	"kube-cost-workspace/internal/application/workspace"
)

const (
	// snapshotReuseWindow is how long a collected snapshot answers other
	// requests for the same cluster. The Cluster section and the cost report
	// both ask for one when the app opens; they share one collection.
	snapshotReuseWindow = 10 * time.Second
)

// ClusterAdapter exposes kubeconfig-backed cluster operations to the dashboard.
type ClusterAdapter struct {
	costMutex sync.Mutex
	lastCosts map[string]costCache

	snapshotMutex sync.Mutex
	snapshots     map[ClusterSnapshotRequest]*snapshotCall

	captureMutex sync.Mutex
	lastCaptures map[string]lastCapture
}

// lastCapture is the latest report written to a cluster's spend history.
type lastCapture struct {
	at   time.Time
	rate float64
}

// snapshotCall is one collection, in flight until done is closed.
type snapshotCall struct {
	done        chan struct{}
	snapshot    clustermode.Snapshot
	err         error
	collectedAt time.Time
}

type KubeconfigContextsRequest struct {
	KubeconfigPath string `json:"kubeconfigPath"`
}

type KubeconfigContext struct {
	Name    string `json:"name"`
	Server  string `json:"server"`
	Cluster string `json:"cluster"`
}

type ClusterConnectionRequest struct {
	KubeconfigPath string `json:"kubeconfigPath"`
	Context        string `json:"context"`
}

type EKSConnectionRequest struct {
	KubeconfigPath string `json:"kubeconfigPath"`
	ClusterName    string `json:"clusterName"`
	Region         string `json:"region"`
	Profile        string `json:"profile"`
	RoleARN        string `json:"roleArn"`
}

type EKSConnectionResult struct {
	KubeconfigPath string `json:"kubeconfigPath"`
	Context        string `json:"context"`
}

type EKSClustersRequest struct {
	Region  string `json:"region"`
	Profile string `json:"profile"`
}

type ClusterConnectionResult struct {
	Context     string `json:"context"`
	Server      string `json:"server"`
	Version     string `json:"version"`
	ConnectedAt int64  `json:"connectedAt"`
}

type ClusterSnapshotRequest struct {
	KubeconfigPath string `json:"kubeconfigPath"`
	Context        string `json:"context"`
	Namespace      string `json:"namespace"`
	Provider       string `json:"provider"`
	ClusterName    string `json:"clusterName"`
	Region         string `json:"region"`
	Profile        string `json:"profile"`
	RoleARN        string `json:"roleArn"`
}

type WorkloadYAMLRequest struct {
	KubeconfigPath string `json:"kubeconfigPath"`
	Context        string `json:"context"`
	Kind           string `json:"kind"`
	Namespace      string `json:"namespace"`
	Name           string `json:"name"`
}

func NewClusterAdapter() *ClusterAdapter {
	return &ClusterAdapter{}
}

func (adapter *ClusterAdapter) ListEKSClusters(request EKSClustersRequest) ([]string, error) {
	return workspace.ListEKSClusters(request.Region, request.Profile)
}

// PrepareEKSConnection creates a kubeconfig context using the user's AWS CLI credentials.
func (adapter *ClusterAdapter) PrepareEKSConnection(request EKSConnectionRequest) (EKSConnectionResult, error) {
	kubeconfigPath, contextName, err := workspace.PrepareEKSConnection(workspace.EKSConnection{
		KubeconfigPath: request.KubeconfigPath,
		ClusterName:    request.ClusterName,
		Region:         request.Region,
		Profile:        request.Profile,
		RoleARN:        request.RoleARN,
	})
	if err != nil {
		return EKSConnectionResult{}, err
	}
	return EKSConnectionResult{KubeconfigPath: kubeconfigPath, Context: contextName}, nil
}

func (adapter *ClusterAdapter) ListKubeconfigContexts(request KubeconfigContextsRequest) ([]KubeconfigContext, error) {
	listed, err := workspace.Contexts(request.KubeconfigPath)
	if err != nil {
		return nil, err
	}
	contexts := make([]KubeconfigContext, 0, len(listed))
	for _, context := range listed {
		contexts = append(contexts, KubeconfigContext{Name: context.Name, Server: context.Server, Cluster: context.Cluster})
	}
	return contexts, nil
}

func (adapter *ClusterAdapter) TestConnection(request ClusterConnectionRequest) (ClusterConnectionResult, error) {
	server, err := workspace.Connect(request.KubeconfigPath, request.Context)
	if err != nil {
		return ClusterConnectionResult{}, err
	}
	return ClusterConnectionResult{
		Context:     server.Context,
		Server:      server.Server,
		Version:     server.Version,
		ConnectedAt: time.Now().UnixMilli(),
	}, nil
}

func (adapter *ClusterAdapter) GetClusterSnapshot(request ClusterSnapshotRequest) (clustermode.Snapshot, error) {
	return adapter.sharedSnapshot(request, collectSnapshot, time.Now)
}

// sharedSnapshot joins a collection already in flight for the same request,
// or reuses one finished within snapshotReuseWindow, before collecting again.
func (adapter *ClusterAdapter) sharedSnapshot(request ClusterSnapshotRequest, collect func(ClusterSnapshotRequest) (clustermode.Snapshot, error), now func() time.Time) (clustermode.Snapshot, error) {
	adapter.snapshotMutex.Lock()
	if adapter.snapshots == nil {
		adapter.snapshots = make(map[ClusterSnapshotRequest]*snapshotCall)
	}
	call := adapter.snapshots[request]
	if call != nil {
		select {
		case <-call.done:
			if call.err != nil || now().Sub(call.collectedAt) > snapshotReuseWindow {
				call = nil
			}
		default:
		}
	}
	if call == nil {
		call = &snapshotCall{done: make(chan struct{})}
		adapter.snapshots[request] = call
		go func() {
			call.snapshot, call.err = collect(request)
			call.collectedAt = now()
			close(call.done)
		}()
	}
	adapter.snapshotMutex.Unlock()

	<-call.done
	return call.snapshot, call.err
}

func collectSnapshot(request ClusterSnapshotRequest) (clustermode.Snapshot, error) {
	return workspace.Snapshot(workspace.Connection{
		KubeconfigPath: request.KubeconfigPath,
		Context:        request.Context,
		Namespace:      request.Namespace,
		Provider:       request.Provider,
		ClusterName:    request.ClusterName,
		Region:         request.Region,
		Profile:        request.Profile,
		RoleARN:        request.RoleARN,
	})
}

// GetWorkloadYAML returns the current, read-only YAML representation of a collected workload.
func (adapter *ClusterAdapter) GetWorkloadYAML(request WorkloadYAMLRequest) (string, error) {
	return workspace.WorkloadYAML(request.KubeconfigPath, request.Context, workspace.WorkloadRef{
		Kind:      request.Kind,
		Namespace: request.Namespace,
		Name:      request.Name,
	})
}
