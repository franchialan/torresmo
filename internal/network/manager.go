package network

import (
	"context"
	"fmt"
	"net"

	"github.com/torresmo-dev/torresmo/internal/runtime"
)

// Manager handles cluster networking.
type Manager struct {
	runtime runtime.ContainerRuntime
}

func NewManager(rt runtime.ContainerRuntime) *Manager {
	return &Manager{runtime: rt}
}

// NetworkName returns the Docker network name for a cluster.
func NetworkName(clusterName string) string {
	return "torresmo-" + clusterName
}

// CreateClusterNetwork creates the Docker bridge network for a cluster.
func (m *Manager) CreateClusterNetwork(ctx context.Context, clusterName, subnet string) error {
	name := NetworkName(clusterName)
	_, err := m.runtime.CreateNetwork(ctx, name, runtime.NetworkOptions{
		Driver: "bridge",
		Subnet: subnet,
		Labels: map[string]string{
			"torresmo.dev/managed": "true",
			"torresmo.dev/cluster": clusterName,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create cluster network %q: %w", name, err)
	}
	return nil
}

// DeleteClusterNetwork removes the Docker network for a cluster.
func (m *Manager) DeleteClusterNetwork(ctx context.Context, clusterName string) error {
	return m.runtime.RemoveNetwork(ctx, NetworkName(clusterName))
}

// ConnectContainer connects a container to the cluster network.
func (m *Manager) ConnectContainer(ctx context.Context, clusterName, containerID string) error {
	return m.runtime.ConnectNetwork(ctx, NetworkName(clusterName), containerID)
}

// AllocateServiceIP generates a deterministic cluster IP for a service.
// Uses the cluster's pod CIDR and assigns IPs starting from .10
func AllocateServiceIP(podCIDR string, serviceIndex int) (string, error) {
	ip, ipNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return "", fmt.Errorf("invalid CIDR %q: %w", podCIDR, err)
	}

	// Start from the base IP + 10 + serviceIndex
	// e.g., 10.244.0.0/16 -> 10.244.0.10, 10.244.0.11, ...
	ip = ip.Mask(ipNet.Mask)
	ip4 := ip.To4()
	if ip4 == nil {
		return "", fmt.Errorf("only IPv4 CIDRs are supported")
	}

	offset := 10 + serviceIndex
	// Add offset to the last two bytes
	ip4[2] += byte(offset / 256)
	ip4[3] += byte(offset % 256)

	return ip4.String(), nil
}
