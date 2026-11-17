package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*MinIO)(nil)

type MinIO struct {
	addon.BaseAddon
}

func NewMinIO(orch *orchestrator.Orchestrator, store storage.Store) *MinIO {
	return &MinIO{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (m *MinIO) Name() string            { return "minio" }
func (m *MinIO) DisplayName() string      { return "MinIO" }
func (m *MinIO) Description() string      { return "S3-compatible object storage" }
func (m *MinIO) DefaultVersion() string   { return "latest" }
func (m *MinIO) AvailableVersions() []string { return []string{"latest"} }

func (m *MinIO) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = m.DefaultVersion()
	}
	image := fmt.Sprintf("minio/minio:%s", version)

	rootUser := "minioadmin"
	if v, ok := opts.Config["root_user"]; ok {
		rootUser = v
	}
	rootPassword := "minioadmin"
	if v, ok := opts.Config["root_password"]; ok {
		rootPassword = v
	}

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "minio-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "minio",
				"torresmo.dev/addon": "minio",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:    "minio",
					Image:   image,
					Command: []string{"server", "/data", "--console-address", ":9001"},
					Ports: []v1.PortSpec{
						{ContainerPort: 9000, Protocol: "tcp"},
						{ContainerPort: 9001, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "MINIO_ROOT_USER", Value: rootUser},
						{Name: "MINIO_ROOT_PASSWORD", Value: rootPassword},
					},
				},
			},
		},
	}

	if err := m.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating minio pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "minio",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "api", Port: 9000, TargetPort: 9000, Protocol: "tcp"},
				{Name: "console", Port: 9001, TargetPort: 9001, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "minio"},
		},
	}

	if err := m.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating minio service: %w", err)
	}

	return nil
}

func (m *MinIO) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = m.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "minio")
	_ = m.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "minio-0")
	return nil
}

func (m *MinIO) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := m.Orchestrator().GetPod(ctx, clusterName, namespace, "minio-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
