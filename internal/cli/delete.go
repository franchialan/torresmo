package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a resource (cluster, namespace, pod)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var deleteClusterCmd = &cobra.Command{
	Use:   "cluster [name]",
	Short: "Delete a cluster",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		mgr, _, err := initDeps()
		if err != nil {
			return err
		}

		fmt.Printf("🐷 Deleting cluster %q...\n", name)

		if err := mgr.DeleteCluster(ctx, name); err != nil {
			return fmt.Errorf("failed to delete cluster: %w", err)
		}

		fmt.Printf("✓ Cluster %q deleted\n", name)
		return nil
	},
}

var deleteNamespaceCmd = &cobra.Command{
	Use:   "namespace [name]",
	Short: "Delete a namespace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		if err := o.DeleteNamespace(ctx, clusterName, name); err != nil {
			return fmt.Errorf("failed to delete namespace: %w", err)
		}

		fmt.Printf("✓ Namespace %q deleted from cluster %q\n", name, clusterName)
		return nil
	},
}

var deletePodCmd = &cobra.Command{
	Use:   "pod [name]",
	Short: "Delete a pod",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		if err := o.DeletePod(ctx, clusterName, namespace, name); err != nil {
			return fmt.Errorf("failed to delete pod: %w", err)
		}

		fmt.Printf("✓ Pod %q deleted\n", name)
		return nil
	},
}

func init() {
	deleteNamespaceCmd.Flags().String("cluster", "", "cluster name")

	deletePodCmd.Flags().String("cluster", "", "cluster name")
	deletePodCmd.Flags().StringP("namespace", "n", "default", "namespace")

	deleteCmd.AddCommand(deleteClusterCmd)
	deleteCmd.AddCommand(deleteNamespaceCmd)
	deleteCmd.AddCommand(deletePodCmd)
	addCommand(deleteCmd)
}
