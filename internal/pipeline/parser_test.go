package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseExampleTorresmofile(t *testing.T) {
	// Find the example file relative to the repo root.
	path := filepath.Join("..", "..", "examples", "Torresmofile")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("example Torresmofile not found at %s: %v", path, err)
	}

	tf, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	if tf.Version != "0.1" {
		t.Errorf("version = %q, want %q", tf.Version, "0.1")
	}

	if len(tf.Bases) != 1 {
		t.Fatalf("got %d bases, want 1", len(tf.Bases))
	}
	if tf.Bases[0].Name != "golang" {
		t.Errorf("base name = %q, want %q", tf.Bases[0].Name, "golang")
	}
	if tf.Bases[0].Image != "golang:1.22-alpine" {
		t.Errorf("base image = %q, want %q", tf.Bases[0].Image, "golang:1.22-alpine")
	}
	if tf.Bases[0].Workdir != "/app" {
		t.Errorf("base workdir = %q, want %q", tf.Bases[0].Workdir, "/app")
	}

	wantTargets := []string{"lint", "test", "build", "integration-test", "docker", "deploy"}
	gotTargets := tf.TargetNames()
	if len(gotTargets) != len(wantTargets) {
		t.Fatalf("got %d targets %v, want %d %v", len(gotTargets), gotTargets, len(wantTargets), wantTargets)
	}
	for i, name := range wantTargets {
		if gotTargets[i] != name {
			t.Errorf("target[%d] = %q, want %q", i, gotTargets[i], name)
		}
	}

	// Check lint target.
	lint, err := tf.GetTarget("lint")
	if err != nil {
		t.Fatal(err)
	}
	if lint.From != "golang" {
		t.Errorf("lint.From = %q, want %q", lint.From, "golang")
	}
	if len(lint.Copies) != 1 {
		t.Fatalf("lint has %d copies, want 1", len(lint.Copies))
	}
	if lint.Copies[0].Src != "." || lint.Copies[0].Dest != "/app" {
		t.Errorf("lint copy = %+v", lint.Copies[0])
	}
	if len(lint.Runs) != 2 {
		t.Fatalf("lint has %d runs, want 2", len(lint.Runs))
	}
	if lint.Runs[0].Command != "go vet ./..." {
		t.Errorf("lint run[0] = %q, want %q", lint.Runs[0].Command, "go vet ./...")
	}
	if lint.Runs[1].Command != "golangci-lint run" {
		t.Errorf("lint run[1] = %q, want %q", lint.Runs[1].Command, "golangci-lint run")
	}

	// Check build target dependencies.
	build, err := tf.GetTarget("build")
	if err != nil {
		t.Fatal(err)
	}
	if len(build.DependsOn) != 2 || build.DependsOn[0] != "lint" || build.DependsOn[1] != "test" {
		t.Errorf("build.DependsOn = %v, want [lint test]", build.DependsOn)
	}
	if len(build.Artifacts) != 1 || build.Artifacts[0].Path != "/out/app" {
		t.Errorf("build artifacts = %+v", build.Artifacts)
	}

	// Check integration-test target.
	intTest, err := tf.GetTarget("integration-test")
	if err != nil {
		t.Fatal(err)
	}
	if !intTest.NeedsCluster {
		t.Error("integration-test.NeedsCluster should be true")
	}
	if len(intTest.Addons) != 2 || intTest.Addons[0] != "redis" || intTest.Addons[1] != "postgres" {
		t.Errorf("integration-test.Addons = %v", intTest.Addons)
	}
	env := intTest.GetEnvVars()
	if env["REDIS_URL"] != "redis.default:6379" {
		t.Errorf("REDIS_URL = %q", env["REDIS_URL"])
	}
	if env["DATABASE_URL"] != "postgres://dev:dev@postgres.default:5432/testdb" {
		t.Errorf("DATABASE_URL = %q", env["DATABASE_URL"])
	}

	// Check docker target (from_image + copy_artifact).
	docker, err := tf.GetTarget("docker")
	if err != nil {
		t.Fatal(err)
	}
	if docker.FromImage != "alpine:3.19" {
		t.Errorf("docker.FromImage = %q", docker.FromImage)
	}
	if len(docker.CopyArtifacts) != 1 {
		t.Fatalf("docker has %d copy_artifacts, want 1", len(docker.CopyArtifacts))
	}
	ca := docker.CopyArtifacts[0]
	if ca.From != "build" || ca.Src != "/out/app" || ca.Dest != "/usr/local/bin/app" {
		t.Errorf("docker copy_artifact = %+v", ca)
	}

	// Check ResolveImage.
	img, err := tf.ResolveImage(lint)
	if err != nil {
		t.Fatal(err)
	}
	if img != "golang:1.22-alpine" {
		t.Errorf("ResolveImage(lint) = %q", img)
	}
	img, err = tf.ResolveImage(docker)
	if err != nil {
		t.Fatal(err)
	}
	if img != "alpine:3.19" {
		t.Errorf("ResolveImage(docker) = %q", img)
	}

	// Check ResolveWorkdir.
	if wd := tf.ResolveWorkdir(lint); wd != "/app" {
		t.Errorf("ResolveWorkdir(lint) = %q", wd)
	}
	if wd := tf.ResolveWorkdir(docker); wd != "" {
		t.Errorf("ResolveWorkdir(docker) = %q, want empty", wd)
	}
}

func TestParseValidation(t *testing.T) {
	tests := []struct {
		name    string
		hcl     string
		wantErr string
	}{
		{
			name:    "missing version",
			hcl:     `target "x" { from_image = "alpine" }`,
			wantErr: "version",
		},
		{
			name: "duplicate target",
			hcl: `
version = "1"
target "a" { from_image = "alpine" }
target "a" { from_image = "alpine" }
`,
			wantErr: "duplicate target",
		},
		{
			name: "no from or from_image",
			hcl: `
version = "1"
target "a" {}
`,
			wantErr: "must have either",
		},
		{
			name: "undefined base",
			hcl: `
version = "1"
target "a" { from = "nonexistent" }
`,
			wantErr: "undefined base",
		},
		{
			name: "undefined dependency",
			hcl: `
version = "1"
target "a" {
  from_image = "alpine"
  depends_on = ["nonexistent"]
}
`,
			wantErr: "undefined target",
		},
		{
			name: "circular dependency",
			hcl: `
version = "1"
target "a" {
  from_image = "alpine"
  depends_on = ["b"]
}
target "b" {
  from_image = "alpine"
  depends_on = ["a"]
}
`,
			wantErr: "circular dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("test.hcl", []byte(tt.hcl))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetTargetNotFound(t *testing.T) {
	tf := &Torresmofile{Version: "1"}
	_, err := tf.GetTarget("nope")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestResolveImageError(t *testing.T) {
	tf := &Torresmofile{Version: "1"}
	target := &Target{Name: "x", From: "missing"}
	_, err := tf.ResolveImage(target)
	if err == nil {
		t.Error("expected error")
	}
}
