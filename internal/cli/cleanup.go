package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove all Torresmo resources and clean up",
	Long: `Remove Torresmo-managed containers, networks, and database records.

By default all resources are cleaned up. Use flags to limit scope.

Examples:
  torresmo cleanup              # remove everything
  torresmo cleanup --dry-run    # show what would be removed
  torresmo cleanup --clusters   # only remove clusters`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		clustersOnly, _ := cmd.Flags().GetBool("clusters")
		imagesOnly, _ := cmd.Flags().GetBool("images")
		all, _ := cmd.Flags().GetBool("all")

		// Default: clean everything if no scope flags are set.
		if !clustersOnly && !imagesOnly {
			all = true
		}

		ctx := context.Background()

		rt, err := getRuntime()
		if err != nil {
			return err
		}

		mgr, store, err := initDeps()
		if err != nil {
			return err
		}

		var containersRemoved, networksRemoved, clustersDeleted int

		if all || clustersOnly {
			// List all managed containers.
			containers, err := rt.ListContainers(ctx, map[string]string{
				"torresmo.dev/managed": "true",
			})
			if err != nil {
				return fmt.Errorf("listing containers: %w", err)
			}

			if dryRun {
				fmt.Printf("Would remove %d container(s):\n", len(containers))
				for _, c := range containers {
					fmt.Printf("  %s  %s  (%s)\n", c.ID[:12], c.Name, c.State)
				}
			} else {
				for _, c := range containers {
					fmt.Printf("  Stopping container %s (%s)...\n", c.Name, c.ID[:12])
					_ = rt.StopContainer(ctx, c.ID, 10)
					if err := rt.RemoveContainer(ctx, c.ID, true); err != nil {
						fmt.Printf("  ⚠ Failed to remove %s: %v\n", c.Name, err)
						continue
					}
					containersRemoved++
				}
			}

			// Remove networks for each known cluster.
			clusters, err := mgr.ListClusters(ctx)
			if err == nil {
				for _, cl := range clusters {
					networkName := "torresmo-" + cl.Name

					if dryRun {
						fmt.Printf("Would remove network: %s\n", networkName)
					} else {
						fmt.Printf("  Removing network %s...\n", networkName)
						if err := rt.RemoveNetwork(ctx, networkName); err != nil {
							fmt.Printf("  ⚠ Failed to remove network %s: %v\n", networkName, err)
						} else {
							networksRemoved++
						}
					}

					// Delete cluster records from store.
					if !dryRun {
						if err := store.DeleteCluster(ctx, cl.Name); err != nil {
							fmt.Printf("  ⚠ Failed to delete cluster record %q: %v\n", cl.Name, err)
						} else {
							clustersDeleted++
						}
					}
				}
			}
		}

		if all || imagesOnly {
			// Image cleanup: list managed containers to find images in use,
			// but actual image removal is not yet implemented.
			containers, _ := rt.ListContainers(ctx, map[string]string{
				"torresmo.dev/managed": "true",
			})
			images := make(map[string]struct{})
			for _, c := range containers {
				images[c.Image] = struct{}{}
			}
			if len(images) > 0 {
				fmt.Println("\nTorresmo-related images (removal not yet implemented):")
				for img := range images {
					fmt.Printf("  %s\n", img)
				}
			}
		}

		// Remove database file when doing a full cleanup.
		if all && !dryRun {
			store.Close()

			home, err := os.UserHomeDir()
			if err == nil {
				dbPath := filepath.Join(home, ".torresmo", "torresmo.db")
				if _, statErr := os.Stat(dbPath); statErr == nil {
					if err := os.Remove(dbPath); err != nil {
						fmt.Printf("  ⚠ Failed to remove database: %v\n", err)
					} else {
						fmt.Println("  Database removed.")
					}
				}
			}
		}

		fmt.Println()
		if dryRun {
			fmt.Println("Dry run complete. No resources were modified.")
		} else {
			fmt.Printf("✅ Cleanup complete: removed %d container(s), %d network(s), %d cluster record(s)\n",
				containersRemoved, networksRemoved, clustersDeleted)
		}
		return nil
	},
}

func init() {
	cleanupCmd.Flags().Bool("all", false, "Remove everything (default when no scope flags)")
	cleanupCmd.Flags().Bool("clusters", false, "Remove clusters, their containers, and networks")
	cleanupCmd.Flags().Bool("images", false, "List Torresmo-related images (removal not yet implemented)")
	cleanupCmd.Flags().Bool("dry-run", false, "Show what would be removed without making changes")
	addCommand(cleanupCmd)
}
