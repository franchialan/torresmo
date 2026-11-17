package cli

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/profile"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage resource profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles := profile.List()

		sort.Slice(profiles, func(i, j int) bool {
			return profiles[i].CPU < profiles[j].CPU
		})

		defaultName := profile.Default()

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tCPU\tMEMORY\tMAX-PODS\tDESCRIPTION")
		for _, p := range profiles {
			name := p.Name
			if name == defaultName {
				name += " (default)"
			}
			fmt.Fprintf(w, "%s\t%.1f\t%s\t%d\t%s\n",
				name,
				p.CPU,
				formatMemory(p.Memory),
				p.MaxPods,
				p.Description,
			)
		}
		return w.Flush()
	},
}

func formatMemory(bytes int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * mb
	)
	if bytes >= gb {
		return fmt.Sprintf("%dGi", bytes/gb)
	}
	return fmt.Sprintf("%dMi", bytes/mb)
}

var profileSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Set the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented yet")
	},
}

var profileCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a custom profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("not implemented yet")
	},
}

func init() {
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileSetCmd)
	profileCmd.AddCommand(profileCreateCmd)
	addCommand(profileCmd)
}
