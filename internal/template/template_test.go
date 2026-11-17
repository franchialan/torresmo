package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestList(t *testing.T) {
	templates := List()
	if len(templates) < 8 {
		t.Errorf("List() returned %d templates, want at least 8", len(templates))
	}

	names := make(map[string]bool)
	for _, tmpl := range templates {
		names[tmpl.Name] = true
	}
	for _, want := range []string{"go-api", "node-fullstack", "java-spring", "python-ml", "minimal", "microservices", "data-pipeline", "fullstack"} {
		if !names[want] {
			t.Errorf("List() missing template %q", want)
		}
	}
}

func TestGet(t *testing.T) {
	for _, name := range []string{"go-api", "node-fullstack", "java-spring", "python-ml", "minimal", "microservices", "data-pipeline", "fullstack"} {
		tmpl, err := Get(name)
		if err != nil {
			t.Errorf("Get(%q): %v", name, err)
			continue
		}
		if tmpl.Name != name {
			t.Errorf("Get(%q).Name = %q", name, tmpl.Name)
		}
		if tmpl.Description == "" {
			t.Errorf("Get(%q).Description is empty", name)
		}
		if tmpl.Nodes <= 0 {
			t.Errorf("Get(%q).Nodes = %d, want > 0", name, tmpl.Nodes)
		}
		if tmpl.Profile == "" {
			t.Errorf("Get(%q).Profile is empty", name)
		}
	}
}

func TestGetNotFound(t *testing.T) {
	_, err := Get("nonexistent-template-xyz")
	if err == nil {
		t.Fatal("expected error for unknown template, got nil")
	}
}

func TestToClusterConfig(t *testing.T) {
	tmpl, err := Get("go-api")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	cluster := tmpl.ToClusterConfig("my-cluster")

	if cluster.Name != "my-cluster" {
		t.Errorf("Name = %q, want my-cluster", cluster.Name)
	}
	if cluster.APIVersion != "torresmo.io/v1" {
		t.Errorf("APIVersion = %q, want torresmo.io/v1", cluster.APIVersion)
	}
	if cluster.Kind != "Cluster" {
		t.Errorf("Kind = %q, want Cluster", cluster.Kind)
	}
	if cluster.Spec.Profile != tmpl.Profile {
		t.Errorf("Profile = %q, want %q", cluster.Spec.Profile, tmpl.Profile)
	}

	// go-api has 3 nodes: 1 control-plane + 2 workers
	if len(cluster.Spec.Nodes) != 2 {
		t.Fatalf("got %d node specs, want 2 (control-plane + worker)", len(cluster.Spec.Nodes))
	}
	if cluster.Spec.Nodes[0].Role != "control-plane" {
		t.Errorf("first node role = %q, want control-plane", cluster.Spec.Nodes[0].Role)
	}
	if cluster.Spec.Nodes[1].Role != "worker" {
		t.Errorf("second node role = %q, want worker", cluster.Spec.Nodes[1].Role)
	}
	if cluster.Spec.Nodes[1].Count != 2 {
		t.Errorf("worker count = %d, want 2", cluster.Spec.Nodes[1].Count)
	}

	if len(cluster.Spec.Addons) != len(tmpl.Addons) {
		t.Errorf("got %d addons, want %d", len(cluster.Spec.Addons), len(tmpl.Addons))
	}
	for i, addon := range cluster.Spec.Addons {
		if addon.Name != tmpl.Addons[i] {
			t.Errorf("addon[%d].Name = %q, want %q", i, addon.Name, tmpl.Addons[i])
		}
	}
}

func TestToClusterConfigMinimal(t *testing.T) {
	tmpl, err := Get("minimal")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	cluster := tmpl.ToClusterConfig("mini")

	// minimal has 1 node: only control-plane, no worker spec
	if len(cluster.Spec.Nodes) != 1 {
		t.Fatalf("got %d node specs, want 1 (control-plane only)", len(cluster.Spec.Nodes))
	}
	if cluster.Spec.Nodes[0].Role != "control-plane" {
		t.Errorf("node role = %q, want control-plane", cluster.Spec.Nodes[0].Role)
	}

	// minimal has no addons
	if len(cluster.Spec.Addons) != 0 {
		t.Errorf("got %d addons, want 0", len(cluster.Spec.Addons))
	}
}

func TestSaveAndGetCustom(t *testing.T) {
	// Override HOME so Save() writes to a temp directory
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	t.Cleanup(func() { _ = os.Setenv("HOME", origHome) })

	custom := Template{
		Name:        "my-custom",
		Description: "Custom template for testing",
		Nodes:       2,
		Profile:     "heavy",
		Addons:      []string{"redis"},
	}

	if err := Save(custom); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify the file exists
	path := filepath.Join(dir, ".torresmo", "templates", "my-custom.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at %s: %v", path, err)
	}

	got, err := Get("my-custom")
	if err != nil {
		t.Fatalf("Get custom: %v", err)
	}
	if got.Name != custom.Name {
		t.Errorf("Name = %q, want %q", got.Name, custom.Name)
	}
	if got.Profile != custom.Profile {
		t.Errorf("Profile = %q, want %q", got.Profile, custom.Profile)
	}
	if got.Nodes != custom.Nodes {
		t.Errorf("Nodes = %d, want %d", got.Nodes, custom.Nodes)
	}
}
