package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"text/tabwriter"

	"github.com/spf13/cobra"

	v1 "github.com/torresmo-dev/torresmo/api/v1"
	"github.com/torresmo-dev/torresmo/internal/mock"
	"github.com/torresmo-dev/torresmo/internal/runtime"
)

var mockCmd = &cobra.Command{
	Use:   "mock",
	Short: "Manage mock services",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var mockCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a mock service",
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath, _ := cmd.Flags().GetString("file")
		name, _ := cmd.Flags().GetString("name")
		port, _ := cmd.Flags().GetInt("port")
		clusterFlag, _ := cmd.Flags().GetString("cluster")

		var def *mock.MockDefinition
		if filePath != "" {
			var err error
			def, err = mock.LoadDefinition(filePath)
			if err != nil {
				return fmt.Errorf("loading mock definition: %w", err)
			}
		} else if name != "" {
			if port == 0 {
				port = 8080
			}
			def = &mock.MockDefinition{
				Name:      name,
				Namespace: "default",
				Port:      port,
				Routes: []mock.Route{
					{
						Method: "*",
						Path:   "/.*",
						Response: mock.RouteResponse{
							Status: 200,
							Body:   `{"status": "ok"}`,
						},
					},
				},
			}
		} else {
			return fmt.Errorf("either --file or --name is required")
		}

		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		script := mock.GeneratePythonServer(def)
		podName := "mock-" + def.Name

		pod := &v1.Pod{
			TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Pod"},
			ObjectMeta: v1.ObjectMeta{
				Name:      podName,
				Namespace: def.Namespace,
				Labels: map[string]string{
					"torresmo.dev/mock":      "true",
					"torresmo.dev/mock-name": def.Name,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.ContainerSpec{
					{
						Name:    "mock-server",
						Image:   "python:3.12-alpine",
						Command: []string{"python3", "-c", script},
						Ports: []v1.PortSpec{
							{ContainerPort: def.Port, Protocol: "tcp"},
						},
					},
				},
			},
		}

		ctx := context.Background()
		if err := o.CreatePod(ctx, clusterName, pod); err != nil {
			return fmt.Errorf("creating mock pod: %w", err)
		}

		svc := &v1.Service{
			TypeMeta: v1.TypeMeta{APIVersion: "torresmo.io/v1", Kind: "Service"},
			ObjectMeta: v1.ObjectMeta{
				Name:      podName,
				Namespace: def.Namespace,
			},
			Spec: v1.ServiceSpec{
				Type: "ClusterIP",
				Ports: []v1.ServicePort{
					{
						Name:       "http",
						Port:       def.Port,
						TargetPort: def.Port,
						Protocol:   "tcp",
					},
				},
				Selector: map[string]string{
					"torresmo.dev/mock-name": def.Name,
				},
			},
		}

		if err := o.ApplyService(ctx, clusterName, svc); err != nil {
			return fmt.Errorf("creating mock service: %w", err)
		}

		fmt.Printf("✓ Mock '%s' created on port %d with %d routes\n", def.Name, def.Port, len(def.Routes))
		return nil
	},
}

var mockListCmd = &cobra.Command{
	Use:   "list",
	Short: "List mock services",
	RunE: func(cmd *cobra.Command, args []string) error {
		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		ctx := context.Background()
		pods, err := o.ListPods(ctx, clusterName, "")
		if err != nil {
			return fmt.Errorf("listing pods: %w", err)
		}

		var mocks []mockListEntry
		for _, p := range pods {
			var labels map[string]string
			_ = json.Unmarshal([]byte(p.Labels), &labels)
			if labels["torresmo.dev/mock"] != "true" {
				continue
			}
			port := parseMockPort(p.Ports)
			mocks = append(mocks, mockListEntry{
				Name:      labels["torresmo.dev/mock-name"],
				Namespace: p.Namespace,
				Port:      port,
				Status:    p.Phase,
			})
		}

		if len(mocks) == 0 {
			fmt.Println("No mock services found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tNAMESPACE\tPORT\tSTATUS")
		for _, m := range mocks {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", m.Name, m.Namespace, m.Port, m.Status)
		}
		return w.Flush()
	},
}

type mockListEntry struct {
	Name      string
	Namespace string
	Port      int
	Status    string
}

func parseMockPort(portsJSON string) int {
	if portsJSON == "" {
		return 0
	}
	var ports []v1.PortSpec
	if err := json.Unmarshal([]byte(portsJSON), &ports); err != nil {
		return 0
	}
	if len(ports) > 0 {
		return ports[0].ContainerPort
	}
	return 0
}

var mockDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a mock service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		namespace, _ := cmd.Flags().GetString("namespace")
		clusterFlag, _ := cmd.Flags().GetString("cluster")

		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		ctx := context.Background()
		podName := "mock-" + name

		if err := o.DeletePod(ctx, clusterName, namespace, podName); err != nil {
			return fmt.Errorf("deleting mock pod: %w", err)
		}

		_ = o.DeleteService(ctx, clusterName, namespace, podName)

		fmt.Printf("✓ Mock '%s' deleted\n", name)
		return nil
	},
}

var mockLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "View mock service logs",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		namespace, _ := cmd.Flags().GetString("namespace")
		clusterFlag, _ := cmd.Flags().GetString("cluster")
		follow, _ := cmd.Flags().GetBool("follow")

		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		ctx := context.Background()
		podName := "mock-" + name

		pod, err := o.GetPod(ctx, clusterName, namespace, podName)
		if err != nil {
			return fmt.Errorf("mock %q not found: %w", name, err)
		}

		if pod.ContainerID == "" {
			return fmt.Errorf("mock %q has no container (phase: %s)", name, pod.Phase)
		}

		rt, err := getRuntime()
		if err != nil {
			return err
		}

		opts := runtime.LogOptions{
			Follow: follow,
			Tail:   "100",
		}

		if follow {
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
			defer stop()

			reader, err := rt.ContainerLogs(ctx, pod.ContainerID, opts)
			if err != nil {
				return fmt.Errorf("getting mock logs: %w", err)
			}
			defer reader.Close()

			io.Copy(os.Stdout, reader)
			return nil
		}

		reader, err := rt.ContainerLogs(ctx, pod.ContainerID, opts)
		if err != nil {
			return fmt.Errorf("getting mock logs: %w", err)
		}
		defer reader.Close()

		_, err = io.Copy(os.Stdout, reader)
		return err
	},
}

func init() {
	mockCreateCmd.Flags().StringP("file", "f", "", "Path to mock definition YAML file")
	mockCreateCmd.Flags().String("name", "", "Mock name (for inline creation)")
	mockCreateCmd.Flags().Int("port", 0, "Mock port (default 8080)")
	mockCreateCmd.Flags().String("cluster", "", "Cluster name")

	mockListCmd.Flags().String("cluster", "", "Cluster name")

	mockDeleteCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	mockDeleteCmd.Flags().String("cluster", "", "Cluster name")

	mockLogsCmd.Flags().StringP("namespace", "n", "default", "Namespace")
	mockLogsCmd.Flags().String("cluster", "", "Cluster name")
	mockLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output")

	mockCmd.AddCommand(mockCreateCmd)
	mockCmd.AddCommand(mockListCmd)
	mockCmd.AddCommand(mockLogsCmd)
	mockCmd.AddCommand(mockDeleteCmd)
	addCommand(mockCmd)
}
