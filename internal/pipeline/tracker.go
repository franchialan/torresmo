package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/torresmo-dev/torresmo/internal/storage"
)

// Tracker tracks pipeline execution and persists state.
type Tracker struct {
	store       storage.Store
	runID       string
	mu          sync.Mutex
	subscribers []chan TargetUpdate
}

// TargetUpdate is sent via SSE to live viewers.
type TargetUpdate struct {
	RunID    string `json:"runId"`
	Target   string `json:"target"`
	Status   string `json:"status"`
	Duration int64  `json:"duration"`
	Error    string `json:"error,omitempty"`
	Output   string `json:"output,omitempty"`
}

// NewTracker creates a new Tracker backed by the given store.
func NewTracker(store storage.Store) *Tracker {
	return &Tracker{
		store: store,
	}
}

// StartRun creates a new pipeline run record.
func (t *Tracker) StartRun(ctx context.Context, file, targetFilter string, totalTargets int) (string, error) {
	t.runID = fmt.Sprintf("run-%d", time.Now().UnixNano())

	now := time.Now().Unix()
	run := &storage.PipelineRunRecord{
		ID:           t.runID,
		Status:       "running",
		StartedAt:    now,
		TotalTargets: totalTargets,
		File:         file,
		TargetFilter: targetFilter,
	}

	if err := t.store.SavePipelineRun(ctx, run); err != nil {
		return "", err
	}

	return t.runID, nil
}

// UpdateTarget updates target status and notifies subscribers.
func (t *Tracker) UpdateTarget(ctx context.Context, name, status string, duration time.Duration, errMsg, output string) {
	now := time.Now().Unix()
	target := &storage.PipelineTargetRecord{
		RunID:      t.runID,
		Name:       name,
		Status:     status,
		StartedAt:  now - int64(duration.Seconds()),
		FinishedAt: now,
		Duration:   duration.Milliseconds(),
		Error:      errMsg,
		Output:     truncateOutput(output, 500),
	}
	_ = t.store.SavePipelineTarget(ctx, target)

	update := TargetUpdate{
		RunID:    t.runID,
		Target:   name,
		Status:   status,
		Duration: duration.Milliseconds(),
		Error:    errMsg,
		Output:   truncateOutput(output, 200),
	}

	t.mu.Lock()
	for _, ch := range t.subscribers {
		select {
		case ch <- update:
		default:
		}
	}
	t.mu.Unlock()
}

// FinishRun updates the run record with final status.
func (t *Tracker) FinishRun(ctx context.Context, results []RunResult) {
	passed, failed := 0, 0
	status := "success"
	for _, r := range results {
		switch r.Status {
		case "success":
			passed++
		case "failed":
			failed++
			status = "failed"
		}
	}

	run := &storage.PipelineRunRecord{
		ID:            t.runID,
		Status:        status,
		FinishedAt:    time.Now().Unix(),
		TotalTargets:  len(results),
		PassedTargets: passed,
		FailedTargets: failed,
	}
	_ = t.store.SavePipelineRun(ctx, run)
}

// Subscribe returns a channel that receives live target updates.
func (t *Tracker) Subscribe() chan TargetUpdate {
	ch := make(chan TargetUpdate, 100)
	t.mu.Lock()
	t.subscribers = append(t.subscribers, ch)
	t.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel.
func (t *Tracker) Unsubscribe(ch chan TargetUpdate) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, sub := range t.subscribers {
		if sub == ch {
			t.subscribers = append(t.subscribers[:i], t.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// RunID returns the current run ID.
func (t *Tracker) RunID() string {
	return t.runID
}

func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-maxLen:]
}
