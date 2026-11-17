package builtin

import (
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

// RegisterAll registers all built-in addons with the given registry.
func RegisterAll(registry *addon.Registry, orch *orchestrator.Orchestrator, store storage.Store) {
	registry.Register(NewRedis(orch, store))
	registry.Register(NewPostgres(orch, store))
	registry.Register(NewPrometheus(orch, store))
	registry.Register(NewGrafana(orch, store))
	registry.Register(NewIstio(orch, store))
	registry.Register(NewMongoDB(orch, store))
	registry.Register(NewNATS(orch, store))
	registry.Register(NewMinIO(orch, store))
	registry.Register(NewJaeger(orch, store))
	registry.Register(NewKafka(orch, store))
}
