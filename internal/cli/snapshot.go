package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/snapshot"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage cluster snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a snapshot of the current cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("snapshot name is required (use --name)")
		}

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		_, store, err := initDeps()
		if err != nil {
			return err
		}

		mgr, err := snapshot.NewManager(store)
		if err != nil {
			return err
		}

		fmt.Printf("🐷 Creating snapshot %q of cluster %q...\n", name, clusterName)

		if err := mgr.Create(ctx, name, clusterName); err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		snap, err := mgr.Get(name)
		if err == nil {
			fmt.Printf("✓ Snapshot %q created (%d nodes, %d pods, %d addons)\n",
				name, len(snap.Nodes), len(snap.Pods), len(snap.Addons))
		} else {
			fmt.Printf("✓ Snapshot %q created\n", name)
		}

		return nil
	},
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, store, err := initDeps()
		if err != nil {
			return err
		}

		mgr, err := snapshot.NewManager(store)
		if err != nil {
			return err
		}

		infos, err := mgr.List()
		if err != nil {
			return fmt.Errorf("listing snapshots: %w", err)
		}

		if len(infos) == 0 {
			fmt.Println("No snapshots found.")
			return nil
		}

		sort.Slice(infos, func(i, j int) bool {
			return infos[i].CreatedAt.After(infos[j].CreatedAt)
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCLUSTER\tPODS\tADDONS\tSIZE\tCREATED")
		for _, info := range infos {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%s\n",
				info.Name,
				info.ClusterName,
				info.Pods,
				info.Addons,
				formatSize(info.Size),
				info.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}
		return w.Flush()
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a cluster from a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		_, store, err := initDeps()
		if err != nil {
			return err
		}

		mgr, err := snapshot.NewManager(store)
		if err != nil {
			return err
		}

		snap, err := mgr.Get(name)
		if err != nil {
			return err
		}

		fmt.Printf("Snapshot: %s\n", snap.Name)
		fmt.Printf("Cluster:  %s\n", snap.ClusterName)
		fmt.Printf("Created:  %s\n", snap.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Println()
		fmt.Println("Would restore:")
		fmt.Printf("  Nodes:       %d\n", len(snap.Nodes))
		fmt.Printf("  Namespaces:  %d\n", len(snap.Namespaces))
		fmt.Printf("  Pods:        %d\n", len(snap.Pods))
		fmt.Printf("  Services:    %d\n", len(snap.Services))
		fmt.Printf("  Deployments: %d\n", len(snap.Deployments))
		fmt.Printf("  Addons:      %d\n", len(snap.Addons))

		if len(snap.Addons) > 0 {
			fmt.Println()
			fmt.Println("Addons:")
			for _, a := range snap.Addons {
				fmt.Printf("  - %s (v%s) [%s]\n", a.Name, a.Version, a.Phase)
			}
		}

		fmt.Println()
		fmt.Println("Note: Full restore requires recreating cluster resources.")
		fmt.Println("Use 'torresmo create cluster' and 'torresmo addon install' to recreate manually.")

		return nil
	},
}

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		_, store, err := initDeps()
		if err != nil {
			return err
		}

		mgr, err := snapshot.NewManager(store)
		if err != nil {
			return err
		}

		if err := mgr.Delete(name); err != nil {
			return err
		}

		fmt.Printf("✓ Snapshot %q deleted\n", name)
		return nil
	},
}

var snapshotExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export a snapshot to a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			output = name + ".snapshot.json"
		}

		_, store, err := initDeps()
		if err != nil {
			return err
		}

		mgr, err := snapshot.NewManager(store)
		if err != nil {
			return err
		}

		if err := mgr.Export(name, output); err != nil {
			return err
		}

		fmt.Printf("✓ Snapshot %q exported to %s\n", name, output)
		return nil
	},
}

var snapshotImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a snapshot from a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file := args[0]

		_, store, err := initDeps()
		if err != nil {
			return err
		}

		mgr, err := snapshot.NewManager(store)
		if err != nil {
			return err
		}

		if err := mgr.Import(file); err != nil {
			return err
		}

		fmt.Printf("✓ Snapshot imported from %s\n", file)
		return nil
	},
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func init() {
	snapshotCreateCmd.Flags().StringP("name", "n", "", "snapshot name")
	snapshotCreateCmd.Flags().String("cluster", "", "cluster name")

	snapshotExportCmd.Flags().StringP("output", "o", "", "output file path")

	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
	snapshotCmd.AddCommand(snapshotExportCmd)
	snapshotCmd.AddCommand(snapshotImportCmd)
	addCommand(snapshotCmd)
}
