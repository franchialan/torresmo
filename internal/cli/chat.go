package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/agent"
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive AI chat session",
	Long:  "Start an interactive conversation with the Torresmo AI agent powered by Ollama.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ag, err := initAgent()
		if err != nil {
			return err
		}

		model, _ := cmd.Flags().GetString("model")

		cfg := agent.LoadConfig()
		if model == "" {
			model = cfg.Agent.Model
		}

		fmt.Printf("🐷 Torresmo Chat v%s (ollama/%s)\n", Version, model)
		fmt.Println("Type your requests in natural language. Type 'exit' or 'quit' to leave.")
		fmt.Println()

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}

			input := strings.TrimSpace(scanner.Text())
			if input == "" {
				continue
			}
			if input == "exit" || input == "quit" || input == "q" {
				fmt.Println("Bye! 🐷")
				break
			}

			if input == "help" {
				printChatHelp()
				continue
			}
			if input == "status" {
				input = "show me cluster status"
			}

			result, err := ag.Execute(ctx, input)
			if err != nil {
				fmt.Printf("❌ %v\n\n", err)
				continue
			}

			fmt.Printf("%s\n\n", result)
		}

		return nil
	},
}

func printChatHelp() {
	fmt.Print(`Available commands:
  help     — Show this help
  status   — Show cluster status
  exit     — Exit the chat

Example prompts:
  create a cluster with 3 nodes
  install redis on test namespace
  show me all pods
  what's the status of my cluster?
  scale my-api to 5 replicas
  run the build pipeline
`)
}

func init() {
	chatCmd.Flags().String("model", "", "Ollama model to use (default from config)")
	addCommand(chatCmd)
}
