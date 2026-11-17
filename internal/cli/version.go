package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/runtime/docker"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of torresmo",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("torresmo version %s\n", Version)
		fmt.Printf("  commit:  %s\n", Commit)
		fmt.Printf("  built:   %s\n", Date)

		rtName, err := runtime.Detect()
		if err != nil {
			fmt.Println("  runtime: none detected")
			return nil
		}

		switch rtName {
		case "docker":
			ctx := context.Background()
			client, err := docker.New(ctx)
			if err != nil {
				fmt.Printf("  runtime: %s (not connected)\n", rtName)
				return nil
			}
			info, err := client.Info(ctx)
			if err != nil {
				fmt.Printf("  runtime: %s (version unknown)\n", rtName)
				return nil
			}
			fmt.Printf("  runtime: %s (v%s)\n", rtName, info.Version)
		default:
			fmt.Printf("  runtime: %s\n", rtName)
		}

		return nil
	},
}

func init() {
	addCommand(versionCmd)
}
