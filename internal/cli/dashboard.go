package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	goruntime "runtime"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/cluster"
	"github.com/torresmo-dev/torresmo/internal/dashboard"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the Torresmo dashboard (TUI or web)",
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterFlag, _ := cmd.Flags().GetString("cluster")
		webMode, _ := cmd.Flags().GetBool("web")
		port, _ := cmd.Flags().GetInt("port")

		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		mgr, store, err := initDeps()
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		if webMode {
			addr := fmt.Sprintf(":%d", port)
			srv := dashboard.NewServer(mgr, o, store, clusterName, addr)

			url := fmt.Sprintf("http://localhost:%d", port)
			fmt.Printf("🐷 Torresmo Dashboard (web) → %s\n", url)

			// Try to open the browser automatically.
			go openBrowser(url)

			if err := srv.Start(ctx); err != nil && ctx.Err() == nil {
				return err
			}
			fmt.Println("\nDashboard closed.")
			return nil
		}

		// TUI mode (default)
		refresh, _ := cmd.Flags().GetInt("refresh")
		if refresh < 1 {
			refresh = 2
		}

		go func() {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "q" || line == "Q" {
					cancel()
					return
				}
			}
		}()

		ticker := time.NewTicker(time.Duration(refresh) * time.Second)
		defer ticker.Stop()

		renderDashboard(ctx, clusterName, mgr, o)

		for {
			select {
			case <-ctx.Done():
				fmt.Println("\nDashboard closed.")
				return nil
			case <-ticker.C:
				renderDashboard(ctx, clusterName, mgr, o)
			}
		}
	},
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}

func renderDashboard(ctx context.Context, clusterName string, mgr *cluster.Manager, o *orchestrator.Orchestrator) {
	// Clear screen and move cursor to top-left.
	fmt.Print("\033[2J\033[H")

	now := time.Now().Format("15:04:05")
	fmt.Printf("🐷 Torresmo Dashboard — cluster: %s             (refreshed: %s)\n\n", clusterName, now)

	// Nodes
	nodes, err := mgr.GetNodes(ctx, clusterName)
	if err == nil {
		fmt.Printf("Nodes (%d)\n", len(nodes))
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  NAME\tROLE\tSTATUS\tADDRESS")
		for _, n := range nodes {
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", n.Name, n.Role, n.Phase, n.Address)
		}
		_ = w.Flush()
	} else {
		fmt.Printf("Nodes: error: %v\n", err)
	}
	fmt.Println()

	// Pods (all namespaces)
	pods, err := o.ListPods(ctx, clusterName, "")
	if err == nil {
		fmt.Printf("Pods (%d)\n", len(pods))
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  NAMESPACE\tNAME\tIMAGE\tSTATUS\tNODE")
		for _, p := range pods {
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", p.Namespace, p.Name, p.Image, p.Phase, p.Node)
		}
		_ = w.Flush()
	} else {
		fmt.Printf("Pods: error: %v\n", err)
	}
	fmt.Println()

	// Services (all namespaces)
	svcs, err := o.ListServices(ctx, clusterName, "")
	if err == nil {
		fmt.Printf("Services (%d)\n", len(svcs))
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "  NAMESPACE\tNAME\tTYPE\tCLUSTER-IP")
		for _, s := range svcs {
			_, _ = fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", s.Namespace, s.Name, s.Type, s.ClusterIP)
		}
		_ = w.Flush()
	} else {
		fmt.Printf("Services: error: %v\n", err)
	}
	fmt.Println()

	// Recent events (last 5, all namespaces)
	events, err := o.ListEvents(ctx, clusterName, "", 5)
	if err == nil && len(events) > 0 {
		fmt.Println("Recent Events")
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		for _, e := range events {
			ts := time.Unix(e.CreatedAt, 0).Format("15:04:05")
			resource := fmt.Sprintf("%s %s", e.Kind, e.Name)
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", ts, e.Type, resource, e.Reason, e.Message)
		}
		_ = w.Flush()
	} else if err != nil {
		fmt.Printf("Events: error: %v\n", err)
	}
	fmt.Println()

	fmt.Println("Press Ctrl+C to exit")
}

func init() {
	dashboardCmd.Flags().String("cluster", "", "Cluster name")
	dashboardCmd.Flags().Int("refresh", 2, "Refresh interval in seconds")
	dashboardCmd.Flags().Bool("web", false, "Open web dashboard in browser")
	dashboardCmd.Flags().Int("port", 9876, "Web dashboard port")
	addCommand(dashboardCmd)
}
