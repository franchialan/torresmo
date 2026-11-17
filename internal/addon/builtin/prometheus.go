package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Prometheus)(nil)

type Prometheus struct {
	addon.BaseAddon
}

func NewPrometheus(orch *orchestrator.Orchestrator, store storage.Store) *Prometheus {
	return &Prometheus{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (p *Prometheus) Name() string            { return "prometheus" }
func (p *Prometheus) DisplayName() string      { return "Prometheus" }
func (p *Prometheus) Description() string      { return "Monitoring and alerting toolkit" }
func (p *Prometheus) DefaultVersion() string   { return "v2.51.0" }
func (p *Prometheus) AvailableVersions() []string { return []string{"v2.51.0", "v2.48.0", "v2.45.0"} }

func (p *Prometheus) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = p.DefaultVersion()
	}
	image := fmt.Sprintf("prom/prometheus:%s", version)

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "prometheus-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "prometheus",
				"torresmo.dev/addon": "prometheus",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "prometheus",
					Image: image,
					Command: []string{
						"--config.file=/etc/prometheus/prometheus.yml",
						"--storage.tsdb.path=/prometheus",
						"--web.enable-lifecycle",
					},
					Ports: []v1.PortSpec{
						{ContainerPort: 9090, Protocol: "tcp"},
					},
				},
			},
		},
	}

	if err := p.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating prometheus pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "prometheus",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "http", Port: 9090, TargetPort: 9090, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "prometheus"},
		},
	}

	if err := p.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating prometheus service: %w", err)
	}

	return nil
}

func (p *Prometheus) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = p.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "prometheus")
	_ = p.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "prometheus-0")
	return nil
}

func (p *Prometheus) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := p.Orchestrator().GetPod(ctx, clusterName, namespace, "prometheus-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
