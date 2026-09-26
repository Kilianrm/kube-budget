package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"

	clustermode "kube-budget/internal/application/cluster"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type EKSClient struct {
	eks *eks.Client
	sts *sts.Client
}

func NewEKSClient(ctx context.Context, region, profile, roleARN string) (*EKSClient, error) {
	options := []func(*awscfg.LoadOptions) error{awscfg.WithRegion(strings.TrimSpace(region))}
	if strings.TrimSpace(profile) != "" {
		options = append(options, awscfg.WithSharedConfigProfile(strings.TrimSpace(profile)))
	}
	config, err := awscfg.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("AWS provider: load credentials: %w", err)
	}
	if roleARN = strings.TrimSpace(roleARN); roleARN != "" {
		config.Credentials = stscreds.NewAssumeRoleProvider(sts.NewFromConfig(config), roleARN)
	}
	return &EKSClient{eks: eks.NewFromConfig(config), sts: sts.NewFromConfig(config)}, nil
}

func (client *EKSClient) Metadata(ctx context.Context, clusterName, region string) (*clustermode.ProviderMetadata, error) {
	// The account, node groups and add-ons only need the cluster's name, so
	// they are read while the cluster is described: a remote round trip each.
	var group sync.WaitGroup
	var accountID string
	var nodeGroups []clustermode.NodeGroup
	var addons []clustermode.Addon
	run := func(call func()) {
		group.Add(1)
		go func() {
			defer group.Done()
			call()
		}()
	}
	run(func() { accountID = client.accountID(ctx) })
	run(func() { nodeGroups = client.nodeGroups(ctx, clusterName) })
	run(func() { addons = client.addons(ctx, clusterName) })

	details, err := client.eks.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
	group.Wait()
	if err != nil {
		return nil, fmt.Errorf("AWS provider: describe EKS cluster: %w", err)
	}
	cluster := details.Cluster
	if cluster == nil {
		return nil, fmt.Errorf("AWS provider: EKS cluster response was empty")
	}
	metadata := &clustermode.ProviderMetadata{
		Provider: "aws-eks", ClusterName: aws.ToString(cluster.Name), ClusterARN: aws.ToString(cluster.Arn),
		Region: region, AccountID: accountID,
		Status: string(cluster.Status), KubernetesVersion: aws.ToString(cluster.Version),
		PlatformVersion: aws.ToString(cluster.PlatformVersion), NodeGroups: nodeGroups, Addons: addons,
	}
	if cluster.ResourcesVpcConfig != nil {
		metadata.VPCID = aws.ToString(cluster.ResourcesVpcConfig.VpcId)
		metadata.SubnetIDs = append([]string{}, cluster.ResourcesVpcConfig.SubnetIds...)
		metadata.SecurityGroupIDs = append([]string{}, cluster.ResourcesVpcConfig.SecurityGroupIds...)
		switch {
		case cluster.ResourcesVpcConfig.EndpointPublicAccess && cluster.ResourcesVpcConfig.EndpointPrivateAccess:
			metadata.EndpointAccess = "Public and private"
		case cluster.ResourcesVpcConfig.EndpointPrivateAccess:
			metadata.EndpointAccess = "Private"
		default:
			metadata.EndpointAccess = "Public"
		}
	}
	metadata.AuthenticationMode = string(cluster.AccessConfig.AuthenticationMode)
	if cluster.CreatedAt != nil {
		metadata.CreatedAt = cluster.CreatedAt.UnixMilli()
	}
	return metadata, nil
}

func (client *EKSClient) accountID(ctx context.Context) string {
	identity, err := client.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return ""
	}
	parsed, err := arn.Parse(aws.ToString(identity.Arn))
	if err != nil {
		return ""
	}
	return parsed.AccountID
}

// nodeGroups describes every node group at once, keeping the listed order.
func (client *EKSClient) nodeGroups(ctx context.Context, clusterName string) []clustermode.NodeGroup {
	listed, err := client.eks.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)})
	if err != nil {
		return []clustermode.NodeGroup{}
	}
	described := make([]*clustermode.NodeGroup, len(listed.Nodegroups))
	var group sync.WaitGroup
	for index, name := range listed.Nodegroups {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := client.eks.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{ClusterName: aws.String(clusterName), NodegroupName: aws.String(name)})
			if err == nil && result.Nodegroup != nil {
				item := nodeGroup(result.Nodegroup)
				described[index] = &item
			}
		}()
	}
	group.Wait()
	nodeGroups := []clustermode.NodeGroup{}
	for _, item := range described {
		if item != nil {
			nodeGroups = append(nodeGroups, *item)
		}
	}
	return nodeGroups
}

// addons describes every add-on at once, keeping the listed order.
func (client *EKSClient) addons(ctx context.Context, clusterName string) []clustermode.Addon {
	listed, err := client.eks.ListAddons(ctx, &eks.ListAddonsInput{ClusterName: aws.String(clusterName)})
	if err != nil {
		return []clustermode.Addon{}
	}
	described := make([]*clustermode.Addon, len(listed.Addons))
	var group sync.WaitGroup
	for index, name := range listed.Addons {
		group.Add(1)
		go func() {
			defer group.Done()
			result, err := client.eks.DescribeAddon(ctx, &eks.DescribeAddonInput{ClusterName: aws.String(clusterName), AddonName: aws.String(name)})
			if err == nil && result.Addon != nil {
				addon := result.Addon
				described[index] = &clustermode.Addon{Name: aws.ToString(addon.AddonName), Version: aws.ToString(addon.AddonVersion), Status: string(addon.Status), Health: addonHealth(addon.Health.Issues), ServiceAccount: aws.ToString(addon.ServiceAccountRoleArn)}
			}
		}()
	}
	group.Wait()
	addons := []clustermode.Addon{}
	for _, item := range described {
		if item != nil {
			addons = append(addons, *item)
		}
	}
	return addons
}

func nodeGroup(group *ekstypes.Nodegroup) clustermode.NodeGroup {
	result := clustermode.NodeGroup{Name: aws.ToString(group.NodegroupName), Status: string(group.Status), InstanceTypes: append([]string{}, group.InstanceTypes...), CapacityType: string(group.CapacityType), AmiType: string(group.AmiType), NodeRole: aws.ToString(group.NodeRole)}
	if group.ScalingConfig != nil {
		result.DesiredSize = int32Value(group.ScalingConfig.DesiredSize)
		result.MinSize = int32Value(group.ScalingConfig.MinSize)
		result.MaxSize = int32Value(group.ScalingConfig.MaxSize)
	}
	return result
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func addonHealth(issues []ekstypes.AddonIssue) string {
	if len(issues) == 0 {
		return "Healthy"
	}
	return fmt.Sprintf("%d issue(s)", len(issues))
}
