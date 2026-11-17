package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/addon"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Set up a complete development environment in one command",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		file, _ := cmd.Flags().GetString("file")
		if file == "" {
			file = "torresmo-env.yaml"
		}

		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hint: Create one with 'torresmo share export' or write a torresmo-env.yaml manually.\n")
			return fmt.Errorf("reading %s: %w", file, err)
		}

		var env v1.Environment
		if err := yaml.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("parsing %s: %w", file, err)
		}

		if env.APIVersion != "torresmo.io/v1" {
			return fmt.Errorf("unsupported apiVersion %q (expected torresmo.io/v1)", env.APIVersion)
		}
		if env.Kind != "Environment" {
			return fmt.Errorf("unexpected kind %q (expected Environment)", env.Kind)
		}

		clusterName := env.ObjectMeta.Name
		if clusterName == "" {
			return fmt.Errorf("environment name is required in metadata.name")
		}

		nodeCount := 0
		for _, n := range env.Spec.Cluster.Nodes {
			c := n.Count
			if c < 1 {
				c = 1
			}
			nodeCount += c
		}
		if nodeCount < 1 {
			nodeCount = 1
		}

		profileName := env.Spec.Cluster.Profile
		if profileName == "" {
			profileName = "standard"
		}

		podCIDR := env.Spec.Cluster.Networking.PodCIDR

		// Merge addons from spec.cluster.addons and spec.addons.
		addonRefs := env.Spec.Cluster.Addons
		for _, a := range env.Spec.Addons {
			addonRefs = appendAddonIfMissing(addonRefs, a)
		}

		mgr, _, err := initDeps()
		if err != nil {
			return err
		}

		fmt.Printf("🐷 Setting up environment %q...\n", clusterName)
		fmt.Printf("   Nodes: %d, Profile: %s\n", nodeCount, profileName)
		if len(addonRefs) > 0 {
			names := make([]string, len(addonRefs))
			for i, a := range addonRefs {
				names[i] = a.Name
			}
			fmt.Printf("   Addons: %s\n", strings.Join(names, ", "))
		}
		fmt.Println()

		fmt.Printf("Creating cluster %q...\n", clusterName)
		if err := mgr.CreateCluster(ctx, clusterName, nodeCount, profileName, podCIDR); err != nil {
			return fmt.Errorf("failed to create cluster: %w", err)
		}
		fmt.Printf("✓ Cluster %q is ready!\n", clusterName)

		if len(addonRefs) > 0 {
			registry, err := getAddonRegistry()
			if err != nil {
				fmt.Printf("⚠ Failed to initialize addon registry: %v\n", err)
			} else {
				fmt.Println()
				for _, ref := range addonRefs {
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
						ClusterName: clusterName,
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
		}

		if len(env.Spec.Manifests) > 0 {
			fmt.Println()
			fmt.Println("Manifests to apply:")
			for _, m := range env.Spec.Manifests {
				fmt.Printf("  - %s\n", m)
			}
			fmt.Println("Note: Manifest application is not yet implemented.")
		}

		fmt.Println()
		fmt.Printf("🐷 Environment %q is ready!\n", clusterName)
		return nil
	},
}

func appendAddonIfMissing(refs []v1.AddonRef, ref v1.AddonRef) []v1.AddonRef {
	for _, existing := range refs {
		if existing.Name == ref.Name {
			return refs
		}
	}
	return append(refs, ref)
}

func init() {
	upCmd.Flags().StringP("file", "f", "", "environment file (default: torresmo-env.yaml)")
	addCommand(upCmd)
}
