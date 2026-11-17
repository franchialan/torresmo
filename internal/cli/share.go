package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

var shareCmd = &cobra.Command{
	Use:   "share",
	Short: "Share cluster state with teammates",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var shareExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export cluster state to a portable bundle",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			output = "torresmo-env.yaml"
		}

		_, store, err := initDeps()
		if err != nil {
			return err
		}

		cluster, err := store.GetCluster(ctx, clusterName)
		if err != nil {
			return fmt.Errorf("cluster %q not found: %w", clusterName, err)
		}

		nodes, _ := store.ListNodes(ctx, clusterName)
		addons, _ := store.ListAddons(ctx, clusterName)

		nodeSpecs := buildNodeSpecs(nodes)

		var addonRefs []v1.AddonRef
		for _, a := range addons {
			addonRefs = append(addonRefs, v1.AddonRef{
				Name:      a.Name,
				Version:   a.Version,
				Namespace: a.Namespace,
			})
		}

		env := v1.Environment{
			TypeMeta:   v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Environment"},
			ObjectMeta: v1.ObjectMeta{Name: clusterName},
			Spec: v1.EnvironmentSpec{
				Cluster: v1.ClusterSpec{
					Nodes:   nodeSpecs,
					Profile: cluster.Profile,
					Networking: v1.NetworkingSpec{
						PodCIDR: cluster.PodCIDR,
					},
				},
				Addons: addonRefs,
			},
		}

		data, err := yaml.Marshal(env)
		if err != nil {
			return fmt.Errorf("marshaling environment: %w", err)
		}

		if err := os.WriteFile(output, data, 0o644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}

		fmt.Printf("✓ Cluster %q exported to %s\n", clusterName, output)
		fmt.Printf("  Nodes: %d, Addons: %d\n", cluster.NodeCount, len(addons))
		return nil
	},
}

var shareImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import cluster state from a bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file := args[0]

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		var env v1.Environment
		if err := yaml.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("parsing environment: %w", err)
		}

		if env.APIVersion != "torresmo.io/v1" {
			return fmt.Errorf("unsupported apiVersion %q (expected torresmo.io/v1)", env.APIVersion)
		}
		if env.Kind != "Environment" {
			return fmt.Errorf("unexpected kind %q (expected Environment)", env.Kind)
		}

		nodeCount := 0
		for _, n := range env.Spec.Cluster.Nodes {
			c := n.Count
			if c < 1 {
				c = 1
			}
			nodeCount += c
		}

		fmt.Printf("Environment: %s\n", env.ObjectMeta.Name)
		fmt.Printf("Profile:     %s\n", env.Spec.Cluster.Profile)
		fmt.Printf("Nodes:       %d\n", nodeCount)
		fmt.Println()

		if len(env.Spec.Addons) > 0 {
			fmt.Println("Addons:")
			for _, a := range env.Spec.Addons {
				version := a.Version
				if version == "" {
					version = "latest"
				}
				fmt.Printf("  - %s (v%s)\n", a.Name, version)
			}
			fmt.Println()
		}

		if len(env.Spec.Manifests) > 0 {
			fmt.Println("Manifests:")
			for _, m := range env.Spec.Manifests {
				fmt.Printf("  - %s\n", m)
			}
			fmt.Println()
		}

		fmt.Println("To create this environment, run:")
		fmt.Printf("  torresmo up -f %s\n", file)

		return nil
	},
}

func buildNodeSpecs(nodes []storage.NodeRecord) []v1.NodeSpec {
	roles := make(map[string]int)
	for _, n := range nodes {
		roles[n.Role]++
	}

	var specs []v1.NodeSpec
	if count, ok := roles["control-plane"]; ok {
		specs = append(specs, v1.NodeSpec{Role: "control-plane", Count: count})
	}
	if count, ok := roles["worker"]; ok {
		specs = append(specs, v1.NodeSpec{Role: "worker", Count: count})
	}
	return specs
}

func init() {
	shareExportCmd.Flags().String("cluster", "", "cluster name")
	shareExportCmd.Flags().StringP("output", "o", "", "output file path")

	shareCmd.AddCommand(shareExportCmd)
	shareCmd.AddCommand(shareImportCmd)
	addCommand(shareCmd)
}
