package addon

import (
	"context"

	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

// Addon defines the interface for installable addons.
type Addon interface {
	// Name returns the addon identifier (e.g., "redis", "postgres")
	Name() string

	// DisplayName returns a human-readable name
	DisplayName() string

	// Description returns a short description
	Description() string

	// DefaultVersion returns the default version to install
	DefaultVersion() string

	// AvailableVersions returns supported versions
	AvailableVersions() []string

	// Install deploys the addon into the cluster
	Install(ctx context.Context, opts InstallOptions) error

	// Uninstall removes the addon from the cluster
	Uninstall(ctx context.Context, opts UninstallOptions) error

	// Status returns the current addon status
	Status(ctx context.Context, clusterName, namespace string) (string, error)
}

type InstallOptions struct {
	ClusterName string
	Namespace   string
	Version     string
	Config      map[string]string
}

type UninstallOptions struct {
	ClusterName string
	Namespace   string
}

// BaseAddon provides common functionality for addons.
type BaseAddon struct {
	orch  *orchestrator.Orchestrator
	store storage.Store
}

func NewBaseAddon(orch *orchestrator.Orchestrator, store storage.Store) BaseAddon {
	return BaseAddon{orch: orch, store: store}
}

func (b *BaseAddon) Orchestrator() *orchestrator.Orchestrator {
	return b.orch
}

func (b *BaseAddon) Store() storage.Store {
	return b.store
}
