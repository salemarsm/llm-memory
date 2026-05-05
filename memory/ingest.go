package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type IngestRequest struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type IngestResponse struct {
	Run       IngestionRun `json:"run"`
	Documents []Document   `json:"documents"`
	Chunks    []Chunk      `json:"chunks"`
	Skipped   []string     `json:"skipped"`
}

type IngestionRun struct {
	ID               string     `json:"id"`
	SourcePath       string     `json:"source_path"`
	Recursive        bool       `json:"recursive"`
	Parser           string     `json:"parser"`
	Status           string     `json:"status"`
	FilesSeen        int        `json:"files_seen"`
	DocumentsCreated int        `json:"documents_created"`
	ChunksCreated    int        `json:"chunks_created"`
	Error            string     `json:"error,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

var textIngestExts = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".html": true, ".htm": true,
	".json": true, ".jsonl": true, ".csv": true, ".tsv": true, ".tex": true,
}

func (s *Store) IngestPath(ctx context.Context, req IngestRequest) (IngestResponse, error) {
	path := strings.TrimSpace(req.Path)
	if path == "" {
		return IngestResponse{}, errors.New("ingest path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return IngestResponse{}, err
	}
	run := IngestionRun{ID: newID("ing"), SourcePath: abs, Recursive: req.Recursive, Parser: ingestParserLabel(), Status: "running", CreatedAt: time.Now().UTC()}
	if err := s.UpsertIngestionRun(ctx, run); err != nil {
		return IngestResponse{}, err
	}
	resp := IngestResponse{Run: run}
	finish := func(status string, runErr error) (IngestResponse, error) {
		now := time.Now().UTC()
		run.Status = status
		run.FilesSeen = len(resp.Documents) + len(resp.Skipped)
		run.DocumentsCreated = len(resp.Documents)
		run.ChunksCreated = len(resp.Chunks)
		run.CompletedAt = &now
		if runErr != nil {
			run.Error = runErr.Error()
		}
		_ = s.UpsertIngestionRun(ctx, run)
		resp.Run = run
		payload, _ := json.Marshal(map[string]any{"run_id": run.ID, "path": run.SourcePath, "recursive": run.Recursive, "status": run.Status, "documents": run.DocumentsCreated, "chunks": run.ChunksCreated, "document_ids": documentIDs(resp.Documents), "chunk_ids": chunkIDs(resp.Chunks), "skipped": resp.Skipped})
		_ = s.AppendEvent(ctx, Event{Kind: "document.ingested", Payload: string(payload), Source: Source{Kind: "ingest", Ref: run.ID}})
		return resp, runErr
	}

	files, err := ingestFileList(abs, req.Recursive)
	if err != nil {
		return finish("error", err)
	}
	for _, file := range files {
		var doc Document
		var chunks []Chunk
		var err error
		switch {
		case isTextIngestFile(file):
			doc, chunks, err = s.ingestTextFile(ctx, run.ID, file)
		case isDoclingIngestFile(file):
			doc, chunks, err = s.ingestDoclingFile(ctx, run.ID, file)
		default:
			resp.Skipped = append(resp.Skipped, file+" (unsupported extension)")
			continue
		}
		if err != nil {
			resp.Skipped = append(resp.Skipped, file+" ("+err.Error()+")")
			continue
		}
		resp.Documents = append(resp.Documents, doc)
		resp.Chunks = append(resp.Chunks, chunks...)
	}
	status := "ok"
	if len(resp.Skipped) > 0 {
		status = "partial"
	}
	return finish(status, nil)
}

func ingestFileList(path string, recursive bool) ([]string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return []string{path}, nil
	}
	var files []string
	if recursive {
		err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") && p != path {
					return filepath.SkipDir
				}
				return nil
			}
			files = append(files, p)
			return nil
		})
	} else {
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return nil, readErr
		}
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(path, e.Name()))
			}
		}
	}
	sort.Strings(files)
	return files, err
}

func isTextIngestFile(path string) bool { return textIngestExts[strings.ToLower(filepath.Ext(path))] }

var doclingIngestExts = map[string]bool{
	".pdf": true, ".docx": true, ".pptx": true, ".xlsx": true,
	".png": true, ".jpg": true, ".jpeg": true, ".tif": true, ".tiff": true,
}

func isDoclingIngestFile(path string) bool {
	return doclingIngestExts[strings.ToLower(filepath.Ext(path))]
}

// IngestFileStatusResult describes whether a file needs re-ingestion.
type IngestFileStatusResult struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // "new", "unchanged", "changed", "unsupported"
	DocumentID string `json:"document_id,omitempty"`
}

// IngestFileStatus checks the current hash of a file against the document store.
func (s *Store) IngestFileStatus(ctx context.Context, path string) (IngestFileStatusResult, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return IngestFileStatusResult{Path: path, Status: "error"}, err
	}
	if !isTextIngestFile(abs) && !isDoclingIngestFile(abs) {
		return IngestFileStatusResult{Path: abs, Status: "unsupported"}, nil
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return IngestFileStatusResult{Path: abs, Status: "error"}, err
	}
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	docID := "doc_" + hash[:32]
	existing, err := s.GetDocument(ctx, docID)
	if err != nil {
		// Check by path to distinguish "changed" from "new"
		var byPath Document
		rows, qErr := s.db.QueryContext(ctx, `SELECT id, sha256 FROM documents WHERE path = ? LIMIT 1`, abs)
		if qErr == nil {
			defer rows.Close()
			if rows.Next() {
				_ = rows.Scan(&byPath.ID, &byPath.SHA256)
			}
		}
		if byPath.ID != "" {
			return IngestFileStatusResult{Path: abs, Status: "changed", DocumentID: byPath.ID}, nil
		}
		return IngestFileStatusResult{Path: abs, Status: "new"}, nil
	}
	_ = existing
	return IngestFileStatusResult{Path: abs, Status: "unchanged", DocumentID: docID}, nil
}

func ingestParserLabel() string {
	if p, err := exec.LookPath("docling"); err == nil {
		version := strings.TrimSpace(string(mustCommandOutput("docling", "--version")))
		if version != "" {
			return "native-text+docling-cli:" + version
		}
		return "native-text+docling-cli:" + p
	}
	return "native-text"
}

func mustCommandOutput(name string, args ...string) []byte {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return nil
	}
	return out
}

func (s *Store) ingestTextFile(ctx context.Context, runID, path string) (Document, []Chunk, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Document{}, nil, err
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return Document{}, nil, errors.New("empty document")
	}
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	docID := "doc_" + hash[:32]

	// Dedupe: if the document already exists with this hash, return existing chunks.
	if existing, err := s.GetDocument(ctx, docID); err == nil && existing.SHA256 == hash {
		chunks, _ := s.ListChunks(ctx, docID)
		return existing, chunks, nil
	}

	doc := Document{ID: docID, IngestionRunID: runID, Path: path, Title: filepath.Base(path), SourceKind: "file", SourceRef: path, SHA256: hash}
	doc, err = s.UpsertDocument(ctx, doc)
	if err != nil {
		return Document{}, nil, err
	}
	if err := s.ReplaceDocumentChunks(ctx, doc.ID, splitMarkdownChunks(text, 1800)); err != nil {
		return Document{}, nil, err
	}
	chunks, err := s.ListChunks(ctx, doc.ID)
	if err != nil {
		return Document{}, nil, err
	}
	return doc, chunks, nil
}

func (s *Store) ingestDoclingFile(ctx context.Context, runID, path string) (Document, []Chunk, error) {
	if _, err := exec.LookPath("docling"); err != nil {
		return Document{}, nil, errors.New("docling CLI not found in PATH")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Document{}, nil, err
	}
	sum := sha256.Sum256(b)
	hash := hex.EncodeToString(sum[:])
	docID := "doc_" + hash[:32]

	// Dedupe: if the document already exists with this hash, skip Docling conversion.
	if existing, err := s.GetDocument(ctx, docID); err == nil && existing.SHA256 == hash {
		chunks, _ := s.ListChunks(ctx, docID)
		return existing, chunks, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	tmp, err := os.MkdirTemp("", "llm-memory-docling-*")
	if err != nil {
		return Document{}, nil, err
	}
	defer os.RemoveAll(tmp)
	cmd := exec.CommandContext(ctx, "docling", "--to", "md", "--output", tmp, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return Document{}, nil, fmt.Errorf("docling conversion failed: %s", msg)
	}
	md, err := readDoclingMarkdown(tmp)
	if err != nil {
		return Document{}, nil, err
	}
	text := strings.TrimSpace(md)
	if text == "" {
		return Document{}, nil, errors.New("docling produced empty markdown")
	}
	doc := Document{ID: docID, IngestionRunID: runID, Path: path, Title: filepath.Base(path), SourceKind: "file:docling", SourceRef: path, SHA256: hash}
	doc, err = s.UpsertDocument(ctx, doc)
	if err != nil {
		return Document{}, nil, err
	}
	if err := s.ReplaceDocumentChunks(ctx, doc.ID, splitMarkdownChunks(text, 1800)); err != nil {
		return Document{}, nil, err
	}
	chunks, err := s.ListChunks(ctx, doc.ID)
	if err != nil {
		return Document{}, nil, err
	}
	return doc, chunks, nil
}

func readDoclingMarkdown(dir string) (string, error) {
	var candidates []string
	if err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".md") {
			candidates = append(candidates, p)
		}
		return nil
	}); err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", errors.New("docling produced no markdown output")
	}
	sort.Strings(candidates)
	b, err := os.ReadFile(candidates[0])
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func splitMarkdownChunks(text string, maxRunes int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n\n")
	out := make([]string, 0, len(parts))
	var cur strings.Builder
	flush := func() {
		chunk := strings.TrimSpace(cur.String())
		if chunk != "" {
			out = append(out, chunk)
		}
		cur.Reset()
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if cur.Len() > 0 && cur.Len()+len(part)+2 > maxRunes {
			flush()
		}
		runes := []rune(part)
		if len(runes) > maxRunes {
			flush()
			for len(runes) > 0 {
				n := maxRunes
				if len(runes) < n {
					n = len(runes)
				}
				chunk := strings.TrimSpace(string(runes[:n]))
				if chunk != "" {
					out = append(out, chunk)
				}
				runes = runes[n:]
			}
			continue
		}
		if cur.Len() > 0 {
			cur.WriteString("\n\n")
		}
		cur.WriteString(part)
	}
	flush()
	if len(out) == 0 {
		return []string{text}
	}
	return out
}

func documentIDs(docs []Document) []string {
	out := make([]string, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.ID)
	}
	return out
}

func chunkIDs(chunks []Chunk) []string {
	out := make([]string, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, c.ID)
	}
	return out
}
