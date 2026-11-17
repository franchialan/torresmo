package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*MongoDB)(nil)

type MongoDB struct {
	addon.BaseAddon
}

func NewMongoDB(orch *orchestrator.Orchestrator, store storage.Store) *MongoDB {
	return &MongoDB{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (m *MongoDB) Name() string            { return "mongodb" }
func (m *MongoDB) DisplayName() string      { return "MongoDB" }
func (m *MongoDB) Description() string      { return "NoSQL document database" }
func (m *MongoDB) DefaultVersion() string   { return "7.0" }
func (m *MongoDB) AvailableVersions() []string { return []string{"7.0", "6.0", "5.0"} }

func (m *MongoDB) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = m.DefaultVersion()
	}
	image := fmt.Sprintf("mongo:%s", version)

	rootUser := "torresmo"
	if v, ok := opts.Config["root_user"]; ok {
		rootUser = v
	}
	rootPassword := "torresmo"
	if v, ok := opts.Config["root_password"]; ok {
		rootPassword = v
	}

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "mongodb-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "mongodb",
				"torresmo.dev/addon": "mongodb",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "mongodb",
					Image: image,
					Ports: []v1.PortSpec{
						{ContainerPort: 27017, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "MONGO_INITDB_ROOT_USERNAME", Value: rootUser},
						{Name: "MONGO_INITDB_ROOT_PASSWORD", Value: rootPassword},
					},
				},
			},
		},
	}

	if err := m.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating mongodb pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "mongodb",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "mongodb", Port: 27017, TargetPort: 27017, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "mongodb"},
		},
	}

	if err := m.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating mongodb service: %w", err)
	}

	return nil
}

func (m *MongoDB) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = m.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "mongodb")
	_ = m.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "mongodb-0")
	return nil
}

func (m *MongoDB) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := m.Orchestrator().GetPod(ctx, clusterName, namespace, "mongodb-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
