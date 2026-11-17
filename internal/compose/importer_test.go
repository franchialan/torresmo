package compose

import (
	"os"
	"path/filepath"
	"testing"
)

const basicCompose = `version: "3"
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    environment:
      - PORT=8080
  api:
    image: myapp/api:v1
    ports:
      - "3000:3000"
    environment:
      APP_ENV: production
      DB_HOST: localhost
`

const addonCompose = `version: "3"
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  db:
    image: postgres:15
    ports:
      - "5432:5432"
    environment:
      POSTGRES_PASSWORD: secret
`

const buildOnlyCompose = `version: "3"
services:
  app:
    build: .
    ports:
      - "8080:80"
  api:
    image: myapp/api:v1
    ports:
      - "3000:3000"
`

const portFormatsCompose = `version: "3"
services:
  svc1:
    image: app:latest
    ports:
      - "8080"
  svc2:
    image: app:latest
    ports:
      - "8080:80"
  svc3:
    image: app:latest
    ports:
      - "8080:80/tcp"
`

const envFormatsCompose = `version: "3"
services:
  list-env:
    image: app:latest
    environment:
      - FOO=bar
      - BAZ=qux
  map-env:
    image: app:latest
    environment:
      KEY1: value1
      KEY2: value2
`

func writeComposeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "docker-compose.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	return path
}

func TestImportBasicCompose(t *testing.T) {
	path := writeComposeFile(t, basicCompose)
	result, err := Import(path)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(result.Deployments) != 2 {
		t.Fatalf("got %d deployments, want 2", len(result.Deployments))
	}
	if len(result.Services) != 2 {
		t.Fatalf("got %d services, want 2", len(result.Services))
	}

	depNames := make(map[string]bool)
	for _, d := range result.Deployments {
		depNames[d.Name] = true
		if d.Spec.Replicas != 1 {
			t.Errorf("deployment %q replicas = %d, want 1", d.Name, d.Spec.Replicas)
		}
		if d.APIVersion != "torresmo.io/v1" {
			t.Errorf("deployment %q apiVersion = %q, want torresmo.io/v1", d.Name, d.APIVersion)
		}
		if d.Kind != "Deployment" {
			t.Errorf("deployment %q kind = %q, want Deployment", d.Name, d.Kind)
		}
	}
	if !depNames["web"] || !depNames["api"] {
		t.Error("expected deployments for web and api")
	}
}

func TestImportDetectsAddons(t *testing.T) {
	path := writeComposeFile(t, addonCompose)
	result, err := Import(path)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(result.KnownAddons) != 2 {
		t.Fatalf("got %d known addons, want 2 (redis + postgres)", len(result.KnownAddons))
	}

	addonSet := make(map[string]bool)
	for _, a := range result.KnownAddons {
		addonSet[a] = true
	}
	if !addonSet["redis"] {
		t.Error("expected redis in KnownAddons")
	}
	if !addonSet["postgres"] {
		t.Error("expected postgres in KnownAddons")
	}

	// Only web should be a deployment (redis and postgres are addons)
	if len(result.Deployments) != 1 {
		t.Errorf("got %d deployments, want 1 (only web)", len(result.Deployments))
	}
}

func TestImportSkipsBuild(t *testing.T) {
	path := writeComposeFile(t, buildOnlyCompose)
	result, err := Import(path)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	// app has build but no image, should be skipped
	if len(result.Deployments) != 1 {
		t.Fatalf("got %d deployments, want 1 (only api)", len(result.Deployments))
	}
	if result.Deployments[0].Name != "api" {
		t.Errorf("deployment name = %q, want api", result.Deployments[0].Name)
	}

	hasWarning := false
	for _, w := range result.Warnings {
		if contains(w, "build") && contains(w, "app") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Error("expected warning about skipped build service")
	}
}

func TestImportHandlesPortFormats(t *testing.T) {
	path := writeComposeFile(t, portFormatsCompose)
	result, err := Import(path)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(result.Deployments) != 3 {
		t.Fatalf("got %d deployments, want 3", len(result.Deployments))
	}

	for _, dep := range result.Deployments {
		containers := dep.Spec.Template.Spec.Containers
		if len(containers) == 0 {
			t.Errorf("deployment %q has no containers", dep.Name)
			continue
		}
		if len(containers[0].Ports) == 0 {
			t.Errorf("deployment %q has no ports", dep.Name)
			continue
		}
		port := containers[0].Ports[0].ContainerPort
		switch dep.Name {
		case "svc1":
			if port != 8080 {
				t.Errorf("svc1 containerPort = %d, want 8080", port)
			}
		case "svc2":
			if port != 80 {
				t.Errorf("svc2 containerPort = %d, want 80", port)
			}
		case "svc3":
			if port != 80 {
				t.Errorf("svc3 containerPort = %d, want 80 (after stripping /tcp)", port)
			}
		}
	}
}

func TestImportHandlesEnvironmentFormats(t *testing.T) {
	path := writeComposeFile(t, envFormatsCompose)
	result, err := Import(path)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(result.Deployments) != 2 {
		t.Fatalf("got %d deployments, want 2", len(result.Deployments))
	}

	for _, dep := range result.Deployments {
		containers := dep.Spec.Template.Spec.Containers
		if len(containers) == 0 {
			t.Errorf("deployment %q has no containers", dep.Name)
			continue
		}
		env := containers[0].Env
		if len(env) != 2 {
			t.Errorf("deployment %q has %d env vars, want 2", dep.Name, len(env))
		}
	}
}

func TestImportEmptyCompose(t *testing.T) {
	emptyCompose := `version: "3"
services: {}
`
	path := writeComposeFile(t, emptyCompose)
	_, err := Import(path)
	if err == nil {
		t.Fatal("expected error for empty compose, got nil")
	}
}

func TestImportFileNotFound(t *testing.T) {
	_, err := Import("/nonexistent/docker-compose.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestWriteManifests(t *testing.T) {
	path := writeComposeFile(t, basicCompose)
	result, err := Import(path)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	outDir := filepath.Join(t.TempDir(), "manifests")
	if err := WriteManifests(result, outDir); err != nil {
		t.Fatalf("WriteManifests: %v", err)
	}

	// Check deployment files
	for _, dep := range result.Deployments {
		fpath := filepath.Join(outDir, dep.Name+"-deployment.yaml")
		if _, err := os.Stat(fpath); err != nil {
			t.Errorf("expected file %s: %v", fpath, err)
		}
	}

	// Check service files
	for _, svc := range result.Services {
		fpath := filepath.Join(outDir, svc.Name+"-service.yaml")
		if _, err := os.Stat(fpath); err != nil {
			t.Errorf("expected file %s: %v", fpath, err)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
