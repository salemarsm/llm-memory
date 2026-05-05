package config

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
)

type Config struct {
	Server    ServerConfig    `json:"server"`
	Database  DatabaseConfig  `json:"database"`
	LLM       LLMConfig       `json:"llm"`
	Embedding EmbeddingConfig `json:"embedding"`
	Agents    AgentsConfig    `json:"agents,omitempty"`
}

// AgentsConfig holds per-agent defaults. Each entry is keyed by agent name
// (e.g. "claude-code", "codex") and overrides global defaults for that agent.
type AgentsConfig map[string]AgentProfile

type AgentProfile struct {
	// MaxContextTokens caps the token budget for memory_context calls from this agent.
	// 0 means use the request value or the global default (800).
	MaxContextTokens int `json:"max_context_tokens,omitempty"`
	// DefaultSubject is used when the agent does not supply a subject.
	DefaultSubject string `json:"default_subject,omitempty"`
	// DefaultScope filters memories to this scope by default.
	DefaultScope string `json:"default_scope,omitempty"`
	// WritePolicy controls whether saves are automatic or require confirmation per scope.
	WritePolicy WritePolicy `json:"write_policy,omitempty"`
}

// WritePolicy configures per-scope save behaviour for an agent.
type WritePolicy struct {
	// AutoSave lists scopes where memory_remember saves immediately (default for all scopes).
	AutoSave []string `json:"auto_save,omitempty"`
	// RequireApproval lists scopes where memory_remember saves as status=pending
	// and waits for human approval (e.g. ["global"] to gate all global memories).
	RequireApproval []string `json:"require_approval,omitempty"`
}

// ScopeRequiresApproval returns true if the given scope requires human approval
// before a memory is made active.
func (p WritePolicy) ScopeRequiresApproval(scope string) bool {
	for _, s := range p.RequireApproval {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

type ServerConfig struct {
	Addr         string `json:"addr"`
	AuthToken    string `json:"auth_token,omitempty"`
	AuthTokenEnv string `json:"auth_token_env,omitempty"`
}

type DatabaseConfig struct {
	Path string `json:"path"`
}

type LLMConfig struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env"`
}

type EmbeddingConfig struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Index     string `json:"index"`
	APIKeyEnv string `json:"api_key_env"`
}

func Default() Config {
	return Config{
		Server:   ServerConfig{Addr: "127.0.0.1:8787"},
		Database: DatabaseConfig{Path: DefaultDBPath()},
		LLM: LLMConfig{
			Provider:  "none",
			Model:     "",
			APIKeyEnv: "",
		},
		Embedding: EmbeddingConfig{
			Provider:  "none",
			Model:     "",
			Index:     "sqlite-fts",
			APIKeyEnv: "",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Server.Addr == "" {
		return errors.New("server.addr is required")
	}
	if c.Database.Path == "" {
		return errors.New("database.path is required")
	}
	if !IsLoopbackAddr(c.Server.Addr) {
		if _, ok := c.Server.BearerToken(); !ok {
			return errors.New("server auth token is required when binding outside loopback; set server.auth_token_env or server.auth_token")
		}
	}
	return nil
}

func (s ServerConfig) BearerToken() (string, bool) {
	if v := strings.TrimSpace(s.AuthToken); v != "" {
		return v, true
	}
	if env := strings.TrimSpace(s.AuthTokenEnv); env != "" {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v, true
		}
	}
	return "", false
}

func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func WriteDefault(path string) error {
	cfg := Default()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
