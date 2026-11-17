package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*NATS)(nil)

type NATS struct {
	addon.BaseAddon
}

func NewNATS(orch *orchestrator.Orchestrator, store storage.Store) *NATS {
	return &NATS{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (n *NATS) Name() string            { return "nats" }
func (n *NATS) DisplayName() string      { return "NATS" }
func (n *NATS) Description() string      { return "Cloud-native messaging system" }
func (n *NATS) DefaultVersion() string   { return "2.10" }
func (n *NATS) AvailableVersions() []string { return []string{"2.10", "2.9"} }

func (n *NATS) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = n.DefaultVersion()
	}
	image := fmt.Sprintf("nats:%s-alpine", version)

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "nats-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "nats",
				"torresmo.dev/addon": "nats",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:    "nats",
					Image:   image,
					Command: []string{"nats-server", "--jetstream"},
					Ports: []v1.PortSpec{
						{ContainerPort: 4222, Protocol: "tcp"},
						{ContainerPort: 8222, Protocol: "tcp"},
					},
				},
			},
		},
	}

	if err := n.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating nats pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "nats",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "client", Port: 4222, TargetPort: 4222, Protocol: "tcp"},
				{Name: "monitor", Port: 8222, TargetPort: 8222, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "nats"},
		},
	}

	if err := n.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating nats service: %w", err)
	}

	return nil
}

func (n *NATS) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = n.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "nats")
	_ = n.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "nats-0")
	return nil
}

func (n *NATS) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := n.Orchestrator().GetPod(ctx, clusterName, namespace, "nats-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
