package pipeline

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/torresmo-dev/torresmo/internal/runtime"
	"golang.org/x/sync/errgroup"
)

// RunResult contains the outcome of a target execution.
type RunResult struct {
	Target   string
	Status   string // "success", "failed", "skipped"
	Duration time.Duration
	Error    error
}

// Runner executes pipeline targets.
type Runner struct {
	runtime   runtime.ContainerRuntime
	tf        *Torresmofile
	workdir   string
	artifacts map[string]string // target name -> host artifact directory
	verbose   bool
	tracker   *Tracker // may be nil
	mu        sync.Mutex
}

// RunOptions configures a pipeline run.
type RunOptions struct {
	Target   string // specific target, or empty for all
	Verbose  bool
	Parallel bool // run independent targets in parallel
}

// NewRunner creates a pipeline runner.
func NewRunner(rt runtime.ContainerRuntime, tf *Torresmofile, workdir string) *Runner {
	return &Runner{
		runtime:   rt,
		tf:        tf,
		workdir:   workdir,
		artifacts: make(map[string]string),
	}
}

// SetTracker attaches a tracker for persisting run state.
func (r *Runner) SetTracker(t *Tracker) {
	r.tracker = t
}

// Run executes the pipeline targets according to the given options.
func (r *Runner) Run(ctx context.Context, opts RunOptions) ([]RunResult, error) {
	r.verbose = opts.Verbose

	// Clean up all artifact directories when the run completes.
	defer func() {
		r.mu.Lock()
		for _, dir := range r.artifacts {
			_ = os.RemoveAll(dir)
		}
		r.mu.Unlock()
	}()

	dag, err := BuildDAG(r.tf, opts.Target)
	if err != nil {
		return nil, fmt.Errorf("building dependency graph: %w", err)
	}

	levels := dag.Levels()
	if len(levels) == 0 {
		return nil, nil
	}

	var allResults []RunResult

	for _, level := range levels {
		if opts.Parallel && len(level) > 1 {
			results, err := r.runLevel(ctx, level)
			if err != nil {
				allResults = append(allResults, results...)
				if r.tracker != nil {
					r.tracker.FinishRun(ctx, allResults)
				}
				return allResults, err
			}
			allResults = append(allResults, results...)
		} else {
			for _, targetName := range level {
				result := r.runTarget(ctx, targetName)
				allResults = append(allResults, *result)
				if result.Status == "failed" {
					if r.tracker != nil {
						r.tracker.FinishRun(ctx, allResults)
					}
					return allResults, fmt.Errorf("target %q failed: %w", targetName, result.Error)
				}
			}
		}
	}

	if r.tracker != nil {
		r.tracker.FinishRun(ctx, allResults)
	}

	return allResults, nil
}

// runLevel runs all targets in a level concurrently.
func (r *Runner) runLevel(ctx context.Context, targets []string) ([]RunResult, error) {
	results := make([]RunResult, len(targets))
	g, gctx := errgroup.WithContext(ctx)

	for i, name := range targets {
		i, name := i, name
		g.Go(func() error {
			result := r.runTarget(gctx, name)
			results[i] = *result
			if result.Status == "failed" {
				return fmt.Errorf("target %q failed: %w", name, result.Error)
			}
			return nil
		})
	}

	err := g.Wait()
	return results, err
}

