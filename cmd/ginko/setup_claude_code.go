package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

func doSetupClaudeCode(args []string) {
	args = normalizeClaudeCodeSetupArgs(args)
	fs := flag.NewFlagSet("setup claude-code", flag.ExitOnError)
	scope := fs.String("scope", "user", "user or project")
	dryRun := fs.Bool("dry-run", false, "print planned changes; do not write")
	noBackup := fs.Bool("no-backup", false, "do not back up existing settings file")
	plugin := fs.Bool("plugin", false, "print plugin marketplace instructions instead of writing JSON")
	noAutostart := fs.Bool("no-autostart", false, "skip autostart prompt for ginko serve")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if *plugin {
		printPluginInstructions()
		return
	}

	target, err := resolveSettingsPath(*scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ginko setup: %v\n", err)
		os.Exit(1)
	}

	updated, action, err := planUpdate(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ginko setup: %v\n", err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(updated, "", "  ")

	if *dryRun {
		fmt.Printf("# target: %s\n# action: %s\n%s\n", target, action, string(out))
		return
	}

	switch action {
	case "noop":
		fmt.Printf("ginko already configured in %s — no changes.\n", target)
		return
	case "create", "merge", "overwrite":
	}

	if !*noBackup {
		if _, err := os.Stat(target); err == nil {
			backup := target + ".backup-" + time.Now().UTC().Format("20060102-150405")
			if err := copyPath(target, backup); err != nil {
				fmt.Fprintf(os.Stderr, "ginko setup: backup failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("backed up existing settings to %s\n", backup)
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "ginko setup: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(target, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "ginko setup: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ginko setup complete (%s) — %s\n", action, target)
	fmt.Println()
	fmt.Println("Restart Claude Code to load the MCP server.")

	if !*noAutostart {
		askAndInstallAutostart()
	}
}

func normalizeClaudeCodeSetupArgs(args []string) []string {
	out := make([]string, 0, len(args)+1)
	for _, arg := range args {
		switch arg {
		case "--local", "-local", "--project", "-project":
			out = append(out, "-scope", "project")
		case "--user", "-user":
			out = append(out, "-scope", "user")
		default:
			out = append(out, arg)
		}
	}
	return out
}

func resolveSettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home: %w", err)
		}
		return filepath.Join(home, ".claude", "settings.json"), nil
	case "project":
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(cwd, ".claude", "settings.json"), nil
	default:
		return "", fmt.Errorf("invalid scope %q (must be user or project)", scope)
	}
}

func desiredGinkoEntry() map[string]any {
	return map[string]any{
		"command": "ginko",
		"args":    []any{"mcp"},
	}
}

func planUpdate(path string) (map[string]any, string, error) {
	settings := map[string]any{}
	action := "create"

	if b, err := os.ReadFile(path); err == nil {
		if len(b) > 0 {
			if err := json.Unmarshal(b, &settings); err != nil {
				return nil, "", fmt.Errorf("parse %s: %w", path, err)
			}
		}
		action = "merge"
	} else if !os.IsNotExist(err) {
		return nil, "", err
	}

	mcp, _ := settings["mcpServers"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
		settings["mcpServers"] = mcp
	}

	desired := desiredGinkoEntry()
	if existing, ok := mcp["ginko"]; ok {
		if reflect.DeepEqual(existing, desired) {
			return settings, "noop", nil
		}
		action = "overwrite"
	}
	mcp["ginko"] = desired
	return settings, action, nil
}

func copyPath(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, b, 0o644)
}

func printPluginInstructions() {
	fmt.Println(`Install ginko as a Claude Code plugin (recommended):

  # Inside a Claude Code session:
  /plugin marketplace add salemarsm/llm-memory
  /plugin install ginko

The plugin includes:
  - MCP server (ginko mcp)
  - Memory Protocol skill (teaches the agent when/how to remember)
  - SessionStart hook (recovers context from previous sessions)
  - PreCompact hook (auto-checkpoints memory before context compaction)
  - Slash commands (/ginko:save, /ginko:recall)

Alternative: configure the MCP server only (no plugin):
  ginko setup claude-code --scope user

This adds 'ginko' to ~/.claude/settings.json mcpServers.`)
}
