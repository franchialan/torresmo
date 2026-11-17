package community

import (
	"context"
	"fmt"

	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

// RegisterAll loads and registers all community addons from the local store
// into the given registry. Built-in addons already in the registry are not overridden.
func RegisterAll(registry *addon.Registry, orch *orchestrator.Orchestrator, store storage.Store) error {
	cs, err := NewStore()
	if err != nil {
		return err
	}

	manifests, err := cs.List()
	if err != nil {
		return err
	}

	for _, m := range manifests {
		if _, err := registry.Get(m.Metadata.Name); err != nil {
			registry.Register(NewManifestAddon(m, orch, store))
		}
	}

	return nil
}

// InstallFromManifest saves a community addon manifest to the local store,
// registers it in the registry, and installs it.
func InstallFromManifest(ctx context.Context, registry *addon.Registry, manifestPath string, orch *orchestrator.Orchestrator, store storage.Store, opts addon.InstallOptions) error {
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return err
	}

	cs, err := NewStore()
	if err != nil {
		return err
	}
	if err := cs.Install(manifest); err != nil {
		return fmt.Errorf("saving manifest: %w", err)
	}

	addonImpl := NewManifestAddon(manifest, orch, store)
	registry.Register(addonImpl)

	return registry.Install(ctx, manifest.Metadata.Name, opts)
}
