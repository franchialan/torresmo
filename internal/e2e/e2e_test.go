//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/addon/builtin"
	"github.com/torresmo-dev/torresmo/internal/cluster"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/pipeline"
	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/storage"
	"github.com/torresmo-dev/torresmo/internal/storage/sqlite"
)

// setupE2E creates all components with a mock runtime and ephemeral SQLite store.
func setupE2E(t *testing.T) (
	*cluster.Manager,
	*orchestrator.Orchestrator,
	*addon.Registry,
	*runtime.MockRuntime,
	storage.Store,
) {
	t.Helper()

	rt := runtime.NewMockRuntime()
	dir := t.TempDir()
	store, err := sqlite.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	mgr := cluster.NewManager(rt, store)
	orch := orchestrator.New(rt, store)

	registry := addon.NewRegistry(orch, store)
	builtin.RegisterAll(registry, orch, store)

	return mgr, orch, registry, rt, store
}

func TestFullClusterLifecycle(t *testing.T) {
	mgr, orch, registry, rt, _ := setupE2E(t)
	ctx := context.Background()

	// 1. Create cluster.
	err := mgr.CreateCluster(ctx, "e2e-test", 3, "standard", "10.244.0.0/16")
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	info, err := mgr.GetCluster(ctx, "e2e-test")
	if err != nil {
		t.Fatalf("get cluster: %v", err)
	}
	if info.Phase != "Running" {
		t.Errorf("expected Running, got %s", info.Phase)
	}
	if info.Nodes != 3 {
		t.Errorf("expected 3 nodes, got %d", info.Nodes)
	}

	nodes, err := mgr.GetNodes(ctx, "e2e-test")
	if err != nil {
		t.Fatalf("get nodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
	}

	if rt.ContainerCount() != 3 {
		t.Errorf("expected 3 containers, got %d", rt.ContainerCount())
	}

	// 2. Install addons.
	err = registry.Install(ctx, "redis", addon.InstallOptions{
		ClusterName: "e2e-test",
		Namespace:   "default",
	})
	if err != nil {
		t.Fatalf("install redis: %v", err)
	}

	err = registry.Install(ctx, "postgres", addon.InstallOptions{
		ClusterName: "e2e-test",
		Namespace:   "default",
		Config:      map[string]string{"user": "testuser", "database": "testdb"},
	})
	if err != nil {
		t.Fatalf("install postgres: %v", err)
	}

	addons, err := registry.List(ctx, "e2e-test")
	if err != nil {
		t.Fatalf("list addons: %v", err)
	}
	if len(addons) != 2 {
		t.Errorf("expected 2 addons, got %d", len(addons))
	}

	// 3 node containers + 2 addon pod containers.
	if rt.ContainerCount() != 5 {
		t.Errorf("expected 5 containers, got %d", rt.ContainerCount())
	}

	// 3. Create namespace and deploy.
	err = orch.CreateNamespace(ctx, "e2e-test", "staging")
	if err != nil {
		t.Fatalf("create namespace: %v", err)
	}

	err = orch.ApplyDeployment(ctx, "e2e-test", &v1.Deployment{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Deployment"},
		ObjectMeta: v1.ObjectMeta{Name: "web-app", Namespace: "staging"},
		Spec: v1.DeploymentSpec{
			Replicas: 2,
			Template: v1.PodTemplate{
				ObjectMeta: v1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec: v1.PodSpec{
					Containers: []v1.ContainerSpec{{Name: "web", Image: "nginx:latest"}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("apply deployment: %v", err)
	}

	pods, err := orch.ListPods(ctx, "e2e-test", "staging")
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if len(pods) != 2 {
		t.Errorf("expected 2 staging pods, got %d", len(pods))
	}

	// 4. Apply service.
	err = orch.ApplyService(ctx, "e2e-test", &v1.Service{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{Name: "web-app", Namespace: "staging"},
		Spec: v1.ServiceSpec{
			Ports:    []v1.ServicePort{{Port: 80, TargetPort: 8080}},
			Selector: map[string]string{"app": "web"},
		},
	})
	if err != nil {
		t.Fatalf("apply service: %v", err)
	}

	services, err := orch.ListServices(ctx, "e2e-test", "staging")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}

	// 5. Verify events were emitted.
	events, err := orch.ListEvents(ctx, "e2e-test", "", 50)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected events to be emitted")
	}

	// 6. Uninstall addon.
	err = registry.Uninstall(ctx, "redis", addon.UninstallOptions{
		ClusterName: "e2e-test",
		Namespace:   "default",
	})
	if err != nil {
		t.Fatalf("uninstall redis: %v", err)
	}

	addons, err = registry.List(ctx, "e2e-test")
	if err != nil {
		t.Fatalf("list addons after uninstall: %v", err)
	}
	if len(addons) != 1 {
		t.Errorf("expected 1 addon after uninstall, got %d", len(addons))
	}

	// 7. Delete cluster.
	err = mgr.DeleteCluster(ctx, "e2e-test")
	if err != nil {
		t.Fatalf("delete cluster: %v", err)
	}

	clusters, err := mgr.ListClusters(ctx)
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	if len(clusters) != 0 {
		t.Errorf("expected 0 clusters, got %d", len(clusters))
	}
}

func TestApplyManifestFile(t *testing.T) {
	mgr, orch, _, _, _ := setupE2E(t)
	ctx := context.Background()

	_ = mgr.CreateCluster(ctx, "manifest-test", 2, "light", "10.244.0.0/16")

	dir := t.TempDir()

	depYAML := `apiVersion: torresmo.io/v1
kind: Deployment
metadata:
  name: test-app
  namespace: default
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: app
          image: alpine:3.19
`
	if err := os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte(depYAML), 0o644); err != nil {
		t.Fatalf("write deployment.yaml: %v", err)
	}

	svcYAML := `apiVersion: torresmo.io/v1
kind: Service
metadata:
  name: test-app
  namespace: default
spec:
  ports:
    - port: 80
      targetPort: 8080
  selector:
    app: test
`
	if err := os.WriteFile(filepath.Join(dir, "service.yaml"), []byte(svcYAML), 0o644); err != nil {
		t.Fatalf("write service.yaml: %v", err)
	}

	err := orch.ApplyManifestFile(ctx, "manifest-test", dir)
	if err != nil {
		t.Fatalf("apply manifest: %v", err)
	}

	pods, err := orch.ListPods(ctx, "manifest-test", "default")
	if err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if len(pods) != 1 {
		t.Errorf("expected 1 pod, got %d", len(pods))
	}

	services, err := orch.ListServices(ctx, "manifest-test", "default")
	if err != nil {
		t.Fatalf("list services: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}
}

func TestPipelineParsing(t *testing.T) {
	tf, err := pipeline.ParseFile("../../examples/Torresmofile")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	names := tf.TargetNames()
	if len(names) != 6 {
		t.Errorf("expected 6 targets, got %d: %v", len(names), names)
	}

	dag, err := pipeline.BuildDAG(tf, "build")
	if err != nil {
		t.Fatalf("build dag: %v", err)
	}

	order := dag.Order()
	buildIdx := -1
	for i, name := range order {
		if name == "build" {
			buildIdx = i
		}
	}
	if buildIdx < 2 {
		t.Errorf("build should be after lint and test, got index %d in %v", buildIdx, order)
	}

	levels := dag.Levels()
	if len(levels) < 2 {
		t.Errorf("expected at least 2 levels, got %d", len(levels))
	}
}
