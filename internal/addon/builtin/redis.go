package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Redis)(nil)

type Redis struct {
	addon.BaseAddon
}

func NewRedis(orch *orchestrator.Orchestrator, store storage.Store) *Redis {
	return &Redis{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (r *Redis) Name() string            { return "redis" }
func (r *Redis) DisplayName() string      { return "Redis" }
func (r *Redis) Description() string      { return "In-memory data store for caching and messaging" }
func (r *Redis) DefaultVersion() string   { return "7.2" }
func (r *Redis) AvailableVersions() []string { return []string{"7.2", "7.0", "6.2"} }

func (r *Redis) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = r.DefaultVersion()
	}
	image := fmt.Sprintf("redis:%s-alpine", version)

	maxMemory := "256mb"
	if v, ok := opts.Config["maxmemory"]; ok {
		maxMemory = v
	}

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "redis-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "redis",
				"torresmo.dev/addon": "redis",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:    "redis",
					Image:   image,
					Command: []string{"redis-server", "--maxmemory", maxMemory, "--maxmemory-policy", "allkeys-lru"},
					Ports: []v1.PortSpec{
						{ContainerPort: 6379, Protocol: "tcp"},
					},
				},
			},
		},
	}

	if err := r.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating redis pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "redis",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "redis", Port: 6379, TargetPort: 6379, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "redis"},
		},
	}

	if err := r.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating redis service: %w", err)
	}

	return nil
}

func (r *Redis) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = r.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "redis")
	_ = r.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "redis-0")
	return nil
}

func (r *Redis) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := r.Orchestrator().GetPod(ctx, clusterName, namespace, "redis-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
