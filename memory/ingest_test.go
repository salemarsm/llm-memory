package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestIngestPathRecursive(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "root.md"), []byte("# Root\n\nMemory is conclusion."), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "child.txt"), []byte("RAG is evidence."), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, err := s.IngestPath(ctx, IngestRequest{Path: dir, Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Run.Status != "ok" || len(resp.Documents) != 2 || len(resp.Chunks) != 2 {
		t.Fatalf("unexpected ingest response: status=%s docs=%d chunks=%d skipped=%d", resp.Run.Status, len(resp.Documents), len(resp.Chunks), len(resp.Skipped))
	}
	docs, err := s.ListDocuments(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}
}

func TestIngestDoclingFileWithCLI(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(bin, "docling")
	if err := os.WriteFile(fake, []byte(`#!/bin/sh
if [ "$1" = "--version" ]; then echo "docling 9.9.9"; exit 0; fi
out=""
last=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then shift; out="$1"; shift; continue; fi
  last="$1"; shift
done
mkdir -p "$out"
printf '# Converted\n\nDocling extracted PDF content from %s.\n' "$last" > "$out/converted.md"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	pdf := filepath.Join(dir, "sample.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp, err := s.IngestPath(ctx, IngestRequest{Path: pdf})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Run.Status != "ok" || len(resp.Documents) != 1 || len(resp.Chunks) != 1 {
		t.Fatalf("unexpected docling ingest response: status=%s docs=%d chunks=%d skipped=%d", resp.Run.Status, len(resp.Documents), len(resp.Chunks), len(resp.Skipped))
	}
	if resp.Run.Parser != "native-text+docling-cli:docling 9.9.9" {
		t.Fatalf("unexpected parser label: %s", resp.Run.Parser)
	}
	if resp.Documents[0].SourceKind != "file:docling" {
		t.Fatalf("expected file:docling source kind, got %s", resp.Documents[0].SourceKind)
	}
}

func TestSearchChunks(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.md")
	if err := os.WriteFile(path, []byte("# Evidence\n\nRAG keeps evidence separate from canonical memory."), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.IngestPath(ctx, IngestRequest{Path: path}); err != nil {
		t.Fatal(err)
	}
	results, err := s.SearchChunks(ctx, ChunkSearchRequest{Text: "canonical evidence", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Document.Title != "evidence.md" {
		t.Fatalf("unexpected chunk results: %#v", results)
	}
}
