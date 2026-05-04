package memory

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitMarkdownChunks_EmptyInput(t *testing.T) {
	if got := splitMarkdownChunks("", 100); got != nil {
		t.Fatalf("expected nil for empty input, got %#v", got)
	}
	if got := splitMarkdownChunks(" \n\t \n", 100); got != nil {
		t.Fatalf("expected nil for whitespace-only input, got %#v", got)
	}
}

func TestSplitMarkdownChunks_SingleParagraph(t *testing.T) {
	got := splitMarkdownChunks("  hello world  ", 100)
	if len(got) != 1 || got[0] != "hello world" {
		t.Fatalf("unexpected chunks: %#v", got)
	}
}

func TestSplitMarkdownChunks_MultiParagraphPacking(t *testing.T) {
	got := splitMarkdownChunks("alpha\n\nbeta\n\ngamma", 100)
	if len(got) != 1 || got[0] != "alpha\n\nbeta\n\ngamma" {
		t.Fatalf("unexpected packed chunks: %#v", got)
	}
}

func TestSplitMarkdownChunks_LongParagraphRuneSplit(t *testing.T) {
	got := splitMarkdownChunks("abcdef", 2)
	want := []string{"ab", "cd", "ef"}
	if len(got) != len(want) {
		t.Fatalf("got %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("chunk %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitMarkdownChunks_UnicodeBoundaryIntegrity(t *testing.T) {
	got := splitMarkdownChunks("áéíó", 2)
	if len(got) != 2 || got[0] != "áé" || got[1] != "íó" {
		t.Fatalf("unexpected unicode chunks: %#v", got)
	}
	for _, chunk := range got {
		if !utf8.ValidString(chunk) {
			t.Fatalf("invalid utf8 chunk: %q", chunk)
		}
	}
}

func TestSplitMarkdownChunks_WhitespaceOnlyParagraphs(t *testing.T) {
	got := splitMarkdownChunks("alpha\n\n   \n\nbeta", 100)
	if len(got) != 1 || strings.Contains(got[0], "\n\n\n") || got[0] != "alpha\n\nbeta" {
		t.Fatalf("unexpected chunks: %#v", got)
	}
}