// runTarget executes a single target inside a container.
func (r *Runner) runTarget(ctx context.Context, targetName string) *RunResult {
	start := time.Now()
	result := &RunResult{Target: targetName}

	if r.tracker != nil {
		r.tracker.UpdateTarget(ctx, targetName, "running", 0, "", "")
	}

	target, err := r.tf.GetTarget(targetName)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.Duration = time.Since(start)
		r.trackFailure(ctx, targetName, result)
		return result
	}

	image, err := r.tf.ResolveImage(target)
	if err != nil {
		result.Status = "failed"
		result.Error = err
		result.Duration = time.Since(start)
		r.trackFailure(ctx, targetName, result)
		return result
	}

	workdir := r.tf.ResolveWorkdir(target)
	if workdir == "" {
		workdir = "/work"
	}

	r.printStep(targetName, fmt.Sprintf("using image %s", image))

	// Ensure image is available.
	if exists, _ := r.runtime.ImageExists(ctx, image); !exists {
		r.printStep(targetName, fmt.Sprintf("pulling image %s", image))
		if err := r.runtime.PullImage(ctx, image); err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("pulling image %q: %w", image, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}
	}

	// Create artifact directory for this target.
	artifactDir, err := os.MkdirTemp("", fmt.Sprintf("torresmo-artifacts-%s-", targetName))
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("creating artifact dir: %w", err)
		result.Duration = time.Since(start)
		r.trackFailure(ctx, targetName, result)
		return result
	}
	// Note: artifact dirs are cleaned up by Runner.Run after all targets complete.

	r.mu.Lock()
	r.artifacts[targetName] = artifactDir
	r.mu.Unlock()

	// Prepare environment variables.
	envVars := target.GetEnvVars()
	var envList []string
	for k, v := range envVars {
		envList = append(envList, k+"="+v)
	}

	// Set up volume mounts.
	volumes := []runtime.VolumeMount{
		{
			Source:   r.workdir,
			Target:   workdir,
			ReadOnly: false,
		},
		{
			Source:   artifactDir,
			Target:   "/artifacts",
			ReadOnly: false,
		},
	}

	containerName := fmt.Sprintf("torresmo-pipeline-%s-%d", targetName, time.Now().UnixNano())

	containerID, err := r.runtime.CreateContainer(ctx, &runtime.ContainerConfig{
		Name:    containerName,
		Image:   image,
		Command: []string{"sleep", "3600"},
		Env:     envList,
		Labels: map[string]string{
			"torresmo.dev/pipeline": "true",
			"torresmo.dev/target":   targetName,
		},
		Volumes: volumes,
	})
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("creating container: %w", err)
		result.Duration = time.Since(start)
		r.trackFailure(ctx, targetName, result)
		return result
	}
	defer func() {
		_ = r.runtime.StopContainer(context.Background(), containerID, 5)
		_ = r.runtime.RemoveContainer(context.Background(), containerID, true)
	}()

	if err := r.runtime.StartContainer(ctx, containerID); err != nil {
		result.Status = "failed"
		result.Error = fmt.Errorf("starting container: %w", err)
		result.Duration = time.Since(start)
		r.trackFailure(ctx, targetName, result)
		return result
	}

	// Handle copy_artifact blocks: copy files from previous target's artifact dir.
	for _, ca := range target.CopyArtifacts {
		r.printStep(targetName, fmt.Sprintf("copying artifact from %s: %s -> %s", ca.From, ca.Src, ca.Dest))

		r.mu.Lock()
		srcArtifactDir, ok := r.artifacts[ca.From]
		r.mu.Unlock()

		if !ok {
			result.Status = "failed"
			result.Error = fmt.Errorf("artifact directory for target %q not found", ca.From)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}

		// Look up the artifact using just the filename, since artifacts are saved by basename.
		srcFile := filepath.Base(ca.Src)
		srcPath := filepath.Join(srcArtifactDir, srcFile)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("reading artifact %q from target %q: %w", ca.Src, ca.From, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}

		// Create destination directory inside container and write the file.
		destDir := filepath.Dir(ca.Dest)
		mkdirCmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", shellEscape(destDir))}
		if _, err := r.runtime.ExecInContainer(ctx, containerID, mkdirCmd, runtime.ExecOptions{}); err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("creating directory %q in container: %w", destDir, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}

		writeCmd := []string{"sh", "-c", fmt.Sprintf("cat > %s", shellEscape(ca.Dest))}
		if _, err := r.runtime.ExecInContainer(ctx, containerID, writeCmd, runtime.ExecOptions{
			Stdin: bytes.NewReader(data),
		}); err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("writing artifact to %q in container: %w", ca.Dest, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}
	}

	// Handle copy blocks (files from host that are not in the bind mount).
	for _, cp := range target.Copies {
		r.printStep(targetName, fmt.Sprintf("copy %s -> %s", cp.Src, cp.Dest))

		srcPath := cp.Src
		if !filepath.IsAbs(srcPath) {
			srcPath = filepath.Join(r.workdir, srcPath)
		}

		info, err := os.Stat(srcPath)
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("stat %q for copy: %w", cp.Src, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}

		// Ensure destination directory exists.
		mkdirCmd := []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", shellEscape(cp.Dest))}
		if info.IsDir() {
			if _, err := r.runtime.ExecInContainer(ctx, containerID, mkdirCmd, runtime.ExecOptions{}); err != nil {
				result.Status = "failed"
				result.Error = fmt.Errorf("creating directory %q in container: %w", cp.Dest, err)
				result.Duration = time.Since(start)
				r.trackFailure(ctx, targetName, result)
				return result
			}
			// If the source is the workdir and dest matches the bind mount, skip (already mounted).
			absSrc, _ := filepath.Abs(srcPath)
			absWorkdir, _ := filepath.Abs(r.workdir)
			if absSrc == absWorkdir && cp.Dest == workdir {
				continue
			}
			// Copy directory contents via tar pipe.
			tarCmd := fmt.Sprintf("tar -cf - -C %s . | tar -xf - -C %s", shellEscape(srcPath), shellEscape(cp.Dest))
			if _, err := r.runtime.ExecInContainer(ctx, containerID, []string{"sh", "-c", tarCmd}, runtime.ExecOptions{}); err != nil {
				result.Status = "failed"
				result.Error = fmt.Errorf("copying directory %q to %q in container: %w", cp.Src, cp.Dest, err)
				result.Duration = time.Since(start)
				r.trackFailure(ctx, targetName, result)
				return result
			}
		} else {
			// Single file copy.
			destDir := filepath.Dir(cp.Dest)
			mkdirCmd = []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", shellEscape(destDir))}
			if _, err := r.runtime.ExecInContainer(ctx, containerID, mkdirCmd, runtime.ExecOptions{}); err != nil {
				result.Status = "failed"
				result.Error = fmt.Errorf("creating directory %q in container: %w", destDir, err)
				result.Duration = time.Since(start)
				r.trackFailure(ctx, targetName, result)
				return result
			}

			data, err := os.ReadFile(srcPath)
			if err != nil {
				result.Status = "failed"
				result.Error = fmt.Errorf("reading file %q for copy: %w", cp.Src, err)
				result.Duration = time.Since(start)
				r.trackFailure(ctx, targetName, result)
				return result
			}

			writeCmd := []string{"sh", "-c", fmt.Sprintf("cat > %s", shellEscape(cp.Dest))}
			if _, err := r.runtime.ExecInContainer(ctx, containerID, writeCmd, runtime.ExecOptions{
				Stdin: bytes.NewReader(data),
			}); err != nil {
				result.Status = "failed"
				result.Error = fmt.Errorf("copying %q to %q in container: %w", cp.Src, cp.Dest, err)
				result.Duration = time.Since(start)
				r.trackFailure(ctx, targetName, result)
				return result
			}
		}
	}

	// Execute run blocks.
	for i, run := range target.Runs {
		r.printStep(targetName, fmt.Sprintf("run [%d/%d]: %s", i+1, len(target.Runs), run.Command))

		cmd := []string{"sh", "-c", run.Command}

		var stdout io.Writer
		if r.verbose {
			stdout = &prefixWriter{
				prefix: fmt.Sprintf("  [%s] │ ", targetName),
				w:      os.Stdout,
			}
		} else {
			stdout = io.Discard
		}

		execResult, err := r.runtime.ExecInContainer(ctx, containerID, cmd, runtime.ExecOptions{
			Stdout: stdout,
			Stderr: stdout,
		})
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("executing %q: %w", run.Command, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}
		if execResult != nil && execResult.ExitCode != 0 {
			result.Status = "failed"
			result.Error = fmt.Errorf("command %q exited with code %d", run.Command, execResult.ExitCode)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}
	}

	// Extract artifacts from the container.
	for _, artifact := range target.Artifacts {
		r.printStep(targetName, fmt.Sprintf("saving artifact %s", artifact.Path))

		catCmd := []string{"sh", "-c", fmt.Sprintf("cat %s", shellEscape(artifact.Path))}
		var buf bytes.Buffer
		execResult, err := r.runtime.ExecInContainer(ctx, containerID, catCmd, runtime.ExecOptions{
			Stdout: &buf,
		})
		if err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("reading artifact %q from container: %w", artifact.Path, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}
		if execResult != nil && execResult.ExitCode != 0 {
			result.Status = "failed"
			result.Error = fmt.Errorf("artifact %q not found in container (exit code %d)", artifact.Path, execResult.ExitCode)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}

		// Use the artifact filename as the destination name.
		destName := filepath.Base(artifact.Path)
		destPath := filepath.Join(artifactDir, destName)
		if err := os.WriteFile(destPath, buf.Bytes(), 0o644); err != nil {
			result.Status = "failed"
			result.Error = fmt.Errorf("writing artifact %q to disk: %w", artifact.Path, err)
			result.Duration = time.Since(start)
			r.trackFailure(ctx, targetName, result)
			return result
		}
	}

	result.Status = "success"
	result.Duration = time.Since(start)
	if r.tracker != nil {
		r.tracker.UpdateTarget(ctx, targetName, "success", result.Duration, "", "")
	}
	r.printStep(targetName, fmt.Sprintf("done (%s)", result.Duration.Round(time.Millisecond)))
	return result
}

