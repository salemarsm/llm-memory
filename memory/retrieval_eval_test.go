package memory

import (
	"context"
	"strings"
	"testing"
)

func TestFTSQuery_KeepsTechnicalShortTokens(t *testing.T) {
	q := ftsQuery("AI ML Go v8 k8s UI and to")
	for _, want := range []string{"ai*", "ml*", "go*", "v8*", "k8s*", "ui*"} {
		if !strings.Contains(q, want) {
			t.Fatalf("expected %q in FTS query %q", want, q)
		}
	}
	if strings.Contains(q, "and*") || strings.Contains(q, "to*") {
		t.Fatalf("stopwords leaked into FTS query %q", q)
	}
}

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

func TestBuildContext_DoesNotRecordBuiltEventWhenEmpty(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	resp, err := s.BuildContext(ctx, ContextRequest{Query: "nothing", Subject: "missing", MaxTokens: 300})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ContextID == "" {
		t.Fatal("expected context_id even for empty context")
	}
	events, err := s.ListEvents(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no context.built event for empty context, got %#v", events)
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

func TestSearchRankedIncludesLexicalScore(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.UpsertMemory(ctx, Memory{Type: TypeDecision, Subject: "ginko", Content: "Endpoint /api/v1/context returns compact memory context.", Source: Source{Kind: "test", Ref: "rank"}, Scope: ScopeProject, Confidence: 0.91})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.SearchRanked(ctx, Query{Text: "/api/v1/context", Subject: "ginko", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected ranked row, got %d", len(rows))
	}
	if rows[0].Ranking.LexicalScore == nil {
		t.Fatalf("expected lexical score: %#v", rows[0].Ranking)
	}
	if rows[0].Ranking.FinalScore <= 0 || rows[0].Ranking.RankReason == "" {
		t.Fatalf("expected useful ranking metadata: %#v", rows[0].Ranking)
	}
}

func TestFTSQuery_CapsAtTwelveTerms(t *testing.T) {
	q := ftsQuery("one two three four five six seven eight nine ten eleven twelve thirteen fourteen")
	terms := strings.Split(q, " OR ")
	if len(terms) != maxFTSQueryTerms {
		t.Fatalf("got %d terms: %q", len(terms), q)
	}
	if strings.Contains(q, "thirteen*") || strings.Contains(q, "fourteen*") {
		t.Fatalf("query exceeded cap: %q", q)
	}
}

func TestFTSQuery_EmptyInput(t *testing.T) {
	if got := ftsQuery(" \n\t "); got != "" {
		t.Fatalf("expected empty query, got %q", got)
	}
}

func TestBuildContext_ExposesRankingsMap(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	m, err := s.UpsertMemory(ctx, Memory{
		Type: TypeDecision, Subject: "ginko", Scope: ScopeProject,
		Content: "Hybrid ranker combines lexical, confidence, recency, provenance.",
		Source:  Source{Kind: "test", Ref: "ranking"}, Confidence: 0.9,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := s.BuildContext(ctx, ContextRequest{Query: "hybrid ranker lexical", Subject: "ginko", MaxTokens: 400})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) == 0 {
		t.Fatal("expected at least one item")
	}
	if resp.Rankings == nil {
		t.Fatal("expected Rankings map to be populated")
	}
	r, ok := resp.Rankings[m.ID]
	if !ok {
		t.Fatalf("expected ranking for memory %s, got keys %v", m.ID, resp.Rankings)
	}
	if r.FinalScore <= 0 {
		t.Fatalf("expected positive final_score, got %v", r)
	}
}

func TestBuildContext_DetectsStructuralConflicts(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Two facts with same subject — structurally conflicting.
	for _, content := range []string{
		"Signal leases must have expires_at.",
		"Signal leases are optional and have no expiry requirement.",
	} {
		if _, err := s.UpsertMemory(ctx, Memory{
			Type: TypeFact, Subject: "signals", Scope: ScopeProject,
			Content: content, Source: Source{Kind: "test", Ref: "conflict"}, Confidence: 0.9,
		}); err != nil {
			t.Fatal(err)
		}
	}

	resp, err := s.BuildContext(ctx, ContextRequest{Query: "signal lease expiry", Subject: "signals", MaxTokens: 600})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Conflicts) == 0 {
		t.Fatal("expected at least one structural conflict")
	}
	if resp.Conflicts[0].Subject != "signals" {
		t.Fatalf("unexpected conflict subject: %q", resp.Conflicts[0].Subject)
	}
	if len(resp.Conflicts[0].IDs) < 2 {
		t.Fatalf("expected at least 2 IDs in conflict, got %v", resp.Conflicts[0].IDs)
	}
}

func TestRunEval_PrecisionAndNDCG(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	rel, err := s.UpsertMemory(ctx, Memory{
		Type: TypeFact, Subject: "eval", Scope: ScopeGlobal,
		Content: "FTS5 BM25 retrieval ranks exact technical terms first.",
		Source:  Source{Kind: "test", Ref: "eval"}, Confidence: 0.95,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpsertMemory(ctx, Memory{
		Type: TypeNote, Subject: "eval", Scope: ScopeGlobal,
		Content: "Unrelated note about something else entirely.",
		Source:  Source{Kind: "test", Ref: "eval"}, Confidence: 0.6,
	})
	if err != nil {
		t.Fatal(err)
	}

	fixtures := []EvalFixture{
		{Label: "exact-match", Query: "BM25 retrieval technical terms", Subject: "eval", RelevantIDs: []string{rel.ID}},
	}
	report, err := s.RunEval(ctx, "test-run", fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Fixtures) != 1 {
		t.Fatalf("expected 1 fixture result, got %d", len(report.Fixtures))
	}
	if report.Fixtures[0].PrecisionAt5 == 0 {
		t.Fatalf("expected non-zero precision@5 for exact-match fixture, got %v", report.Fixtures[0])
	}
}

func TestPrecisionAtK(t *testing.T) {
	retrieved := []string{"a", "b", "c", "d", "e"}
	relevant := []string{"b", "d"}
	if got := precisionAtK(retrieved, relevant, 5); got != 0.4 {
		t.Fatalf("expected 0.4, got %v", got)
	}
	if got := precisionAtK(retrieved, relevant, 2); got != 0.5 {
		t.Fatalf("expected 0.5, got %v", got)
	}
	if got := precisionAtK(nil, relevant, 5); got != 0 {
		t.Fatalf("expected 0 for empty retrieved, got %v", got)
	}
}

func TestNDCGAtK(t *testing.T) {
	retrieved := []string{"a", "b", "c"}
	relevant := []string{"a", "b"}
	// Perfect order: nDCG should be 1.0
	if got := ndcgAtK(retrieved, relevant, 3); got < 0.99 {
		t.Fatalf("expected ~1.0 for perfect order, got %v", got)
	}
	// No overlap: nDCG should be 0
	if got := ndcgAtK([]string{"x", "y"}, relevant, 2); got != 0 {
		t.Fatalf("expected 0 for no overlap, got %v", got)
	}
}
