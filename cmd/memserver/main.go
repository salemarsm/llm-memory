package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/salemarsm/llm-memory/config"
	"github.com/salemarsm/llm-memory/internal/version"
	"github.com/salemarsm/llm-memory/memory"
	"github.com/salemarsm/llm-memory/server"
)

func main() {
	_ = config.MaybeMigrateLegacyDataDir()
	configPath := flag.String("config", "", "path to JSON config")
	writeConfig := flag.String("write-config", "", "write default JSON config and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	logFormat := flag.String("log-format", "text", "log format: text or json")
	flag.Parse()

	if *showVersion {
		fmt.Println("memserver", version.String())
		return
	}

	logger := newLogger(*logFormat)
	slog.SetDefault(logger)

	if *writeConfig != "" {
		if err := config.WriteDefault(*writeConfig); err != nil {
			slog.Error("write-config failed", "path", *writeConfig, "error", err)
			os.Exit(1)
		}
		slog.Info("wrote config", "path", *writeConfig)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", "error", err)
		os.Exit(1)
	}

	store, err := memory.Open(cfg.Database.Path)
	if err != nil {
		slog.Error("open database failed", "path", cfg.Database.Path, "error", err)
		os.Exit(1)
	}
	defer store.Close()

	_, hasAuth := cfg.Server.BearerToken()
	slog.Info("llm-memory starting",
		"version", version.Version,
		"addr", cfg.Server.Addr,
		"database", cfg.Database.Path,
		"llm", cfg.LLM.Provider+"/"+cfg.LLM.Model,
		"embedding", cfg.Embedding.Provider+"/"+cfg.Embedding.Model,
		"auth", hasAuth,
	)

	srv := server.New(store, cfg)
	httpServer := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func newLogger(format string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if format == "json" {
		return slog.New(slog.NewJSONHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, opts))
}
