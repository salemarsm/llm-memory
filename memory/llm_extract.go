package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

const extractionSystemPrompt = `You are a memory extraction system for software development projects.
Given a document excerpt, identify facts, decisions, conventions, and preferences worth storing as durable memories for future agent sessions.

Return ONLY a JSON array (no markdown, no explanation):
[{"type":"fact"|"decision"|"preference"|"task","content":"...","confidence":0.65-1.0,"reason":"..."}]

Guidelines:
- Extract 0-5 candidates. Return [] if nothing is worth saving.
- "content" must be a complete, standalone sentence (max 250 chars).
- Only extract non-obvious knowledge not derivable from reading source code directly.
- "fact": stable truth about the system, architecture, or project.
- "decision": architecture or design choice with implied rationale.
- "preference": team convention, style guide, or explicitly stated preference.
- "task": pending work item or known gap.
- confidence: 0.65 for uncertain, 0.85+ for explicit statements, 1.0 for unambiguous facts.
- Never extract trivial facts, todos already tracked in code, or sensitive data.`

type llmCandidate struct {
	Type       string  `json:"type"`
	Content    string  `json:"content"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// ExtractMemoriesFromDocument uses the LLM adapter to extract memory candidates
// from a document's chunks. Falls back to heuristic extraction when no LLM is set.
// Chunks are batched to stay within a reasonable prompt size.
func (s *Store) ExtractMemoriesFromDocument(ctx context.Context, req ChunkSuggestRequest) (ChunkSuggestResponse, error) {
	if s.llmAdapter == nil {
		return s.SuggestMemoriesFromDocument(ctx, req)
	}
	doc, err := s.GetDocument(ctx, req.DocumentID)
	if err != nil {
		return ChunkSuggestResponse{}, err
	}
	chunks, err := s.ListChunks(ctx, doc.ID)
	if err != nil {
		return ChunkSuggestResponse{}, err
	}

	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = doc.Title
	}
	scope := req.Scope
	if scope == "" {
		scope = ScopeProject
	}
	limit := req.Limit
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	minConf := req.MinConfidence
	if minConf <= 0 {
		minConf = 0.65
	}

	candidates := extractWithLLM(ctx, s.llmAdapter, doc, chunks, subject, scope, limit, minConf)

	resp := ChunkSuggestResponse{Document: doc, Candidates: candidates}
	if req.Store {
		for _, c := range candidates {
			m, err := s.UpsertMemory(ctx, c.Memory)
			if err != nil {
				return ChunkSuggestResponse{}, err
			}
			resp.Stored = append(resp.Stored, m)
		}
	}
	return resp, nil
}

func extractWithLLM(ctx context.Context, llm LLMAdapter, doc Document, chunks []Chunk, subject string, scope Scope, limit int, minConf float64) []MemoryCandidate {
	const maxBatchChars = 6000

	// Batch chunks to stay within a reasonable prompt size.
	batches := batchChunks(chunks, maxBatchChars)

	seen := map[string]bool{}
	var out []MemoryCandidate

	for _, batch := range batches {
		if len(out) >= limit {
			break
		}
		userPrompt := fmt.Sprintf("Document: %s\nSubject: %s\n\n%s", doc.Title, subject, batch.text)
		raw, err := llm.Complete(ctx, extractionSystemPrompt, userPrompt)
		if err != nil {
			log.Printf("ginko: LLM extraction failed for doc %s: %v", doc.ID, err)
			continue
		}
		parsed := parseLLMCandidates(raw)
		for _, lc := range parsed {
			if lc.Confidence < minConf {
				continue
			}
			mtype := parseMemoryType(lc.Type)
			content := compactWhitespace(lc.Content)
			if len([]rune(content)) < 12 {
				continue
			}
			key := strings.ToLower(string(mtype) + "|" + subject + "|" + content)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, MemoryCandidate{
				Memory: Memory{
					Type:          mtype,
					Subject:       subject,
					Content:       content,
					Source:        Source{Kind: "chunk", Ref: doc.ID + ":" + batch.firstChunkID},
					Scope:         scope,
					Confidence:    lc.Confidence,
					Tags:          []string{"extracted", "rag", "doc:" + doc.ID},
					EmbeddingRefs: EmbeddingRefs{},
				},
				Reason:               lc.Reason,
				RequiresConfirmation: true,
			})
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

type chunkBatch struct {
	text         string
	firstChunkID string
}

func batchChunks(chunks []Chunk, maxChars int) []chunkBatch {
	var batches []chunkBatch
	var cur strings.Builder
	firstID := ""
	for _, c := range chunks {
		if cur.Len() > 0 && cur.Len()+len(c.Content) > maxChars {
			batches = append(batches, chunkBatch{text: cur.String(), firstChunkID: firstID})
			cur.Reset()
			firstID = ""
		}
		if firstID == "" {
			firstID = c.ID
		}
		if c.HeadingPath != "" {
			cur.WriteString("## " + c.HeadingPath + "\n")
		}
		cur.WriteString(c.Content)
		cur.WriteByte('\n')
	}
	if cur.Len() > 0 {
		batches = append(batches, chunkBatch{text: cur.String(), firstChunkID: firstID})
	}
	return batches
}

func parseLLMCandidates(raw string) []llmCandidate {
	raw = strings.TrimSpace(raw)
	// Strip markdown code fences if the LLM wrapped the JSON.
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) >= 3 {
			raw = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end <= start {
		return nil
	}
	raw = raw[start : end+1]
	var out []llmCandidate
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		log.Printf("ginko: failed to parse LLM extraction output: %v — raw: %.200s", err, raw)
		return nil
	}
	return out
}

func parseMemoryType(s string) MemoryType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "decision":
		return TypeDecision
	case "preference", "convention":
		return TypePreference
	case "task":
		return TypeTask
	case "note":
		return TypeNote
	default:
		return TypeFact
	}
}
