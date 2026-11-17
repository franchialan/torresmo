package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Grafana)(nil)

type Grafana struct {
	addon.BaseAddon
}

func NewGrafana(orch *orchestrator.Orchestrator, store storage.Store) *Grafana {
	return &Grafana{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (g *Grafana) Name() string            { return "grafana" }
func (g *Grafana) DisplayName() string      { return "Grafana" }
func (g *Grafana) Description() string      { return "Observability dashboards and visualization" }
func (g *Grafana) DefaultVersion() string   { return "10.4.0" }
func (g *Grafana) AvailableVersions() []string { return []string{"10.4.0", "10.2.0", "9.5.0"} }

func (g *Grafana) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = g.DefaultVersion()
	}
	image := fmt.Sprintf("grafana/grafana:%s", version)

	adminUser := "admin"
	if v, ok := opts.Config["admin_user"]; ok {
		adminUser = v
	}
	adminPassword := "admin"
	if v, ok := opts.Config["admin_password"]; ok {
		adminPassword = v
	}

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "grafana-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "grafana",
				"torresmo.dev/addon": "grafana",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "grafana",
					Image: image,
					Ports: []v1.PortSpec{
						{ContainerPort: 3000, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "GF_SECURITY_ADMIN_USER", Value: adminUser},
						{Name: "GF_SECURITY_ADMIN_PASSWORD", Value: adminPassword},
						{Name: "GF_AUTH_ANONYMOUS_ENABLED", Value: "true"},
					},
				},
			},
		},
	}

	if err := g.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating grafana pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "grafana",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "http", Port: 3000, TargetPort: 3000, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "grafana"},
		},
	}

	if err := g.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating grafana service: %w", err)
	}

	return nil
}

func (g *Grafana) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = g.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "grafana")
	_ = g.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "grafana-0")
	return nil
}

func (g *Grafana) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := g.Orchestrator().GetPod(ctx, clusterName, namespace, "grafana-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
