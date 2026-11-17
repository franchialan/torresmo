package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/torresmo-dev/torresmo/internal/template"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Manage environment templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		templates := template.List()

		if len(templates) == 0 {
			fmt.Println("No templates found.")
			return nil
		}

		sort.Slice(templates, func(i, j int) bool {
			return templates[i].Name < templates[j].Name
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tNODES\tPROFILE\tADDONS")
		for _, t := range templates {
			addons := "-"
			if len(t.Addons) > 0 {
				addons = strings.Join(t.Addons, ", ")
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				t.Name,
				t.Description,
				t.Nodes,
				t.Profile,
				addons,
			)
		}
		return w.Flush()
	},
}

var templateSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save current environment as a template",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("template name is required (use --name)")
		}

		desc, _ := cmd.Flags().GetString("description")

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		_, store, err := initDeps()
		if err != nil {
			return err
		}

		cluster, err := store.GetCluster(ctx, clusterName)
		if err != nil {
			return fmt.Errorf("cluster %q not found: %w", clusterName, err)
		}

		addons, _ := store.ListAddons(ctx, clusterName)
		var addonNames []string
		for _, a := range addons {
			addonNames = append(addonNames, a.Name)
		}

		t := template.Template{
			Name:        name,
			Description: desc,
			Nodes:       cluster.NodeCount,
			Profile:     cluster.Profile,
			Addons:      addonNames,
		}

		if err := template.Save(t); err != nil {
			return fmt.Errorf("saving template: %w", err)
		}

		fmt.Printf("✓ Template %q saved from cluster %q (%d nodes, %d addons)\n",
			name, clusterName, cluster.NodeCount, len(addonNames))
		return nil
	},
}

var templateExportCmd = &cobra.Command{
	Use:   "export <name>",
	Short: "Export a template to a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		output, _ := cmd.Flags().GetString("output")
		if output == "" {
			output = name + ".template.yaml"
		}

		t, err := template.Get(name)
		if err != nil {
			return err
		}

		data, err := yaml.Marshal(t)
		if err != nil {
			return fmt.Errorf("marshaling template: %w", err)
		}

		if err := os.WriteFile(output, data, 0o644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}

		fmt.Printf("✓ Template %q exported to %s\n", name, output)
		return nil
	},
}

var templateImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a template from a file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		file := args[0]

		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}

		var t template.Template
		if err := yaml.Unmarshal(data, &t); err != nil {
			return fmt.Errorf("parsing template: %w", err)
		}

		if t.Name == "" {
			return fmt.Errorf("template name is required in the YAML file")
		}

		if err := template.Save(t); err != nil {
			return fmt.Errorf("saving template: %w", err)
		}

		fmt.Printf("✓ Template %q imported from %s\n", t.Name, file)
		return nil
	},
}

func init() {
	templateSaveCmd.Flags().StringP("name", "n", "", "template name")
	templateSaveCmd.Flags().StringP("description", "d", "", "template description")
	templateSaveCmd.Flags().String("cluster", "", "cluster name")

	templateExportCmd.Flags().StringP("output", "o", "", "output file path")

	templateCmd.AddCommand(templateListCmd)
	templateCmd.AddCommand(templateSaveCmd)
	templateCmd.AddCommand(templateExportCmd)
	templateCmd.AddCommand(templateImportCmd)
	addCommand(templateCmd)
}
