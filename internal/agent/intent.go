package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Intent represents a parsed user intention.
type Intent struct {
	Action    string            // "create_cluster", "delete_cluster", "install_addon", "list_pods", "query", "unknown", etc.
	Resource  string            // "cluster", "namespace", "pod", "addon", "pipeline", etc.
	Name      string            // resource name
	Namespace string            // namespace if specified
	Params    map[string]string // additional parameters (nodes, profile, version, etc.)
	Raw       string            // original query
}

const intentSystemPrompt = `You are a CLI assistant for Torresmo, a container orchestrator for local development.
Parse the user's natural language request into a JSON action.

Available actions:
- create_cluster: Create a new cluster. Params: name, nodes (default 3), profile (light/standard/heavy)
- delete_cluster: Delete a cluster. Params: name
- create_namespace: Create a namespace. Params: name, cluster
- delete_namespace: Delete a namespace. Params: name, cluster
- install_addon: Install an addon. Params: name, namespace (default "default"), version, config (key=value pairs)
- remove_addon: Remove an addon. Params: name, namespace
- list_clusters: List all clusters
- list_pods: List pods. Params: namespace
- list_services: List services. Params: namespace
- list_nodes: List nodes. Params: cluster
- list_addons: List installed addons
- list_namespaces: List namespaces
- get_events: Get cluster events
- apply_manifest: Apply a YAML manifest. Params: file
- run_pipeline: Run a pipeline. Params: target
- query: General question about the cluster state or help
- unknown: Cannot determine the action

Respond with ONLY a JSON object, no markdown, no explanation:
{"action": "...", "resource": "...", "name": "...", "namespace": "...", "params": {"key": "value"}}

Examples:
User: "create a cluster with 3 nodes"
{"action": "create_cluster", "name": "dev", "params": {"nodes": "3"}}

User: "install redis on test namespace"
{"action": "install_addon", "name": "redis", "namespace": "test"}

User: "show me all pods"
{"action": "list_pods", "namespace": ""}

User: "delete the dev cluster"
{"action": "delete_cluster", "name": "dev"}

User: "what pods are failing?"
{"action": "query", "params": {"question": "what pods are failing?"}}
`

// ParseIntent uses Ollama to parse a natural language query into an Intent.
func ParseIntent(ctx context.Context, client *OllamaClient, query string) (*Intent, error) {
	messages := []ChatMessage{
		{Role: "system", Content: intentSystemPrompt},
		{Role: "user", Content: query},
	}

	response, err := client.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to parse intent: %w", err)
	}

	response = cleanJSON(response)

	var raw struct {
		Action    string            `json:"action"`
		Resource  string            `json:"resource"`
		Name      string            `json:"name"`
		Namespace string            `json:"namespace"`
		Params    map[string]string `json:"params"`
	}

	if err := json.Unmarshal([]byte(response), &raw); err != nil {
		return &Intent{Action: "unknown", Raw: query}, nil
	}

	return &Intent{
		Action:    raw.Action,
		Resource:  raw.Resource,
		Name:      raw.Name,
		Namespace: raw.Namespace,
		Params:    raw.Params,
		Raw:       query,
	}, nil
}

// cleanJSON extracts JSON from a potentially markdown-wrapped response.
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		if len(jsonLines) > 0 {
			return strings.Join(jsonLines, "\n")
		}
	}

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
