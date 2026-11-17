package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/compose"
)

var composeCmd = &cobra.Command{
	Use:   "compose",
	Short: "Work with Docker Compose files",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var composeImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a Docker Compose file as Torresmo manifests",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		outputDir, _ := cmd.Flags().GetString("output")

		fmt.Printf("Converting %s...\n", filePath)

		result, err := compose.Import(filePath)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		for _, addonName := range result.KnownAddons {
			fmt.Printf("  ℹ Detected addon → use `torresmo addon install %s`\n", addonName)
		}
		for _, dep := range result.Deployments {
			fmt.Printf("  ✓ Converted service %q → Deployment\n", dep.Name)
		}
		for _, svc := range result.Services {
			fmt.Printf("  ✓ Created Service for %q\n", svc.Name)
		}
		for _, w := range result.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}

		totalManifests := len(result.Deployments) + len(result.Services)

		if outputDir != "" {
			if err := compose.WriteManifests(result, outputDir); err != nil {
				return fmt.Errorf("writing manifests: %w", err)
			}
			fmt.Printf("✓ Generated %d manifests in %s\n", totalManifests, outputDir)
		} else {
			fmt.Printf("\nDry run: would generate %d manifests. Use -o <dir> to write them.\n", totalManifests)
		}

		return nil
	},
}

var composeUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Start services from a Docker Compose file",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("--file/-f is required")
		}

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		fmt.Printf("Converting %s...\n", filePath)

		result, err := compose.Import(filePath)
		if err != nil {
			return fmt.Errorf("import failed: %w", err)
		}

		for _, w := range result.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}

		// Install known addons
		if len(result.KnownAddons) > 0 {
			registry, err := getAddonRegistry()
			if err != nil {
				return err
			}
			for _, addonName := range result.KnownAddons {
				fmt.Printf("  Installing addon %q...\n", addonName)
				err := registry.Install(ctx, addonName, addon.InstallOptions{
					ClusterName: clusterName,
					Namespace:   "default",
				})
				if err != nil {
					fmt.Printf("  ⚠ Addon %q: %v\n", addonName, err)
				} else {
					fmt.Printf("  ✓ Addon %q installed\n", addonName)
				}
			}
		}

		// Apply deployments and services
		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		for i := range result.Deployments {
			dep := &result.Deployments[i]
			fmt.Printf("  Applying deployment %q...\n", dep.Name)
			if err := o.ApplyDeployment(ctx, clusterName, dep); err != nil {
				return fmt.Errorf("applying deployment %q: %w", dep.Name, err)
			}
			fmt.Printf("  ✓ Deployment %q applied\n", dep.Name)
		}

		for i := range result.Services {
			svc := &result.Services[i]
			fmt.Printf("  Applying service %q...\n", svc.Name)
			if err := o.ApplyService(ctx, clusterName, svc); err != nil {
				return fmt.Errorf("applying service %q: %w", svc.Name, err)
			}
			fmt.Printf("  ✓ Service %q applied\n", svc.Name)
		}

		total := len(result.Deployments) + len(result.Services) + len(result.KnownAddons)
		fmt.Printf("✓ Deployed %d resources to cluster %q\n", total, clusterName)
		return nil
	},
}

func init() {
	composeImportCmd.Flags().StringP("output", "o", "", "Output directory for generated manifests")

	composeUpCmd.Flags().StringP("file", "f", "", "Path to docker-compose.yaml")
	composeUpCmd.Flags().String("cluster", "", "Cluster name")

	composeCmd.AddCommand(composeImportCmd)
	composeCmd.AddCommand(composeUpCmd)
	addCommand(composeCmd)
}
