package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/addon"
	"github.com/torresmo-dev/torresmo/internal/addon/builtin"
	"github.com/torresmo-dev/torresmo/internal/addon/community"
	"github.com/torresmo-dev/torresmo/internal/agent"
	"github.com/torresmo-dev/torresmo/internal/cluster"
	"github.com/torresmo-dev/torresmo/internal/orchestrator"
	"github.com/torresmo-dev/torresmo/internal/runtime"
	"github.com/torresmo-dev/torresmo/internal/runtime/docker"
	"github.com/torresmo-dev/torresmo/internal/runtime/podman"
	"github.com/torresmo-dev/torresmo/internal/storage"
	"github.com/torresmo-dev/torresmo/internal/storage/sqlite"
)

var (
	Version = "0.1.0-dev"
	Commit  = "none"
	Date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "torresmo",
	Short: "A container orchestrator for local development",
	Long: `Torresmo is a container orchestrator for local development.

It runs on top of Docker or Podman and gives you Kubernetes-style
multi-node clusters, one-command addon installs (Redis, Postgres,
Istio, Prometheus, …), local CI/CD pipelines, and an AI-powered
agent mode — all without leaving your laptop.`,
	Args: cobra.ArbitraryArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Future: load config, detect runtime, etc.
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		query := strings.Join(args, " ")

		ag, err := initAgent()
		if err != nil {
			return err
		}

		fmt.Printf("🐷 %s\n\n", query)

		result, err := ag.Execute(context.Background(), query)
		if err != nil {
			return fmt.Errorf("agent: %w", err)
		}

		fmt.Println(result)
		return nil
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// knownCommands is populated by init() in each subcommand file.
var knownCommands []*cobra.Command

func addCommand(cmd *cobra.Command) {
	knownCommands = append(knownCommands, cmd)
}

// Execute runs the root command.
func Execute() error {
	for _, cmd := range knownCommands {
		rootCmd.AddCommand(cmd)
	}
	return rootCmd.Execute()
}

var (
	initOnce      sync.Once
	initErr       error
	clusterMgr    *cluster.Manager
	storeClient   storage.Store
	orch          *orchestrator.Orchestrator
	runtimeClient runtime.ContainerRuntime
	addonRegistry *addon.Registry

	agentClient  *agent.Agent
	ollamaClient *agent.OllamaClient
)

func initDeps() (*cluster.Manager, storage.Store, error) {
	initOnce.Do(func() {
		dbPath, err := sqlite.DefaultDBPath()
		if err != nil {
			initErr = fmt.Errorf("failed to determine database path: %w", err)
			return
		}
		storeClient, err = sqlite.New(dbPath)
		if err != nil {
			initErr = fmt.Errorf("failed to open storage: %w", err)
			return
		}

		rtName, err := runtime.Detect()
		if err != nil {
			initErr = err
			return
		}

		var rt runtime.ContainerRuntime
		switch rtName {
		case "docker":
			rt, err = docker.New(context.Background())
			if err != nil {
				initErr = fmt.Errorf("failed to connect to Docker: %w", err)
				return
			}
		case "podman":
			rt, err = podman.New(context.Background())
			if err != nil {
				initErr = fmt.Errorf("failed to connect to Podman: %w", err)
				return
			}
		default:
			initErr = fmt.Errorf("runtime %q is not yet supported", rtName)
			return
		}

		clusterMgr = cluster.NewManager(rt, storeClient)
		orch = orchestrator.New(rt, storeClient)
		runtimeClient = rt

		addonRegistry = addon.NewRegistry(orch, storeClient)
		builtin.RegisterAll(addonRegistry, orch, storeClient)

		// Register community addons
		_ = community.RegisterAll(addonRegistry, orch, storeClient)
	})
	return clusterMgr, storeClient, initErr
}

func initAgent() (*agent.Agent, error) {
	cfg := agent.LoadConfig()

	ollamaClient = agent.NewOllamaClient(cfg.ToOllamaConfig())

	if !ollamaClient.IsAvailable(context.Background()) {
		fmt.Fprintf(os.Stderr, "\nInstall Ollama from https://ollama.ai and run:\n  ollama pull %s\n", cfg.Agent.Model)
		return nil, fmt.Errorf("ollama is not available at %s", cfg.Agent.BaseURL)
	}

	_, _, err := initDeps()
	if err != nil {
		return nil, err
	}

	agentClient = agent.NewAgent(ollamaClient, clusterMgr, orch, storeClient, addonRegistry)
	return agentClient, nil
}

func getOrchestrator() (*orchestrator.Orchestrator, error) {
	_, _, err := initDeps()
	if err != nil {
		return nil, err
	}
	return orch, nil
}

func getRuntime() (runtime.ContainerRuntime, error) {
	_, _, err := initDeps()
	if err != nil {
		return nil, err
	}
	return runtimeClient, nil
}

func getAddonRegistry() (*addon.Registry, error) {
	_, _, err := initDeps()
	if err != nil {
		return nil, err
	}
	return addonRegistry, nil
}

// resolveCluster returns the cluster name, using the flag if provided,
// then checking the active context, then auto-selecting if there is exactly one cluster.
func resolveCluster(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	// Check active context
	if ctx := cluster.GetContext(); ctx != "" {
		return ctx, nil
	}

	mgr, _, err := initDeps()
	if err != nil {
		return "", err
	}
	clusters, err := mgr.ListClusters(context.Background())
	if err != nil {
		return "", err
	}
	if len(clusters) == 0 {
		return "", fmt.Errorf("no clusters found; create one with 'torresmo create cluster'")
	}
	if len(clusters) == 1 {
		return clusters[0].Name, nil
	}
	return "", fmt.Errorf("multiple clusters found; use --cluster to specify one or set context with 'torresmo context set <name>'")
}
