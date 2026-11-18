package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/cluster"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show or switch the active cluster context",
	RunE: func(cmd *cobra.Command, args []string) error {
		current := cluster.GetContext()
		if current == "" {
			fmt.Println("No active context. Create a cluster or set one with:")
			fmt.Println("  torresmo context set <cluster-name>")
			return nil
		}
		fmt.Printf("Active context: %s\n", cluster.ContextName(current))
		fmt.Printf("Cluster:        %s\n", current)
		return nil
	},
}

var contextSetCmd = &cobra.Command{
	Use:   "set <cluster-name>",
	Short: "Set the active cluster context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		mgr, _, err := initDeps()
		if err != nil {
			return err
		}

		if _, err := mgr.GetCluster(cmd.Context(), name); err != nil {
			return fmt.Errorf("cluster %q not found", name)
		}

		if err := cluster.SetContext(name); err != nil {
			return fmt.Errorf("failed to set context: %w", err)
		}

		fmt.Printf("✓ Context switched to %s\n", cluster.ContextName(name))
		return nil
	},
}

var contextClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the active cluster context",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = cluster.ClearContext()
		fmt.Println("✓ Context cleared")
		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextSetCmd)
	contextCmd.AddCommand(contextClearCmd)
	addCommand(contextCmd)
}
