package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/addon/community"
)

var addonCmd = &cobra.Command{
	Use:   "addon",
	Short: "Manage addons (install, list, remove, search, info)",
}

var addonInstallCmd = &cobra.Command{
	Use:   "install <name>",
	Short: "Install an addon",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		registry, err := getAddonRegistry()
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		version, _ := cmd.Flags().GetString("version")
		configStrs, _ := cmd.Flags().GetStringArray("config")
		fromManifest, _ := cmd.Flags().GetString("from-manifest")

		configMap := make(map[string]string)
		for _, kv := range configStrs {
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) == 2 {
				configMap[parts[0]] = parts[1]
			}
		}

		opts := addon.InstallOptions{
			ClusterName: clusterName,
			Namespace:   namespace,
			Version:     version,
			Config:      configMap,
		}

		if fromManifest != "" {
			fmt.Printf("Installing addon from manifest %q...\n", fromManifest)

			if err := community.InstallFromManifest(ctx, registry, fromManifest, orch, storeClient, opts); err != nil {
				return fmt.Errorf("failed to install addon from manifest: %w", err)
			}

			fmt.Printf("✓ Community addon installed successfully from %q\n", fromManifest)
			return nil
		}

		versionStr := version
		if versionStr == "" {
			versionStr = "latest"
		}
		fmt.Printf("Installing addon %q (version %s) in namespace %q...\n", name, versionStr, namespace)

		if err := registry.Install(ctx, name, opts); err != nil {
			return fmt.Errorf("failed to install addon %q: %w", name, err)
		}

		fmt.Printf("✓ Addon %q installed successfully\n", name)
		printConnectionInfo(name, namespace)
		return nil
	},
}

func printConnectionInfo(name, namespace string) {
	switch name {
	case "redis":
		fmt.Printf("  Redis available at redis.%s:6379\n", namespace)
	case "postgres":
		fmt.Printf("  PostgreSQL available at postgres.%s:5432\n", namespace)
	case "grafana":
		fmt.Printf("  Grafana available at grafana.%s:3000\n", namespace)
	case "prometheus":
		fmt.Printf("  Prometheus available at prometheus.%s:9090\n", namespace)
	}
}

var addonRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an addon",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := args[0]

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		registry, err := getAddonRegistry()
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")

		opts := addon.UninstallOptions{
			ClusterName: clusterName,
			Namespace:   namespace,
		}

		if err := registry.Uninstall(ctx, name, opts); err != nil {
			return fmt.Errorf("failed to remove addon %q: %w", name, err)
		}

		fmt.Printf("✓ Addon %q removed from namespace %q\n", name, namespace)
		return nil
	},
}

var addonListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed addons",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		registry, err := getAddonRegistry()
		if err != nil {
			return err
		}

		addons, err := registry.List(ctx, clusterName)
		if err != nil {
			return fmt.Errorf("failed to list addons: %w", err)
		}

		if len(addons) == 0 {
			fmt.Println("No addons installed.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tNAMESPACE\tVERSION\tSTATUS\tAGE")
		for _, a := range addons {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				a.Name,
				a.Namespace,
				a.Version,
				a.Phase,
				formatAge(a.CreatedAt),
			)
		}
		return w.Flush()
	},
}

var addonSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search available addons",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		registry, err := getAddonRegistry()
		if err != nil {
			return err
		}

		var query string
		if len(args) > 0 {
			query = strings.ToLower(args[0])
		}

		available := registry.Available()
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tDEFAULT VERSION\tVERSIONS")

		for _, name := range available {
			if query != "" && !strings.Contains(strings.ToLower(name), query) {
				continue
			}
			info, err := registry.Info(name)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				info.Name,
				info.Description,
				info.Default,
				strings.Join(info.Versions, ", "),
			)
		}
		return w.Flush()
	},
}

var addonInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed addon information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		registry, err := getAddonRegistry()
		if err != nil {
			return err
		}

		info, err := registry.Info(name)
		if err != nil {
			return fmt.Errorf("addon %q not found: %w", name, err)
		}

		fmt.Printf("Name:        %s\n", info.Name)
		fmt.Printf("Display:     %s\n", info.DisplayName)
		fmt.Printf("Description: %s\n", info.Description)
		fmt.Printf("Default:     %s\n", info.Default)
		fmt.Printf("Versions:    %s\n", strings.Join(info.Versions, ", "))
		return nil
	},
}

var addonPublishCmd = &cobra.Command{
	Use:   "publish <path>",
	Short: "Publish a community addon from a manifest file to the local store",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		manifestPath := args[0]

		manifest, err := community.LoadManifest(manifestPath)
		if err != nil {
			return fmt.Errorf("failed to load manifest: %w", err)
		}

		cs, err := community.NewStore()
		if err != nil {
			return fmt.Errorf("failed to open community store: %w", err)
		}

		if err := cs.Install(manifest); err != nil {
			return fmt.Errorf("failed to save manifest: %w", err)
		}

		fmt.Printf("✓ Addon %q published to local store\n", manifest.Metadata.Name)
		return nil
	},
}

func init() {
	addonInstallCmd.Flags().String("cluster", "", "Cluster name")
	addonInstallCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	addonInstallCmd.Flags().String("version", "", "Addon version")
	addonInstallCmd.Flags().StringArray("config", nil, "Configuration key=value pairs (repeatable)")
	addonInstallCmd.Flags().String("from-manifest", "", "Install from a community addon manifest file")

	addonRemoveCmd.Flags().String("cluster", "", "Cluster name")
	addonRemoveCmd.Flags().StringP("namespace", "n", "default", "Namespace")

	addonListCmd.Flags().String("cluster", "", "Cluster name")

	addonCmd.AddCommand(addonInstallCmd)
	addonCmd.AddCommand(addonListCmd)
	addonCmd.AddCommand(addonRemoveCmd)
	addonCmd.AddCommand(addonSearchCmd)
	addonCmd.AddCommand(addonInfoCmd)
	addonCmd.AddCommand(addonPublishCmd)
	addCommand(addonCmd)
}
