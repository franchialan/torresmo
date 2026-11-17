package addon

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

// Registry manages addon lifecycle.
type Registry struct {
	addons map[string]Addon
	orch   *orchestrator.Orchestrator
	store  storage.Store
}

func NewRegistry(orch *orchestrator.Orchestrator, store storage.Store) *Registry {
	return &Registry{
		addons: make(map[string]Addon),
		orch:   orch,
		store:  store,
	}
}

// Register adds an addon to the registry.
func (r *Registry) Register(addon Addon) {
	r.addons[addon.Name()] = addon
}

// Get returns an addon by name.
func (r *Registry) Get(name string) (Addon, error) {
	a, ok := r.addons[name]
	if !ok {
		return nil, fmt.Errorf("unknown addon %q; available: %v", name, r.Available())
	}
	return a, nil
}

// Available returns names of all registered addons.
func (r *Registry) Available() []string {
	names := make([]string, 0, len(r.addons))
	for name := range r.addons {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Info returns addon metadata without installing.
func (r *Registry) Info(name string) (*AddonInfo, error) {
	a, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	return &AddonInfo{
		Name:        a.Name(),
		DisplayName: a.DisplayName(),
		Description: a.Description(),
		Versions:    a.AvailableVersions(),
		Default:     a.DefaultVersion(),
	}, nil
}

type AddonInfo struct {
	Name        string
	DisplayName string
	Description string
	Versions    []string
	Default     string
}

// Install installs an addon.
func (r *Registry) Install(ctx context.Context, name string, opts InstallOptions) error {
	addon, err := r.Get(name)
	if err != nil {
		return err
	}

	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	if opts.Version == "" {
		opts.Version = addon.DefaultVersion()
	}

	if existing, err := r.store.GetAddon(ctx, opts.ClusterName, opts.Namespace, name); err == nil {
		if existing.Phase == "Running" {
			return fmt.Errorf("addon %q is already installed in namespace %q", name, opts.Namespace)
		}
	}

	configJSON, _ := json.Marshal(opts.Config)
	now := time.Now().Unix()
	rec := &storage.AddonRecord{
		Name:        name,
		ClusterName: opts.ClusterName,
		Namespace:   opts.Namespace,
		Version:     opts.Version,
		Phase:       "Installing",
		Config:      string(configJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.store.SaveAddon(ctx, rec); err != nil {
		return fmt.Errorf("saving addon record: %w", err)
	}

	if err := addon.Install(ctx, opts); err != nil {
		rec.Phase = "Failed"
		rec.UpdatedAt = time.Now().Unix()
		_ = r.store.SaveAddon(ctx, rec)
		return fmt.Errorf("installing addon %q: %w", name, err)
	}

	rec.Phase = "Running"
	rec.UpdatedAt = time.Now().Unix()
	if err := r.store.SaveAddon(ctx, rec); err != nil {
		return fmt.Errorf("updating addon record: %w", err)
	}

	return nil
}

// Uninstall removes an addon.
func (r *Registry) Uninstall(ctx context.Context, name string, opts UninstallOptions) error {
	addon, err := r.Get(name)
	if err != nil {
		return err
	}

	if opts.Namespace == "" {
		opts.Namespace = "default"
	}

	if _, err := r.store.GetAddon(ctx, opts.ClusterName, opts.Namespace, name); err != nil {
		return fmt.Errorf("addon %q is not installed in namespace %q", name, opts.Namespace)
	}

	if err := addon.Uninstall(ctx, opts); err != nil {
		return fmt.Errorf("uninstalling addon %q: %w", name, err)
	}

	if err := r.store.DeleteAddon(ctx, opts.ClusterName, opts.Namespace, name); err != nil {
		return fmt.Errorf("deleting addon record: %w", err)
	}

	return nil
}

// List returns all installed addons for a cluster.
func (r *Registry) List(ctx context.Context, clusterName string) ([]storage.AddonRecord, error) {
	return r.store.ListAddons(ctx, clusterName)
}
