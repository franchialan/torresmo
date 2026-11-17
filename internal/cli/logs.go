package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/runtime"
)

var logsCmd = &cobra.Command{
	Use:   "logs <pod>",
	Short: "View pod logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		podName := args[0]

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetString("tail")
		timestamps, _ := cmd.Flags().GetBool("timestamps")

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

		rt, err := getRuntime()
		if err != nil {
			return err
		}

		opts := runtime.LogOptions{
			Follow:     follow,
			Tail:       tail,
			Timestamps: timestamps,
		}

		if follow {
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
			defer stop()

			reader, err := rt.ContainerLogs(ctx, pod.ContainerID, opts)
			if err != nil {
				return fmt.Errorf("failed to get logs: %w", err)
			}
			defer reader.Close()

			io.Copy(os.Stdout, reader)
			return nil
		}

		reader, err := rt.ContainerLogs(ctx, pod.ContainerID, opts)
		if err != nil {
			return fmt.Errorf("failed to get logs: %w", err)
		}
		defer reader.Close()

		_, err = io.Copy(os.Stdout, reader)
		return err
	},
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().String("tail", "", "Number of lines from the end of the logs")
	logsCmd.Flags().Bool("timestamps", false, "Show timestamps")
	logsCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	logsCmd.Flags().String("cluster", "", "Cluster name")
	addCommand(logsCmd)
}
