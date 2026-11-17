package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/tunnel"
)

var tunnelCmd = &cobra.Command{
	Use:   "tunnel [pod-name] [flags]",
	Short: "Expose local services via a public tunnel",
	Long: `Create a tunnel from your local machine to a pod or service in the cluster.

Examples:
  torresmo tunnel my-redis --port 6379
  torresmo tunnel my-redis --port 6379 --local-port 16379
  torresmo tunnel my-redis --port 6379 --provider cloudflared
  torresmo tunnel --auto --cluster dev`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		autoMode, _ := cmd.Flags().GetBool("auto")
		if autoMode {
			return runAutoTunnel(ctx, clusterName)
		}

		if len(args) < 1 {
			return fmt.Errorf("usage: torresmo tunnel <pod-name> --port <port>")
		}

		podName := args[0]
		port, _ := cmd.Flags().GetInt("port")
		if port == 0 {
			return fmt.Errorf("--port is required")
		}

		localPort, _ := cmd.Flags().GetInt("local-port")
		if localPort == 0 {
			localPort = port
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		provider, _ := cmd.Flags().GetString("provider")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		pod, err := o.GetPod(ctx, clusterName, namespace, podName)
		if err != nil {
			return fmt.Errorf("failed to find pod %q: %w", podName, err)
		}

		if pod.PodIP == "" {
			return fmt.Errorf("pod %q has no IP address (phase: %s)", podName, pod.Phase)
		}

		remoteAddr := fmt.Sprintf("%s:%d", pod.PodIP, port)

		mgr := tunnel.NewManager()
		defer mgr.StopAll()

		var t *tunnel.Tunnel
		switch provider {
		case "cloudflared":
			t, err = mgr.CreateCloudflared(ctx, localPort, remoteAddr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cloudflared failed: %v\nFalling back to local tunnel.\n\n", err)
				t, err = mgr.CreateLocal(ctx, localPort, remoteAddr)
			}
		default:
			t, err = mgr.CreateLocal(ctx, localPort, remoteAddr)
		}
		if err != nil {
			return fmt.Errorf("failed to create tunnel: %w", err)
		}

		fmt.Println("🐷 Torresmo Tunnel")
		fmt.Printf("Local:    localhost:%d\n", t.LocalPort)
		fmt.Printf("Remote:   %s:%d\n", pod.PodIP, port)
		fmt.Printf("Provider: %s\n", t.Provider)
		fmt.Println("Status:   Connected ✓")
		fmt.Println()
		fmt.Println("Press Ctrl+C to stop")

		ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
		<-ctx.Done()

		fmt.Println("\nStopping tunnel...")
		return nil
	},
}

func runAutoTunnel(ctx context.Context, clusterName string) error {
	o, err := getOrchestrator()
	if err != nil {
		return err
	}

	services, err := o.ListServices(ctx, clusterName, "")
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	if len(services) == 0 {
		fmt.Println("No services found to tunnel.")
		return nil
	}

	mgr := tunnel.NewManager()
	defer mgr.StopAll()

	type tunnelServicePort struct {
		Port       int    `json:"port"`
		TargetPort int    `json:"targetPort"`
		Protocol   string `json:"protocol"`
	}

	fmt.Println("🐷 Torresmo Tunnel (auto)")

	count := 0
	for _, svc := range services {
		if svc.ClusterIP == "" {
			continue
		}

		var ports []tunnelServicePort
		if svc.Ports != "" {
			if err := json.Unmarshal([]byte(svc.Ports), &ports); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to parse ports for service %q: %v\n", svc.Name, err)
				continue
			}
		}

		for _, p := range ports {
			remoteAddr := fmt.Sprintf("%s:%d", svc.ClusterIP, p.TargetPort)
			_, err := mgr.CreateLocal(ctx, p.Port, remoteAddr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to tunnel port %d for service %q: %v\n", p.Port, svc.Name, err)
				continue
			}

			fmt.Printf("%s.%s:%d     → localhost:%d\n", svc.Name, svc.Namespace, p.Port, p.Port)
			count++
		}
	}

	if count == 0 {
		fmt.Println("No tunnels created.")
		return nil
	}

	fmt.Println()
	fmt.Println("Press Ctrl+C to stop")

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	<-ctx.Done()

	fmt.Println("\nStopping all tunnels...")
	return nil
}

func init() {
	tunnelCmd.Flags().String("cluster", "", "cluster name")
	tunnelCmd.Flags().StringP("namespace", "n", "default", "namespace")
	tunnelCmd.Flags().Int("port", 0, "remote port on the pod")
	tunnelCmd.Flags().Int("local-port", 0, "local port (defaults to --port)")
	tunnelCmd.Flags().String("provider", "local", "tunnel provider (local, cloudflared)")
	tunnelCmd.Flags().Bool("auto", false, "auto-tunnel all services in the cluster")
	addCommand(tunnelCmd)
}