// trackFailure notifies the tracker of a failed target, if present.
func (r *Runner) trackFailure(ctx context.Context, targetName string, result *RunResult) {
	if r.tracker != nil {
		errMsg := ""
		if result.Error != nil {
			errMsg = result.Error.Error()
		}
		r.tracker.UpdateTarget(ctx, targetName, "failed", result.Duration, errMsg, "")
	}
}

// printStep prints a step header.
func (r *Runner) printStep(target, step string) {
	fmt.Printf("  [%s] %s\n", target, step)
}

// prefixWriter wraps a writer and prepends a prefix to each line.
type prefixWriter struct {
	prefix string
	w      io.Writer
	buf    []byte
}

func (pw *prefixWriter) Write(p []byte) (int, error) {
	pw.buf = append(pw.buf, p...)
	total := len(p)

	var remaining []byte
	lastNewline := bytes.LastIndexByte(pw.buf, '\n')

	if lastNewline == -1 {
		return total, nil
	}

	scanner := bufio.NewScanner(bytes.NewReader(pw.buf[:lastNewline+1]))
	for scanner.Scan() {
		line := scanner.Text()
		_, _ = fmt.Fprintf(pw.w, "%s%s\n", pw.prefix, line)
	}

	if lastNewline < len(pw.buf)-1 {
		remaining = make([]byte, len(pw.buf)-lastNewline-1)
		copy(remaining, pw.buf[lastNewline+1:])
	}
	pw.buf = remaining

	return total, nil
}

// shellEscape wraps a path in single quotes for safe shell usage.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
