package memory

import (
	"context"
	"strings"
	"testing"
)

func TestSuggestMemories(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	resp, err := s.SuggestMemories(context.Background(), SuggestRequest{
		Subject:    "botmaster",
		Scope:      ScopeGlobal,
		UserPrompt: "Prefiro respostas diretas e técnicas. Decidimos usar SQLite como fonte canônica.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Candidates) < 2 {
		t.Fatalf("expected candidates, got %#v", resp)
	}
	joined := ""
	for _, c := range resp.Candidates {
		joined += string(c.Memory.Type) + ":" + c.Memory.Content + "\n"
	}
	if !strings.Contains(joined, "preference") || !strings.Contains(joined, "decision") {
		t.Fatalf("missing expected candidate types: %s", joined)
	}
}
