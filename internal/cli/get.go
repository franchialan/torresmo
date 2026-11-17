package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get resources (clusters, nodes, pods, services, deployments, namespaces)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var getClustersCmd = &cobra.Command{
	Use:   "clusters",
	Short: "List clusters",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		mgr, _, err := initDeps()
		if err != nil {
			return err
		}

		clusters, err := mgr.ListClusters(ctx)
		if err != nil {
			return fmt.Errorf("failed to list clusters: %w", err)
		}

		if len(clusters) == 0 {
			fmt.Println("No clusters found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tRUNTIME\tPROFILE\tSTATUS\tNODES\tAGE")
		for _, c := range clusters {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d/%d\t%s\n",
				c.Name,
				c.Runtime,
				c.Profile,
				c.Phase,
				c.ReadyNodes,
				c.Nodes,
				formatAge(c.CreatedAt),
			)
		}
		return w.Flush()
	},
}

var getNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List nodes in a cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		mgr, _, err := initDeps()
		if err != nil {
			return err
		}

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		nodes, err := mgr.GetNodes(ctx, clusterName)
		if err != nil {
			return fmt.Errorf("failed to get nodes: %w", err)
		}

		if len(nodes) == 0 {
			fmt.Printf("No nodes found for cluster %q.\n", clusterName)
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tROLE\tSTATUS\tADDRESS\tAGE")
		for _, n := range nodes {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				n.Name,
				n.Role,
				n.Phase,
				n.Address,
				formatAge(n.CreatedAt),
			)
		}
		return w.Flush()
	},
}

var getPodsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List pods",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		pods, err := o.ListPods(ctx, clusterName, namespace)
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}

		if len(pods) == 0 {
			fmt.Println("No pods found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		if namespace == "" {
			_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tIMAGE\tSTATUS\tNODE\tRESTARTS\tAGE")
			for _, p := range pods {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
					p.Namespace,
					p.Name,
					p.Image,
					p.Phase,
					p.Node,
					p.Restarts,
					formatAge(p.CreatedAt),
				)
			}
		} else {
			_, _ = fmt.Fprintln(w, "NAME\tIMAGE\tSTATUS\tNODE\tRESTARTS\tAGE")
			for _, p := range pods {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
					p.Name,
					p.Image,
					p.Phase,
					p.Node,
					p.Restarts,
					formatAge(p.CreatedAt),
				)
			}
		}
		return w.Flush()
	},
}

var getServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List services",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		services, err := o.ListServices(ctx, clusterName, namespace)
		if err != nil {
			return fmt.Errorf("failed to list services: %w", err)
		}

		if len(services) == 0 {
			fmt.Println("No services found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		if namespace == "" {
			_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tTYPE\tCLUSTER-IP\tPORTS\tAGE")
			for _, s := range services {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					s.Namespace,
					s.Name,
					s.Type,
					s.ClusterIP,
					formatServicePorts(s.Ports),
					formatAge(s.CreatedAt),
				)
			}
		} else {
			_, _ = fmt.Fprintln(w, "NAME\tTYPE\tCLUSTER-IP\tPORTS\tAGE")
			for _, s := range services {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					s.Name,
					s.Type,
					s.ClusterIP,
					formatServicePorts(s.Ports),
					formatAge(s.CreatedAt),
				)
			}
		}
		return w.Flush()
	},
}

var getDeploymentsCmd = &cobra.Command{
	Use:   "deployments",
	Short: "List deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		deployments, err := o.ListDeployments(ctx, clusterName, namespace)
		if err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		if len(deployments) == 0 {
			fmt.Println("No deployments found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		if namespace == "" {
			_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tREADY\tSTATUS\tAGE")
			for _, d := range deployments {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d/%d\t%s\t%s\n",
					d.Namespace,
					d.Name,
					d.ReadyReplicas,
					d.Replicas,
					d.Phase,
					formatAge(d.CreatedAt),
				)
			}
		} else {
			_, _ = fmt.Fprintln(w, "NAME\tREADY\tSTATUS\tAGE")
			for _, d := range deployments {
				_, _ = fmt.Fprintf(w, "%s\t%d/%d\t%s\t%s\n",
					d.Name,
					d.ReadyReplicas,
					d.Replicas,
					d.Phase,
					formatAge(d.CreatedAt),
				)
			}
		}
		return w.Flush()
	},
}

var getNamespacesCmd = &cobra.Command{
	Use:   "namespaces",
	Short: "List namespaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		namespaces, err := o.ListNamespaces(ctx, clusterName)
		if err != nil {
			return fmt.Errorf("failed to list namespaces: %w", err)
		}

		if len(namespaces) == 0 {
			fmt.Println("No namespaces found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tSTATUS\tAGE")
		for _, ns := range namespaces {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
				ns.Name,
				ns.Phase,
				formatAge(ns.CreatedAt),
			)
		}
		return w.Flush()
	},
}

func init() {
	getNodesCmd.Flags().String("cluster", "", "cluster name")

	getPodsCmd.Flags().String("cluster", "", "cluster name")
	getPodsCmd.Flags().StringP("namespace", "n", "", "namespace (empty = all namespaces)")

	getServicesCmd.Flags().String("cluster", "", "cluster name")
	getServicesCmd.Flags().StringP("namespace", "n", "", "namespace (empty = all namespaces)")

	getDeploymentsCmd.Flags().String("cluster", "", "cluster name")
	getDeploymentsCmd.Flags().StringP("namespace", "n", "", "namespace (empty = all namespaces)")

	getNamespacesCmd.Flags().String("cluster", "", "cluster name")

	getCmd.AddCommand(getClustersCmd)
	getCmd.AddCommand(getNodesCmd)
	getCmd.AddCommand(getPodsCmd)
	getCmd.AddCommand(getServicesCmd)
	getCmd.AddCommand(getDeploymentsCmd)
	getCmd.AddCommand(getNamespacesCmd)
	addCommand(getCmd)
}

// servicePort matches the JSON structure serialized in ServiceRecord.Ports.
type servicePort struct {
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
	Protocol   string `json:"protocol"`
	NodePort   int    `json:"nodePort,omitempty"`
}

func formatServicePorts(portsJSON string) string {
	if portsJSON == "" {
		return "<none>"
	}
	var ports []servicePort
	if err := json.Unmarshal([]byte(portsJSON), &ports); err != nil {
		return portsJSON
	}
	if len(ports) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "TCP"
		}
		s := fmt.Sprintf("%d:%d/%s", p.Port, p.TargetPort, proto)
		parts = append(parts, s)
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += "," + p
	}
	return result
}

// formatAge wraps time.Unix for int64 timestamps used in storage records.
func formatAge(ts interface{}) string {
	switch v := ts.(type) {
	case time.Time:
		return formatAgeTime(v)
	case int64:
		return formatAgeTime(time.Unix(v, 0))
	default:
		return "?"
	}
}

func formatAgeTime(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
