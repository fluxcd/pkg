//go:build integration
// +build integration

/*
Copyright 2026 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package integration

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/smithy-go"
)

// preDestroyCleanupAWS tears down AWS resources that frequently leave
// dangling ENIs in the VPC and cause `terraform destroy` to fail with
// DependencyViolation on security group deletion.
//
// Sequence:
//  1. Delete EKS node groups and wait for termination (frees CNI pod ENIs).
//  2. Delete the EKS cluster and wait (releases control-plane ENIs).
//  3. Delete any classic/v2 load balancers in the VPC (frees ELB ENIs).
//  4. Sweep remaining non-NAT ENIs in the VPC with retries.
//
// All steps are idempotent: missing resources are treated as success.
func preDestroyCleanupAWS(ctx context.Context, clusterName, region string) {
	log.Printf("AWS pre-destroy cleanup: cluster=%s region=%s", clusterName, region)

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		log.Printf("AWS pre-destroy: load config: %v", err)
		return
	}
	eksClient := eks.NewFromConfig(cfg)
	ec2Client := ec2.NewFromConfig(cfg)
	elbClient := elasticloadbalancing.NewFromConfig(cfg)
	elbv2Client := elasticloadbalancingv2.NewFromConfig(cfg)

	vpcID := eksClusterVPC(ctx, eksClient, clusterName)

	deleteEKSNodeGroups(ctx, eksClient, clusterName)
	deleteEKSCluster(ctx, eksClient, clusterName)

	if vpcID == "" {
		return
	}

	deleteLoadBalancers(ctx, elbClient, elbv2Client, vpcID)
	sweepENIs(ctx, ec2Client, vpcID)
}

func eksClusterVPC(ctx context.Context, client *eks.Client, name string) string {
	out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
	if err != nil {
		if isNotFound(err) {
			log.Printf("AWS pre-destroy: cluster %s already gone", name)
		} else {
			log.Printf("AWS pre-destroy: describe cluster: %v", err)
		}
		return ""
	}
	if out.Cluster == nil || out.Cluster.ResourcesVpcConfig == nil {
		return ""
	}
	return aws.ToString(out.Cluster.ResourcesVpcConfig.VpcId)
}

func deleteEKSNodeGroups(ctx context.Context, client *eks.Client, clusterName string) {
	out, err := client.ListNodegroups(ctx, &eks.ListNodegroupsInput{ClusterName: aws.String(clusterName)})
	if err != nil {
		if !isNotFound(err) {
			log.Printf("AWS pre-destroy: list nodegroups: %v", err)
		}
		return
	}
	for _, ng := range out.Nodegroups {
		log.Printf("AWS pre-destroy: deleting nodegroup %s", ng)
		_, err := client.DeleteNodegroup(ctx, &eks.DeleteNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(ng),
		})
		if err != nil && !isNotFound(err) {
			log.Printf("AWS pre-destroy: delete nodegroup %s: %v", ng, err)
		}
	}
	waiter := eks.NewNodegroupDeletedWaiter(client)
	for _, ng := range out.Nodegroups {
		if err := waiter.Wait(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   aws.String(clusterName),
			NodegroupName: aws.String(ng),
		}, 15*time.Minute); err != nil {
			log.Printf("AWS pre-destroy: wait nodegroup %s deleted: %v", ng, err)
		}
	}
}

func deleteEKSCluster(ctx context.Context, client *eks.Client, name string) {
	log.Printf("AWS pre-destroy: deleting cluster %s", name)
	_, err := client.DeleteCluster(ctx, &eks.DeleteClusterInput{Name: aws.String(name)})
	if err != nil && !isNotFound(err) {
		log.Printf("AWS pre-destroy: delete cluster: %v", err)
	}
	waiter := eks.NewClusterDeletedWaiter(client)
	if err := waiter.Wait(ctx, &eks.DescribeClusterInput{Name: aws.String(name)}, 15*time.Minute); err != nil {
		log.Printf("AWS pre-destroy: wait cluster deleted: %v", err)
	}
}

func deleteLoadBalancers(
	ctx context.Context,
	elbClient *elasticloadbalancing.Client,
	elbv2Client *elasticloadbalancingv2.Client,
	vpcID string,
) {
	// Classic ELBs.
	clp := elasticloadbalancing.NewDescribeLoadBalancersPaginator(elbClient, &elasticloadbalancing.DescribeLoadBalancersInput{})
	for clp.HasMorePages() {
		page, err := clp.NextPage(ctx)
		if err != nil {
			log.Printf("AWS pre-destroy: describe classic ELBs: %v", err)
			break
		}
		for _, lb := range page.LoadBalancerDescriptions {
			if aws.ToString(lb.VPCId) != vpcID {
				continue
			}
			name := aws.ToString(lb.LoadBalancerName)
			log.Printf("AWS pre-destroy: deleting classic ELB %s", name)
			_, err := elbClient.DeleteLoadBalancer(ctx, &elasticloadbalancing.DeleteLoadBalancerInput{
				LoadBalancerName: aws.String(name),
			})
			if err != nil {
				log.Printf("AWS pre-destroy: delete classic ELB %s: %v", name, err)
			}
		}
	}

	// ELBv2 (ALB/NLB).
	v2p := elasticloadbalancingv2.NewDescribeLoadBalancersPaginator(elbv2Client, &elasticloadbalancingv2.DescribeLoadBalancersInput{})
	for v2p.HasMorePages() {
		page, err := v2p.NextPage(ctx)
		if err != nil {
			log.Printf("AWS pre-destroy: describe ELBv2s: %v", err)
			break
		}
		for _, lb := range page.LoadBalancers {
			if aws.ToString(lb.VpcId) != vpcID {
				continue
			}
			arn := aws.ToString(lb.LoadBalancerArn)
			log.Printf("AWS pre-destroy: deleting ELBv2 %s", arn)
			_, err := elbv2Client.DeleteLoadBalancer(ctx, &elasticloadbalancingv2.DeleteLoadBalancerInput{
				LoadBalancerArn: aws.String(arn),
			})
			if err != nil {
				log.Printf("AWS pre-destroy: delete ELBv2 %s: %v", arn, err)
			}
		}
	}
}

func sweepENIs(ctx context.Context, client *ec2.Client, vpcID string) {
	const maxAttempts = 30
	for i := 1; i <= maxAttempts; i++ {
		out, err := client.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
			Filters: []ec2types.Filter{{
				Name:   aws.String("vpc-id"),
				Values: []string{vpcID},
			}},
		})
		if err != nil {
			log.Printf("AWS pre-destroy: describe ENIs: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		var pending []ec2types.NetworkInterface
		for _, eni := range out.NetworkInterfaces {
			// NAT gateway ENIs are deleted by the VPC module when it
			// destroys the NAT gateway.
			if eni.InterfaceType == ec2types.NetworkInterfaceTypeNatGateway {
				continue
			}
			// Skip AWS-service-managed attachments (Lambda, VPC endpoint,
			// transit gateway, etc.). The attachment id prefix `ela-attach-`
			// indicates we cannot detach or delete it; AWS reaps these when
			// the owning service is removed.
			if eni.Attachment != nil && eni.Attachment.AttachmentId != nil &&
				strings.HasPrefix(aws.ToString(eni.Attachment.AttachmentId), "ela-attach-") {
				continue
			}
			pending = append(pending, eni)
		}
		if len(pending) == 0 {
			log.Printf("AWS pre-destroy: ENI sweep clean (attempt %d)", i)
			return
		}
		log.Printf("AWS pre-destroy: ENI sweep attempt %d, %d remaining", i, len(pending))

		for _, eni := range pending {
			eniID := aws.ToString(eni.NetworkInterfaceId)
			if eni.Attachment != nil && eni.Attachment.AttachmentId != nil {
				_, err := client.DetachNetworkInterface(ctx, &ec2.DetachNetworkInterfaceInput{
					AttachmentId: eni.Attachment.AttachmentId,
					Force:        aws.Bool(true),
				})
				if err != nil {
					log.Printf("AWS pre-destroy: detach %s: %v", eniID, err)
				}
			}
			_, err := client.DeleteNetworkInterface(ctx, &ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: aws.String(eniID),
			})
			if err != nil {
				log.Printf("AWS pre-destroy: delete %s: %v", eniID, err)
			}
		}
		time.Sleep(10 * time.Second)
	}
	log.Printf("AWS pre-destroy: ENI sweep timed out after %d attempts", maxAttempts)
}

func isNotFound(err error) bool {
	var ekNf *ekstypes.ResourceNotFoundException
	if errors.As(err, &ekNf) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return code == "ResourceNotFoundException" ||
			code == "InvalidNetworkInterfaceID.NotFound" ||
			code == "LoadBalancerNotFound"
	}
	return false
}
