package wails

import (
	"testing"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func kubeconfigWith(cluster string, exec *clientcmdapi.ExecConfig) *clientcmdapi.Config {
	return &clientcmdapi.Config{
		Contexts:  map[string]*clientcmdapi.Context{"ctx": {Cluster: cluster, AuthInfo: "user"}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"user": {Exec: exec}},
	}
}

func TestEKSTargetIsReadFromTheAWSGetTokenExec(t *testing.T) {
	// what `aws eks update-kubeconfig` writes
	config := kubeconfigWith("arn:aws:eks:us-east-1:111122223333:cluster/kube-budget-showcase", &clientcmdapi.ExecConfig{
		Command: "aws",
		Args:    []string{"--region", "us-east-1", "eks", "get-token", "--cluster-name", "kube-budget-showcase", "--output", "json"},
		Env:     []clientcmdapi.ExecEnvVar{{Name: "AWS_PROFILE", Value: "default"}},
	})
	target, ok := eksTargetFor(config, "ctx")
	if !ok || target != (eksTarget{ClusterName: "kube-budget-showcase", Region: "us-east-1", Profile: "default"}) {
		t.Errorf("eksTargetFor() = %+v, %v; want the showcase with the default profile", target, ok)
	}

	// flags written with =, a role, and the region only in the ARN
	config = kubeconfigWith("arn:aws:eks:eu-west-1:111122223333:cluster/shop", &clientcmdapi.ExecConfig{
		Command: "/usr/local/bin/aws",
		Args:    []string{"eks", "get-token", "--cluster-name=shop", "--role-arn=arn:aws:iam::111122223333:role/viewer", "--profile=ops"},
	})
	target, ok = eksTargetFor(config, "ctx")
	if !ok || target != (eksTarget{ClusterName: "shop", Region: "eu-west-1", Profile: "ops", RoleARN: "arn:aws:iam::111122223333:role/viewer"}) {
		t.Errorf("eksTargetFor() = %+v, %v; want shop in eu-west-1 as ops assuming viewer", target, ok)
	}
}

func TestOtherAuthenticatorsAreLeftAlone(t *testing.T) {
	for name, exec := range map[string]*clientcmdapi.ExecConfig{
		"no exec":           nil,
		"iam authenticator": {Command: "aws-iam-authenticator", Args: []string{"token", "-i", "shop"}},
		"gke":               {Command: "gke-gcloud-auth-plugin"},
		"aws without eks":   {Command: "aws", Args: []string{"sts", "get-caller-identity"}},
	} {
		if target, ok := eksTargetFor(kubeconfigWith("arn:aws:eks:us-east-1:111122223333:cluster/shop", exec), "ctx"); ok {
			t.Errorf("%s: eksTargetFor() = %+v, want nothing", name, target)
		}
	}
	if _, ok := eksTargetFor(kubeconfigWith("shop", &clientcmdapi.ExecConfig{Command: "aws", Args: []string{"eks", "get-token", "--cluster-name", "shop"}}), "ctx"); ok {
		t.Error("eksTargetFor() without any region, want nothing")
	}
	if _, ok := eksTargetFor(kubeconfigWith("x", nil), "missing"); ok {
		t.Error("eksTargetFor() for an unknown context, want nothing")
	}
}
