package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/pipeline"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run tests against the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		integration, _ := cmd.Flags().GetBool("integration")
		file, _ := cmd.Flags().GetString("file")
		target, _ := cmd.Flags().GetString("target")
		keep, _ := cmd.Flags().GetBool("keep")
		verbose, _ := cmd.Flags().GetBool("verbose")

		// Suppress unused-variable lint; --keep and --report are wired for
		// future cluster lifecycle and report-generation support.
		_ = keep

		if _, err := os.Stat(file); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Hint: Create a Torresmofile with a \"test\" target to run tests.\nSee: https://torresmo.dev/docs/pipeline\n")
			return fmt.Errorf("torresmofile not found at %q", file)
		}

		tf, err := pipeline.ParseFile(file)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", file, err)
		}

		if integration {
			return runIntegrationTest(tf, target, verbose)
		}
		return runUnitTest(tf, target, verbose)
	},
}

func runIntegrationTest(tf *pipeline.Torresmofile, target string, verbose bool) error {
	if target == "" {
		// Prefer "integration-test" target; fall back to the --target flag's
		// default of "test".
		if _, err := tf.GetTarget("integration-test"); err == nil {
			target = "integration-test"
		} else {
			target = "test"
		}
	}

	t, err := tf.GetTarget(target)
	if err != nil {
		return fmt.Errorf("target %q not found in Torresmofile", target)
	}

	if t.NeedsCluster {
		clusterName, err := resolveCluster("")
		if err != nil {
			fmt.Println("⚠  Target requires a cluster but none was found.")
			fmt.Println("   Create one with: torresmo create cluster --name my-cluster")
			return err
		}
		fmt.Printf("Using cluster %q for integration tests\n", clusterName)
	}

	return runPipelineTarget(tf, target, verbose)
}

func runUnitTest(tf *pipeline.Torresmofile, target string, verbose bool) error {
	if target == "" {
		target = "test"
	}

	if _, err := tf.GetTarget(target); err != nil {
		names := tf.TargetNames()
		return fmt.Errorf("target %q not found in Torresmofile\nAvailable targets: %s", target, strings.Join(names, ", "))
	}

	return runPipelineTarget(tf, target, verbose)
}

func runPipelineTarget(tf *pipeline.Torresmofile, target string, verbose bool) error {
	fmt.Println("🐷 Torresmo Pipeline")
	fmt.Printf("Running target: %s\n", target)

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
		Parallel: true,
	})
	if err != nil {
		return fmt.Errorf("test run failed: %w", err)
	}

	printRunResults(results)
	return checkRunResults(results)
}

func init() {
	testCmd.Flags().Bool("integration", false, "run integration tests")
	testCmd.Flags().String("cluster", "", "cluster for integration tests")
	testCmd.Flags().String("target", "", "pipeline target to run (default: \"test\" or \"integration-test\")")
	testCmd.Flags().String("file", "Torresmofile", "path to Torresmofile")
	testCmd.Flags().Bool("keep", false, "keep ephemeral cluster after tests")
	testCmd.Flags().Bool("verbose", false, "verbose output")
	testCmd.Flags().String("report", "", "report format (e.g. \"junit\")")
	testCmd.Flags().String("output", "", "report output file")
	addCommand(testCmd)
}
