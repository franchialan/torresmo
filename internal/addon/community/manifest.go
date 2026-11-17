package community

import (
	"fmt"
	"os"
	"strings"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"gopkg.in/yaml.v3"
)

// AddonManifest defines a community addon in YAML.
type AddonManifest struct {
	APIVersion string       `yaml:"apiVersion"` // torresmo.io/v1
	Kind       string       `yaml:"kind"`       // Addon
	Metadata   ManifestMeta `yaml:"metadata"`
	Spec       ManifestSpec `yaml:"spec"`
}

type ManifestMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Author      string `yaml:"author"`
	Version     string `yaml:"version"`
}

type ManifestSpec struct {
	Versions      []string        `yaml:"versions"`
	Default       string          `yaml:"default"`
	ImageTemplate string          `yaml:"imageTemplate"` // e.g. "elasticsearch:{version}"
	Containers    []ContainerDef  `yaml:"containers"`
	Ports         []PortDef       `yaml:"ports"`
	Env           []EnvDef        `yaml:"env"`
	Config        []ConfigParam   `yaml:"config"`
	Healthcheck   *HealthcheckDef `yaml:"healthcheck,omitempty"`
}

type ContainerDef struct {
	Name    string   `yaml:"name"`
	Image   string   `yaml:"image"` // can use {version} placeholder
	Command []string `yaml:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"`
}

type PortDef struct {
	Name          string `yaml:"name"`
	ContainerPort int    `yaml:"containerPort"`
	Protocol      string `yaml:"protocol"`
}

type EnvDef struct {
	Name       string `yaml:"name"`
	Value      string `yaml:"value"`
	FromConfig string `yaml:"fromConfig,omitempty"` // maps to a config param
}

type ConfigParam struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"` // string, integer, boolean
	Default     string `yaml:"default"`
	Description string `yaml:"description"`
}

type HealthcheckDef struct {
	Path string `yaml:"path"`
	Port int    `yaml:"port"`
}

// LoadManifest reads an addon manifest YAML file.
func LoadManifest(path string) (*AddonManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	return ParseManifest(data)
}

// ParseManifest parses addon manifest YAML content.
func ParseManifest(data []byte) (*AddonManifest, error) {
	var m AddonManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	if m.APIVersion != "torresmo.io/v1" {
		return nil, fmt.Errorf("unsupported apiVersion %q", m.APIVersion)
	}
	if m.Kind != "Addon" {
		return nil, fmt.Errorf("unexpected kind %q (expected Addon)", m.Kind)
	}
	if m.Metadata.Name == "" {
		return nil, fmt.Errorf("addon name is required")
	}
	if m.Spec.Default == "" && len(m.Spec.Versions) > 0 {
		m.Spec.Default = m.Spec.Versions[0]
	}
	return &m, nil
}

// ToPod converts the manifest to a v1.Pod for a given version, namespace, and config overrides.
func (m *AddonManifest) ToPod(version, namespace string, config map[string]string) *v1.Pod {
	if version == "" {
		version = m.Spec.Default
	}
	if namespace == "" {
		namespace = "default"
	}

	var containers []v1.ContainerSpec
	for _, cdef := range m.Spec.Containers {
		image := cdef.Image
		image = strings.ReplaceAll(image, "{version}", version)

		var env []v1.EnvVar
		for _, e := range m.Spec.Env {
			value := e.Value
			if e.FromConfig != "" {
				if v, ok := config[e.FromConfig]; ok {
					value = v
				} else {
					for _, p := range m.Spec.Config {
						if p.Name == e.FromConfig {
							value = p.Default
							break
						}
					}
				}
			}
			env = append(env, v1.EnvVar{Name: e.Name, Value: value})
		}

		var ports []v1.PortSpec
		for _, p := range m.Spec.Ports {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			ports = append(ports, v1.PortSpec{
				ContainerPort: p.ContainerPort,
				Protocol:      proto,
			})
		}

		containers = append(containers, v1.ContainerSpec{
			Name:    cdef.Name,
			Image:   image,
			Command: cdef.Command,
			Args:    cdef.Args,
			Env:     env,
			Ports:   ports,
		})
	}

	return &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      m.Metadata.Name + "-0",
			Namespace: namespace,
			Labels: map[string]string{
				"app":                m.Metadata.Name,
				"torresmo.dev/addon": m.Metadata.Name,
			},
		},
		Spec: v1.PodSpec{Containers: containers},
	}
}

// ToService converts the manifest to a v1.Service.
func (m *AddonManifest) ToService(namespace string) *v1.Service {
	if namespace == "" {
		namespace = "default"
	}

	var ports []v1.ServicePort
	for _, p := range m.Spec.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		ports = append(ports, v1.ServicePort{
			Name:       p.Name,
			Port:       p.ContainerPort,
			TargetPort: p.ContainerPort,
			Protocol:   proto,
		})
	}

	return &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      m.Metadata.Name,
			Namespace: namespace,
		},
		Spec: v1.ServiceSpec{
			Type:     "ClusterIP",
			Ports:    ports,
			Selector: map[string]string{"app": m.Metadata.Name},
		},
	}
}
