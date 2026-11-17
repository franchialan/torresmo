package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/torresmo-dev/torresmo/internal/storage"
)

// Snapshot represents a saved cluster state.
type Snapshot struct {
	Name        string                    `json:"name"`
	ClusterName string                    `json:"cluster_name"`
	CreatedAt   time.Time                 `json:"created_at"`
	Cluster     storage.ClusterRecord     `json:"cluster"`
	Nodes       []storage.NodeRecord      `json:"nodes"`
	Namespaces  []storage.NamespaceRecord `json:"namespaces"`
	Pods        []storage.PodRecord       `json:"pods"`
	Services    []storage.ServiceRecord   `json:"services"`
	Deployments []storage.DeploymentRecord `json:"deployments"`
	Addons      []storage.AddonRecord     `json:"addons"`
}

// SnapshotInfo is a lightweight summary of a snapshot.
type SnapshotInfo struct {
	Name        string
	ClusterName string
	CreatedAt   time.Time
	Size        int64
	Pods        int
	Addons      int
}

// Manager handles snapshot operations.
type Manager struct {
	store storage.Store
	dir   string
}

// NewManager creates a new snapshot manager.
func NewManager(store storage.Store) (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".torresmo", "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Manager{store: store, dir: dir}, nil
}

// Create saves the current state of a cluster.
func (m *Manager) Create(ctx context.Context, name, clusterName string) error {
	cluster, err := m.store.GetCluster(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("cluster %q not found: %w", clusterName, err)
	}

	nodes, _ := m.store.ListNodes(ctx, clusterName)
	namespaces, _ := m.store.ListNamespaces(ctx, clusterName)
	pods, _ := m.store.ListAllPods(ctx, clusterName)
	services, _ := m.store.ListServices(ctx, clusterName, "")
	deployments, _ := m.store.ListDeployments(ctx, clusterName, "")
	addons, _ := m.store.ListAddons(ctx, clusterName)

	snap := &Snapshot{
		Name:        name,
		ClusterName: clusterName,
		CreatedAt:   time.Now(),
		Cluster:     *cluster,
		Nodes:       nodes,
		Namespaces:  namespaces,
		Pods:        pods,
		Services:    services,
		Deployments: deployments,
		Addons:      addons,
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing snapshot: %w", err)
	}

	path := filepath.Join(m.dir, name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}

	return nil
}

// List returns all available snapshots.
func (m *Manager) List() ([]SnapshotInfo, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var infos []SnapshotInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(m.dir, entry.Name()))
		if err != nil {
			continue
		}

		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}

		fi, _ := entry.Info()
		size := int64(0)
		if fi != nil {
			size = fi.Size()
		}

		infos = append(infos, SnapshotInfo{
			Name:        snap.Name,
			ClusterName: snap.ClusterName,
			CreatedAt:   snap.CreatedAt,
			Size:        size,
			Pods:        len(snap.Pods),
			Addons:      len(snap.Addons),
		})
	}

	return infos, nil
}

// Get loads a snapshot by name.
func (m *Manager) Get(name string) (*Snapshot, error) {
	path := filepath.Join(m.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot %q not found", name)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing snapshot: %w", err)
	}
	return &snap, nil
}

// Delete removes a snapshot.
func (m *Manager) Delete(name string) error {
	path := filepath.Join(m.dir, name+".json")
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("deleting snapshot %q: %w", name, err)
	}
	return nil
}

// Export copies a snapshot to the given path.
func (m *Manager) Export(name, destPath string) error {
	srcPath := filepath.Join(m.dir, name+".json")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("snapshot %q not found", name)
	}
	return os.WriteFile(destPath, data, 0o644)
}

// Import loads a snapshot from an external file.
func (m *Manager) Import(srcPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("invalid snapshot file: %w", err)
	}
	destPath := filepath.Join(m.dir, snap.Name+".json")
	return os.WriteFile(destPath, data, 0o644)
}
