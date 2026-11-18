package workflow

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/torresmo-dev/torresmo/internal/runtime"

	"gopkg.in/yaml.v3"
)

// WorkflowFile represents a parsed GitHub Actions workflow.
type WorkflowFile struct {
	Name     string
	Path     string // absolute path
	FileName string // e.g. "ci.yml"
	Jobs     []string
}

// RunOptions configures a workflow run.
type RunOptions struct {
	WorkflowFile string            // specific workflow file, or empty for all
	Job          string            // specific job, or empty for all
	Event        string            // trigger event (default: "push")
	Verbose      bool
	DryRun       bool
	Secrets      map[string]string
	Env          map[string]string
}

// RunResult contains the outcome of a workflow execution.
type RunResult struct {
	Workflow string
	Job      string
	Status   string // "success", "failed"
	Duration time.Duration
	Error    error
}

// Runner executes GitHub Actions workflows locally.
type Runner struct {
	runtime runtime.ContainerRuntime
	workdir string
	verbose bool
}

// NewRunner creates a workflow runner.
func NewRunner(rt runtime.ContainerRuntime, workdir string) *Runner {
	return &Runner{
		runtime: rt,
		workdir: workdir,
	}
}

// DiscoverWorkflows finds all .github/workflows/*.yml files.
func DiscoverWorkflows(workdir string) ([]WorkflowFile, error) {
	workflowDir := filepath.Join(workdir, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no .github/workflows directory found in %s", workdir)
		}
		return nil, err
	}

	var workflows []WorkflowFile
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		path := filepath.Join(workflowDir, entry.Name())
		wf, err := parseWorkflowFile(path, entry.Name())
		if err != nil {
			continue // skip unparseable files
		}
		workflows = append(workflows, *wf)
	}

	return workflows, nil
}

// parseWorkflowFile reads basic info from a workflow YAML.
func parseWorkflowFile(path, fileName string) (*WorkflowFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	wf := &WorkflowFile{
		Path:     path,
		FileName: fileName,
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	if name, ok := raw["name"].(string); ok {
		wf.Name = name
	} else {
		wf.Name = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	}

	if jobs, ok := raw["jobs"].(map[string]interface{}); ok {
		for jobName := range jobs {
			wf.Jobs = append(wf.Jobs, jobName)
		}
		sort.Strings(wf.Jobs)
	}

	return wf, nil
}

// Run executes a GitHub Actions workflow locally using act.
func (r *Runner) Run(ctx context.Context, opts RunOptions) ([]RunResult, error) {
	r.verbose = opts.Verbose

	// Strategy: prefer locally installed act (faster), fallback to container
	if _, err := exec.LookPath("act"); err == nil {
		return r.runLocalAct(ctx, opts, time.Now())
	}

	// Fallback: run act inside a Docker container
	actImage := "ghcr.io/catthehacker/ubuntu:act-latest"

	event := opts.Event
	if event == "" {
		event = "push"
	}

	actCmd := []string{"act", event}

	if opts.WorkflowFile != "" {
		actCmd = append(actCmd, "-W", ".github/workflows/"+opts.WorkflowFile)
	}
	if opts.Job != "" {
		actCmd = append(actCmd, "-j", opts.Job)
	}
	if opts.Verbose {
		actCmd = append(actCmd, "-v")
	}
	if opts.DryRun {
		actCmd = append(actCmd, "-n")
	}
	for k, v := range opts.Secrets {
		actCmd = append(actCmd, "-s", k+"="+v)
	}
	for k, v := range opts.Env {
		actCmd = append(actCmd, "--env", k+"="+v)
	}

	start := time.Now()

	containerName := fmt.Sprintf("torresmo-workflow-%d", time.Now().UnixNano())

	if exists, _ := r.runtime.ImageExists(ctx, actImage); !exists {
		fmt.Println("  Pulling act runner image (first time only)...")
		if err := r.runtime.PullImage(ctx, actImage); err != nil {
			return nil, fmt.Errorf("failed to pull act image: %w\n\nInstall act locally for faster execution: brew install act", err)
		}
	}

	containerID, err := r.runtime.CreateContainer(ctx, &runtime.ContainerConfig{
		Name:    containerName,
		Image:   actImage,
		Command: actCmd,
		Volumes: []runtime.VolumeMount{
			{Source: r.workdir, Target: "/workspace", ReadOnly: true},
			{Source: "/var/run/docker.sock", Target: "/var/run/docker.sock"},
		},
		Labels: map[string]string{
			"torresmo.dev/workflow": "true",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create workflow runner container: %w", err)
	}

	defer func() {
		_ = r.runtime.StopContainer(context.Background(), containerID, 5)
		_ = r.runtime.RemoveContainer(context.Background(), containerID, true)
	}()

	if err := r.runtime.StartContainer(ctx, containerID); err != nil {
		return nil, fmt.Errorf("failed to start workflow runner: %w", err)
	}

	reader, err := r.runtime.ContainerLogs(ctx, containerID, runtime.LogOptions{
		Follow: true,
	})
	if err == nil {
		_, _ = io.Copy(os.Stdout, reader)
		_ = reader.Close()
	}

	duration := time.Since(start)

	detail, err := r.runtime.InspectContainer(ctx, containerID)
	status := "success"
	if err != nil || (detail != nil && detail.State != "exited") {
		status = "unknown"
	}

	result := RunResult{
		Workflow: opts.WorkflowFile,
		Job:      opts.Job,
		Status:   status,
		Duration: duration,
	}

	return []RunResult{result}, nil
}

// runLocalAct tries to run act if it's installed locally.
func (r *Runner) runLocalAct(ctx context.Context, opts RunOptions, start time.Time) ([]RunResult, error) {
	actPath, err := exec.LookPath("act")
	if err != nil {
		return nil, fmt.Errorf("act is not installed. Install it from https://github.com/nektos/act\n  brew install act")
	}

	event := opts.Event
	if event == "" {
		event = "push"
	}

	args := []string{event}

	if opts.WorkflowFile != "" {
		args = append(args, "-W", ".github/workflows/"+opts.WorkflowFile)
	}
	if opts.Job != "" {
		args = append(args, "-j", opts.Job)
	}
	if opts.Verbose {
		args = append(args, "-v")
	}
	if opts.DryRun {
		args = append(args, "-n")
	}
	for k, v := range opts.Secrets {
		args = append(args, "-s", k+"="+v)
	}
	for k, v := range opts.Env {
		args = append(args, "--env", k+"="+v)
	}

	cmd := exec.CommandContext(ctx, actPath, args...)
	cmd.Dir = r.workdir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	duration := time.Since(start)

	status := "success"
	var runErr error
	if err != nil {
		status = "failed"
		runErr = err
	}

	return []RunResult{{
		Workflow: opts.WorkflowFile,
		Job:      opts.Job,
		Status:   status,
		Duration: duration,
		Error:    runErr,
	}}, nil
}
