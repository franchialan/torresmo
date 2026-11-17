package agent

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the agent configuration from ~/.torresmo/config.yaml
type Config struct {
	Agent struct {
		Provider string `yaml:"provider"` // "ollama" (default) or "openai"
		Model    string `yaml:"model"`    // model name
		BaseURL  string `yaml:"base_url"` // custom base URL
		APIKey   string `yaml:"api_key"`  // for openai provider
	} `yaml:"agent"`
}

// LoadConfig loads agent configuration from ~/.torresmo/config.yaml
// Returns defaults if the file doesn't exist.
func LoadConfig() *Config {
	cfg := &Config{}
	cfg.Agent.Provider = "ollama"
	cfg.Agent.Model = "llama3.1:8b"
	cfg.Agent.BaseURL = "http://localhost:11434"

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}

	path := filepath.Join(home, ".torresmo", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	_ = yaml.Unmarshal(data, cfg)

	// Apply defaults for empty fields
	if cfg.Agent.Provider == "" {
		cfg.Agent.Provider = "ollama"
	}
	if cfg.Agent.Model == "" {
		cfg.Agent.Model = "llama3.1:8b"
	}
	if cfg.Agent.BaseURL == "" {
		cfg.Agent.BaseURL = "http://localhost:11434"
	}

	return cfg
}

// ToOllamaConfig converts agent config to OllamaConfig.
func (c *Config) ToOllamaConfig() OllamaConfig {
	return OllamaConfig{
		BaseURL: c.Agent.BaseURL,
		Model:   c.Agent.Model,
	}
}
