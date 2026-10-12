package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// EKSConnection names an EKS cluster and the AWS identity to reach it with.
type EKSConnection struct {
	KubeconfigPath string
	ClusterName    string
	Region         string
	Profile        string
	RoleARN        string
}

// ListEKSClusters lists the EKS clusters of a region with the AWS CLI.
func ListEKSClusters(region, profile string) ([]string, error) {
	if strings.TrimSpace(region) == "" {
		return nil, fmt.Errorf("EKS discovery: AWS region is required")
	}

	awsPath, err := exec.LookPath("aws")
	if err != nil {
		return nil, fmt.Errorf("EKS discovery: AWS CLI was not found; install AWS CLI v2 and make sure it is on the PATH: %w", err)
	}

	args := []string{"eks", "list-clusters", "--region", strings.TrimSpace(region), "--output", "json"}
	if profile := strings.TrimSpace(profile); profile != "" {
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

// PrepareEKSConnection writes a kubeconfig context for an EKS cluster with the
// user's AWS CLI credentials and returns the kubeconfig path and context.
func PrepareEKSConnection(connection EKSConnection) (string, string, error) {
	if err := validateEKSConnection(connection); err != nil {
		return "", "", err
	}

	awsPath, err := exec.LookPath("aws")
	if err != nil {
		return "", "", fmt.Errorf("EKS connection: AWS CLI was not found; install AWS CLI v2 and make sure it is on the PATH: %w", err)
	}

	kubeconfigPath, err := expandKubeconfigPath(connection.KubeconfigPath)
	if err != nil {
		return "", "", err
	}
	if kubeconfigPath == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return "", "", fmt.Errorf("EKS connection: resolve kubeconfig path: %w", homeErr)
		}
		kubeconfigPath = filepath.Join(home, ".kube", "config")
	}

	contextName := eksContextName(connection.ClusterName, connection.Region)
	args := []string{"eks", "update-kubeconfig", "--name", strings.TrimSpace(connection.ClusterName), "--region", strings.TrimSpace(connection.Region), "--kubeconfig", kubeconfigPath, "--alias", contextName}
	if profile := strings.TrimSpace(connection.Profile); profile != "" {
		args = append(args, "--profile", profile)
	}
	if roleARN := strings.TrimSpace(connection.RoleARN); roleARN != "" {
		args = append(args, "--role-arn", roleARN)
	}

	command := exec.CommandContext(context.Background(), awsPath, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		details := strings.TrimSpace(string(output))
		if details == "" {
			details = err.Error()
		}
		return "", "", fmt.Errorf("EKS connection: AWS CLI could not update kubeconfig: %s", details)
	}

	return kubeconfigPath, contextName, nil
}

func validateEKSConnection(connection EKSConnection) error {
	if strings.TrimSpace(connection.ClusterName) == "" {
		return fmt.Errorf("EKS connection: cluster name is required")
	}
	if strings.TrimSpace(connection.Region) == "" {
		return fmt.Errorf("EKS connection: AWS region is required")
	}
	return nil
}

func eksContextName(clusterName, region string) string {
	return "eks/" + strings.TrimSpace(clusterName) + "/" + strings.TrimSpace(region)
}
