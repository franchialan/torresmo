package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/torresmo-dev/torresmo/internal/storage"
	sqlitestore "github.com/torresmo-dev/torresmo/internal/storage/sqlite"
)

func setupTestManager(t *testing.T) (*Manager, *sqlitestore.Store) {
	t.Helper()
	dir := t.TempDir()

	dbPath := filepath.Join(dir, "test.db")
	store, err := sqlitestore.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	snapDir := filepath.Join(dir, "snapshots")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		t.Fatalf("failed to create snapshot dir: %v", err)
	}

	mgr := &Manager{store: store, dir: snapDir}
	return mgr, store
}

func seedCluster(t *testing.T, store *sqlitestore.Store, clusterName string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UnixMilli()

	cluster := &storage.ClusterRecord{
		Name:       clusterName,
		Runtime:    "docker",
		Profile:    "standard",
		Phase:      "Running",
		NodeCount:  2,
		ReadyNodes: 2,
		PodCIDR:    "10.244.0.0/16",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := store.SaveCluster(ctx, cluster); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	node := &storage.NodeRecord{
		Name:        "node-1",
		ClusterName: clusterName,
		Role:        "control-plane",
		ContainerID: "abc123",
		Address:     "172.18.0.2",
		Phase:       "Running",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	ns := &storage.NamespaceRecord{
		Name:        "default",
		ClusterName: clusterName,
		Phase:       "Active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveNamespace(ctx, ns); err != nil {
		t.Fatalf("SaveNamespace: %v", err)
	}

	pod := &storage.PodRecord{
		Name:        "web-pod",
		ClusterName: clusterName,
		Namespace:   "default",
		Image:       "nginx:latest",
		Phase:       "Running",
		Labels:      `{"app":"web"}`,
		Env:         "[]",
		Ports:       `[{"containerPort":80}]`,
		Command:     "[]",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SavePod(ctx, pod); err != nil {
		t.Fatalf("SavePod: %v", err)
	}

	svc := &storage.ServiceRecord{
		Name:        "web-svc",
		ClusterName: clusterName,
		Namespace:   "default",
		Type:        "ClusterIP",
		Ports:       `[{"port":80,"targetPort":80}]`,
		Selector:    `{"app":"web"}`,
		ClusterIP:   "10.244.0.10",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveService(ctx, svc); err != nil {
		t.Fatalf("SaveService: %v", err)
	}

	dep := &storage.DeploymentRecord{
		Name:        "web",
		ClusterName: clusterName,
		Namespace:   "default",
		Replicas:    1,
		Template:    "{}",
		Labels:      `{"app":"web"}`,
		Selector:    `{"app":"web"}`,
		Phase:       "Available",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveDeployment(ctx, dep); err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}

	addon := &storage.AddonRecord{
		Name:        "redis",
		ClusterName: clusterName,
		Namespace:   "default",
		Version:     "7.0",
		Phase:       "Running",
		Config:      "{}",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveAddon(ctx, addon); err != nil {
		t.Fatalf("SaveAddon: %v", err)
	}
}

func TestCreateAndGet(t *testing.T) {
	mgr, store := setupTestManager(t)
	ctx := context.Background()
	seedCluster(t, store, "snap-cluster")

	if err := mgr.Create(ctx, "snap-1", "snap-cluster"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	snap, err := mgr.Get("snap-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if snap.Name != "snap-1" {
		t.Errorf("Name = %q, want snap-1", snap.Name)
	}
	if snap.ClusterName != "snap-cluster" {
		t.Errorf("ClusterName = %q, want snap-cluster", snap.ClusterName)
	}
	if snap.Cluster.Name != "snap-cluster" {
		t.Errorf("Cluster.Name = %q, want snap-cluster", snap.Cluster.Name)
	}
	if len(snap.Nodes) != 1 {
		t.Errorf("got %d nodes, want 1", len(snap.Nodes))
	}
	if len(snap.Namespaces) != 1 {
		t.Errorf("got %d namespaces, want 1", len(snap.Namespaces))
	}
	if len(snap.Pods) != 1 {
		t.Errorf("got %d pods, want 1", len(snap.Pods))
	}
	if len(snap.Services) != 1 {
		t.Errorf("got %d services, want 1", len(snap.Services))
	}
	if len(snap.Deployments) != 1 {
		t.Errorf("got %d deployments, want 1", len(snap.Deployments))
	}
	if len(snap.Addons) != 1 {
		t.Errorf("got %d addons, want 1", len(snap.Addons))
	}
	if snap.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
}

func TestList(t *testing.T) {
	mgr, store := setupTestManager(t)
	ctx := context.Background()
	seedCluster(t, store, "list-cluster")

	if err := mgr.Create(ctx, "snap-a", "list-cluster"); err != nil {
		t.Fatalf("Create snap-a: %v", err)
	}
	if err := mgr.Create(ctx, "snap-b", "list-cluster"); err != nil {
		t.Fatalf("Create snap-b: %v", err)
	}

	infos, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d snapshots, want 2", len(infos))
	}

	names := make(map[string]bool)
	for _, info := range infos {
		names[info.Name] = true
		if info.ClusterName != "list-cluster" {
			t.Errorf("ClusterName = %q, want list-cluster", info.ClusterName)
		}
		if info.Pods != 1 {
			t.Errorf("Pods = %d, want 1", info.Pods)
		}
		if info.Addons != 1 {
			t.Errorf("Addons = %d, want 1", info.Addons)
		}
		if info.Size <= 0 {
			t.Errorf("Size = %d, want > 0", info.Size)
		}
	}
	if !names["snap-a"] || !names["snap-b"] {
		t.Error("expected both snap-a and snap-b in list")
	}
}

func TestDelete(t *testing.T) {
	mgr, store := setupTestManager(t)
	ctx := context.Background()
	seedCluster(t, store, "del-cluster")

	if err := mgr.Create(ctx, "snap-del", "del-cluster"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := mgr.Delete("snap-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := mgr.Get("snap-del")
	if err == nil {
		t.Fatal("expected error getting deleted snapshot, got nil")
	}
}

func TestExportImport(t *testing.T) {
	mgr, store := setupTestManager(t)
	ctx := context.Background()
	seedCluster(t, store, "export-cluster")

	if err := mgr.Create(ctx, "snap-export", "export-cluster"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Export
	exportPath := filepath.Join(t.TempDir(), "exported.json")
	if err := mgr.Export("snap-export", exportPath); err != nil {
		t.Fatalf("Export: %v", err)
	}

	if _, err := os.Stat(exportPath); err != nil {
		t.Fatalf("expected exported file: %v", err)
	}

	// Create a second manager to import into
	mgr2, _ := setupTestManager(t)

	if err := mgr2.Import(exportPath); err != nil {
		t.Fatalf("Import: %v", err)
	}

	snap, err := mgr2.Get("snap-export")
	if err != nil {
		t.Fatalf("Get after import: %v", err)
	}
	if snap.ClusterName != "export-cluster" {
		t.Errorf("ClusterName = %q, want export-cluster", snap.ClusterName)
	}
	if snap.Cluster.Name != "export-cluster" {
		t.Errorf("Cluster.Name = %q, want export-cluster", snap.Cluster.Name)
	}
	if len(snap.Pods) != 1 {
		t.Errorf("got %d pods, want 1", len(snap.Pods))
	}
	if len(snap.Addons) != 1 {
		t.Errorf("got %d addons, want 1", len(snap.Addons))
	}
}

func TestCreateClusterNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)
	ctx := context.Background()

	err := mgr.Create(ctx, "snap-fail", "nonexistent-cluster")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster, got nil")
	}
}

func TestGetNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)

	_, err := mgr.Get("nonexistent-snapshot")
	if err == nil {
		t.Fatal("expected error for nonexistent snapshot, got nil")
	}
}

func TestDeleteNotFound(t *testing.T) {
	mgr, _ := setupTestManager(t)

	err := mgr.Delete("nonexistent-snapshot")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent snapshot, got nil")
	}
}
