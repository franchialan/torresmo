package compose

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"gopkg.in/yaml.v3"
)

// ComposeFile represents a simplified docker-compose.yaml.
type ComposeFile struct {
	Version  string                    `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
}

// ComposeService represents a single service in a docker-compose file.
type ComposeService struct {
	Image       string      `yaml:"image"`
	Build       interface{} `yaml:"build"`   // string or struct, we just detect presence
	Command     interface{} `yaml:"command"` // string or []string
	Ports       []string    `yaml:"ports"`
	Environment interface{} `yaml:"environment"` // []string or map[string]string
	Volumes     []string    `yaml:"volumes"`
	DependsOn   interface{} `yaml:"depends_on"` // []string or map
	Restart     string      `yaml:"restart"`
}

// ConvertResult contains the converted Torresmo resources.
type ConvertResult struct {
	Deployments []v1.Deployment
	Services    []v1.Service
	KnownAddons []string // services that match known addons (redis, postgres, etc.)
	Warnings    []string
}

// knownAddonImages maps container image prefixes to addon names.
var knownAddonImages = map[string]string{
	"redis":           "redis",
	"postgres":        "postgres",
	"mysql":           "postgres", // suggest postgres as alternative
	"mongo":           "mongodb",
	"grafana":         "grafana",
	"prom/prometheus": "prometheus",
	"nats":            "nats",
	"minio":           "minio",
}

// Import reads a docker-compose.yaml and converts it to Torresmo resources.
func Import(path string) (*ConvertResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading compose file: %w", err)
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil, fmt.Errorf("parsing compose file: %w", err)
	}

	if len(compose.Services) == 0 {
		return nil, fmt.Errorf("no services found in compose file")
	}

	result := &ConvertResult{}

	for name, svc := range compose.Services {
		if len(svc.Volumes) > 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Service %q defines volumes — volume mounts are not converted", name))
		}

		// Check if this is a known addon
		if addonName := matchAddon(svc.Image); addonName != "" {
			result.KnownAddons = append(result.KnownAddons, addonName)
			result.Warnings = append(result.Warnings, fmt.Sprintf("Service %q uses %q — consider using: torresmo addon install %s", name, svc.Image, addonName))
			continue
		}

		// Skip services with build context but no image
		if svc.Build != nil && svc.Image == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Service %q uses 'build' without 'image' — skipped (build the image first)", name))
			continue
		}

		if svc.Image == "" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Service %q has no image — skipped", name))
			continue
		}

		dep := convertToDeployment(name, svc)
		result.Deployments = append(result.Deployments, dep)

		if s := convertToService(name, svc); s != nil {
			result.Services = append(result.Services, *s)
		}
	}

	return result, nil
}

func matchAddon(image string) string {
	imageLower := strings.ToLower(image)
	for prefix, addon := range knownAddonImages {
		if strings.HasPrefix(imageLower, prefix) {
			return addon
		}
	}
	return ""
}

func convertToDeployment(name string, svc ComposeService) v1.Deployment {
	container := v1.ContainerSpec{
		Name:  name,
		Image: svc.Image,
	}

	switch cmd := svc.Command.(type) {
	case string:
		if cmd != "" {
			container.Command = strings.Fields(cmd)
		}
	case []interface{}:
		for _, c := range cmd {
			if s, ok := c.(string); ok {
				container.Command = append(container.Command, s)
			}
		}
	}

	container.Env = parseEnvironment(svc.Environment)

	for _, p := range svc.Ports {
		port := parsePort(p)
		if port.ContainerPort > 0 {
			container.Ports = append(container.Ports, port)
		}
	}

	return v1.Deployment{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Deployment"},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    map[string]string{"app": name},
		},
		Spec: v1.DeploymentSpec{
			Replicas: 1,
			Template: v1.PodTemplate{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{"app": name},
				},
				Spec: v1.PodSpec{
					Containers: []v1.ContainerSpec{container},
				},
			},
		},
	}
}

func convertToService(name string, svc ComposeService) *v1.Service {
	if len(svc.Ports) == 0 {
		return nil
	}

	var ports []v1.ServicePort
	for _, p := range svc.Ports {
		sp := parseServicePort(p)
		if sp.Port > 0 {
			ports = append(ports, sp)
		}
	}

	if len(ports) == 0 {
		return nil
	}

	return &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1.ServiceSpec{
			Type:     "ClusterIP",
			Ports:    ports,
			Selector: map[string]string{"app": name},
		},
	}
}

func parseEnvironment(env interface{}) []v1.EnvVar {
	var result []v1.EnvVar
	switch e := env.(type) {
	case []interface{}:
		for _, item := range e {
			if s, ok := item.(string); ok {
				parts := strings.SplitN(s, "=", 2)
				if len(parts) == 2 {
					result = append(result, v1.EnvVar{Name: parts[0], Value: parts[1]})
				}
			}
		}
	case map[string]interface{}:
		for k, v := range e {
			result = append(result, v1.EnvVar{Name: k, Value: fmt.Sprintf("%v", v)})
		}
	}
	return result
}

func parsePort(p string) v1.PortSpec {
	// Handle formats: "8080", "8080:80", "8080:80/tcp"
	p = strings.Split(p, "/")[0] // remove protocol
	parts := strings.Split(p, ":")

	var containerPort int
	if len(parts) >= 2 {
		containerPort, _ = strconv.Atoi(parts[len(parts)-1])
	} else {
		containerPort, _ = strconv.Atoi(parts[0])
	}

	return v1.PortSpec{
		ContainerPort: containerPort,
		Protocol:      "tcp",
	}
}

func parseServicePort(p string) v1.ServicePort {
	p = strings.Split(p, "/")[0]
	parts := strings.Split(p, ":")

	var hostPort, containerPort int
	if len(parts) >= 2 {
		hostPort, _ = strconv.Atoi(parts[0])
		containerPort, _ = strconv.Atoi(parts[1])
	} else {
		containerPort, _ = strconv.Atoi(parts[0])
		hostPort = containerPort
	}

	return v1.ServicePort{
		Port:       hostPort,
		TargetPort: containerPort,
		Protocol:   "tcp",
	}
}

// WriteManifests writes the converted resources as YAML files.
func WriteManifests(result *ConvertResult, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	for _, dep := range result.Deployments {
		data, err := yaml.Marshal(dep)
		if err != nil {
			continue
		}
		path := fmt.Sprintf("%s/%s-deployment.yaml", outputDir, dep.Name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}

	for _, svc := range result.Services {
		data, err := yaml.Marshal(svc)
		if err != nil {
			continue
		}
		path := fmt.Sprintf("%s/%s-service.yaml", outputDir, svc.Name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}

	return nil
}
