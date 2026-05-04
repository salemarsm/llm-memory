package memory

import (
	"context"
	"strings"
	"testing"
)

func TestRetrieval_BM25PreferenceWinsOverRecentWeakMatch(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	relevant, err := s.UpsertMemory(ctx, Memory{Type: TypePreference, Subject: "botmaster", Content: "Prefers direct technical answers with minimal fluff.", Source: Source{Kind: "test", Ref: "pref"}, Scope: ScopeGlobal, Confidence: 0.95})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpsertMemory(ctx, Memory{Type: TypeTask, Subject: "botmaster", Content: "Recent task: answer a release checklist comment tomorrow.", Source: Source{Kind: "test", Ref: "task"}, Scope: ScopeProject, Confidence: 0.8})
	if err != nil {
		t.Fatal(err)
	}

	items, err := s.Search(ctx, Query{Text: "direct technical answer", Subject: "botmaster", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected retrieval candidates")
	}
	if items[0].ID != relevant.ID {
		t.Fatalf("expected BM25-ranked preference first, got %s: %q", items[0].Type, items[0].Content)
	}
}

func TestBuildContext_FallsBackToSubjectMemoriesWhenFTSMisses(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.UpsertMemory(ctx, Memory{Type: TypePreference, Subject: "botmaster", Content: "Prefere respostas diretas e técnicas.", Source: Source{Kind: "test", Ref: "pref"}, Scope: ScopeGlobal, Confidence: 0.95})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpsertMemory(ctx, Memory{Type: TypeNote, Subject: "botmaster", Content: "Low confidence draft that should not auto-inject.", Source: Source{Kind: "test", Ref: "draft"}, Scope: ScopeGlobal, Confidence: 0.4})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := s.BuildContext(ctx, ContextRequest{Query: "como devo agir?", Subject: "botmaster", Scopes: []Scope{ScopeGlobal}, MaxTokens: 300})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ContextID == "" {
		t.Fatal("expected context_id")
	}
	if !strings.Contains(resp.Context, "respostas diretas") {
		t.Fatalf("expected subject fallback memory, got %q", resp.Context)
	}
	if strings.Contains(resp.Context, "Low confidence") {
		t.Fatalf("low-confidence memory leaked into context: %q", resp.Context)
	}
}

func TestContextFeedbackRecordsEvent(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.RecordContextFeedback(ctx, ContextFeedback{ContextID: "ctx_test", Useful: true, MemoryIDsUsed: []string{"mem_1"}, Source: Source{Kind: "test", Ref: "feedback"}}); err != nil {
		t.Fatal(err)
	}
	events, err := s.ListEvents(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != "context.feedback" || !strings.Contains(events[0].Payload, "ctx_test") {
		t.Fatalf("expected context.feedback event, got %#v", events)
	}
}
