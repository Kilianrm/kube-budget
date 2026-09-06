package wails

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const clusterConnectionTimeout = 10 * time.Second

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

type ClusterConnectionResult struct {
	Context     string `json:"context"`
	Server      string `json:"server"`
	Version     string `json:"version"`
	ConnectedAt int64  `json:"connectedAt"`
}

func NewClusterAdapter() *ClusterAdapter {
	return &ClusterAdapter{}
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

	config, err := loadKubeconfig(request.KubeconfigPath, request.Context)
	if err != nil {
		return ClusterConnectionResult{}, err
	}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{CurrentContext: request.Context}).ClientConfig()
	if err != nil {
		return ClusterConnectionResult{}, fmt.Errorf("cluster connection: build client config: %w", err)
	}
	clientConfig.Timeout = clusterConnectionTimeout

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
