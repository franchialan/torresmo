package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/cluster"
	"github.com/torresmo-dev/torresmo/internal/template"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a resource (cluster, namespace)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var createClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Create a new cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		file, _ := cmd.Flags().GetString("file")
		name, _ := cmd.Flags().GetString("name")
		nodeCount, _ := cmd.Flags().GetInt("nodes")
		profileName, _ := cmd.Flags().GetString("profile")
		podCIDR, _ := cmd.Flags().GetString("pod-cidr")

		var addonNames []string

		if file != "" {
			cfg, err := cluster.LoadConfig(file)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			name = cfg.ObjectMeta.Name
			if cfg.Spec.Profile != "" {
				profileName = cfg.Spec.Profile
			}
			if cfg.Spec.Networking.PodCIDR != "" {
				podCIDR = cfg.Spec.Networking.PodCIDR
			}
			total := 0
			for _, n := range cfg.Spec.Nodes {
				c := n.Count
				if c < 1 {
					c = 1
				}
				total += c
			}
			if total > 0 {
				nodeCount = total
			}
			for _, a := range cfg.Spec.Addons {
				addonNames = append(addonNames, a.Name)
			}
		}

		// Apply template if specified.
		tmplName, _ := cmd.Flags().GetString("template")
		var tmplAddonRefs []v1.AddonRef
		if tmplName != "" {
			tmpl, err := template.Get(tmplName)
			if err != nil {
				return fmt.Errorf("failed to load template: %w", err)
			}
			cfg := tmpl.ToClusterConfig(name)
			if name == "" {
				name = cfg.ObjectMeta.Name
			}
			if !cmd.Flags().Changed("profile") {
				profileName = cfg.Spec.Profile
			}
			if !cmd.Flags().Changed("nodes") {
				total := 0
				for _, n := range cfg.Spec.Nodes {
					c := n.Count
					if c < 1 {
						c = 1
					}
					total += c
				}
				if total > 0 {
					nodeCount = total
				}
			}
			for _, a := range cfg.Spec.Addons {
				addonNames = append(addonNames, a.Name)
				tmplAddonRefs = append(tmplAddonRefs, a)
			}
		}

		if name == "" {
			return fmt.Errorf("cluster name is required (use --name or -f)")
		}

		mgr, _, err := initDeps()
		if err != nil {
			return err
		}

		fmt.Printf("🐷 Creating cluster %q with %d nodes (profile: %s)...\n", name, nodeCount, profileName)

		if err := mgr.CreateCluster(ctx, name, nodeCount, profileName, podCIDR); err != nil {
			return fmt.Errorf("failed to create cluster: %w", err)
		}

		fmt.Printf("✓ Cluster %q is ready!\n", name)

		// Auto-install addons declared in the config file or template.
		if len(addonNames) > 0 {
			registry, err := getAddonRegistry()
			if err != nil {
				fmt.Printf("⚠ Failed to initialize addon registry: %v\n", err)
				return nil
			}

			var cfgAddons []v1.AddonRef
			if file != "" {
				cfg, _ := cluster.LoadConfig(file)
				if cfg != nil {
					cfgAddons = cfg.Spec.Addons
				}
			}
			cfgAddons = append(cfgAddons, tmplAddonRefs...)

			fmt.Println()
			for _, ref := range cfgAddons {
				ns := ref.Namespace
				if ns == "" {
					ns = "default"
				}
				version := ref.Version
				fmt.Printf("  Installing addon %q", ref.Name)
				if version != "" {
					fmt.Printf(" (version %s)", version)
				}
				fmt.Printf(" in namespace %q...\n", ns)

				err := registry.Install(ctx, ref.Name, addon.InstallOptions{
					ClusterName: name,
					Namespace:   ns,
					Version:     version,
					Config:      ref.Config,
				})
				if err != nil {
					fmt.Printf("  ⚠ Failed to install %q: %v\n", ref.Name, err)
				} else {
					fmt.Printf("  ✓ %s installed\n", ref.Name)
				}
			}
		}

		return nil
	},
}

var createNamespaceCmd = &cobra.Command{
	Use:   "namespace [name]",
	Short: "Create a namespace in the current cluster",
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

		if err := o.CreateNamespace(ctx, clusterName, name); err != nil {
			return fmt.Errorf("failed to create namespace: %w", err)
		}

		fmt.Printf("✓ Namespace %q created in cluster %q\n", name, clusterName)
		return nil
	},
}

func init() {
	createClusterCmd.Flags().StringP("name", "n", "", "cluster name")
	createClusterCmd.Flags().Int("nodes", 3, "number of nodes")
	createClusterCmd.Flags().String("profile", "standard", "resource profile")
	createClusterCmd.Flags().String("pod-cidr", "10.244.0.0/16", "pod CIDR range")
	createClusterCmd.Flags().StringP("file", "f", "", "config file path")
	createClusterCmd.Flags().StringP("template", "t", "", "template name (e.g., go-api, minimal)")

	createNamespaceCmd.Flags().String("cluster", "", "cluster name")

	createCmd.AddCommand(createClusterCmd)
	createCmd.AddCommand(createNamespaceCmd)
	addCommand(createCmd)
}
