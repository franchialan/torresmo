package agent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/cluster"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/storage"
)

// Agent executes intents against the torresmo system.
type Agent struct {
	ollama     *OllamaClient
	clusterMgr *cluster.Manager
	orch       *orchestrator.Orchestrator
	store      storage.Store
	addonReg   *addon.Registry
}

func NewAgent(ollama *OllamaClient, mgr *cluster.Manager, orch *orchestrator.Orchestrator, store storage.Store, addonReg *addon.Registry) *Agent {
	return &Agent{
		ollama:     ollama,
		clusterMgr: mgr,
		orch:       orch,
		store:      store,
		addonReg:   addonReg,
	}
}

// Execute parses a natural language query and executes the corresponding action.
// Returns a human-readable response string.
func (a *Agent) Execute(ctx context.Context, query string) (string, error) {
	intent, err := ParseIntent(ctx, a.ollama, query)
	if err != nil {
		return "", fmt.Errorf("understanding request: %w", err)
	}

	return a.executeIntent(ctx, intent)
}

func (a *Agent) executeIntent(ctx context.Context, intent *Intent) (string, error) {
	switch intent.Action {
	case "create_cluster":
		return a.createCluster(ctx, intent)
	case "delete_cluster":
		return a.deleteCluster(ctx, intent)
	case "create_namespace":
		return a.createNamespace(ctx, intent)
	case "delete_namespace":
		return a.deleteNamespace(ctx, intent)
	case "install_addon":
		return a.installAddon(ctx, intent)
	case "remove_addon":
		return a.removeAddon(ctx, intent)
	case "list_clusters":
		return a.listClusters(ctx)
	case "list_pods":
		return a.listPods(ctx, intent)
	case "list_services":
		return a.listServices(ctx, intent)
	case "list_nodes":
		return a.listNodes(ctx, intent)
	case "list_addons":
		return a.listAddons(ctx)
	case "list_namespaces":
		return a.listNamespaces(ctx)
	case "get_events":
		return a.getEvents(ctx, intent)
	case "apply_manifest":
		return a.applyManifest(ctx, intent)
	case "run_pipeline":
		return a.runPipeline(ctx, intent)
	case "query":
		return a.answerQuery(ctx, intent)
	default:
		return "", fmt.Errorf("I don't understand %q. Try something like:\n  - create a cluster\n  - install redis\n  - show me all pods\n  - run the build pipeline", intent.Raw)
	}
}

func (a *Agent) resolveActiveCluster(ctx context.Context) (string, error) {
	clusters, err := a.clusterMgr.ListClusters(ctx)
	if err != nil {
		return "", err
	}
	if len(clusters) == 0 {
		return "", fmt.Errorf("no clusters found. Create one first with: torresmo create cluster --name dev")
	}
	if len(clusters) == 1 {
		return clusters[0].Name, nil
	}
	for _, c := range clusters {
		if c.Phase == "Running" {
			return c.Name, nil
		}
	}
	return clusters[0].Name, nil
}

// --- Cluster operations ---

func (a *Agent) createCluster(ctx context.Context, intent *Intent) (string, error) {
	name := intent.Name
	if name == "" {
		name = "dev"
	}

	nodes := 3
	if n, ok := intent.Params["nodes"]; ok {
		if v, err := strconv.Atoi(n); err == nil {
			nodes = v
		}
	}

	profile := "standard"
	if p, ok := intent.Params["profile"]; ok {
		profile = p
	}

	err := a.clusterMgr.CreateCluster(ctx, name, nodes, profile, "10.244.0.0/16")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Cluster %q created with %d nodes (profile: %s)", name, nodes, profile), nil
}

func (a *Agent) deleteCluster(ctx context.Context, intent *Intent) (string, error) {
	name := intent.Name
	if name == "" {
		return "", fmt.Errorf("please specify which cluster to delete, e.g.: delete the dev cluster")
	}

	err := a.clusterMgr.DeleteCluster(ctx, name)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Cluster %q deleted", name), nil
}

