package cluster

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/torresmo-dev/torresmo/internal/profile"
	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

// nodeImage is the base image for cluster nodes.
// TODO: Replace with a custom torresmo/node image that includes containerd.
const nodeImage = "alpine:3.19"

var clusterNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ClusterInfo is a read-only view of a cluster.
type ClusterInfo struct {
	Name       string
	Runtime    string
	Profile    string
	Phase      string
	Nodes      int
	ReadyNodes int
	PodCIDR    string
	CreatedAt  time.Time
}

// NodeInfo is a read-only view of a node.
type NodeInfo struct {
	Name        string
	ClusterName string
	Role        string
	ContainerID string
	Address     string
	Phase       string
	CreatedAt   time.Time
}

// Manager orchestrates cluster lifecycle operations.
type Manager struct {
	runtime runtime.ContainerRuntime
	store   storage.Store
}

// NewManager creates a new cluster manager.
func NewManager(rt runtime.ContainerRuntime, store storage.Store) *Manager {
	return &Manager{
		runtime: rt,
		store:   store,
	}
}

func validateClusterName(name string) error {
	if !clusterNameRe.MatchString(name) {
		return fmt.Errorf("invalid cluster name %q: must match [a-z0-9][a-z0-9-]{0,61}[a-z0-9]", name)
	}
	return nil
}

