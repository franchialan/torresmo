package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/torresmo-dev/torresmo/internal/storage"
)

func setupTestDB(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func makeCluster(name string, createdAt int64) *storage.ClusterRecord {
	return &storage.ClusterRecord{
		Name:       name,
		Runtime:    "docker",
		Profile:    "standard",
		Phase:      "Running",
		NodeCount:  3,
		ReadyNodes: 3,
		PodCIDR:    "10.244.0.0/16",
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}
}

// --- Cluster CRUD ---

func TestSaveAndGetCluster(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("test-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	got, err := store.GetCluster(ctx, "test-cluster")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}

	if got.Name != c.Name {
		t.Errorf("Name = %q, want %q", got.Name, c.Name)
	}
	if got.Runtime != c.Runtime {
		t.Errorf("Runtime = %q, want %q", got.Runtime, c.Runtime)
	}
	if got.Profile != c.Profile {
		t.Errorf("Profile = %q, want %q", got.Profile, c.Profile)
	}
	if got.Phase != c.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, c.Phase)
	}
	if got.NodeCount != c.NodeCount {
		t.Errorf("NodeCount = %d, want %d", got.NodeCount, c.NodeCount)
	}
	if got.ReadyNodes != c.ReadyNodes {
		t.Errorf("ReadyNodes = %d, want %d", got.ReadyNodes, c.ReadyNodes)
	}
	if got.PodCIDR != c.PodCIDR {
		t.Errorf("PodCIDR = %q, want %q", got.PodCIDR, c.PodCIDR)
	}
	if got.CreatedAt != c.CreatedAt {
		t.Errorf("CreatedAt = %d, want %d", got.CreatedAt, c.CreatedAt)
	}
	if got.UpdatedAt != c.UpdatedAt {
		t.Errorf("UpdatedAt = %d, want %d", got.UpdatedAt, c.UpdatedAt)
	}
}

func TestListClusters(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	for i, name := range []string{"alpha", "beta", "gamma"} {
		c := makeCluster(name, now+int64(i)*1000)
		if err := store.SaveCluster(ctx, c); err != nil {
			t.Fatalf("SaveCluster(%q): %v", name, err)
		}
	}

	clusters, err := store.ListClusters(ctx)
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(clusters) != 3 {
		t.Fatalf("got %d clusters, want 3", len(clusters))
	}
	// Newest first
	if clusters[0].Name != "gamma" {
		t.Errorf("first cluster = %q, want gamma (newest)", clusters[0].Name)
	}
	if clusters[2].Name != "alpha" {
		t.Errorf("last cluster = %q, want alpha (oldest)", clusters[2].Name)
	}
}

func TestDeleteCluster(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("doomed", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}
	if err := store.DeleteCluster(ctx, "doomed"); err != nil {
		t.Fatalf("DeleteCluster: %v", err)
	}

	_, err := store.GetCluster(ctx, "doomed")
	if err == nil {
		t.Fatal("expected error getting deleted cluster, got nil")
	}
}

func TestDeleteClusterCascadesNodes(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("cascade-test", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	node := &storage.NodeRecord{
		Name:        "node-1",
		ClusterName: "cascade-test",
		Role:        "worker",
		Phase:       "Running",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}

	if err := store.DeleteCluster(ctx, "cascade-test"); err != nil {
		t.Fatalf("DeleteCluster: %v", err)
	}

	nodes, err := store.ListNodes(ctx, "cascade-test")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes after cascade delete, got %d", len(nodes))
	}
}

