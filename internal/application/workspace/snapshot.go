package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	kubernetesadapter "kube-cost-workspace/internal/adapters/kubernetes"
	clustermode "kube-cost-workspace/internal/application/cluster"
	awsprovider "kube-cost-workspace/internal/providers/aws"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

// Snapshot collects a read-only snapshot of a cluster: its Kubernetes
// resources, the cloud provider's metadata when it runs on EKS, and the
// platform that decides which price list it uses.
func Snapshot(connection Connection) (clustermode.Snapshot, error) {
	if strings.TrimSpace(connection.Context) == "" {
		return clustermode.Snapshot{}, fmt.Errorf("cluster snapshot: context is required")
	}

	clientConfig, err := buildClientConfig(connection.KubeconfigPath, connection.Context)
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

	collector := kubernetesadapter.NewCollector(client, clustermode.ClusterInfo{
		Context: connection.Context,
		Server:  redactServerURL(clientConfig.Host),
		Version: version.GitVersion,
	}, CollectionNamespace(connection.Namespace))
	ctx, cancel := collectionContext()
	defer cancel()

	// The cloud provider's metadata does not depend on the Kubernetes API, so
	// both are read at the same time. An EKS connection names the cluster; a
	// kubeconfig context that authenticates with `aws eks get-token` names it
	// too, with the AWS identity it already uses.
	target, fromKubeconfig := eksTarget{ClusterName: connection.ClusterName, Region: connection.Region, Profile: connection.Profile, RoleARN: connection.RoleARN}, false
	if !strings.EqualFold(strings.TrimSpace(connection.Provider), "aws-eks") {
		target, fromKubeconfig = kubeconfigEKSTarget(connection.KubeconfigPath, connection.Context)
	}
	var metadata *clustermode.ProviderMetadata
	var metadataErr error
	var metadataDone sync.WaitGroup
	if target.ClusterName != "" {
		metadataDone.Add(1)
		go func() {
			defer metadataDone.Done()
			providerClient, err := awsprovider.NewEKSClient(ctx, target.Region, target.Profile, target.RoleARN)
			if err != nil {
				metadataErr = err
				return
			}
			metadata, metadataErr = providerClient.Metadata(ctx, target.ClusterName, target.Region)
		}()
	}

	snapshot, err := clustermode.New(collector).Snapshot(ctx)
	metadataDone.Wait()
	if err != nil {
		return clustermode.Snapshot{}, fmt.Errorf("cluster snapshot: collect resources: %w", err)
	}
	if metadataErr != nil {
		message := metadataErr.Error()
		if fromKubeconfig {
			message = fmt.Sprintf("read EKS cluster %s with the kubeconfig's AWS identity: %s. Node groups and spot come from node labels instead, and node-count changes have no command.", target.ClusterName, message)
		}
		snapshot.Warnings = append(snapshot.Warnings, clustermode.Warning{Resource: "AWS provider", Message: message})
	} else if metadata != nil {
		snapshot.Provider = metadata
	}
	snapshot.Platform = clusterPlatform(snapshot.Nodes, connection.Provider, connection.Region)
	return snapshot, nil
}

// WorkloadRef names one live workload.
type WorkloadRef struct {
	Kind      string
	Namespace string
	Name      string
}

// WorkloadYAML returns the current, read-only YAML of a live workload.
func WorkloadYAML(kubeconfigPath, contextName string, workload WorkloadRef) (string, error) {
	if strings.TrimSpace(contextName) == "" {
		return "", fmt.Errorf("workload YAML: context is required")
	}
	if strings.TrimSpace(workload.Namespace) == "" || strings.TrimSpace(workload.Name) == "" {
		return "", fmt.Errorf("workload YAML: namespace and name are required")
	}

	clientConfig, err := buildClientConfig(kubeconfigPath, contextName)
	if err != nil {
		return "", err
	}
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return "", fmt.Errorf("workload YAML: create Kubernetes client: %w", err)
	}
	ctx, cancel := collectionContext()
	defer cancel()

	return getWorkloadYAML(ctx, client, workload)
}

func getWorkloadYAML(ctx context.Context, client kubernetes.Interface, workload WorkloadRef) (string, error) {
	var object runtime.Object
	var err error
	switch strings.TrimSpace(workload.Kind) {
	case "Deployment":
		object, err = client.AppsV1().Deployments(workload.Namespace).Get(ctx, workload.Name, metav1.GetOptions{})
	case "StatefulSet":
		object, err = client.AppsV1().StatefulSets(workload.Namespace).Get(ctx, workload.Name, metav1.GetOptions{})
	case "DaemonSet":
		object, err = client.AppsV1().DaemonSets(workload.Namespace).Get(ctx, workload.Name, metav1.GetOptions{})
	default:
		return "", fmt.Errorf("workload YAML: unsupported workload kind %q", workload.Kind)
	}
	if err != nil {
		return "", fmt.Errorf("workload YAML: get %s %s/%s: %w", workload.Kind, workload.Namespace, workload.Name, err)
	}

	return marshalLiveYAML(object, workload.Kind)
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
