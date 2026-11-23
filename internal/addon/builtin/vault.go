package builtin

import (
	"context"
	"fmt"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var _ addon.Addon = (*Vault)(nil)

type Vault struct {
	addon.BaseAddon
}

func NewVault(orch *orchestrator.Orchestrator, store storage.Store) *Vault {
	return &Vault{BaseAddon: addon.NewBaseAddon(orch, store)}
}

func (v *Vault) Name() string            { return "vault" }
func (v *Vault) DisplayName() string      { return "Vault" }
func (v *Vault) Description() string      { return "Secrets management, encryption, and identity-based access" }
func (v *Vault) DefaultVersion() string   { return "1.19.0" }
func (v *Vault) AvailableVersions() []string { return []string{"1.19.0", "1.18.0", "1.17.0"} }

func (v *Vault) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = v.DefaultVersion()
	}
	image := fmt.Sprintf("hashicorp/vault:%s", version)

	rootToken := "root"
	if val, ok := opts.Config["root_token"]; ok {
		rootToken = val
	}

	pod := &v1.Pod{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "vault-0",
			Namespace: opts.Namespace,
			Labels: map[string]string{
				"app":                "vault",
				"torresmo.dev/addon": "vault",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.ContainerSpec{
				{
					Name:    "vault",
					Image:   image,
					Command: []string{"server", "-dev"},
					Ports: []v1.PortSpec{
						{ContainerPort: 8200, Protocol: "tcp"},
					},
					Env: []v1.EnvVar{
						{Name: "VAULT_DEV_ROOT_TOKEN_ID", Value: rootToken},
						{Name: "VAULT_DEV_LISTEN_ADDRESS", Value: "0.0.0.0:8200"},
						{Name: "VAULT_ADDR", Value: "http://0.0.0.0:8200"},
						{Name: "VAULT_TOKEN", Value: rootToken},
					},
				},
			},
		},
	}

	if err := v.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating vault pod: %w", err)
	}

	svc := &v1.Service{
		TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
		ObjectMeta: v1.ObjectMeta{
			Name:      "vault",
			Namespace: opts.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type: "ClusterIP",
			Ports: []v1.ServicePort{
				{Name: "http", Port: 8200, TargetPort: 8200, Protocol: "tcp"},
			},
			Selector: map[string]string{"app": "vault"},
		},
	}

	if err := v.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating vault service: %w", err)
	}

	return nil
}

func (v *Vault) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = v.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, "vault")
	_ = v.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, "vault-0")
	return nil
}

func (v *Vault) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := v.Orchestrator().GetPod(ctx, clusterName, namespace, "vault-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
