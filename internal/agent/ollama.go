package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OllamaClient communicates with the Ollama API.
type OllamaClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// OllamaConfig configures the Ollama client.
type OllamaConfig struct {
	BaseURL string // default: http://localhost:11434
	Model   string // default: llama3.1:8b
}

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`    // "system", "user", "assistant"
	Content string `json:"content"`
}

type generateResponse struct {
	Response string `json:"response"`
}

type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

type modelsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func NewOllamaClient(cfg OllamaConfig) *OllamaClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "llama3.1:8b"
	}
	return &OllamaClient{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		model:      cfg.Model,
		httpClient: &http.Client{},
	}
}

// Ping checks if Ollama is reachable by calling GET /api/tags.
func (c *OllamaClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("creating ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connecting to Ollama at %s: %w", c.baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}
	return nil
}

// Generate sends a prompt to the Ollama generate endpoint and returns the response.
func (c *OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":  c.model,
		"prompt": prompt,
		"stream": false,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling generate request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating generate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Ollama generate: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama generate returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding generate response: %w", err)
	}
	return result.Response, nil
}

// Chat sends a chat conversation to Ollama and returns the assistant's response.
func (c *OllamaClient) Chat(ctx context.Context, messages []ChatMessage) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model":    c.model,
		"messages": messages,
		"stream":   false,
	})
	if err != nil {
		return "", fmt.Errorf("marshalling chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling Ollama chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama chat returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding chat response: %w", err)
	}
	return result.Message.Content, nil
}

// StreamChat sends a chat conversation to Ollama with streaming enabled.
// Each content chunk is passed to the callback as it arrives.
func (c *OllamaClient) StreamChat(ctx context.Context, messages []ChatMessage, callback func(string)) error {
	body, err := json.Marshal(map[string]any{
		"model":    c.model,
		"messages": messages,
		"stream":   true,
	})
	if err != nil {
		return fmt.Errorf("marshalling stream chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating stream chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling Ollama stream chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Ollama stream chat returned status %d: %s", resp.StatusCode, string(respBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var chunk chatResponse
		if err := json.Unmarshal(line, &chunk); err != nil {
			continue
		}

		if chunk.Message.Content != "" {
			callback(chunk.Message.Content)
		}

		if chunk.Done {
			break
		}
	}
	return scanner.Err()
}

// ListModels returns the names of all models available in Ollama.
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("creating list models request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling Ollama list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama list models returned status %d", resp.StatusCode)
	}

	var result modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding list models response: %w", err)
	}

	names := make([]string, len(result.Models))
	for i, m := range result.Models {
		names[i] = m.Name
	}
	return names, nil
}

// IsAvailable returns true if Ollama is reachable.
func (c *OllamaClient) IsAvailable(ctx context.Context) bool {
	return c.Ping(ctx) == nil
}
