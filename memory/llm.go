package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// LLMAdapter is the optional language model extension point used for intelligent
// memory extraction from documents. The system runs fully without one — heuristic
// extraction is the fallback.
type LLMAdapter interface {
	// Complete sends a system + user prompt and returns the model's text response.
	Complete(ctx context.Context, system, user string) (string, error)
	// Provider returns a short identifier, e.g. "anthropic", "openai", "ollama".
	Provider() string
}

// NewLLMAdapter constructs the right adapter from config values.
// Returns nil when provider is "none" or empty.
func NewLLMAdapter(provider, model, apiKeyEnv string) LLMAdapter {
	key := strings.TrimSpace(os.Getenv(apiKeyEnv))
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		if model == "" {
			model = "claude-haiku-4-5-20251001"
		}
		return &anthropicAdapter{model: model, apiKey: key}
	case "openai":
		if model == "" {
			model = "gpt-4o-mini"
		}
		return &openaiAdapter{model: model, apiKey: key, baseURL: "https://api.openai.com"}
	case "ollama":
		if model == "" {
			model = "llama3"
		}
		return &openaiAdapter{model: model, apiKey: "ollama", baseURL: "http://localhost:11434"}
	default:
		return nil
	}
}

// --- Anthropic adapter ---

type anthropicAdapter struct {
	model  string
	apiKey string
}

func (a *anthropicAdapter) Provider() string { return "anthropic" }

func (a *anthropicAdapter) Complete(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      a.model,
		"max_tokens": 1024,
		"system":     system,
		"messages":   []map[string]string{{"role": "user", "content": user}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Content) == 0 {
		return "", nil
	}
	return out.Content[0].Text, nil
}

// --- OpenAI-compatible adapter (OpenAI, Ollama, etc.) ---

type openaiAdapter struct {
	model   string
	apiKey  string
	baseURL string
}

func (a *openaiAdapter) Provider() string {
	if strings.Contains(a.baseURL, "localhost") {
		return "ollama"
	}
	return "openai"
}

func (a *openaiAdapter) Complete(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      a.model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("authorization", "Bearer "+a.apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s %d: %s", a.Provider(), resp.StatusCode, raw)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", nil
	}
	return out.Choices[0].Message.Content, nil
}
