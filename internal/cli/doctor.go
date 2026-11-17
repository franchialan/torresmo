package cli

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/runtime/docker"
	"github.com/torresmo-dev/torresmo/internal/runtime/podman"
	"github.com/torresmo-dev/torresmo/internal/storage/sqlite"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system requirements and diagnose issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("🐷 Torresmo Doctor v%s\n\n", Version)

		ready := true

		// --- Runtime check ---
		fmt.Println("Runtime:")
		rtName, err := runtime.Detect()
		if err != nil {
			fmt.Println("  ✗ No container runtime found (install Docker or Podman)")
			ready = false
		} else {
			switch rtName {
			case "docker":
				ctx := context.Background()
				client, err := docker.New(ctx)
				if err != nil {
					fmt.Printf("  ⚠ Docker detected but daemon is not running\n")
					ready = false
				} else {
					info, err := client.Info(ctx)
					if err != nil {
						fmt.Printf("  ⚠ Docker connected but failed to get version\n")
					} else {
						fmt.Printf("  ✓ Docker v%s detected\n", info.Version)
					}
					fmt.Println("  ✓ Docker daemon is running")
				}
			case "podman":
				ctx := context.Background()
				client, err := podman.New(ctx)
				if err != nil {
					fmt.Printf("  ⚠ Podman detected but daemon is not running\n")
					ready = false
				} else {
					info, err := client.Info(ctx)
					if err != nil {
						fmt.Printf("  ⚠ Podman connected but failed to get version\n")
					} else {
						fmt.Printf("  ✓ Podman v%s detected\n", info.Version)
					}
					fmt.Println("  ✓ Podman daemon is running")
				}
			default:
				fmt.Printf("  ✓ %s detected\n", rtName)
			}
		}

		// --- Storage check ---
		fmt.Println("\nStorage:")
		dbPath, err := sqlite.DefaultDBPath()
		if err != nil {
			fmt.Printf("  ✗ Failed to determine database path: %v\n", err)
		} else {
			store, err := sqlite.New(dbPath)
			if err != nil {
				fmt.Printf("  ✗ Failed to open database: %v\n", err)
			} else {
				fmt.Printf("  ✓ Database at %s\n", dbPath)
				_ = store.Close()
			}
		}

		// --- Ollama check ---
		fmt.Println("\nAgent:")
		if _, err := exec.LookPath("ollama"); err != nil {
			fmt.Println("  ✗ Ollama not found (install from https://ollama.ai)")
		} else {
			fmt.Println("  ✓ Ollama is installed")
		}

		// --- Clusters check ---
		fmt.Println("\nClusters:")
		mgr, _, depsErr := initDeps()
		if depsErr != nil {
			fmt.Println("  ⚠ Cannot check clusters (runtime unavailable)")
		} else {
			clusters, err := mgr.ListClusters(context.Background())
			if err != nil {
				fmt.Printf("  ⚠ Failed to list clusters: %v\n", err)
			} else if len(clusters) == 0 {
				fmt.Println("  No clusters found")
			} else {
				for _, c := range clusters {
					fmt.Printf("  • %s (%s, %d/%d nodes ready)\n", c.Name, c.Phase, c.ReadyNodes, c.Nodes)
				}
			}
		}

		// --- Overall ---
		fmt.Println()
		if ready {
			fmt.Println("Overall: Ready ✓")
		} else {
			fmt.Println("Overall: Not ready ✗")
		}

		return nil
	},
}

func init() {
	addCommand(doctorCmd)
}