func TestSaveClusterUpsert(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("upsert-test", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	later := now + 5000
	c.Phase = "Stopped"
	c.UpdatedAt = later
	c.CreatedAt = later // This should NOT update created_at due to ON CONFLICT behavior
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster upsert: %v", err)
	}

	got, err := store.GetCluster(ctx, "upsert-test")
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if got.Phase != "Stopped" {
		t.Errorf("Phase = %q, want Stopped", got.Phase)
	}
	if got.UpdatedAt != later {
		t.Errorf("UpdatedAt = %d, want %d", got.UpdatedAt, later)
	}
	// The upsert does NOT update created_at (not in the DO UPDATE SET clause)
	if got.CreatedAt != now {
		t.Errorf("CreatedAt = %d, want %d (should be preserved)", got.CreatedAt, now)
	}
}

func TestGetClusterNotFound(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	_, err := store.GetCluster(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster, got nil")
	}
}

// --- Node CRUD ---

func TestSaveAndGetNode(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("node-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	node := &storage.NodeRecord{
		Name:        "control-plane",
		ClusterName: "node-cluster",
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

	got, err := store.GetNode(ctx, "node-cluster", "control-plane")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Name != node.Name {
		t.Errorf("Name = %q, want %q", got.Name, node.Name)
	}
	if got.Role != node.Role {
		t.Errorf("Role = %q, want %q", got.Role, node.Role)
	}
	if got.ContainerID != node.ContainerID {
		t.Errorf("ContainerID = %q, want %q", got.ContainerID, node.ContainerID)
	}
	if got.Address != node.Address {
		t.Errorf("Address = %q, want %q", got.Address, node.Address)
	}
	if got.Phase != node.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, node.Phase)
	}
}

func TestListNodes(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("list-nodes", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, name := range []string{"node-a", "node-b", "node-c"} {
		node := &storage.NodeRecord{
			Name:        name,
			ClusterName: "list-nodes",
			Role:        "worker",
			Phase:       "Running",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveNode(ctx, node); err != nil {
			t.Fatalf("SaveNode(%q): %v", name, err)
		}
	}

	nodes, err := store.ListNodes(ctx, "list-nodes")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(nodes))
	}
	// Ordered by name
	if nodes[0].Name != "node-a" {
		t.Errorf("first node = %q, want node-a", nodes[0].Name)
	}
	if nodes[2].Name != "node-c" {
		t.Errorf("last node = %q, want node-c", nodes[2].Name)
	}
}

