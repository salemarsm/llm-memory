package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func openclawConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openclaw", "openclaw.json"), nil
}

func desiredOpenClawGinkoEntry() map[string]any {
	return map[string]any{
		"command": "ginko",
		"args":    []any{"mcp"},
	}
}

// setupOpenClaw writes the ginko MCP server entry into ~/.openclaw/openclaw.json.
func setupOpenClaw(dryRun, noBackup bool) error {
	cfgPath, err := openclawConfigPath()
	if err != nil {
		return fmt.Errorf("openclaw config: %w", err)
	}

	cfg := map[string]any{}
	action := "create"

	if b, err := os.ReadFile(cfgPath); err == nil {
		if len(b) > 0 {
			if err := json.Unmarshal(b, &cfg); err != nil {
				return fmt.Errorf("parse %s: %w", cfgPath, err)
			}
		}
		action = "merge"
	} else if !os.IsNotExist(err) {
		return err
	}

	desired := desiredOpenClawGinkoEntry()

	// Navigate/create mcp.servers
	mcp, _ := cfg["mcp"].(map[string]any)
	if mcp == nil {
		mcp = map[string]any{}
		cfg["mcp"] = mcp
	}
	servers, _ := mcp["servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		mcp["servers"] = servers
	}

	if existing, ok := servers["ginko"]; ok {
		// check if it already matches
		existingJSON, _ := json.Marshal(existing)
		desiredJSON, _ := json.Marshal(desired)
		if string(existingJSON) == string(desiredJSON) {
			fmt.Printf("ginko already configured in %s — no changes.\n", cfgPath)
			return nil
		}
		action = "overwrite"
	}
	servers["ginko"] = desired

	out, _ := json.MarshalIndent(cfg, "", "  ")

	if dryRun {
		fmt.Printf("# target: %s\n# action: %s\n%s\n", cfgPath, action, string(out))
		return nil
	}

	if !noBackup {
		if _, err := os.Stat(cfgPath); err == nil {
			backup := cfgPath + ".backup-" + time.Now().UTC().Format("20060102-150405")
			if err := copyPath(cfgPath, backup); err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}
			fmt.Printf("backed up existing settings to %s\n", backup)
		}
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, append(out, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("ginko setup complete (%s) — %s\n", action, cfgPath)
	return nil
}
