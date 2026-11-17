package cli

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/devsession"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Manage dev sessions with hot-reload",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var devStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a dev session with file sync and hot-reload",
	Long: `Start a development session that watches local files, syncs them
into a running pod's container, and optionally restarts a command on changes.

Example:
  torresmo dev start --pod myapp --sync ./src:/app/src --command "go run ./cmd/api"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		podName, _ := cmd.Flags().GetString("pod")
		if podName == "" {
			return fmt.Errorf("--pod is required")
		}

		syncFlags, _ := cmd.Flags().GetStringSlice("sync")
		if len(syncFlags) == 0 {
			return fmt.Errorf("at least one --sync mapping is required (format: local:remote)")
		}

		var syncs []devsession.SyncRule
		for _, s := range syncFlags {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid sync format %q: expected local:remote", s)
			}
			syncs = append(syncs, devsession.SyncRule{Local: parts[0], Remote: parts[1]})
		}

		command, _ := cmd.Flags().GetString("command")
		clusterFlag, _ := cmd.Flags().GetString("cluster")
		namespace, _ := cmd.Flags().GetString("namespace")

		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		ctx := context.Background()
		pod, err := o.GetPod(ctx, clusterName, namespace, podName)
		if err != nil {
			return fmt.Errorf("pod %q not found: %w", podName, err)
		}
		if pod.ContainerID == "" {
			return fmt.Errorf("pod %q has no container (phase: %s)", podName, pod.Phase)
		}

		rt, err := getRuntime()
		if err != nil {
			return err
		}

		cfg := devsession.SessionConfig{
			ClusterName: clusterName,
			Namespace:   namespace,
			PodName:     podName,
			ContainerID: pod.ContainerID,
			Syncs:       syncs,
			Command:     command,
		}

		session := devsession.NewSession(rt, cfg)

		ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		return session.Start(ctx)
	},
}

var devStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the current dev session",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Dev sessions are stopped by pressing Ctrl+C in the terminal running 'torresmo dev start'.")
		return nil
	},
}

func init() {
	devStartCmd.Flags().String("pod", "", "Pod name to attach the dev session to (required)")
	devStartCmd.Flags().StringSlice("sync", nil, "Sync mapping in local:remote format (repeatable)")
	devStartCmd.Flags().String("command", "", "Command to run/restart on changes")
	devStartCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	devStartCmd.Flags().String("cluster", "", "Cluster name")

	devCmd.AddCommand(devStartCmd)
	devCmd.AddCommand(devStopCmd)
	addCommand(devCmd)
}