func TestDeleteNode(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("del-node", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	node := &storage.NodeRecord{
		Name:        "victim",
		ClusterName: "del-node",
		Role:        "worker",
		Phase:       "Running",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveNode(ctx, node); err != nil {
		t.Fatalf("SaveNode: %v", err)
	}
	if err := store.DeleteNode(ctx, "del-node", "victim"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	_, err := store.GetNode(ctx, "del-node", "victim")
	if err == nil {
		t.Fatal("expected error getting deleted node, got nil")
	}
}

// --- Namespace CRUD ---

func TestSaveAndGetNamespace(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("ns-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	ns := &storage.NamespaceRecord{
		Name:        "kube-system",
		ClusterName: "ns-cluster",
		Phase:       "Active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveNamespace(ctx, ns); err != nil {
		t.Fatalf("SaveNamespace: %v", err)
	}

	got, err := store.GetNamespace(ctx, "ns-cluster", "kube-system")
	if err != nil {
		t.Fatalf("GetNamespace: %v", err)
	}
	if got.Name != ns.Name {
		t.Errorf("Name = %q, want %q", got.Name, ns.Name)
	}
	if got.Phase != ns.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, ns.Phase)
	}
	if got.ClusterName != ns.ClusterName {
		t.Errorf("ClusterName = %q, want %q", got.ClusterName, ns.ClusterName)
	}
}

func TestListNamespaces(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("ns-list", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, name := range []string{"default", "kube-system", "monitoring"} {
		ns := &storage.NamespaceRecord{
			Name:        name,
			ClusterName: "ns-list",
			Phase:       "Active",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveNamespace(ctx, ns); err != nil {
			t.Fatalf("SaveNamespace(%q): %v", name, err)
		}
	}

	nsList, err := store.ListNamespaces(ctx, "ns-list")
	if err != nil {
		t.Fatalf("ListNamespaces: %v", err)
	}
	if len(nsList) != 3 {
		t.Fatalf("got %d namespaces, want 3", len(nsList))
	}
}

func TestDeleteNamespace(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("ns-del", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	ns := &storage.NamespaceRecord{
		Name:        "temp-ns",
		ClusterName: "ns-del",
		Phase:       "Active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveNamespace(ctx, ns); err != nil {
		t.Fatalf("SaveNamespace: %v", err)
	}
	if err := store.DeleteNamespace(ctx, "ns-del", "temp-ns"); err != nil {
		t.Fatalf("DeleteNamespace: %v", err)
	}

	_, err := store.GetNamespace(ctx, "ns-del", "temp-ns")
	if err == nil {
		t.Fatal("expected error getting deleted namespace, got nil")
	}
}

// --- Pod CRUD ---

func TestSaveAndGetPod(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("pod-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	pod := &storage.PodRecord{
		Name:        "web-abc123",
		ClusterName: "pod-cluster",
		Namespace:   "default",
		ContainerID: "sha256:deadbeef",
		Image:       "nginx:latest",
		Command:     `["nginx","-g","daemon off;"]`,
		Phase:       "Running",
		Node:        "node-1",
		PodIP:       "10.244.0.5",
		Restarts:    2,
		Labels:      `{"app":"web","tier":"frontend"}`,
		Env:         `[{"name":"PORT","value":"8080"}]`,
		Ports:       `[{"containerPort":80,"protocol":"tcp"}]`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SavePod(ctx, pod); err != nil {
		t.Fatalf("SavePod: %v", err)
	}

	got, err := store.GetPod(ctx, "pod-cluster", "default", "web-abc123")
	if err != nil {
		t.Fatalf("GetPod: %v", err)
	}
	if got.Name != pod.Name {
		t.Errorf("Name = %q, want %q", got.Name, pod.Name)
	}
	if got.Image != pod.Image {
		t.Errorf("Image = %q, want %q", got.Image, pod.Image)
	}
	if got.Labels != pod.Labels {
		t.Errorf("Labels = %q, want %q", got.Labels, pod.Labels)
	}
	if got.Ports != pod.Ports {
		t.Errorf("Ports = %q, want %q", got.Ports, pod.Ports)
	}
	if got.Restarts != pod.Restarts {
		t.Errorf("Restarts = %d, want %d", got.Restarts, pod.Restarts)
	}
	if got.PodIP != pod.PodIP {
		t.Errorf("PodIP = %q, want %q", got.PodIP, pod.PodIP)
	}
}

func TestListPods(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("pod-list", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, name := range []string{"pod-a", "pod-b"} {
		pod := &storage.PodRecord{
			Name:        name,
			ClusterName: "pod-list",
			Namespace:   "default",
			Image:       "nginx",
			Phase:       "Running",
			Labels:      "{}",
			Env:         "[]",
			Ports:       "[]",
			Command:     "[]",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SavePod(ctx, pod); err != nil {
			t.Fatalf("SavePod(%q): %v", name, err)
		}
	}

	// Add one in a different namespace
	pod := &storage.PodRecord{
		Name:        "pod-other",
		ClusterName: "pod-list",
		Namespace:   "kube-system",
		Image:       "coredns",
		Phase:       "Running",
		Labels:      "{}",
		Env:         "[]",
		Ports:       "[]",
		Command:     "[]",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SavePod(ctx, pod); err != nil {
		t.Fatalf("SavePod: %v", err)
	}

	pods, err := store.ListPods(ctx, "pod-list", "default")
	if err != nil {
		t.Fatalf("ListPods: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("got %d pods in default ns, want 2", len(pods))
	}
}

func TestListAllPods(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("pod-all", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	namespaces := []string{"default", "kube-system"}
	for _, ns := range namespaces {
		pod := &storage.PodRecord{
			Name:        "pod-in-" + ns,
			ClusterName: "pod-all",
			Namespace:   ns,
			Image:       "busybox",
			Phase:       "Running",
			Labels:      "{}",
			Env:         "[]",
			Ports:       "[]",
			Command:     "[]",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SavePod(ctx, pod); err != nil {
			t.Fatalf("SavePod: %v", err)
		}
	}

	pods, err := store.ListAllPods(ctx, "pod-all")
	if err != nil {
		t.Fatalf("ListAllPods: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("got %d pods, want 2", len(pods))
	}
}

func TestDeletePod(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("pod-del", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	pod := &storage.PodRecord{
		Name:        "deleteme",
		ClusterName: "pod-del",
		Namespace:   "default",
		Image:       "nginx",
		Phase:       "Running",
		Labels:      "{}",
		Env:         "[]",
		Ports:       "[]",
		Command:     "[]",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SavePod(ctx, pod); err != nil {
		t.Fatalf("SavePod: %v", err)
	}
	if err := store.DeletePod(ctx, "pod-del", "default", "deleteme"); err != nil {
		t.Fatalf("DeletePod: %v", err)
	}

	_, err := store.GetPod(ctx, "pod-del", "default", "deleteme")
	if err == nil {
		t.Fatal("expected error getting deleted pod, got nil")
	}
}

// --- Service CRUD ---

func TestSaveAndGetService(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("svc-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	svc := &storage.ServiceRecord{
		Name:        "web-svc",
		ClusterName: "svc-cluster",
		Namespace:   "default",
		Type:        "ClusterIP",
		Ports:       `[{"port":80,"targetPort":8080,"protocol":"tcp"}]`,
		Selector:    `{"app":"web"}`,
		ClusterIP:   "10.244.0.10",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveService(ctx, svc); err != nil {
		t.Fatalf("SaveService: %v", err)
	}

	got, err := store.GetService(ctx, "svc-cluster", "default", "web-svc")
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if got.Name != svc.Name {
		t.Errorf("Name = %q, want %q", got.Name, svc.Name)
	}
	if got.Type != svc.Type {
		t.Errorf("Type = %q, want %q", got.Type, svc.Type)
	}
	if got.ClusterIP != svc.ClusterIP {
		t.Errorf("ClusterIP = %q, want %q", got.ClusterIP, svc.ClusterIP)
	}
	if got.Ports != svc.Ports {
		t.Errorf("Ports = %q, want %q", got.Ports, svc.Ports)
	}
}

func TestListServicesAllNamespaces(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("svc-all", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, ns := range []string{"default", "monitoring"} {
		svc := &storage.ServiceRecord{
			Name:        "svc-in-" + ns,
			ClusterName: "svc-all",
			Namespace:   ns,
			Type:        "ClusterIP",
			Ports:       "[]",
			Selector:    "{}",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveService(ctx, svc); err != nil {
			t.Fatalf("SaveService: %v", err)
		}
	}

	svcs, err := store.ListServices(ctx, "svc-all", "")
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != 2 {
		t.Fatalf("got %d services, want 2", len(svcs))
	}
}

func TestListServicesFilterNamespace(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("svc-filter", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, ns := range []string{"default", "monitoring"} {
		svc := &storage.ServiceRecord{
			Name:        "svc-in-" + ns,
			ClusterName: "svc-filter",
			Namespace:   ns,
			Type:        "ClusterIP",
			Ports:       "[]",
			Selector:    "{}",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveService(ctx, svc); err != nil {
			t.Fatalf("SaveService: %v", err)
		}
	}

	svcs, err := store.ListServices(ctx, "svc-filter", "default")
	if err != nil {
		t.Fatalf("ListServices: %v", err)
	}
	if len(svcs) != 1 {
		t.Fatalf("got %d services in default ns, want 1", len(svcs))
	}
	if svcs[0].Namespace != "default" {
		t.Errorf("Namespace = %q, want default", svcs[0].Namespace)
	}
}

// --- Deployment CRUD ---

func TestSaveAndGetDeployment(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("dep-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	dep := &storage.DeploymentRecord{
		Name:          "web",
		ClusterName:   "dep-cluster",
		Namespace:     "default",
		Replicas:      3,
		ReadyReplicas: 2,
		Template:      `{"containers":[{"name":"web","image":"nginx"}]}`,
		Labels:        `{"app":"web"}`,
		Selector:      `{"app":"web"}`,
		Phase:         "Progressing",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := store.SaveDeployment(ctx, dep); err != nil {
		t.Fatalf("SaveDeployment: %v", err)
	}

	got, err := store.GetDeployment(ctx, "dep-cluster", "default", "web")
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if got.Name != dep.Name {
		t.Errorf("Name = %q, want %q", got.Name, dep.Name)
	}
	if got.Replicas != dep.Replicas {
		t.Errorf("Replicas = %d, want %d", got.Replicas, dep.Replicas)
	}
	if got.ReadyReplicas != dep.ReadyReplicas {
		t.Errorf("ReadyReplicas = %d, want %d", got.ReadyReplicas, dep.ReadyReplicas)
	}
	if got.Phase != dep.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, dep.Phase)
	}
}

func TestListDeployments(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("dep-list", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, name := range []string{"api", "web", "worker"} {
		dep := &storage.DeploymentRecord{
			Name:        name,
			ClusterName: "dep-list",
			Namespace:   "default",
			Replicas:    1,
			Template:    "{}",
			Labels:      "{}",
			Selector:    "{}",
			Phase:       "Available",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveDeployment(ctx, dep); err != nil {
			t.Fatalf("SaveDeployment(%q): %v", name, err)
		}
	}

	deps, err := store.ListDeployments(ctx, "dep-list", "default")
	if err != nil {
		t.Fatalf("ListDeployments: %v", err)
	}
	if len(deps) != 3 {
		t.Fatalf("got %d deployments, want 3", len(deps))
	}
}

// --- Event CRUD ---

func TestSaveAndListEvents(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("event-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for i := 0; i < 5; i++ {
		event := &storage.EventRecord{
			ClusterName: "event-cluster",
			Namespace:   "default",
			Kind:        "Pod",
			Name:        "web-pod",
			Type:        "Normal",
			Reason:      "Scheduled",
			Message:     "Successfully assigned",
			CreatedAt:   now + int64(i)*1000,
		}
		if err := store.SaveEvent(ctx, event); err != nil {
			t.Fatalf("SaveEvent: %v", err)
		}
	}

	events, err := store.ListEvents(ctx, "event-cluster", "default", 3)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("got %d events, want 3 (limit)", len(events))
	}
	// Newest first
	if events[0].CreatedAt <= events[1].CreatedAt {
		t.Error("events should be newest first")
	}
}

func TestListEventsByResource(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("event-res", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	// Events for two different resources
	for _, name := range []string{"web-pod", "api-pod"} {
		event := &storage.EventRecord{
			ClusterName: "event-res",
			Namespace:   "default",
			Kind:        "Pod",
			Name:        name,
			Type:        "Normal",
			Reason:      "Started",
			Message:     "Container started",
			CreatedAt:   now,
		}
		if err := store.SaveEvent(ctx, event); err != nil {
			t.Fatalf("SaveEvent: %v", err)
		}
	}

	events, err := store.ListEventsByResource(ctx, "event-res", "Pod", "web-pod", 10)
	if err != nil {
		t.Fatalf("ListEventsByResource: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events for web-pod, want 1", len(events))
	}
	if events[0].Name != "web-pod" {
		t.Errorf("Name = %q, want web-pod", events[0].Name)
	}
}

func TestListEventsAllNamespaces(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("event-all", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, ns := range []string{"default", "monitoring"} {
		event := &storage.EventRecord{
			ClusterName: "event-all",
			Namespace:   ns,
			Kind:        "Pod",
			Name:        "pod-in-" + ns,
			Type:        "Normal",
			Reason:      "Started",
			Message:     "ok",
			CreatedAt:   now,
		}
		if err := store.SaveEvent(ctx, event); err != nil {
			t.Fatalf("SaveEvent: %v", err)
		}
	}

	events, err := store.ListEvents(ctx, "event-all", "", 10)
	if err != nil {
		t.Fatalf("ListEvents all: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events across namespaces, want 2", len(events))
	}
}

// --- Addon CRUD ---

func TestSaveAndGetAddon(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("addon-cluster", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	addon := &storage.AddonRecord{
		Name:        "redis",
		ClusterName: "addon-cluster",
		Namespace:   "default",
		Version:     "7.0",
		Phase:       "Running",
		Config:      `{"maxmemory":"256mb"}`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.SaveAddon(ctx, addon); err != nil {
		t.Fatalf("SaveAddon: %v", err)
	}

	got, err := store.GetAddon(ctx, "addon-cluster", "default", "redis")
	if err != nil {
		t.Fatalf("GetAddon: %v", err)
	}
	if got.Name != addon.Name {
		t.Errorf("Name = %q, want %q", got.Name, addon.Name)
	}
	if got.Version != addon.Version {
		t.Errorf("Version = %q, want %q", got.Version, addon.Version)
	}
	if got.Phase != addon.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, addon.Phase)
	}
	if got.Config != addon.Config {
		t.Errorf("Config = %q, want %q", got.Config, addon.Config)
	}
}

func TestListAddons(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("addon-list", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	for _, name := range []string{"grafana", "prometheus", "redis"} {
		addon := &storage.AddonRecord{
			Name:        name,
			ClusterName: "addon-list",
			Namespace:   "default",
			Version:     "latest",
			Phase:       "Running",
			Config:      "{}",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := store.SaveAddon(ctx, addon); err != nil {
			t.Fatalf("SaveAddon(%q): %v", name, err)
		}
	}

	addons, err := store.ListAddons(ctx, "addon-list")
	if err != nil {
		t.Fatalf("ListAddons: %v", err)
	}
	if len(addons) != 3 {
		t.Fatalf("got %d addons, want 3", len(addons))
	}
}

func TestDeleteAddon(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	c := makeCluster("addon-del", now)
	if err := store.SaveCluster(ctx, c); err != nil {
		t.Fatalf("SaveCluster: %v", err)
	}

	addon := &storage.AddonRecord{
		Name:        "redis",
		ClusterName: "addon-del",
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
	if err := store.DeleteAddon(ctx, "addon-del", "default", "redis"); err != nil {
		t.Fatalf("DeleteAddon: %v", err)
	}

	_, err := store.GetAddon(ctx, "addon-del", "default", "redis")
	if err == nil {
		t.Fatal("expected error getting deleted addon, got nil")
	}
}

// --- Pipeline CRUD ---

func TestSaveAndGetPipelineRun(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	run := &storage.PipelineRunRecord{
		ID:            "run-001",
		ClusterName:   "pipeline-cluster",
		Status:        "running",
		StartedAt:     now,
		FinishedAt:    0,
		TotalTargets:  5,
		PassedTargets: 0,
		FailedTargets: 0,
		File:          "Torresmofile",
		TargetFilter:  "build",
	}
	if err := store.SavePipelineRun(ctx, run); err != nil {
		t.Fatalf("SavePipelineRun: %v", err)
	}

	got, err := store.GetPipelineRun(ctx, "run-001")
	if err != nil {
		t.Fatalf("GetPipelineRun: %v", err)
	}
	if got.ID != run.ID {
		t.Errorf("ID = %q, want %q", got.ID, run.ID)
	}
	if got.Status != run.Status {
		t.Errorf("Status = %q, want %q", got.Status, run.Status)
	}
	if got.TotalTargets != run.TotalTargets {
		t.Errorf("TotalTargets = %d, want %d", got.TotalTargets, run.TotalTargets)
	}
	if got.File != run.File {
		t.Errorf("File = %q, want %q", got.File, run.File)
	}
	if got.TargetFilter != run.TargetFilter {
		t.Errorf("TargetFilter = %q, want %q", got.TargetFilter, run.TargetFilter)
	}
}

func TestListPipelineRuns(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	for i := 0; i < 5; i++ {
		run := &storage.PipelineRunRecord{
			ID:        fmt.Sprintf("run-%03d", i),
			Status:    "success",
			StartedAt: now + int64(i)*1000,
			File:      "Torresmofile",
		}
		if err := store.SavePipelineRun(ctx, run); err != nil {
			t.Fatalf("SavePipelineRun: %v", err)
		}
	}

	runs, err := store.ListPipelineRuns(ctx, 3)
	if err != nil {
		t.Fatalf("ListPipelineRuns: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("got %d runs, want 3 (limit)", len(runs))
	}
	// Newest first
	if runs[0].ID != "run-004" {
		t.Errorf("first run = %q, want run-004 (newest)", runs[0].ID)
	}
}

func TestSavePipelineRunUpsert(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	run := &storage.PipelineRunRecord{
		ID:           "run-upsert",
		ClusterName:  "my-cluster",
		Status:       "running",
		StartedAt:    now,
		TotalTargets: 3,
		File:         "Torresmofile",
	}
	if err := store.SavePipelineRun(ctx, run); err != nil {
		t.Fatalf("SavePipelineRun: %v", err)
	}

	later := now + 10000
	run.Status = "success"
	run.FinishedAt = later
	run.PassedTargets = 3
	run.StartedAt = later // Should not change started_at
	if err := store.SavePipelineRun(ctx, run); err != nil {
		t.Fatalf("SavePipelineRun upsert: %v", err)
	}

	got, err := store.GetPipelineRun(ctx, "run-upsert")
	if err != nil {
		t.Fatalf("GetPipelineRun: %v", err)
	}
	if got.Status != "success" {
		t.Errorf("Status = %q, want success", got.Status)
	}
	if got.PassedTargets != 3 {
		t.Errorf("PassedTargets = %d, want 3", got.PassedTargets)
	}
	if got.StartedAt != now {
		t.Errorf("StartedAt = %d, want %d (should be preserved)", got.StartedAt, now)
	}
}

func TestSaveAndListPipelineTargets(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().UnixMilli()

	run := &storage.PipelineRunRecord{
		ID:        "run-targets",
		Status:    "running",
		StartedAt: now,
		File:      "Torresmofile",
	}
	if err := store.SavePipelineRun(ctx, run); err != nil {
		t.Fatalf("SavePipelineRun: %v", err)
	}

	targets := []storage.PipelineTargetRecord{
		{RunID: "run-targets", Name: "build", Status: "success", StartedAt: now, FinishedAt: now + 1000, Duration: 1000},
		{RunID: "run-targets", Name: "test", Status: "running", StartedAt: now + 1000},
		{RunID: "run-targets", Name: "deploy", Status: "pending", StartedAt: now + 2000},
	}
	for _, t2 := range targets {
		t2 := t2
		if err := store.SavePipelineTarget(ctx, &t2); err != nil {
			t.Fatalf("SavePipelineTarget(%q): %v", t2.Name, err)
		}
	}

	got, err := store.ListPipelineTargets(ctx, "run-targets")
	if err != nil {
		t.Fatalf("ListPipelineTargets: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d targets, want 3", len(got))
	}
	// Ordered by started_at
	if got[0].Name != "build" {
		t.Errorf("first target = %q, want build", got[0].Name)
	}
	if got[0].Status != "success" {
		t.Errorf("first target status = %q, want success", got[0].Status)
	}
}
