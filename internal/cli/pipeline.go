package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/pipeline"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage local CI/CD pipelines",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var pipelineRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a pipeline from Torresmofile",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		target, _ := cmd.Flags().GetString("target")
		verbose, _ := cmd.Flags().GetBool("verbose")
		noParallel, _ := cmd.Flags().GetBool("no-parallel")

		tf, err := loadTorresmofile(file)
		if err != nil {
			return err
		}

		fmt.Println("🐷 Torresmo Pipeline")
		if target != "" {
			fmt.Printf("Running target: %s\n", target)
		} else {
			fmt.Printf("Running all targets: %s\n", strings.Join(tf.TargetNames(), ", "))
		}

		rt, err := getRuntime()
		if err != nil {
			return err
		}

		workdir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		runner := pipeline.NewRunner(rt, tf, workdir)
		results, err := runner.Run(ctx, pipeline.RunOptions{
			Target:   target,
			Verbose:  verbose,
			Parallel: !noParallel,
		})
		if err != nil {
			return fmt.Errorf("pipeline run failed: %w", err)
		}

		printRunResults(results)
		return checkRunResults(results)
	},
}

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pipeline targets",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")

		tf, err := loadTorresmofile(file)
		if err != nil {
			return err
		}

		fmt.Printf("%-20s %-25s %-25s %s\n", "TARGET", "DEPENDS ON", "IMAGE", "HAS ARTIFACTS")
		fmt.Printf("%-20s %-25s %-25s %s\n", "------", "----------", "-----", "-------------")

		for _, t := range tf.Targets {
			deps := "-"
			if len(t.DependsOn) > 0 {
				deps = strings.Join(t.DependsOn, ", ")
			}

			image, _ := tf.ResolveImage(&t)
			if image == "" {
				image = "-"
			}

			hasArtifacts := "no"
			if len(t.Artifacts) > 0 {
				hasArtifacts = "yes"
			}

			fmt.Printf("%-20s %-25s %-25s %s\n", t.Name, deps, image, hasArtifacts)
		}

		return nil
	},
}

var pipelineTargetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "Show the pipeline DAG structure",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")

		tf, err := loadTorresmofile(file)
		if err != nil {
			return err
		}

		dag, err := pipeline.BuildDAG(tf, "")
		if err != nil {
			return fmt.Errorf("failed to build DAG: %w", err)
		}

		levels := dag.Levels()
		fmt.Println("Pipeline DAG:")
		for i, level := range levels {
			indent := strings.Repeat("  ", i)
			for _, name := range level {
				t, _ := tf.GetTarget(name)
				detail := ""
				if t != nil && len(t.DependsOn) > 0 {
					detail = fmt.Sprintf(" (depends on: %s)", strings.Join(t.DependsOn, ", "))
				}
				fmt.Printf("%s├── %s%s\n", indent, name, detail)
			}
		}

		return nil
	},
}

var pipelineCacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage pipeline cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var pipelineCacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the pipeline cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		cache, err := pipeline.NewCache()
		if err != nil {
			return fmt.Errorf("failed to initialize cache: %w", err)
		}
		if err := cache.Clear(); err != nil {
			return fmt.Errorf("failed to clear cache: %w", err)
		}
		fmt.Println("✓ Pipeline cache cleared")
		return nil
	},
}

// loadTorresmofile resolves and parses a Torresmofile from the given path.
func loadTorresmofile(file string) (*pipeline.Torresmofile, error) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, fmt.Errorf("Torresmofile not found at %q\n\nCreate one to define your pipeline targets.\nSee: https://torresmo.dev/docs/pipeline", file)
	}
	tf, err := pipeline.ParseFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", file, err)
	}
	return tf, nil
}

// printRunResults prints a summary table of run results.
func printRunResults(results []pipeline.RunResult) {
	fmt.Println()
	fmt.Println("Results:")

	for _, r := range results {
		icon := "✓"
		switch r.Status {
		case "failed":
			icon = "✗"
		case "skipped":
			icon = "○"
		}

		suffix := ""
		if r.Error != nil {
			suffix = fmt.Sprintf(" — %v", r.Error)
		}

		fmt.Printf("  %s %-12s (%s)%s\n", icon, r.Target, formatDuration(r.Duration), suffix)
	}
	fmt.Println()
}

// checkRunResults returns an error if any target failed.
func checkRunResults(results []pipeline.RunResult) error {
	var passed, failed int
	var total time.Duration
	for _, r := range results {
		total += r.Duration
		switch r.Status {
		case "success":
			passed++
		case "failed":
			failed++
		}
	}

	fmt.Printf("Pipeline completed: %d passed, %d failed (%s total)\n", passed, failed, formatDuration(total))

	if failed > 0 {
		return fmt.Errorf("%d target(s) failed", failed)
	}
	return nil
}

// formatDuration formats a duration as a human-readable string (e.g. "2.3s").
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func init() {
	pipelineRunCmd.Flags().String("target", "", "specific target to run")
	pipelineRunCmd.Flags().Bool("verbose", false, "verbose output")
	pipelineRunCmd.Flags().String("file", "Torresmofile", "path to Torresmofile")
	pipelineRunCmd.Flags().Bool("no-parallel", false, "disable parallel execution")

	pipelineListCmd.Flags().String("file", "Torresmofile", "path to Torresmofile")

	pipelineTargetsCmd.Flags().String("file", "Torresmofile", "path to Torresmofile")

	pipelineCacheCmd.AddCommand(pipelineCacheClearCmd)

	pipelineCmd.AddCommand(pipelineRunCmd)
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineTargetsCmd)
	pipelineCmd.AddCommand(pipelineCacheCmd)
	addCommand(pipelineCmd)
}