func (a *Agent) listClusters(ctx context.Context) (string, error) {
	clusters, err := a.clusterMgr.ListClusters(ctx)
	if err != nil {
		return "", err
	}

	if len(clusters) == 0 {
		return "No clusters found. Create one with: torresmo create cluster --name dev", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d cluster(s):\n", len(clusters)))
	for _, c := range clusters {
		sb.WriteString(fmt.Sprintf("  • %s — %s (nodes: %d/%d, profile: %s, cidr: %s)\n",
			c.Name, c.Phase, c.ReadyNodes, c.Nodes, c.Profile, c.PodCIDR))
	}
	return sb.String(), nil
}

// --- Namespace operations ---

func (a *Agent) createNamespace(ctx context.Context, intent *Intent) (string, error) {
	clusterName := intent.Params["cluster"]
	if clusterName == "" {
		var err error
		clusterName, err = a.resolveActiveCluster(ctx)
		if err != nil {
			return "", err
		}
	}

	name := intent.Name
	if name == "" {
		return "", fmt.Errorf("please specify a namespace name, e.g.: create namespace staging")
	}

	err := a.orch.CreateNamespace(ctx, clusterName, name)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Namespace %q created in cluster %q", name, clusterName), nil
}

func (a *Agent) deleteNamespace(ctx context.Context, intent *Intent) (string, error) {
	clusterName := intent.Params["cluster"]
	if clusterName == "" {
		var err error
		clusterName, err = a.resolveActiveCluster(ctx)
		if err != nil {
			return "", err
		}
	}

	name := intent.Name
	if name == "" {
		return "", fmt.Errorf("please specify which namespace to delete, e.g.: delete namespace staging")
	}

	err := a.orch.DeleteNamespace(ctx, clusterName, name)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Namespace %q deleted from cluster %q", name, clusterName), nil
}

func (a *Agent) listNamespaces(ctx context.Context) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	namespaces, err := a.orch.ListNamespaces(ctx, clusterName)
	if err != nil {
		return "", err
	}

	if len(namespaces) == 0 {
		return fmt.Sprintf("No namespaces found in cluster %q.", clusterName), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d namespace(s) in cluster %q:\n", len(namespaces), clusterName))
	for _, ns := range namespaces {
		sb.WriteString(fmt.Sprintf("  • %s — %s\n", ns.Name, ns.Phase))
	}
	return sb.String(), nil
}

// --- Addon operations ---

func (a *Agent) installAddon(ctx context.Context, intent *Intent) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	addonName := intent.Name
	if addonName == "" {
		return "", fmt.Errorf("please specify which addon to install, e.g.: install redis")
	}

	ns := intent.Namespace
	if ns == "" {
		ns = "default"
	}

	version := intent.Params["version"]

	var config map[string]string
	if len(intent.Params) > 0 {
		config = make(map[string]string)
		for k, v := range intent.Params {
			if k != "version" {
				config[k] = v
			}
		}
	}

	err = a.addonReg.Install(ctx, addonName, addon.InstallOptions{
		ClusterName: clusterName,
		Namespace:   ns,
		Version:     version,
		Config:      config,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Addon %q installed in namespace %q on cluster %q", addonName, ns, clusterName), nil
}

func (a *Agent) removeAddon(ctx context.Context, intent *Intent) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	addonName := intent.Name
	if addonName == "" {
		return "", fmt.Errorf("please specify which addon to remove, e.g.: remove redis")
	}

	ns := intent.Namespace
	if ns == "" {
		ns = "default"
	}

	err = a.addonReg.Uninstall(ctx, addonName, addon.UninstallOptions{
		ClusterName: clusterName,
		Namespace:   ns,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Addon %q removed from namespace %q on cluster %q", addonName, ns, clusterName), nil
}

func (a *Agent) listAddons(ctx context.Context) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	addons, err := a.addonReg.List(ctx, clusterName)
	if err != nil {
		return "", err
	}

	if len(addons) == 0 {
		available := a.addonReg.Available()
		return fmt.Sprintf("No addons installed. Available addons: %s", strings.Join(available, ", ")), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d addon(s) in cluster %q:\n", len(addons), clusterName))
	for _, ad := range addons {
		sb.WriteString(fmt.Sprintf("  • %s — %s (version: %s, namespace: %s)\n",
			ad.Name, ad.Phase, ad.Version, ad.Namespace))
	}
	return sb.String(), nil
}

// --- Pod operations ---

func (a *Agent) listPods(ctx context.Context, intent *Intent) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	pods, err := a.orch.ListPods(ctx, clusterName, intent.Namespace)
	if err != nil {
		return "", err
	}

	if len(pods) == 0 {
		return "No pods found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d pod(s):\n", len(pods)))
	for _, p := range pods {
		sb.WriteString(fmt.Sprintf("  • %s/%s — %s (node: %s, image: %s)\n",
			p.Namespace, p.Name, p.Phase, p.Node, p.Image))
	}
	return sb.String(), nil
}

// --- Service operations ---

func (a *Agent) listServices(ctx context.Context, intent *Intent) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	services, err := a.orch.ListServices(ctx, clusterName, intent.Namespace)
	if err != nil {
		return "", err
	}

	if len(services) == 0 {
		return "No services found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d service(s):\n", len(services)))
	for _, s := range services {
		sb.WriteString(fmt.Sprintf("  • %s/%s — %s (type: %s, clusterIP: %s)\n",
			s.Namespace, s.Name, s.Ports, s.Type, s.ClusterIP))
	}
	return sb.String(), nil
}

