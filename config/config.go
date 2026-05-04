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
		Database: DatabaseConfig{Path: "./memory.db"},
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
