package udev

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/aws/ec2"
	"github.com/awslabs/amazon-eks-ami/nodeadm/internal/util"
	"github.com/stretchr/testify/assert"
)

type mockEC2Client struct {
	tags map[string]string
	err  error
}

func (m *mockEC2Client) GetInterfaceTags(ctx context.Context, mac string) (map[string]string, error) {
	return m.tags, m.err
}

var _ ec2.Client = &mockEC2Client{}

func TestDetermineManager(t *testing.T) {
	// Create a temporary directory for the cache
	tmpDir, err := os.MkdirTemp("", "udev-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	interfaceName := "lo"
	mockMac := "00:00:00:00:00:00"

	mockMacFetcher := func(iface string) (string, error) {
		return mockMac, nil
	}

	t.Run("CloudInitNotDone", func(t *testing.T) {
		b := &fsBroker{
			cache: util.NewFSCache(tmpDir),
			statFunc: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			macFetcher: mockMacFetcher,
		}
		manager, err := b.determineManager(interfaceName)
		assert.NoError(t, err)
		assert.Equal(t, ManagerSystemd, manager)
	})

	t.Run("CloudInitDone_NoTags", func(t *testing.T) {
		b := &fsBroker{
			cache: util.NewFSCache(tmpDir),
			ec2Client: &mockEC2Client{
				tags: nil,
			},
			statFunc: func(path string) (os.FileInfo, error) {
				return nil, nil // File exists
			},
			macFetcher: mockMacFetcher,
		}
		manager, err := b.determineManager(interfaceName)
		assert.NoError(t, err)
		assert.Equal(t, ManagerCNI, manager)
	})

	t.Run("CloudInitDone_ManagedTag", func(t *testing.T) {
		b := &fsBroker{
			cache: util.NewFSCache(tmpDir),
			ec2Client: &mockEC2Client{
				tags: map[string]string{"nodeadm:managed": "true"},
			},
			statFunc: func(path string) (os.FileInfo, error) {
				return nil, nil // File exists
			},
			macFetcher: mockMacFetcher,
		}
		manager, err := b.determineManager(interfaceName)
		assert.NoError(t, err)
		assert.Equal(t, ManagerSystemd, manager)
	})

	t.Run("CloudInitDone_UnmanagedTag", func(t *testing.T) {
		b := &fsBroker{
			cache: util.NewFSCache(tmpDir),
			ec2Client: &mockEC2Client{
				tags: map[string]string{"nodeadm:managed": "false"},
			},
			statFunc: func(path string) (os.FileInfo, error) {
				return nil, nil // File exists
			},
			macFetcher: mockMacFetcher,
		}
		manager, err := b.determineManager(interfaceName)
		assert.NoError(t, err)
		assert.Equal(t, ManagerCNI, manager)
	})

	t.Run("CloudInitDone_EC2Error", func(t *testing.T) {
		b := &fsBroker{
			cache: util.NewFSCache(tmpDir),
			ec2Client: &mockEC2Client{
				err: fmt.Errorf("ec2 error"),
			},
			statFunc: func(path string) (os.FileInfo, error) {
				return nil, nil // File exists
			},
			macFetcher: mockMacFetcher,
		}
		manager, err := b.determineManager(interfaceName)
		assert.NoError(t, err)
		assert.Equal(t, ManagerCNI, manager)
	})
}
