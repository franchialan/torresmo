package addon

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/storage"
	"github.com/torresmo-dev/torresmo/internal/storage/sqlite"
)

// fakeAddon is a minimal Addon implementation for testing the registry.
type fakeAddon struct {
	BaseAddon
	name string
}

func newFakeAddon(name string, orch *orchestrator.Orchestrator, store storage.Store) *fakeAddon {
	return &fakeAddon{
		BaseAddon: NewBaseAddon(orch, store),
		name:      name,
	}
}

func (f *fakeAddon) Name() string              { return f.name }
func (f *fakeAddon) DisplayName() string        { return strings.Title(f.name) }
func (f *fakeAddon) Description() string        { return f.name + " addon" }
func (f *fakeAddon) DefaultVersion() string     { return "1.0" }
func (f *fakeAddon) AvailableVersions() []string { return []string{"1.0", "2.0"} }

func (f *fakeAddon) Install(ctx context.Context, opts InstallOptions) error {
	pod := &v1.Pod{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{Name: f.name + "-0", Namespace: opts.Namespace, Labels: map[string]string{"app": f.name}},
		Spec:       v1.PodSpec{Containers: []v1.ContainerSpec{{Name: f.name, Image: "alpine:3.19"}}},
	}
	if err := f.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return err
	}
	svc := &v1.Service{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{Name: f.name, Namespace: opts.Namespace},
		Spec: v1.ServiceSpec{
			Type:     "ClusterIP",
			Ports:    []v1.ServicePort{{Name: f.name, Port: 8080, TargetPort: 8080}},
			Selector: map[string]string{"app": f.name},
		},
	}
	return f.Orchestrator().ApplyService(ctx, opts.ClusterName, svc)
}

func (f *fakeAddon) Uninstall(ctx context.Context, opts UninstallOptions) error {
	_ = f.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, f.name)
	_ = f.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, f.name+"-0")
	return nil
}

func (f *fakeAddon) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := f.Orchestrator().GetPod(ctx, clusterName, namespace, f.name+"-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}

func setupAddonTest(t *testing.T) (*Registry, *orchestrator.Orchestrator, storage.Store) {
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

	orch := orchestrator.New(rt, store)
	reg := NewRegistry(orch, store)
	return reg, orch, store
}

func TestRegistryAvailable(t *testing.T) {
	reg, orch, store := setupAddonTest(t)
	reg.Register(newFakeAddon("alpha", orch, store))
	reg.Register(newFakeAddon("beta", orch, store))

	avail := reg.Available()
	if len(avail) != 2 {
		t.Fatalf("Available() returned %d addons, want 2", len(avail))
	}
	if avail[0] != "alpha" || avail[1] != "beta" {
		t.Errorf("Available() = %v, want [alpha beta]", avail)
	}
}

func TestRegistryGet(t *testing.T) {
	reg, orch, store := setupAddonTest(t)
	reg.Register(newFakeAddon("redis", orch, store))

	a, err := reg.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if a.Name() != "redis" {
		t.Errorf("addon name = %q, want redis", a.Name())
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	reg, _, _ := setupAddonTest(t)

	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown addon, got nil")
	}
	if !strings.Contains(err.Error(), "unknown addon") {
		t.Errorf("error = %q, want 'unknown addon' substring", err)
	}
}

func TestRegistryInstall(t *testing.T) {
	reg, orch, store := setupAddonTest(t)
	reg.Register(newFakeAddon("mydb", orch, store))
	ctx := context.Background()

	err := reg.Install(ctx, "mydb", InstallOptions{
		ClusterName: "test",
		Namespace:   "default",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	rec, err := store.GetAddon(ctx, "test", "default", "mydb")
	if err != nil {
		t.Fatalf("GetAddon: %v", err)
	}
	if rec.Phase != "Running" {
		t.Errorf("addon phase = %q, want Running", rec.Phase)
	}
	if rec.Version != "1.0" {
		t.Errorf("addon version = %q, want 1.0", rec.Version)
	}

	pod, err := store.GetPod(ctx, "test", "default", "mydb-0")
	if err != nil {
		t.Fatalf("addon pod not found: %v", err)
	}
	if pod.Phase != "Running" {
		t.Errorf("addon pod phase = %q, want Running", pod.Phase)
	}

	svc, err := store.GetService(ctx, "test", "default", "mydb")
	if err != nil {
		t.Fatalf("addon service not found: %v", err)
	}
	if svc.Type != "ClusterIP" {
		t.Errorf("service type = %q, want ClusterIP", svc.Type)
	}
}

func TestRegistryInstallDuplicate(t *testing.T) {
	reg, orch, store := setupAddonTest(t)
	reg.Register(newFakeAddon("cache", orch, store))
	ctx := context.Background()

	opts := InstallOptions{ClusterName: "test", Namespace: "default"}
	if err := reg.Install(ctx, "cache", opts); err != nil {
		t.Fatal(err)
	}

	err := reg.Install(ctx, "cache", opts)
	if err == nil {
		t.Fatal("expected error for duplicate install, got nil")
	}
	if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("error = %q, want 'already installed' substring", err)
	}
}

func TestRegistryUninstall(t *testing.T) {
	reg, orch, store := setupAddonTest(t)
	reg.Register(newFakeAddon("temp", orch, store))
	ctx := context.Background()

	opts := InstallOptions{ClusterName: "test", Namespace: "default"}
	if err := reg.Install(ctx, "temp", opts); err != nil {
		t.Fatal(err)
	}

	err := reg.Uninstall(ctx, "temp", UninstallOptions{ClusterName: "test", Namespace: "default"})
	if err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	_, err = store.GetAddon(ctx, "test", "default", "temp")
	if err == nil {
		t.Error("expected addon record to be deleted after uninstall")
	}
}

func TestRegistryList(t *testing.T) {
	reg, orch, store := setupAddonTest(t)
	reg.Register(newFakeAddon("aaa", orch, store))
	reg.Register(newFakeAddon("bbb", orch, store))
	ctx := context.Background()

	if err := reg.Install(ctx, "aaa", InstallOptions{ClusterName: "test", Namespace: "default"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Install(ctx, "bbb", InstallOptions{ClusterName: "test", Namespace: "default"}); err != nil {
		t.Fatal(err)
	}

	addons, err := reg.List(ctx, "test")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(addons) != 2 {
		t.Fatalf("List returned %d addons, want 2", len(addons))
	}

	names := map[string]bool{}
	for _, a := range addons {
		names[a.Name] = true
	}
	if !names["aaa"] || !names["bbb"] {
		t.Errorf("List returned %v, want aaa and bbb", addons)
	}
}
