package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/network"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy <pod-name> <localPort:remotePort> [flags]",
	Short: "Start a local proxy to cluster services",
	Long: `Forward a local port to a pod, or use --auto to forward all services.

Examples:
  torresmo proxy my-redis 6379:6379
  torresmo proxy --auto --cluster dev
  torresmo proxy --auto --env`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		autoMode, _ := cmd.Flags().GetBool("auto")
		envMode, _ := cmd.Flags().GetBool("env")
		namespace, _ := cmd.Flags().GetString("namespace")

		if autoMode {
			return runAutoProxy(ctx, clusterName, namespace, envMode)
		}

		if len(args) < 2 {
			return fmt.Errorf("usage: torresmo proxy <pod-name> <localPort:remotePort>")
		}

		podName := args[0]
		portMapping := args[1]

		localPort, remotePort, err := parsePortMapping(portMapping)
		if err != nil {
			return err
		}

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

		remoteAddr := fmt.Sprintf("%s:%d", pod.PodIP, remotePort)

		pm := network.NewProxyManager()
		defer pm.StopAll()

		pf, err := pm.Forward(ctx, localPort, remoteAddr)
		if err != nil {
			return fmt.Errorf("failed to start port-forward: %w", err)
		}

		fmt.Printf("Forwarding localhost:%d → %s:%d\n", pf.LocalPort, pod.PodIP, remotePort)
		fmt.Println("Press Ctrl+C to stop.")

		ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
		<-ctx.Done()

		fmt.Println("\nStopping port-forward...")
		return nil
	},
}

func parsePortMapping(mapping string) (localPort, remotePort int, err error) {
	parts := strings.SplitN(mapping, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("port mapping must be in localPort:remotePort format (e.g., 6379:6379)")
	}
	localPort, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid local port %q: %w", parts[0], err)
	}
	remotePort, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid remote port %q: %w", parts[1], err)
	}
	return localPort, remotePort, nil
}

func runAutoProxy(ctx context.Context, clusterName, namespace string, envMode bool) error {
	o, err := getOrchestrator()
	if err != nil {
		return err
	}

	services, err := o.ListServices(ctx, clusterName, namespace)
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	if len(services) == 0 {
		fmt.Println("No services found to forward.")
		return nil
	}

	pm := network.NewProxyManager()
	defer pm.StopAll()

	type autoServicePort struct {
		Port       int    `json:"port"`
		TargetPort int    `json:"targetPort"`
		Protocol   string `json:"protocol"`
	}

	for _, svc := range services {
		if svc.ClusterIP == "" {
			continue
		}

		var ports []autoServicePort
		if svc.Ports != "" {
			if err := json.Unmarshal([]byte(svc.Ports), &ports); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to parse ports for service %q: %v\n", svc.Name, err)
				continue
			}
		}

		for _, p := range ports {
			remoteAddr := fmt.Sprintf("%s:%d", svc.ClusterIP, p.TargetPort)
			pf, err := pm.Forward(ctx, p.Port, remoteAddr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to forward port %d for service %q: %v\n", p.Port, svc.Name, err)
				continue
			}

			if envMode {
				envKey := strings.ToUpper(strings.ReplaceAll(svc.Name, "-", "_"))
				fmt.Printf("%s_HOST=127.0.0.1\n", envKey)
				fmt.Printf("%s_PORT=%d\n", envKey, pf.LocalPort)
			} else {
				fmt.Printf("Forwarding localhost:%d → %s (service/%s)\n", pf.LocalPort, remoteAddr, svc.Name)
			}
		}
	}

	if !envMode {
		fmt.Println("Press Ctrl+C to stop all forwards.")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	<-ctx.Done()

	if !envMode {
		fmt.Println("\nStopping all port-forwards...")
	}
	return nil
}

func init() {
	proxyCmd.Flags().String("cluster", "", "cluster name")
	proxyCmd.Flags().StringP("namespace", "n", "default", "namespace")
	proxyCmd.Flags().Bool("auto", false, "auto-forward all services")
	proxyCmd.Flags().Bool("env", false, "output in KEY=value format (with --auto)")
	addCommand(proxyCmd)
}
