package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/torresmo-dev/torresmo/internal/storage"
)

// Scheduler assigns pods to nodes using round-robin.
type Scheduler struct {
	store storage.Store
	mu    sync.Mutex
	index map[string]int // cluster -> last used worker index
}

func NewScheduler(store storage.Store) *Scheduler {
	return &Scheduler{
		store: store,
		index: make(map[string]int),
	}
}

// PickNode selects a node for a pod in the given cluster.
// It prefers worker nodes and uses round-robin to distribute.
func (s *Scheduler) PickNode(ctx context.Context, clusterName string) (*storage.NodeRecord, error) {
	nodes, err := s.store.ListNodes(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	// Filter to worker nodes only; fall back to any node if no workers.
	var workers []storage.NodeRecord
	for _, n := range nodes {
		if n.Role == "worker" && n.Phase == "Running" {
			workers = append(workers, n)
		}
	}
	if len(workers) == 0 {
		for _, n := range nodes {
			if n.Phase == "Running" {
				workers = append(workers, n)
			}
		}
	}
	if len(workers) == 0 {
		return nil, fmt.Errorf("no running nodes available in cluster %q", clusterName)
	}

	s.mu.Lock()
	idx := s.index[clusterName] % len(workers)
	s.index[clusterName] = idx + 1
	s.mu.Unlock()

	return &workers[idx], nil
}