// --- Node operations ---

func (a *Agent) listNodes(ctx context.Context, intent *Intent) (string, error) {
	clusterName := intent.Params["cluster"]
	if clusterName == "" {
		var err error
		clusterName, err = a.resolveActiveCluster(ctx)
		if err != nil {
			return "", err
		}
	}

	nodes, err := a.clusterMgr.GetNodes(ctx, clusterName)
	if err != nil {
		return "", err
	}

	if len(nodes) == 0 {
		return fmt.Sprintf("No nodes found in cluster %q.", clusterName), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d node(s) in cluster %q:\n", len(nodes), clusterName))
	for _, n := range nodes {
		sb.WriteString(fmt.Sprintf("  • %s — %s (role: %s, address: %s)\n",
			n.Name, n.Phase, n.Role, n.Address))
	}
	return sb.String(), nil
}

// --- Event operations ---

func (a *Agent) getEvents(ctx context.Context, intent *Intent) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	events, err := a.orch.ListEvents(ctx, clusterName, intent.Namespace, 50)
	if err != nil {
		return "", err
	}

	if len(events) == 0 {
		return "No events found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Last %d event(s):\n", len(events)))
	for _, e := range events {
		ts := time.Unix(e.CreatedAt, 0).Format("15:04:05")
		sb.WriteString(fmt.Sprintf("  • [%s] %s %s/%s — %s: %s\n",
			ts, e.Type, e.Kind, e.Name, e.Reason, e.Message))
	}
	return sb.String(), nil
}

// --- Manifest operations ---

func (a *Agent) applyManifest(ctx context.Context, intent *Intent) (string, error) {
	clusterName, err := a.resolveActiveCluster(ctx)
	if err != nil {
		return "", err
	}

	file := intent.Params["file"]
	if file == "" {
		return "", fmt.Errorf("please specify a manifest file, e.g.: apply manifest.yaml")
	}

	err = a.orch.ApplyManifestFile(ctx, clusterName, file)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("✓ Manifest %q applied to cluster %q", file, clusterName), nil
}

// --- Pipeline operations ---

func (a *Agent) runPipeline(_ context.Context, intent *Intent) (string, error) {
	target := intent.Params["target"]
	if target == "" {
		target = "."
	}

	return fmt.Sprintf("Pipeline execution is a long-running operation. Please use the CLI directly:\n\n  torresmo pipeline run %s\n\nThis ensures proper output streaming and signal handling.", target), nil
}

// --- Query (general questions) ---

func (a *Agent) answerQuery(ctx context.Context, intent *Intent) (string, error) {
	clusterInfo := a.gatherContext(ctx)

	messages := []ChatMessage{
		{Role: "system", Content: fmt.Sprintf(`You are a helpful assistant for Torresmo, a container orchestrator.
Here is the current cluster state:
%s

Answer the user's question concisely. If you suggest commands, use torresmo CLI syntax.`, clusterInfo)},
		{Role: "user", Content: intent.Raw},
	}

	return a.ollama.Chat(ctx, messages)
}

func (a *Agent) gatherContext(ctx context.Context) string {
	var sb strings.Builder

	clusters, err := a.clusterMgr.ListClusters(ctx)
	if err != nil {
		sb.WriteString("Clusters: unavailable\n")
	} else {
		sb.WriteString(fmt.Sprintf("Clusters: %d\n", len(clusters)))
		for _, c := range clusters {
			sb.WriteString(fmt.Sprintf("  - %s (status: %s, nodes: %d/%d, profile: %s)\n",
				c.Name, c.Phase, c.ReadyNodes, c.Nodes, c.Profile))
		}
	}

	if len(clusters) > 0 {
		clusterName := clusters[0].Name

		pods, err := a.orch.ListPods(ctx, clusterName, "")
		if err == nil {
			sb.WriteString(fmt.Sprintf("Pods: %d\n", len(pods)))
			for _, p := range pods {
				sb.WriteString(fmt.Sprintf("  - %s/%s (status: %s, image: %s)\n",
					p.Namespace, p.Name, p.Phase, p.Image))
			}
		}

		addons, err := a.addonReg.List(ctx, clusterName)
		if err == nil {
			sb.WriteString(fmt.Sprintf("Addons: %d\n", len(addons)))
			for _, ad := range addons {
				sb.WriteString(fmt.Sprintf("  - %s (version: %s, status: %s)\n",
					ad.Name, ad.Version, ad.Phase))
			}
		}
	}

	return sb.String()
}
