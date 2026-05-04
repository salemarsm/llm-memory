package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectConfigOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".llm-memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".llm-memory", "config.json"), []byte(`{"project":"My Cool_Project"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectProject(filepath.Join(dir, "sub")); got != "my-cool-project" {
		t.Fatalf("got %q", got)
	}
}

func TestDetectProjectGitRemote(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[remote \"origin\"]\n\turl = git@github.com:salemarsm/llm-memory.git\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectProject(dir); got != "salemarsm-llm-memory" {
		t.Fatalf("got %q", got)
	}
}
