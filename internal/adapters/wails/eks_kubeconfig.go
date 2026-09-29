package wails

import (
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// eksTarget is the EKS cluster behind a kubeconfig context, with the AWS
// identity the context already authenticates as.
type eksTarget struct {
	ClusterName string
	Region      string
	Profile     string
	RoleARN     string
}

// eksTargetFor reads the EKS cluster from a context that authenticates with
// `aws eks get-token`, the form `aws eks update-kubeconfig` writes. Other
// authenticators do not name the cluster, region and profile reliably, so
// they are left alone.
func eksTargetFor(config *clientcmdapi.Config, contextName string) (eksTarget, bool) {
	if config == nil {
		return eksTarget{}, false
	}
	context := config.Contexts[contextName]
	if context == nil {
		return eksTarget{}, false
	}
	auth := config.AuthInfos[context.AuthInfo]
	if auth == nil || auth.Exec == nil {
		return eksTarget{}, false
	}
	command := strings.TrimSuffix(strings.ToLower(filepath.Base(auth.Exec.Command)), ".exe")
	if command != "aws" || !containsSequence(auth.Exec.Args, "eks", "get-token") {
		return eksTarget{}, false
	}

	target := eksTarget{
		ClusterName: flagValue(auth.Exec.Args, "--cluster-name"),
		Region:      flagValue(auth.Exec.Args, "--region"),
		Profile:     flagValue(auth.Exec.Args, "--profile"),
		RoleARN:     flagValue(auth.Exec.Args, "--role-arn"),
	}
	for _, variable := range auth.Exec.Env {
		switch variable.Name {
		case "AWS_PROFILE":
			if target.Profile == "" {
				target.Profile = variable.Value
			}
		case "AWS_REGION", "AWS_DEFAULT_REGION":
			if target.Region == "" {
				target.Region = variable.Value
			}
		}
	}
	// update-kubeconfig names the kubeconfig cluster after the EKS ARN.
	if parsed, err := arn.Parse(context.Cluster); err == nil && parsed.Service == "eks" {
		if target.Region == "" {
			target.Region = parsed.Region
		}
		if name, ok := strings.CutPrefix(parsed.Resource, "cluster/"); ok && target.ClusterName == "" {
			target.ClusterName = name
		}
	}
	if target.ClusterName == "" || target.Region == "" {
		return eksTarget{}, false
	}
	return target, true
}

// flagValue reads --flag value or --flag=value.
func flagValue(args []string, flag string) string {
	for index, arg := range args {
		if arg == flag && index+1 < len(args) {
			return args[index+1]
		}
		if value, ok := strings.CutPrefix(arg, flag+"="); ok {
			return value
		}
	}
	return ""
}

func containsSequence(args []string, first, second string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == first && args[index+1] == second {
			return true
		}
	}
	return false
}
