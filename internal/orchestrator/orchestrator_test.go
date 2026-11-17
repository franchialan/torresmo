package orchestrator

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/storage"
	"github.com/torresmo-dev/torresmo/internal/storage/sqlite"
)

func setupTest(t *testing.T) (*Orchestrator, *runtime.MockRuntime, storage.Store) {
	t.Helper()
	rt := runtime.NewMockRuntime()
	dir := t.TempDir()
	store, err := sqlite.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	now := time.Now().Unix()
	_ = store.SaveCluster(ctx, &storage.ClusterRecord{
		Name: "test", Runtime: "mock", Profile: "standard", Phase: "Running",
		NodeCount: 1, ReadyNodes: 1, PodCIDR: "10.244.0.0/16",
		CreatedAt: now, UpdatedAt: now,
	})
	_ = store.SaveNode(ctx, &storage.NodeRecord{
		Name: "test-worker-1", ClusterName: "test", Role: "worker",
		ContainerID: "mock-0", Phase: "Running", CreatedAt: now, UpdatedAt: now,
	})

	orch := New(rt, store)
	return orch, rt, store
}

func testPod(name, image string) *v1.Pod {
	return &v1.Pod{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{Name: name, Image: image},
			},
		},
	}
}

func TestCreateNamespace(t *testing.T) {
	orch, _, store := setupTest(t)
	ctx := context.Background()

	if err := orch.CreateNamespace(ctx, "test", "staging"); err != nil {
		t.Fatalf("CreateNamespace: %v", err)
	}

	ns, err := store.GetNamespace(ctx, "test", "staging")
	if err != nil {
		t.Fatalf("GetNamespace: %v", err)
	}
	if ns.Phase != "Active" {
		t.Errorf("namespace phase = %q, want Active", ns.Phase)
	}
}

func TestCreateNamespaceDuplicate(t *testing.T) {
	orch, _, _ := setupTest(t)
	ctx := context.Background()

	if err := orch.CreateNamespace(ctx, "test", "staging"); err != nil {
		t.Fatal(err)
	}
	err := orch.CreateNamespace(ctx, "test", "staging")
	if err == nil {
		t.Fatal("expected error for duplicate namespace, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists' substring", err)
	}
}

func TestCreatePod(t *testing.T) {
	orch, rt, store := setupTest(t)
	ctx := context.Background()

	pod := testPod("nginx", "alpine:3.19")
	if err := orch.CreatePod(ctx, "test", pod); err != nil {
		t.Fatalf("CreatePod: %v", err)
	}

	if rt.ContainerCount() != 1 {
		t.Errorf("container count = %d, want 1", rt.ContainerCount())
	}

	rec, err := store.GetPod(ctx, "test", "default", "nginx")
	if err != nil {
		t.Fatalf("GetPod: %v", err)
	}
	if rec.Phase != "Running" {
		t.Errorf("pod phase = %q, want Running", rec.Phase)
	}
	if rec.Image != "alpine:3.19" {
		t.Errorf("pod image = %q, want alpine:3.19", rec.Image)
	}
	if rec.Node != "test-worker-1" {
		t.Errorf("pod node = %q, want test-worker-1", rec.Node)
	}
}

func TestCreatePodAutoNamespace(t *testing.T) {
	orch, _, store := setupTest(t)
	ctx := context.Background()

	pod := &v1.Pod{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{Name: "auto-ns-pod"},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{{Name: "c", Image: "alpine:3.19"}},
		},
	}
	if err := orch.CreatePod(ctx, "test", pod); err != nil {
		t.Fatalf("CreatePod: %v", err)
	}

	ns, err := store.GetNamespace(ctx, "test", "default")
	if err != nil {
		t.Fatalf("default namespace should be auto-created: %v", err)
	}
	if ns.Phase != "Active" {
		t.Errorf("namespace phase = %q, want Active", ns.Phase)
	}
}

