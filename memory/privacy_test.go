package memory

import (
	"context"
	"testing"
)

func TestStripPrivateTags(t *testing.T) {
	got := StripPrivateTags("keep <private>secret token</private> also keep")
	if got != "keep also keep" {
		t.Fatalf("got %q", got)
	}
}

func TestStripPrivateTagsMultilineCaseInsensitive(t *testing.T) {
	got := StripPrivateTags("alpha <PRIVATE>line1\nline2</PRIVATE> omega")
	if got != "alpha omega" {
		t.Fatalf("got %q", got)
	}
}

func TestUpsertMemoryStripsPrivateTags(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m, err := s.UpsertMemory(testContext(), Memory{Type: TypeFact, Subject: "project", Content: "keep <private>secret</private> public", Source: Source{Kind: "test", Ref: "privacy"}, Scope: ScopeProject, Confidence: 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if m.Content != "keep public" {
		t.Fatalf("private content leaked: %q", m.Content)
	}
	got, err := s.GetMemory(testContext(), m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "keep public" {
		t.Fatalf("stored private content leaked: %q", got.Content)
	}
}

func TestUpsertMemoryRejectsAllPrivateContent(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_, err = s.UpsertMemory(testContext(), Memory{Type: TypeFact, Subject: "project", Content: "<private>secret</private>", Source: Source{Kind: "test", Ref: "privacy"}, Scope: ScopeProject, Confidence: 0.9})
	if err == nil {
		t.Fatal("expected empty content after private stripping to be rejected")
	}
}

func testContext() context.Context { return context.Background() }
