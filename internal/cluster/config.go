package cluster

import (
	"fmt"
	"os"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"gopkg.in/yaml.v3"
)

// LoadConfig reads a torresmo.yaml file and returns the Cluster config.
func LoadConfig(path string) (*v1.Cluster, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %q: %w", path, err)
	}

	var cluster v1.Cluster
	if err := yaml.Unmarshal(data, &cluster); err != nil {
		return nil, fmt.Errorf("failed to parse config %q: %w", path, err)
	}

	if cluster.APIVersion != "torresmo.io/v1" {
		return nil, fmt.Errorf("unsupported apiVersion %q (expected torresmo.io/v1)", cluster.APIVersion)
	}
	if cluster.Kind != "Cluster" {
		return nil, fmt.Errorf("unexpected kind %q (expected Cluster)", cluster.Kind)
	}
	if cluster.Name == "" {
		return nil, fmt.Errorf("cluster name is required in metadata.name")
	}

	return &cluster, nil
}