// CreateCluster creates a new cluster with the given parameters.
func (m *Manager) CreateCluster(ctx context.Context, name string, nodeCount int, profileName, podCIDR string) error {
	if err := validateClusterName(name); err != nil {
		return err
	}

	// Check for duplicate.
	if _, err := m.store.GetCluster(ctx, name); err == nil {
		return fmt.Errorf("cluster %q already exists", name)
	}

	if profileName == "" {
		profileName = profile.Default()
	}
	prof, err := profile.Get(profileName)
	if err != nil {
		return err
	}

	if podCIDR == "" {
		podCIDR = "10.244.0.0/16"
	}
	if nodeCount < 1 {
		nodeCount = 1
	}

	now := time.Now().Unix()
	rtInfo, err := m.runtime.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to query runtime: %w", err)
	}

	// Ensure the node image is available locally.
	exists, err := m.runtime.ImageExists(ctx, nodeImage)
	if err != nil {
		return fmt.Errorf("failed to check node image: %w", err)
	}
	if !exists {
		if err := m.runtime.PullImage(ctx, nodeImage); err != nil {
			return fmt.Errorf("failed to pull node image %q: %w", nodeImage, err)
		}
	}

	clusterRecord := &storage.ClusterRecord{
		Name:       name,
		Runtime:    rtInfo.Name,
		Profile:    profileName,
		Phase:      "Creating",
		NodeCount:  nodeCount,
		ReadyNodes: 0,
		PodCIDR:    podCIDR,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := m.store.SaveCluster(ctx, clusterRecord); err != nil {
		return fmt.Errorf("failed to save cluster: %w", err)
	}

	// Create the Docker network.
	networkName := "torresmo-" + name
	_, err = m.runtime.CreateNetwork(ctx, networkName, runtime.NetworkOptions{
		Driver: "bridge",
		Subnet: podCIDR,
		Labels: map[string]string{
			"torresmo.dev/managed": "true",
			"torresmo.dev/cluster": name,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	baseLabels := map[string]string{
		"torresmo.dev/managed": "true",
		"torresmo.dev/cluster": name,
	}

	// Create and start control-plane node.
	cpName := name + "-control-plane"
	cpLabels := copyLabels(baseLabels)
	cpLabels["torresmo.dev/role"] = "control-plane"

	cpID, err := m.runtime.CreateContainer(ctx, &runtime.ContainerConfig{
		Name:       cpName,
		Image:      nodeImage,
		Command:    []string{"sleep", "infinity"},
		Labels:     cpLabels,
		Networks:   []string{networkName},
		Privileged: true,
		CPULimit:   prof.CPU,
		MemLimit:   prof.Memory,
	})
	if err != nil {
		return fmt.Errorf("failed to create control-plane container: %w", err)
	}

	if err := m.runtime.StartContainer(ctx, cpID); err != nil {
		return fmt.Errorf("failed to start control-plane container: %w", err)
	}

	cpDetail, err := m.runtime.InspectContainer(ctx, cpID)
	if err != nil {
		return fmt.Errorf("failed to inspect control-plane container: %w", err)
	}

	cpNode := &storage.NodeRecord{
		Name:        cpName,
		ClusterName: name,
		Role:        "control-plane",
		ContainerID: cpID,
		Address:     containerIP(cpDetail, networkName),
		Phase:       "Running",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := m.store.SaveNode(ctx, cpNode); err != nil {
		return fmt.Errorf("failed to save control-plane node: %w", err)
	}

	// Create and start worker nodes.
	for i := 1; i < nodeCount; i++ {
		workerName := name + "-worker-" + strconv.Itoa(i)
		workerLabels := copyLabels(baseLabels)
		workerLabels["torresmo.dev/role"] = "worker"

		wID, err := m.runtime.CreateContainer(ctx, &runtime.ContainerConfig{
			Name:       workerName,
			Image:      nodeImage,
			Command:    []string{"sleep", "infinity"},
			Labels:     workerLabels,
			Networks:   []string{networkName},
			Privileged: true,
			CPULimit:   prof.CPU,
			MemLimit:   prof.Memory,
		})
		if err != nil {
			return fmt.Errorf("failed to create worker container %q: %w", workerName, err)
		}

		if err := m.runtime.StartContainer(ctx, wID); err != nil {
			return fmt.Errorf("failed to start worker container %q: %w", workerName, err)
		}

		wDetail, err := m.runtime.InspectContainer(ctx, wID)
		if err != nil {
			return fmt.Errorf("failed to inspect worker container %q: %w", workerName, err)
		}

		wNode := &storage.NodeRecord{
			Name:        workerName,
			ClusterName: name,
			Role:        "worker",
			ContainerID: wID,
			Address:     containerIP(wDetail, networkName),
			Phase:       "Running",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := m.store.SaveNode(ctx, wNode); err != nil {
			return fmt.Errorf("failed to save worker node %q: %w", workerName, err)
		}
	}

	// Update cluster status to Running.
	clusterRecord.Phase = "Running"
	clusterRecord.ReadyNodes = nodeCount
	clusterRecord.UpdatedAt = time.Now().Unix()
	if err := m.store.SaveCluster(ctx, clusterRecord); err != nil {
		return fmt.Errorf("failed to update cluster status: %w", err)
	}

	return nil
}

// DeleteCluster tears down a cluster and removes it from storage.
func (m *Manager) DeleteCluster(ctx context.Context, name string) error {
	if _, err := m.store.GetCluster(ctx, name); err != nil {
		return fmt.Errorf("cluster %q not found: %w", name, err)
	}

	// Remove all containers belonging to this cluster (pods + nodes).
	containers, err := m.runtime.ListContainers(ctx, map[string]string{
		"torresmo.dev/managed": "true",
		"torresmo.dev/cluster": name,
	})
	if err == nil {
		for _, c := range containers {
			_ = m.runtime.StopContainer(ctx, c.ID, 10)
			_ = m.runtime.RemoveContainer(ctx, c.ID, true)
		}
	}

	networkName := "torresmo-" + name
	_ = m.runtime.RemoveNetwork(ctx, networkName)

	if err := m.store.DeleteCluster(ctx, name); err != nil {
		return fmt.Errorf("failed to delete cluster from storage: %w", err)
	}

	return nil
}

// GetCluster returns information about a cluster.
func (m *Manager) GetCluster(ctx context.Context, name string) (*ClusterInfo, error) {
	rec, err := m.store.GetCluster(ctx, name)
	if err != nil {
		return nil, err
	}
	return clusterInfoFromRecord(rec), nil
}

// ListClusters returns all clusters.
func (m *Manager) ListClusters(ctx context.Context) ([]ClusterInfo, error) {
	records, err := m.store.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	infos := make([]ClusterInfo, len(records))
	for i, rec := range records {
		infos[i] = *clusterInfoFromRecord(&rec)
	}
	return infos, nil
}

// GetNodes returns all nodes for a cluster.
func (m *Manager) GetNodes(ctx context.Context, clusterName string) ([]NodeInfo, error) {
	records, err := m.store.ListNodes(ctx, clusterName)
	if err != nil {
		return nil, err
	}
	infos := make([]NodeInfo, len(records))
	for i, rec := range records {
		infos[i] = NodeInfo{
			Name:        rec.Name,
			ClusterName: rec.ClusterName,
			Role:        rec.Role,
			ContainerID: rec.ContainerID,
			Address:     rec.Address,
			Phase:       rec.Phase,
			CreatedAt:   time.Unix(rec.CreatedAt, 0),
		}
	}
	return infos, nil
}

func clusterInfoFromRecord(rec *storage.ClusterRecord) *ClusterInfo {
	return &ClusterInfo{
		Name:       rec.Name,
		Runtime:    rec.Runtime,
		Profile:    rec.Profile,
		Phase:      rec.Phase,
		Nodes:      rec.NodeCount,
		ReadyNodes: rec.ReadyNodes,
		PodCIDR:    rec.PodCIDR,
		CreatedAt:  time.Unix(rec.CreatedAt, 0),
	}
}

func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// containerIP extracts the IP address from the container detail,
// preferring the cluster network endpoint over the default IP.
func containerIP(detail *runtime.ContainerDetail, networkName string) string {
	if ep, ok := detail.NetworkSettings.Networks[networkName]; ok && ep.IPAddress != "" {
		return ep.IPAddress
	}
	return detail.NetworkSettings.IPAddress
}
