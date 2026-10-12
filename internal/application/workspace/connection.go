// Package workspace implements the cluster workflows every entry point shares:
// connecting with the user's kubeconfig, collecting a cluster snapshot,
// pricing it, recording its spend and simulating a manifest on it. The
// desktop adapter and the CLI translate their own requests into these calls;
// neither implements cluster or cost behavior of its own.
package workspace

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	clusterConnectionTimeout = 10 * time.Second
	clusterCollectionTimeout = 30 * time.Second
)

// Connection names a cluster through the user's kubeconfig. Provider is
// "aws-eks" for a connection made through EKS, which then names the cluster,
// region and AWS identity itself; any other connection reads them from the
// kubeconfig context when it authenticates with `aws eks get-token`.
type Connection struct {
	KubeconfigPath string
	Context        string
	// Namespace limits the collection; empty means every namespace.
	Namespace   string
	Provider    string
	ClusterName string
	Region      string
	Profile     string
	RoleARN     string
}

// KubeconfigContext is one context of a kubeconfig.
type KubeconfigContext struct {
	Name    string `json:"name"`
	Server  string `json:"server"`
	Cluster string `json:"cluster"`
	Current bool   `json:"current"`
}

// ServerInfo is the API server a connection reached.
type ServerInfo struct {
	Context string
	Server  string
	Version string
}

// Contexts lists the contexts of a kubeconfig, sorted by name. An empty path
// uses the default kubeconfig loading rules (KUBECONFIG, then ~/.kube/config).
func Contexts(kubeconfigPath string) ([]KubeconfigContext, error) {
	config, err := loadKubeconfig(kubeconfigPath, "")
	if err != nil {
		return nil, err
	}

	contexts := make([]KubeconfigContext, 0, len(config.Contexts))
	for name, context := range config.Contexts {
		cluster, ok := config.Clusters[context.Cluster]
		if !ok {
			continue
		}
		contexts = append(contexts, KubeconfigContext{Name: name, Server: redactServerURL(cluster.Server), Cluster: context.Cluster, Current: name == config.CurrentContext})
	}
	sort.Slice(contexts, func(i, j int) bool { return contexts[i].Name < contexts[j].Name })
	return contexts, nil
}

// CurrentContext is the kubeconfig's current context, the one kubectl uses.
func CurrentContext(kubeconfigPath string) (string, error) {
	config, err := loadKubeconfig(kubeconfigPath, "")
	if err != nil {
		return "", err
	}
	if config.CurrentContext == "" {
		return "", fmt.Errorf("cluster connection: the kubeconfig has no current context; choose one")
	}
	return config.CurrentContext, nil
}

// Connect checks that a context reaches its API server.
func Connect(kubeconfigPath, contextName string) (ServerInfo, error) {
	if strings.TrimSpace(contextName) == "" {
		return ServerInfo{}, fmt.Errorf("cluster connection: context is required")
	}

	clientConfig, err := buildClientConfig(kubeconfigPath, contextName)
	if err != nil {
		return ServerInfo{}, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(clientConfig)
	if err != nil {
		return ServerInfo{}, fmt.Errorf("cluster connection: create client: %w", err)
	}
	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return ServerInfo{}, fmt.Errorf("cluster connection: validate API access: %w", err)
	}

	return ServerInfo{Context: contextName, Server: redactServerURL(clientConfig.Host), Version: version.GitVersion}, nil
}

// CollectionNamespace maps the UI's "All namespaces" choice onto the empty
// namespace the Kubernetes client uses for a cluster-wide list.
func CollectionNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if strings.EqualFold(namespace, "all namespaces") {
		return ""
	}
	return namespace
}

// kubeconfigEKSTarget reads the EKS cluster behind a kubeconfig context, if
// the context authenticates with `aws eks get-token`.
func kubeconfigEKSTarget(path, contextName string) (eksTarget, bool) {
	config, err := loadKubeconfig(path, contextName)
	if err != nil {
		return eksTarget{}, false
	}
	return eksTargetFor(config, contextName)
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

// collectionContext bounds one cluster collection.
func collectionContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), clusterCollectionTimeout)
}
