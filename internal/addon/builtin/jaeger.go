package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Jaeger)(nil)

type Jaeger struct {
	addon.BaseAddon
}

func NewJaeger(orch *orchestrator.Orchestrator, store storage.Store) *Jaeger {
	return &Jaeger{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (j *Jaeger) Name() string            { return "jaeger" }
func (j *Jaeger) DisplayName() string      { return "Jaeger" }
func (j *Jaeger) Description() string      { return "Distributed tracing system" }
func (j *Jaeger) DefaultVersion() string   { return "1.57" }
func (j *Jaeger) AvailableVersions() []string { return []string{"1.57", "1.53"} }

func (j *Jaeger) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = j.DefaultVersion()
	}
	image := fmt.Sprintf("jaegertracing/all-in-one:%s", version)

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "jaeger-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "jaeger",
				"torresmo.dev/addon": "jaeger",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "jaeger",
					Image: image,
					Ports: []v1.PortSpec{
						{ContainerPort: 16686, Protocol: "tcp"},
						{ContainerPort: 14268, Protocol: "tcp"},
						{ContainerPort: 4317, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "COLLECTOR_OTLP_ENABLED", Value: "true"},
					},
				},
			},
		},
	}

	if err := j.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating jaeger pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "jaeger",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "ui", Port: 16686, TargetPort: 16686, Protocol: "tcp"},
				{Name: "collector", Port: 14268, TargetPort: 14268, Protocol: "tcp"},
				{Name: "otlp-grpc", Port: 4317, TargetPort: 4317, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "jaeger"},
		},
	}

	if err := j.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating jaeger service: %w", err)
	}

	return nil
}

func (j *Jaeger) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = j.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "jaeger")
	_ = j.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "jaeger-0")
	return nil
}

func (j *Jaeger) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := j.Orchestrator().GetPod(ctx, clusterName, namespace, "jaeger-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
