package network

import (
	"context"
	"fmt"
	"strings"

	"github.com/torresmo-dev/torresmo/internal/runtime"
)

// DNSEntry holds DNS configuration for a service entry.
type DNSEntry struct {
	ServiceName string
	Namespace   string
	ClusterIP   string
}

// DNSManager manages DNS resolution within the cluster.
// It uses /etc/hosts injection into each container for service discovery.
type DNSManager struct {
	runtime runtime.ContainerRuntime
}

func NewDNSManager(rt runtime.ContainerRuntime) *DNSManager {
	return &DNSManager{runtime: rt}
}

// BuildHostsEntries generates /etc/hosts style entries for all services.
// Format:
//
//	<clusterIP>  <serviceName>.<namespace>.svc.torresmo.local <serviceName>.<namespace>.svc <serviceName>.<namespace>
func BuildHostsEntries(entries []DNSEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		fqdn := fmt.Sprintf("%s.%s.svc.torresmo.local", e.ServiceName, e.Namespace)
		short := fmt.Sprintf("%s.%s", e.ServiceName, e.Namespace)
		svc := fmt.Sprintf("%s.%s.svc", e.ServiceName, e.Namespace)
		sb.WriteString(fmt.Sprintf("%s\t%s %s %s\n", e.ClusterIP, fqdn, svc, short))
	}
	return sb.String()
}

// UpdateContainerHosts injects hosts entries into a running container's /etc/hosts
// by exec'ing a shell command.
func (d *DNSManager) UpdateContainerHosts(ctx context.Context, containerID string, entries []DNSEntry) error {
	if len(entries) == 0 {
		return nil
	}

	hostsContent := BuildHostsEntries(entries)

	// Append to /etc/hosts inside the container
	cmd := []string{"sh", "-c", fmt.Sprintf(
		`printf '\n# Torresmo service discovery\n%s' >> /etc/hosts`,
		hostsContent,
	)}

	result, err := d.runtime.ExecInContainer(ctx, containerID, cmd, runtime.ExecOptions{})
	if err != nil {
		return fmt.Errorf("failed to update /etc/hosts in container %s: %w", containerID, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to update /etc/hosts (exit code %d): %s", result.ExitCode, result.Output)
	}

	return nil
}

// UpdateAllContainersHosts updates /etc/hosts in all containers of a cluster.
func (d *DNSManager) UpdateAllContainersHosts(ctx context.Context, clusterName string, entries []DNSEntry) error {
	containers, err := d.runtime.ListContainers(ctx, map[string]string{
		"torresmo.dev/managed": "true",
		"torresmo.dev/cluster": clusterName,
	})
	if err != nil {
		return fmt.Errorf("failed to list cluster containers: %w", err)
	}

	for _, c := range containers {
		if c.State != "running" {
			continue
		}
		if err := d.UpdateContainerHosts(ctx, c.ID, entries); err != nil {
			// Log but don't fail — some containers might not have sh
			continue
		}
	}

	return nil
}
