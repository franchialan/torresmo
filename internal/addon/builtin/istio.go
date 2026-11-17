package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Istio)(nil)

type Istio struct {
	addon.BaseAddon
}

func NewIstio(orch *orchestrator.Orchestrator, store storage.Store) *Istio {
	return &Istio{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (i *Istio) Name() string            { return "istio" }
func (i *Istio) DisplayName() string      { return "Istio" }
func (i *Istio) Description() string      { return "Service mesh for traffic management, security, and observability" }
func (i *Istio) DefaultVersion() string   { return "1.21.0" }
func (i *Istio) AvailableVersions() []string { return []string{"1.21.0", "1.20.0", "1.19.0"} }

func (i *Istio) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = i.DefaultVersion()
	}
	image := fmt.Sprintf("istio/pilot:%s", version)

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "istiod-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "istiod",
				"torresmo.dev/addon": "istio",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "istiod",
					Image: image,
					Ports: []v1.PortSpec{
						{ContainerPort: 15010, Protocol: "tcp"},
						{ContainerPort: 15017, Protocol: "tcp"},
					},
				},
			},
		},
	}

	if err := i.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating istiod pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "istiod",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "grpc", Port: 15010, TargetPort: 15010, Protocol: "tcp"},
				{Name: "webhook", Port: 15017, TargetPort: 15017, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "istiod"},
		},
	}

	if err := i.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating istiod service: %w", err)
	}

	return nil
}

func (i *Istio) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = i.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "istiod")
	_ = i.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "istiod-0")
	return nil
}

func (i *Istio) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := i.Orchestrator().GetPod(ctx, clusterName, namespace, "istiod-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
