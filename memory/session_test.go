package memory

import (
	"context"
	"strings"
	"testing"
)

func TestSessionLifecycleAndContext(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	started, err := s.StartSession(ctx, "My Project")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.StartSession(ctx, "my-project")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != started.ID {
		t.Fatalf("expected active session reuse")
	}
	ended, err := s.EndActiveSession(ctx, "my-project", "Decided to keep SQLite canonical.")
	if err != nil {
		t.Fatal(err)
	}
	if ended.EndedAt == nil || ended.Summary == "" {
		t.Fatalf("bad ended session: %#v", ended)
	}

	resp, err := s.BuildContext(ctx, ContextRequest{Project: "my-project", Query: "start", MaxTokens: 400})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Context, "Session context:") || !strings.Contains(resp.Context, "SQLite canonical") {
		t.Fatalf("missing session context: %q", resp.Context)
	}
	if _, err := s.ActiveSession(ctx, "my-project"); err != nil {
		t.Fatalf("expected auto active session: %v", err)
	}
}
