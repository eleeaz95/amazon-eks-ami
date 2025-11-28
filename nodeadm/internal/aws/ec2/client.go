package ec2

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type Client interface {
	GetInterfaceTags(ctx context.Context, mac string) (map[string]string, error)
}

type client struct {
	ec2 *ec2.Client
}

func NewClient(ctx context.Context) (Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}
	return &client{
		ec2: ec2.NewFromConfig(cfg),
	}, nil
}

func (c *client) GetInterfaceTags(ctx context.Context, mac string) (map[string]string, error) {
	output, err := c.ec2.DescribeNetworkInterfaces(ctx, &ec2.DescribeNetworkInterfacesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("mac-address"),
				Values: []string{mac},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe network interfaces: %w", err)
	}

	if len(output.NetworkInterfaces) == 0 {
		return nil, fmt.Errorf("no network interface found with mac %s", mac)
	}

	tags := make(map[string]string)
	for _, tag := range output.NetworkInterfaces[0].TagSet {
		if tag.Key != nil && tag.Value != nil {
			tags[*tag.Key] = *tag.Value
		}
	}
	return tags, nil
}
