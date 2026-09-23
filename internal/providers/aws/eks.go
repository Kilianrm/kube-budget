package aws

import (
	"context"
	"fmt"
	"strings"

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
	details, err := client.eks.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
	if err != nil {
		return nil, fmt.Errorf("AWS provider: describe EKS cluster: %w", err)
	}
	cluster := details.Cluster
	if cluster == nil {
		return nil, fmt.Errorf("AWS provider: EKS cluster response was empty")
	}
	metadata := &clustermode.ProviderMetadata{
		Provider: "aws-eks", ClusterName: aws.ToString(cluster.Name), ClusterARN: aws.ToString(cluster.Arn),
		Region: region,
		Status: string(cluster.Status), KubernetesVersion: aws.ToString(cluster.Version),
		PlatformVersion: aws.ToString(cluster.PlatformVersion), NodeGroups: []clustermode.NodeGroup{}, Addons: []clustermode.Addon{},
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
	if identity, identityErr := client.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}); identityErr == nil {
		if parsed, parseErr := arn.Parse(aws.ToString(identity.Arn)); parseErr == nil {
			metadata.AccountID = parsed.AccountID
		}
	}
	if nodegroups, listErr := client.eks.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: cluster.Name}); listErr == nil {
		for _, name := range nodegroups.Nodegroups {
			if nodegroup, describeErr := client.eks.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{ClusterName: cluster.Name, NodegroupName: aws.String(name)}); describeErr == nil && nodegroup.Nodegroup != nil {
				metadata.NodeGroups = append(metadata.NodeGroups, nodeGroup(nodegroup.Nodegroup))
			}
		}
	}
	if addons, listErr := client.eks.ListAddons(ctx, &eks.ListAddonsInput{ClusterName: cluster.Name}); listErr == nil {
		for _, name := range addons.Addons {
			if addon, describeErr := client.eks.DescribeAddon(ctx, &eks.DescribeAddonInput{ClusterName: cluster.Name, AddonName: aws.String(name)}); describeErr == nil && addon.Addon != nil {
				metadata.Addons = append(metadata.Addons, clustermode.Addon{Name: aws.ToString(addon.Addon.AddonName), Version: aws.ToString(addon.Addon.AddonVersion), Status: string(addon.Addon.Status), Health: addonHealth(addon.Addon.Health.Issues), ServiceAccount: aws.ToString(addon.Addon.ServiceAccountRoleArn)})
			}
		}
	}
	return metadata, nil
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
