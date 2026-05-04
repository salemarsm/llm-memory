package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPlanUpdate_Create(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	out, action, err := planUpdate(path)
	if err != nil {
		t.Fatal(err)
	}
	if action != "create" {
		t.Fatalf("action=%s, want create", action)
	}
	mcp := out["mcpServers"].(map[string]any)
	if _, ok := mcp["ginko"]; !ok {
		t.Fatal("ginko entry missing")
	}
}

func TestPlanUpdate_Merge(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	existing := map[string]any{
		"mcpServers": map[string]any{
			"github": map[string]any{"command": "github-mcp-server"},
		},
		"otherSetting": "preserved",
	}
	b, _ := json.Marshal(existing)
	_ = os.WriteFile(path, b, 0o644)

	out, action, err := planUpdate(path)
	if err != nil {
		t.Fatal(err)
	}
	if action != "merge" {
		t.Fatalf("action=%s, want merge", action)
	}
	if out["otherSetting"] != "preserved" {
		t.Fatal("otherSetting was lost")
	}
	mcp := out["mcpServers"].(map[string]any)
	if _, ok := mcp["github"]; !ok {
		t.Fatal("github entry was lost")
	}
	if _, ok := mcp["ginko"]; !ok {
		t.Fatal("ginko entry missing")
	}
}

func TestNormalizeClaudeCodeSetupArgs_LocalAlias(t *testing.T) {
	got := normalizeClaudeCodeSetupArgs([]string{"--dry-run", "--local"})
	want := []string{"--dry-run", "-scope", "project"}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

func TestPlanUpdate_Noop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	existing := map[string]any{
		"mcpServers": map[string]any{
			"ginko": map[string]any{"command": "ginko", "args": []any{"mcp"}},
		},
	}
	b, _ := json.Marshal(existing)
	_ = os.WriteFile(path, b, 0o644)

	_, action, err := planUpdate(path)
	if err != nil {
		t.Fatal(err)
	}
	if action != "noop" {
		t.Fatalf("action=%s, want noop", action)
	}
}
