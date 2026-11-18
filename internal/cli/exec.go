package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/runtime"
)

var execCmd = &cobra.Command{
	Use:                "exec [-it] <pod> -- <command>",
	Short:              "Execute a command in a pod",
	DisableFlagParsing: false,
	Args:               cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		podName := args[0]
		execArgs := cmd.Flags().Args()

		// Everything after the pod name (cobra puts remaining args after --)
		// into args; the first arg is the pod, the rest form the command.
		var command []string
		if len(args) > 1 {
			command = args[1:]
		}
		if len(command) == 0 && len(execArgs) > 1 {
			command = execArgs[1:]
		}
		if len(command) == 0 {
			command = []string{"/bin/sh"}
		}

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		interactive, _ := cmd.Flags().GetBool("interactive")
		tty, _ := cmd.Flags().GetBool("tty")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		ctx := context.Background()
		pod, err := o.GetPod(ctx, clusterName, namespace, podName)
		if err != nil {
			return fmt.Errorf("failed to find pod %q: %w", podName, err)
		}

		if pod.ContainerID == "" {
			return fmt.Errorf("pod %q has no container (phase: %s)", podName, pod.Phase)
		}

		// Use orchestrator to route exec through DinD node if needed
		opts := runtime.ExecOptions{}
		if interactive {
			opts.Interactive = true
			opts.Stdin = os.Stdin
		}
		if tty {
			opts.TTY = true
		}
		opts.Stdout = os.Stdout
		opts.Stderr = os.Stderr

		// Use orchestrator to route exec through DinD node if needed
		result, err := o.ExecInPod(ctx, pod, command, opts)
		if err != nil {
			return fmt.Errorf("exec failed: %w", err)
		}

		if !interactive && result.Output != "" {
			fmt.Print(result.Output)
		}

		if result.ExitCode != 0 {
			os.Exit(result.ExitCode)
		}
		return nil
	},
}

func init() {
	execCmd.Flags().BoolP("interactive", "i", false, "Keep stdin open")
	execCmd.Flags().BoolP("tty", "t", false, "Allocate a pseudo-TTY")
	execCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	execCmd.Flags().String("cluster", "", "Cluster name")
	addCommand(execCmd)
}