func TestDeletePod(t *testing.T) {
	orch, rt, store := setupTest(t)
	ctx := context.Background()

	pod := testPod("to-delete", "alpine:3.19")
	if err := orch.CreatePod(ctx, "test", pod); err != nil {
		t.Fatal(err)
	}
	if rt.ContainerCount() != 1 {
		t.Fatalf("setup: container count = %d, want 1", rt.ContainerCount())
	}

	if err := orch.DeletePod(ctx, "test", "default", "to-delete"); err != nil {
		t.Fatalf("DeletePod: %v", err)
	}

	if rt.ContainerCount() != 0 {
		t.Errorf("container count after delete = %d, want 0", rt.ContainerCount())
	}
	_, err := store.GetPod(ctx, "test", "default", "to-delete")
	if err == nil {
		t.Error("expected pod to be deleted from store")
	}
}

func TestApplyDeployment(t *testing.T) {
	orch, rt, store := setupTest(t)
	ctx := context.Background()

	dep := &v1.Deployment{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Deployment"},
		ObjectMeta: v1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: v1.DeploymentSpec{
			Replicas: 2,
			Template: v1.PodTemplate{
				ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec:       v1.PodSpec{Containers: []v1.ContainerSpec{{Name: "web", Image: "alpine:3.19"}}},
			},
		},
	}
	if err := orch.ApplyDeployment(ctx, "test", dep); err != nil {
		t.Fatalf("ApplyDeployment: %v", err)
	}

	if rt.ContainerCount() != 2 {
		t.Errorf("container count = %d, want 2", rt.ContainerCount())
	}

	rec, err := store.GetDeployment(ctx, "test", "default", "web")
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if rec.Replicas != 2 {
		t.Errorf("replicas = %d, want 2", rec.Replicas)
	}
	if rec.Phase != "Available" {
		t.Errorf("phase = %q, want Available", rec.Phase)
	}
}

func TestApplyDeploymentScale(t *testing.T) {
	orch, rt, store := setupTest(t)
	ctx := context.Background()

	dep := &v1.Deployment{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Deployment"},
		ObjectMeta: v1.ObjectMeta{Name: "scale-me", Namespace: "default"},
		Spec: v1.DeploymentSpec{
			Replicas: 2,
			Template: v1.PodTemplate{
				ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"app": "scale"}},
				Spec:       v1.PodSpec{Containers: []v1.ContainerSpec{{Name: "app", Image: "alpine:3.19"}}},
			},
		},
	}
	if err := orch.ApplyDeployment(ctx, "test", dep); err != nil {
		t.Fatal(err)
	}
	if rt.ContainerCount() != 2 {
		t.Fatalf("initial containers = %d, want 2", rt.ContainerCount())
	}

	dep.Spec.Replicas = 3
	if err := orch.ApplyDeployment(ctx, "test", dep); err != nil {
		t.Fatalf("scale up: %v", err)
	}
	if rt.ContainerCount() != 3 {
		t.Errorf("scaled containers = %d, want 3", rt.ContainerCount())
	}

	rec, err := store.GetDeployment(ctx, "test", "default", "scale-me")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Replicas != 3 {
		t.Errorf("replicas = %d, want 3", rec.Replicas)
	}
}

func TestDeleteDeployment(t *testing.T) {
	orch, rt, store := setupTest(t)
	ctx := context.Background()

	dep := &v1.Deployment{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Deployment"},
		ObjectMeta: v1.ObjectMeta{Name: "temp", Namespace: "default"},
		Spec: v1.DeploymentSpec{
			Replicas: 2,
			Template: v1.PodTemplate{
				ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"app": "temp"}},
				Spec:       v1.PodSpec{Containers: []v1.ContainerSpec{{Name: "app", Image: "alpine:3.19"}}},
			},
		},
	}
	if err := orch.ApplyDeployment(ctx, "test", dep); err != nil {
		t.Fatal(err)
	}

	if err := orch.DeleteDeployment(ctx, "test", "default", "temp"); err != nil {
		t.Fatalf("DeleteDeployment: %v", err)
	}

	if rt.ContainerCount() != 0 {
		t.Errorf("container count after delete = %d, want 0", rt.ContainerCount())
	}
	_, err := store.GetDeployment(ctx, "test", "default", "temp")
	if err == nil {
		t.Error("expected deployment to be deleted from store")
	}
}

