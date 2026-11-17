package community

import (
	"context"
	"fmt"

	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

// ManifestAddon adapts an AddonManifest to the addon.Addon interface.
type ManifestAddon struct {
	addon.BaseAddon
	manifest *AddonManifest
}

var _ addon.Addon = (*ManifestAddon)(nil)

func NewManifestAddon(m *AddonManifest, orch *orchestrator.Orchestrator, store storage.Store) *ManifestAddon {
	return &ManifestAddon{
		BaseAddon: addon.NewBaseAddon(orch, store),
		manifest:  m,
	}
}

func (a *ManifestAddon) Name() string        { return a.manifest.Metadata.Name }
func (a *ManifestAddon) DisplayName() string  { return a.manifest.Metadata.Name }
func (a *ManifestAddon) Description() string  { return a.manifest.Metadata.Description }
func (a *ManifestAddon) DefaultVersion() string { return a.manifest.Spec.Default }

func (a *ManifestAddon) AvailableVersions() []string {
	if len(a.manifest.Spec.Versions) > 0 {
		return a.manifest.Spec.Versions
	}
	return []string{a.manifest.Spec.Default}
}

func (a *ManifestAddon) Install(ctx context.Context, opts addon.InstallOptions) error {
	version := opts.Version
	if version == "" {
		version = a.DefaultVersion()
	}

	pod := a.manifest.ToPod(version, opts.Namespace, opts.Config)
	if err := a.Orchestrator().CreatePod(ctx, opts.ClusterName, pod); err != nil {
		return fmt.Errorf("creating pod for addon %q: %w", a.Name(), err)
	}

	svc := a.manifest.ToService(opts.Namespace)
	if err := a.Orchestrator().ApplyService(ctx, opts.ClusterName, svc); err != nil {
		return fmt.Errorf("creating service for addon %q: %w", a.Name(), err)
	}

	return nil
}

func (a *ManifestAddon) Uninstall(ctx context.Context, opts addon.UninstallOptions) error {
	_ = a.Orchestrator().DeleteService(ctx, opts.ClusterName, opts.Namespace, a.manifest.Metadata.Name)
	_ = a.Orchestrator().DeletePod(ctx, opts.ClusterName, opts.Namespace, a.manifest.Metadata.Name+"-0")
	return nil
}

func (a *ManifestAddon) Status(ctx context.Context, clusterName, namespace string) (string, error) {
	pod, err := a.Orchestrator().GetPod(ctx, clusterName, namespace, a.manifest.Metadata.Name+"-0")
	if err != nil {
		return "Not installed", nil
	}
	return pod.Phase, nil
}
