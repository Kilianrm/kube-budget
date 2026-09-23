package wails

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	kubernetesadapter "kube-budget/internal/adapters/kubernetes"
	clustermode "kube-budget/internal/application/cluster"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/yaml"
)

const (
	clusterConnectionTimeout = 10 * time.Second
	clusterCollectionTimeout = 30 * time.Second
)

// ClusterAdapter exposes kubeconfig-backed cluster operations to the dashboard.
type ClusterAdapter struct{}

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
	if strings.TrimSpace(request.Region) == "" {
		return nil, fmt.Errorf("EKS discovery: AWS region is required")
	}

	awsPath, err := exec.LookPath("aws")
	if err != nil {
		return nil, fmt.Errorf("EKS discovery: AWS CLI was not found; install AWS CLI v2 and make sure it is available to the desktop app: %w", err)
	}

	args := []string{"eks", "list-clusters", "--region", strings.TrimSpace(request.Region), "--output", "json"}
	if profile := strings.TrimSpace(request.Profile); profile != "" {
		args = append(args, "--profile", profile)
	}

	command := exec.CommandContext(context.Background(), awsPath, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		details := strings.TrimSpace(string(output))
		if details == "" {
			details = err.Error()
		}
		return nil, fmt.Errorf("EKS discovery: AWS CLI could not list clusters: %s", details)
	}

	var response struct {
		Clusters []string `json:"clusters"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("EKS discovery: decode AWS CLI response: %w", err)
	}
	sort.Strings(response.Clusters)
	return response.Clusters, nil
}

// PrepareEKSConnection creates a kubeconfig context using the user's AWS CLI credentials.
func (adapter *ClusterAdapter) PrepareEKSConnection(request EKSConnectionRequest) (EKSConnectionResult, error) {
	if err := validateEKSConnectionRequest(request); err != nil {
		return EKSConnectionResult{}, err
	}

	awsPath, err := exec.LookPath("aws")
	if err != nil {
		return EKSConnectionResult{}, fmt.Errorf("EKS connection: AWS CLI was not found; install AWS CLI v2 and make sure it is available to the desktop app: %w", err)
	}

	kubeconfigPath, err := expandKubeconfigPath(request.KubeconfigPath)
	if err != nil {
		return EKSConnectionResult{}, err
	}
	if kubeconfigPath == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return EKSConnectionResult{}, fmt.Errorf("EKS connection: resolve kubeconfig path: %w", homeErr)
		}
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	contextName := eksContextName(request.ClusterName, request.Region)
	args := []string{"eks", "update-kubeconfig", "--name", strings.TrimSpace(request.ClusterName), "--region", strings.TrimSpace(request.Region), "--kubeconfig", kubeconfigPath, "--alias", contextName}
	if profile := strings.TrimSpace(request.Profile); profile != "" {
		args = append(args, "--profile", profile)
	}
	if roleARN := strings.TrimSpace(request.RoleARN); roleARN != "" {
		args = append(args, "--role-arn", roleARN)
	}

	command := exec.CommandContext(context.Background(), awsPath, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		details := strings.TrimSpace(string(output))
		if details == "" {
			details = err.Error()
		}
		return EKSConnectionResult{}, fmt.Errorf("EKS connection: AWS CLI could not update kubeconfig: %s", details)
	}

	return EKSConnectionResult{KubeconfigPath: kubeconfigPath, Context: contextName}, nil
}

func validateEKSConnectionRequest(request EKSConnectionRequest) error {
	if strings.TrimSpace(request.ClusterName) == "" {
		return fmt.Errorf("EKS connection: cluster name is required")
	}
	if strings.TrimSpace(request.Region) == "" {
		return fmt.Errorf("EKS connection: AWS region is required")
	}
	return nil
}

func eksContextName(clusterName, region string) string {
	return "eks/" + strings.TrimSpace(clusterName) + "/" + strings.TrimSpace(region)
}

func (adapter *ClusterAdapter) ListKubeconfigContexts(request KubeconfigContextsRequest) ([]KubeconfigContext, error) {
	config, err := loadKubeconfig(request.KubeconfigPath, "")
	if err != nil {
		return nil, err
	}

	contexts := make([]KubeconfigContext, 0, len(config.Contexts))
	for name, context := range config.Contexts {
		cluster, ok := config.Clusters[context.Cluster]
		if !ok {
			continue
		}
		contexts = append(contexts, KubeconfigContext{Name: name, Server: cluster.Server, Cluster: context.Cluster})
	}
	return contexts, nil
}

func (adapter *ClusterAdapter) TestConnection(request ClusterConnectionRequest) (ClusterConnectionResult, error) {
	if strings.TrimSpace(request.Context) == "" {
		return ClusterConnectionResult{}, fmt.Errorf("cluster connection: context is required")
	}

	clientConfig, err := buildClientConfig(request.KubeconfigPath, request.Context)
	if err != nil {
		return ClusterConnectionResult{}, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		return ClusterConnectionResult{}, fmt.Errorf("cluster connection: create client: %w", err)
	}
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return ClusterConnectionResult{}, fmt.Errorf("cluster connection: validate API access: %w", err)
	}

	return ClusterConnectionResult{
		Context:     request.Context,
		Server:      redactServerURL(clientConfig.Host),
		Version:     version.GitVersion,
		ConnectedAt: time.Now().UnixMilli(),
	}, nil
}

func (adapter *ClusterAdapter) GetClusterSnapshot(request ClusterSnapshotRequest) (clustermode.Snapshot, error) {
	if strings.TrimSpace(request.Context) == "" {
		return clustermode.Snapshot{}, fmt.Errorf("cluster snapshot: context is required")
	}

	clientConfig, err := buildClientConfig(request.KubeconfigPath, request.Context)
	if err != nil {
		return clustermode.Snapshot{}, err
	}
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return clustermode.Snapshot{}, fmt.Errorf("cluster snapshot: create Kubernetes client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		return clustermode.Snapshot{}, fmt.Errorf("cluster snapshot: create discovery client: %w", err)
	}
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return clustermode.Snapshot{}, fmt.Errorf("cluster snapshot: discover server version: %w", err)
	}

	namespace := strings.TrimSpace(request.Namespace)
	if strings.EqualFold(namespace, "all namespaces") {
		namespace = ""
	}
	collector := kubernetesadapter.NewCollector(client, clustermode.ClusterInfo{
		Context: request.Context,
		Server:  redactServerURL(clientConfig.Host),
		Version: version.GitVersion,
	}, namespace)
	ctx, cancel := context.WithTimeout(context.Background(), clusterCollectionTimeout)
	defer cancel()

	snapshot, err := clustermode.New(collector).Snapshot(ctx)
	if err != nil {
		return clustermode.Snapshot{}, fmt.Errorf("cluster snapshot: collect resources: %w", err)
	}
	return snapshot, nil
}

// GetWorkloadYAML returns the current, read-only YAML representation of a collected workload.
func (adapter *ClusterAdapter) GetWorkloadYAML(request WorkloadYAMLRequest) (string, error) {
	if strings.TrimSpace(request.Context) == "" {
		return "", fmt.Errorf("workload YAML: context is required")
	}
	if strings.TrimSpace(request.Namespace) == "" || strings.TrimSpace(request.Name) == "" {
		return "", fmt.Errorf("workload YAML: namespace and name are required")
	}

	clientConfig, err := buildClientConfig(request.KubeconfigPath, request.Context)
	if err != nil {
		return "", err
	}
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return "", fmt.Errorf("workload YAML: create Kubernetes client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), clusterCollectionTimeout)
	defer cancel()

	return getWorkloadYAML(ctx, client, request)
}

func getWorkloadYAML(ctx context.Context, client kubernetes.Interface, request WorkloadYAMLRequest) (string, error) {
	var object runtime.Object
	var err error
	switch strings.TrimSpace(request.Kind) {
	case "Deployment":
		object, err = client.AppsV1().Deployments(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
	case "StatefulSet":
		object, err = client.AppsV1().StatefulSets(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
	case "DaemonSet":
		object, err = client.AppsV1().DaemonSets(request.Namespace).Get(ctx, request.Name, metav1.GetOptions{})
	default:
		return "", fmt.Errorf("workload YAML: unsupported workload kind %q", request.Kind)
	}
	if err != nil {
		return "", fmt.Errorf("workload YAML: get %s %s/%s: %w", request.Kind, request.Namespace, request.Name, err)
	}

	return marshalLiveYAML(object, request.Kind)
}

func marshalLiveYAML(object runtime.Object, kind string) (string, error) {
	jsonData, err := json.Marshal(object)
	if err != nil {
		return "", fmt.Errorf("workload YAML: marshal live object: %w", err)
	}
	manifest := make(map[string]any)
	if err := json.Unmarshal(jsonData, &manifest); err != nil {
		return "", fmt.Errorf("workload YAML: decode live object: %w", err)
	}
	manifest["apiVersion"] = "apps/v1"
	manifest["kind"] = kind
	yamlData, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("workload YAML: encode live object: %w", err)
	}
	return string(yamlData), nil
}

func buildClientConfig(path, contextName string) (*rest.Config, error) {
	config, err := loadKubeconfig(path, contextName)
	if err != nil {
		return nil, err
	}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{CurrentContext: contextName}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("cluster connection: build client config: %w", err)
	}
	clientConfig.Timeout = clusterConnectionTimeout
	return clientConfig, nil
}

func loadKubeconfig(path string, contextName string) (*clientcmdapi.Config, error) {
	path, err := expandKubeconfigPath(path)
	if err != nil {
		return nil, err
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if path != "" {
		rules.ExplicitPath = path
	}
	config, err := rules.Load()
	if err != nil {
		return nil, fmt.Errorf("cluster connection: load kubeconfig: %w", err)
	}
	if contextName != "" {
		if _, ok := config.Contexts[contextName]; !ok {
			return nil, fmt.Errorf("cluster connection: context %q was not found", contextName)
		}
	}
	return config, nil
}

func expandKubeconfigPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cluster connection: resolve home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

func redactServerURL(server string) string {
	parsed, err := url.Parse(server)
	if err != nil || parsed.Host == "" {
		return server
	}
	parsed.User = nil
	return parsed.String()
}