func TestApplyService(t *testing.T) {
	orch, _, store := setupTest(t)
	ctx := context.Background()

	svc := &v1.Service{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{Name: "my-svc", Namespace: "default"},
		Spec: v1.ServiceSpec{
			Type:     "ClusterIP",
			Ports:    []v1.ServicePort{{Name: "http", Port: 80, TargetPort: 8080}},
			Selector: map[string]string{"app": "web"},
		},
	}
	if err := orch.ApplyService(ctx, "test", svc); err != nil {
		t.Fatalf("ApplyService: %v", err)
	}

	rec, err := store.GetService(ctx, "test", "default", "my-svc")
	if err != nil {
		t.Fatalf("GetService: %v", err)
	}
	if rec.Type != "ClusterIP" {
		t.Errorf("service type = %q, want ClusterIP", rec.Type)
	}
	if !strings.HasPrefix(rec.ClusterIP, "10.244.0.") {
		t.Errorf("service ClusterIP = %q, want 10.244.0.x prefix", rec.ClusterIP)
	}
}

func TestApplyManifestPod(t *testing.T) {
	orch, rt, store := setupTest(t)
	ctx := context.Background()

	manifest := []byte(`
apiVersion: torresmo.io/v1
kind: Pod
metadata:
  name: manifest-pod
  namespace: default
spec:
  containers:
    - name: app
      image: alpine:3.19
`)
	if err := orch.ApplyManifest(ctx, "test", manifest); err != nil {
		t.Fatalf("ApplyManifest pod: %v", err)
	}

	if rt.ContainerCount() != 1 {
		t.Errorf("container count = %d, want 1", rt.ContainerCount())
	}
	rec, err := store.GetPod(ctx, "test", "default", "manifest-pod")
	if err != nil {
		t.Fatalf("GetPod: %v", err)
	}
	if rec.Image != "alpine:3.19" {
		t.Errorf("image = %q, want alpine:3.19", rec.Image)
	}
}

func TestApplyManifestDeployment(t *testing.T) {
	orch, rt, store := setupTest(t)
	ctx := context.Background()

	manifest := []byte(`
apiVersion: torresmo.io/v1
kind: Deployment
metadata:
  name: yaml-dep
  namespace: default
spec:
  replicas: 2
  template:
    metadata:
      labels:
        app: yaml-dep
    spec:
      containers:
        - name: app
          image: alpine:3.19
`)
	if err := orch.ApplyManifest(ctx, "test", manifest); err != nil {
		t.Fatalf("ApplyManifest deployment: %v", err)
	}

	if rt.ContainerCount() != 2 {
		t.Errorf("container count = %d, want 2", rt.ContainerCount())
	}
	rec, err := store.GetDeployment(ctx, "test", "default", "yaml-dep")
	if err != nil {
		t.Fatalf("GetDeployment: %v", err)
	}
	if rec.Replicas != 2 {
		t.Errorf("replicas = %d, want 2", rec.Replicas)
	}
}

func TestApplyManifestUnsupportedKind(t *testing.T) {
	orch, _, _ := setupTest(t)
	ctx := context.Background()

	manifest := []byte(`
apiVersion: torresmo.io/v1
kind: ConfigMap
metadata:
  name: test
`)
	err := orch.ApplyManifest(ctx, "test", manifest)
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
	if !strings.Contains(err.Error(), "unsupported kind") {
		t.Errorf("error = %q, want 'unsupported kind' substring", err)
	}
}

func TestListEvents(t *testing.T) {
	orch, _, _ := setupTest(t)
	ctx := context.Background()

	if err := orch.CreateNamespace(ctx, "test", "staging"); err != nil {
		t.Fatal(err)
	}

	events, err := orch.ListEvents(ctx, "test", "", 50)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one event after creating a namespace")
	}

	found := false
	for _, e := range events {
		if e.Kind == "Namespace" && e.Name == "staging" && e.Reason == "Created" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a 'Created' event for namespace 'staging'")
	}
}
