package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/workflow"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Run GitHub Actions workflows locally",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available GitHub Actions workflows",
	RunE: func(cmd *cobra.Command, args []string) error {
		workdir, _ := os.Getwd()

		workflows, err := workflow.DiscoverWorkflows(workdir)
		if err != nil {
			return err
		}

		if len(workflows) == 0 {
			fmt.Println("No workflows found in .github/workflows/")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tFILE\tJOBS")
		for _, wf := range workflows {
			jobs := strings.Join(wf.Jobs, ", ")
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", wf.Name, wf.FileName, jobs)
		}
		return w.Flush()
	},
}

var workflowRunCmd = &cobra.Command{
	Use:   "run [workflow-file]",
	Short: "Run a GitHub Actions workflow locally",
	Long: `Run GitHub Actions workflows locally using act.

If act is not installed, it will be run inside a Docker container.
Install act for faster execution: brew install act`,
	RunE: func(cmd *cobra.Command, args []string) error {
		workdir, _ := os.Getwd()

		file, _ := cmd.Flags().GetString("file")
		job, _ := cmd.Flags().GetString("job")
		event, _ := cmd.Flags().GetString("event")
		verbose, _ := cmd.Flags().GetBool("verbose")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		secretStrs, _ := cmd.Flags().GetStringArray("secret")
		envStrs, _ := cmd.Flags().GetStringArray("env")

		if len(args) > 0 && file == "" {
			file = args[0]
		}

		secrets := make(map[string]string)
		for _, s := range secretStrs {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				secrets[parts[0]] = parts[1]
			}
		}

		env := make(map[string]string)
		for _, e := range envStrs {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}

		rt, err := getRuntime()
		if err != nil {
			return err
		}

		_, store, _ := initDeps()
		runner := workflow.NewRunner(rt, store, workdir)

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		fmt.Println("🐷 Torresmo Workflow Runner")
		if file != "" {
			fmt.Printf("  Workflow: %s\n", file)
		} else {
			fmt.Println("  Running all workflows")
		}
		if job != "" {
			fmt.Printf("  Job: %s\n", job)
		}
		fmt.Printf("  Event: %s\n", event)
		if dryRun {
			fmt.Println("  Mode: dry-run")
		}
		fmt.Println()

		results, err := runner.Run(ctx, workflow.RunOptions{
			WorkflowFile: file,
			Job:          job,
			Event:        event,
			Verbose:      verbose,
			DryRun:       dryRun,
			Secrets:      secrets,
			Env:          env,
		})
		if err != nil {
			return fmt.Errorf("workflow run failed: %w", err)
		}

		fmt.Println()
		fmt.Println("Results:")
		for _, r := range results {
			icon := "✓"
			if r.Status == "failed" {
				icon = "✗"
			}
			wfName := r.Workflow
			if wfName == "" {
				wfName = "all"
			}
			fmt.Printf("  %s %s (%s)\n", icon, wfName, r.Duration.Round(time.Millisecond))
			if r.Error != nil {
				fmt.Printf("    Error: %v\n", r.Error)
			}
		}

		for _, r := range results {
			if r.Status == "failed" {
				return fmt.Errorf("workflow failed")
			}
		}

		return nil
	},
}

func init() {
	workflowRunCmd.Flags().StringP("file", "f", "", "specific workflow file (e.g., ci.yml)")
	workflowRunCmd.Flags().StringP("job", "j", "", "specific job to run")
	workflowRunCmd.Flags().String("event", "push", "trigger event (push, pull_request, etc.)")
	workflowRunCmd.Flags().BoolP("verbose", "v", false, "verbose output")
	workflowRunCmd.Flags().Bool("dry-run", false, "show what would be run without executing")
	workflowRunCmd.Flags().StringArray("secret", nil, "secrets (KEY=VALUE, repeatable)")
	workflowRunCmd.Flags().StringArray("env", nil, "environment variables (KEY=VALUE, repeatable)")

	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowRunCmd)
	addCommand(workflowCmd)
}
