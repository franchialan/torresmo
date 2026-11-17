package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply -f <file>",
	Short: "Apply a manifest file to the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		filePath, _ := cmd.Flags().GetString("filename")
		if filePath == "" {
			return fmt.Errorf("--filename/-f is required")
		}

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		fmt.Printf("Applying manifests from %q to cluster %q...\n", filePath, clusterName)

		if err := o.ApplyManifestFile(ctx, clusterName, filePath); err != nil {
			return fmt.Errorf("failed to apply manifests: %w", err)
		}

		fmt.Println("✓ Manifests applied successfully")
		return nil
	},
}

func init() {
	applyCmd.Flags().StringP("filename", "f", "", "Path to the manifest file")
	applyCmd.Flags().String("cluster", "", "cluster name")
	addCommand(applyCmd)
}
