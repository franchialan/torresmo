package devsession

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/torresmo-dev/torresmo/internal/runtime"
)

// SyncRule defines a local-to-remote file sync mapping.
type SyncRule struct {
	Local  string
	Remote string
}

// SessionConfig defines a dev session.
type SessionConfig struct {
	ClusterName string
	Namespace   string
	PodName     string
	ContainerID string
	Syncs       []SyncRule
	Command     string   // command to run/restart on changes
	Ports       []string // port forwards "local:remote"
	Env         map[string]string
}

// Session manages a hot-reload development session.
type Session struct {
	config  SessionConfig
	runtime runtime.ContainerRuntime
	done    chan struct{}
	mu      sync.Mutex
	running bool
}

func NewSession(rt runtime.ContainerRuntime, cfg SessionConfig) *Session {
	return &Session{
		config:  cfg,
		runtime: rt,
		done:    make(chan struct{}),
	}
}

// Start begins the dev session: watches for file changes and restarts on change.
func (s *Session) Start(ctx context.Context) error {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	fmt.Printf("🔥 Dev session started for pod %q\n", s.config.PodName)
	fmt.Printf("  Watching for changes...\n")
	if s.config.Command != "" {
		fmt.Printf("  Command: %s\n", s.config.Command)
	}
	for _, sync := range s.config.Syncs {
		fmt.Printf("  Sync: %s → %s\n", sync.Local, sync.Remote)
	}
	fmt.Println()

	if err := s.syncAll(ctx); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	if s.config.Command != "" {
		if err := s.runCommand(ctx); err != nil {
			fmt.Printf("  ⚠ Command failed: %v\n", err)
		}
	}

	// Simple polling-based file watcher (every 2 seconds).
	// A production version would use fsnotify.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastSync := time.Now()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n🛑 Dev session stopped")
			return nil
		case <-s.done:
			return nil
		case <-ticker.C:
			changed, err := s.detectChanges(lastSync)
			if err != nil {
				continue
			}
			if changed {
				lastSync = time.Now()
				fmt.Printf("  📁 Changes detected, syncing...\n")
				if err := s.syncAll(ctx); err != nil {
					fmt.Printf("  ⚠ Sync failed: %v\n", err)
					continue
				}
				if s.config.Command != "" {
					fmt.Printf("  🔄 Restarting: %s\n", s.config.Command)
					if err := s.runCommand(ctx); err != nil {
						fmt.Printf("  ⚠ Command failed: %v\n", err)
					}
				}
			}
		}
	}
}

// Stop stops the dev session.
func (s *Session) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		s.running = false
		close(s.done)
	}
}

// syncAll syncs all configured directories to the container.
func (s *Session) syncAll(ctx context.Context) error {
	for _, rule := range s.config.Syncs {
		if err := s.syncDir(ctx, rule.Local, rule.Remote); err != nil {
			return fmt.Errorf("syncing %s → %s: %w", rule.Local, rule.Remote, err)
		}
	}
	return nil
}

// syncDir copies a local directory into the container.
func (s *Session) syncDir(ctx context.Context, local, remote string) error {
	mkdirCmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", remote)}
	if _, err := s.runtime.ExecInContainer(ctx, s.config.ContainerID, mkdirCmd, runtime.ExecOptions{}); err != nil {
		return err
	}

	absLocal, _ := filepath.Abs(local)

	return filepath.WalkDir(absLocal, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(absLocal, path)
		remotePath := filepath.Join(remote, rel)

		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		remoteDir := filepath.Dir(remotePath)
		mkdirCmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p '%s'", remoteDir)}
		s.runtime.ExecInContainer(ctx, s.config.ContainerID, mkdirCmd, runtime.ExecOptions{})

		writeCmd := []string{"sh", "-c", fmt.Sprintf("cat > '%s'", remotePath)}
		_, err = s.runtime.ExecInContainer(ctx, s.config.ContainerID, writeCmd, runtime.ExecOptions{
			Stdin: strings.NewReader(string(data)),
		})
		return err
	})
}

// runCommand executes the dev command inside the container.
func (s *Session) runCommand(ctx context.Context) error {
	cmd := []string{"sh", "-c", s.config.Command}
	result, err := s.runtime.ExecInContainer(ctx, s.config.ContainerID, cmd, runtime.ExecOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	if result != nil && result.ExitCode != 0 {
		return fmt.Errorf("exit code %d", result.ExitCode)
	}
	return nil
}

// detectChanges checks if any watched files have been modified since lastSync.
func (s *Session) detectChanges(since time.Time) (bool, error) {
	for _, rule := range s.config.Syncs {
		absLocal, _ := filepath.Abs(rule.Local)
		changed := false
		filepath.WalkDir(absLocal, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.ModTime().After(since) {
				changed = true
				return filepath.SkipAll
			}
			return nil
		})
		if changed {
			return true, nil
		}
	}
	return false, nil
}
