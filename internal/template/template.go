package template

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
)

// Template represents a reusable cluster template.
type Template struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Nodes       int      `yaml:"nodes"`
	Profile     string   `yaml:"profile"`
	Addons      []string `yaml:"addons"`
}

var builtinTemplates = map[string]Template{
	"go-api": {
		Name:        "go-api",
		Description: "Go API with Redis + Postgres",
		Nodes:       3,
		Profile:     "standard",
		Addons:      []string{"redis", "postgres", "prometheus"},
	},
	"node-fullstack": {
		Name:        "node-fullstack",
		Description: "Node.js + React fullstack",
		Nodes:       3,
		Profile:     "standard",
		Addons:      []string{"redis", "postgres"},
	},
	"java-spring": {
		Name:        "java-spring",
		Description: "Spring Boot microservices",
		Nodes:       3,
		Profile:     "heavy",
		Addons:      []string{"postgres", "prometheus", "grafana"},
	},
	"python-ml": {
		Name:        "python-ml",
		Description: "Python ML/Data stack",
		Nodes:       2,
		Profile:     "heavy",
		Addons:      []string{"postgres", "redis"},
	},
	"minimal": {
		Name:        "minimal",
		Description: "Minimal cluster with no addons",
		Nodes:       1,
		Profile:     "light",
		Addons:      nil,
	},
	"microservices": {
		Name:        "microservices",
		Description: "Microservices with service mesh and tracing",
		Nodes:       3,
		Profile:     "heavy",
		Addons:      []string{"istio", "jaeger", "prometheus", "grafana"},
	},
	"data-pipeline": {
		Name:        "data-pipeline",
		Description: "Data pipeline with Kafka and storage",
		Nodes:       3,
		Profile:     "heavy",
		Addons:      []string{"kafka", "postgres", "redis", "minio"},
	},
	"fullstack": {
		Name:        "fullstack",
		Description: "Complete fullstack with caching, DB, and monitoring",
		Nodes:       3,
		Profile:     "standard",
		Addons:      []string{"redis", "postgres", "prometheus", "grafana"},
	},
	"aws-dev": {
		Name:        "aws-dev",
		Description: "AWS development with LocalStack, Vault, and Postgres",
		Nodes:       2,
		Profile:     "standard",
		Addons:      []string{"localstack", "vault", "postgres", "redis"},
	},
}

// List returns all available templates (builtin + custom).
func List() []Template {
	result := make([]Template, 0, len(builtinTemplates))
	for _, t := range builtinTemplates {
		result = append(result, t)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, ".torresmo", "templates")
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				if filepath.Ext(entry.Name()) != ".yaml" {
					continue
				}
				data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
				if err != nil {
					continue
				}
				var t Template
				if err := yaml.Unmarshal(data, &t); err != nil {
					continue
				}
				result = append(result, t)
			}
		}
	}

	return result
}

// Get returns a template by name.
func Get(name string) (*Template, error) {
	if t, ok := builtinTemplates[name]; ok {
		return &t, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("template %q not found", name)
	}
	path := filepath.Join(home, ".torresmo", "templates", name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("template %q not found", name)
	}
	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}
	return &t, nil
}

// Save saves a custom template.
func Save(t Template) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".torresmo", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, t.Name+".yaml"), data, 0o644)
}

// ToClusterConfig converts a template to a v1.Cluster config.
func (t *Template) ToClusterConfig(clusterName string) *v1.Cluster {
	nodes := []v1.NodeSpec{{Role: "control-plane"}}
	if t.Nodes > 1 {
		nodes = append(nodes, v1.NodeSpec{Role: "worker", Count: t.Nodes - 1})
	}

	var addons []v1.AddonRef
	for _, a := range t.Addons {
		addons = append(addons, v1.AddonRef{Name: a})
	}

	return &v1.Cluster{
		TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Cluster"},
		ObjectMeta: v1.ObjectMeta{Name: clusterName},
		Spec: v1.ClusterSpec{
			Nodes:   nodes,
			Profile: t.Profile,
			Addons:  addons,
		},
	}
}
