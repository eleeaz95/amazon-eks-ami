package udev

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/aws/ec2"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/util"
	"go.uber.org/zap"
)

type NetworkInterfaceBroker interface {
	ManagerFor(interfaceName string) (string, error)
}

const NetworkManagerCacheDir = "/etc/eks/nodeadm/udev-net-manager"

type fsBroker struct {
	cache      util.FSCache
	ec2Client  ec2.Client
	statFunc   func(string) (os.FileInfo, error)
	macFetcher func(string) (string, error)
}

func NewFSBroker(instanceID string) *fsBroker {
	return &fsBroker{
		cache:      util.NewFSCache(filepath.Join(NetworkManagerCacheDir, instanceID)),
		statFunc:   os.Stat,
		macFetcher: getInterfaceMAC,
	}
}

func (b *fsBroker) getEC2Client() (ec2.Client, error) {
	if b.ec2Client != nil {
		return b.ec2Client, nil
	}
	// we lazily initialize the client because we don't want to create it if we
	// don't need it.
	client, err := ec2.NewClient(context.TODO())
	if err != nil {
		return nil, err
	}
	b.ec2Client = client
	return b.ec2Client, nil
}

func (b *fsBroker) determineManager(interfaceName string) (string, error) {
	// this code checks whether cloud-init has finished booting the node, which
	// is indicative of most user-controlled actions being completed. it's not
	// perfect but it works under the basic assumptions.
	const cloudInitBootResultPath = "/run/cloud-init/result.json"
	if _, err := b.statFunc(cloudInitBootResultPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ManagerSystemd, nil
		}
		return "", err
	}

	// if cloud-init is done, we check if the interface has a specific tag that
	// tells us to manage it.
	ec2Client, err := b.getEC2Client()
	if err != nil {
		zap.L().Warn("failed to create ec2 client to check for interface tags", zap.Error(err))
		return ManagerCNI, nil
	}

	mac, err := b.macFetcher(interfaceName)
	if err != nil {
		zap.L().Warn("failed to get interface mac to check for interface tags", zap.Error(err))
		return ManagerCNI, nil
	}

	tags, err := ec2Client.GetInterfaceTags(context.TODO(), mac)
	if err != nil {
		zap.L().Warn("failed to get interface tags", zap.Error(err))
		return ManagerCNI, nil
	}

	if val, ok := tags["nodeadm:managed"]; ok && val == "true" {
		return ManagerSystemd, nil
	}

	return ManagerCNI, nil
}

func (b *fsBroker) ManagerFor(interfaceName string) (string, error) {
	// we check whether there is a manager already cached for this interface,
	// because we dont want to reconfigure interfaces from a previous boot for
	// the same EC2 instance.
	if manager, err := b.cache.Read(interfaceName); err == nil {
		return manager, nil
	}

	manager, err := b.determineManager(interfaceName)
	if err != nil {
		return "", err
	}

	if err := b.cache.Write(interfaceName, manager); err != nil {
		zap.L().Warn("failed writing manager back to cache", zap.Error(err), zap.String("interface", interfaceName), zap.String("manager", manager))
	}
	return manager, nil
}
