package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Kafka)(nil)

type Kafka struct {
	addon.BaseAddon
}

func NewKafka(orch *orchestrator.Orchestrator, store storage.Store) *Kafka {
	return &Kafka{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (k *Kafka) Name() string            { return "kafka" }
func (k *Kafka) DisplayName() string      { return "Kafka" }
func (k *Kafka) Description() string      { return "Distributed event streaming platform" }
func (k *Kafka) DefaultVersion() string   { return "3.7" }
func (k *Kafka) AvailableVersions() []string { return []string{"3.7", "3.6"} }

func (k *Kafka) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = k.DefaultVersion()
	}
	image := fmt.Sprintf("bitnami/kafka:%s", version)

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "kafka-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "kafka",
				"torresmo.dev/addon": "kafka",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "kafka",
					Image: image,
					Ports: []v1.PortSpec{
						{ContainerPort: 9092, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "KAFKA_CFG_NODE_ID", Value: "0"},
						{Name: "KAFKA_CFG_PROCESS_ROLES", Value: "controller,broker"},
						{Name: "KAFKA_CFG_CONTROLLER_QUORUM_VOTERS", Value: "0@localhost:9093"},
						{Name: "KAFKA_CFG_LISTENERS", Value: "PLAINTEXT://:9092,CONTROLLER://:9093"},
						{Name: "KAFKA_CFG_CONTROLLER_LISTENER_NAMES", Value: "CONTROLLER"},
						{Name: "ALLOW_PLAINTEXT_LISTENER", Value: "yes"},
					},
				},
			},
		},
	}

	if err := k.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating kafka pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "kafka",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "kafka", Port: 9092, TargetPort: 9092, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "kafka"},
		},
	}

	if err := k.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating kafka service: %w", err)
	}

	return nil
}

func (k *Kafka) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = k.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "kafka")
	_ = k.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "kafka-0")
	return nil
}

func (k *Kafka) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := k.Orchestrator().GetPod(ctx, clusterName, namespace, "kafka-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
