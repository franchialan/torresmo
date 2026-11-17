package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Postgres)(nil)

type Postgres struct {
	addon.BaseAddon
}

func NewPostgres(orch *orchestrator.Orchestrator, store storage.Store) *Postgres {
	return &Postgres{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (p *Postgres) Name() string            { return "postgres" }
func (p *Postgres) DisplayName() string      { return "PostgreSQL" }
func (p *Postgres) Description() string      { return "PostgreSQL relational database" }
func (p *Postgres) DefaultVersion() string   { return "16" }
func (p *Postgres) AvailableVersions() []string { return []string{"16", "15", "14"} }

func (p *Postgres) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = p.DefaultVersion()
	}
	image := fmt.Sprintf("postgres:%s-alpine", version)

	user := "torresmo"
	if v, ok := opts.Config["user"]; ok {
		user = v
	}
	password := "torresmo"
	if v, ok := opts.Config["password"]; ok {
		password = v
	}
	database := "torresmo"
	if v, ok := opts.Config["database"]; ok {
		database = v
	}

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "postgres-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "postgres",
				"torresmo.dev/addon": "postgres",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:  "postgres",
					Image: image,
					Ports: []v1.PortSpec{
						{ContainerPort: 5432, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "POSTGRES_USER", Value: user},
						{Name: "POSTGRES_PASSWORD", Value: password},
						{Name: "POSTGRES_DB", Value: database},
					},
				},
			},
		},
	}

	if err := p.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating postgres pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "postgres",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "postgres", Port: 5432, TargetPort: 5432, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "postgres"},
		},
	}

	if err := p.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating postgres service: %w", err)
	}

	return nil
}

func (p *Postgres) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = p.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "postgres")
	_ = p.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "postgres-0")
	return nil
}

func (p *Postgres) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := p.Orchestrator().GetPod(ctx, clusterName, namespace, "postgres-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
