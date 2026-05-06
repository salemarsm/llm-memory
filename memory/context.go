package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

const defaultContextTokenBudget = 1200

type ContextRequest struct {
	Query       string       `json:"query"`
	Project     string       `json:"project,omitempty"`
	Subject     string       `json:"subject"`
	Types       []MemoryType `json:"types"`
	Scopes      []Scope      `json:"scopes"`
	Tags        []string     `json:"tags"`
	MaxTokens   int          `json:"max_tokens"`
	MaxMemories int          `json:"max_memories"`
}

// ChunkCitation links a memory in context to its source chunk and document.
type ChunkCitation struct {
	MemoryID   string `json:"memory_id"`
	DocumentID string `json:"document_id"`
	ChunkID    string `json:"chunk_id"`
	Path       string `json:"path,omitempty"`
}

// ContextConflict flags two or more active memories in the same context result
// that share the same (subject, type) — a structural signal that supersession
// may be incomplete. Content-level contradiction detection requires an LLM pass.
type ContextConflict struct {
	Subject string     `json:"subject"`
	Type    MemoryType `json:"type"`
	IDs     []string   `json:"ids"`
}

type ContextResponse struct {
	ContextID       string                     `json:"context_id"`
	Context         string                     `json:"context"`
	Items           []Memory                   `json:"items"`
	Rankings        map[string]RankingMetadata `json:"rankings,omitempty"`
	Conflicts       []ContextConflict          `json:"conflicts,omitempty"`
	Citations       []ChunkCitation            `json:"citations,omitempty"`
	EstimatedTokens int                        `json:"estimated_tokens"`
	BudgetTokens    int                        `json:"budget_tokens"`
	Truncated       bool                       `json:"truncated"`
}

func (s *Store) BuildContext(ctx context.Context, req ContextRequest) (ContextResponse, error) {
	project := ""
	if strings.TrimSpace(req.Project) != "" {
		project = NormalizeProject(req.Project)
	}
	if project != "" && req.Subject == "" {
		req.Subject = project
	}
	var sessionBlock string
	if project != "" {
		if _, err := s.EnsureActiveSession(ctx, project); err != nil {
			log.Printf("ginko: ensure active session failed: %v", err)
		}
		sessionBlock = s.contextSessionBlock(ctx, project)
	}
	budget := req.MaxTokens
	if budget <= 0 {
		budget = defaultContextTokenBudget
	}
	if budget > 8000 {
		budget = 8000
	}
	limit := req.MaxMemories
	if limit <= 0 {
		limit = 12
	}
	if limit > 50 {
		limit = 50
	}

	ranked, err := s.SearchRanked(ctx, Query{
		Text:    req.Query,
		Types:   req.Types,
		Scopes:  req.Scopes,
		Subject: req.Subject,
		Tags:    req.Tags,
		Limit:   limit,
	})
	if err != nil {
		return ContextResponse{}, err
	}
	if len(ranked) == 0 && strings.TrimSpace(req.Query) != "" && req.Subject != "" {
		// FTS is candidate generation, not truth. Conversational prompts can miss
		// durable subject memories entirely, so fall back to high-confidence active
		// subject memories and let final_score ordering keep context compact.
		ranked, err = s.SearchRanked(ctx, Query{Types: req.Types, Scopes: req.Scopes, Subject: req.Subject, Tags: req.Tags, Limit: limit})
		if err != nil {
			return ContextResponse{}, err
		}
	}

	// Re-sort by hybrid final_score DESC. SQL orders by BM25 alone; final_score
	// additionally weighs confidence, provenance, and recency.
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Ranking.FinalScore > ranked[j].Ranking.FinalScore
	})

	var b strings.Builder
	used := 0
	if sessionBlock != "" {
		cost := EstimateTokens(sessionBlock)
		if cost <= budget {
			b.WriteString(sessionBlock)
			b.WriteByte('\n')
			used += cost
		}
	}
	selected := make([]RankedMemory, 0, len(ranked))
	truncated := false
	for _, rm := range ranked {
		if rm.Memory.Confidence < 0.5 {
			continue
		}
		line := formatContextMemory(rm.Memory)
		cost := EstimateTokens(line)
		if used+cost > budget {
			truncated = true
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
		used += cost
		selected = append(selected, rm)
	}

	contextID := newID("ctx")
	selectedMemories := rankedToMemories(selected)
	if len(selected) > 0 {
		payload, _ := json.Marshal(map[string]any{
			"context_id":       contextID,
			"query":            req.Query,
			"project":          project,
			"subject":          req.Subject,
			"memory_ids":       memoryIDs(selectedMemories),
			"estimated_tokens": used,
			"budget_tokens":    budget,
			"truncated":        truncated,
		})
		if err := s.AppendEvent(ctx, Event{Kind: "context.built", Payload: string(payload), Source: Source{Kind: "retrieval", Ref: "BuildContext"}, CreatedAt: time.Now().UTC()}); err != nil {
			log.Printf("ginko: append context.built event failed: %v", err)
		}
	}

	// Resolve chunk citations for memories sourced from RAG documents.
	var citations []ChunkCitation
	for _, rm := range selected {
		m := rm.Memory
		if m.Source.Kind == "chunk" && strings.Contains(m.Source.Ref, ":") {
			parts := strings.SplitN(m.Source.Ref, ":", 2)
			if len(parts) == 2 {
				cite := ChunkCitation{MemoryID: m.ID, DocumentID: parts[0], ChunkID: parts[1]}
				if doc, err := s.GetDocument(ctx, parts[0]); err == nil {
					cite.Path = doc.Path
				}
				citations = append(citations, cite)
			}
		}
	}

	rankings := make(map[string]RankingMetadata, len(selected))
	for _, rm := range selected {
		rankings[rm.Memory.ID] = rm.Ranking
	}

	return ContextResponse{
		ContextID:       contextID,
		Context:         strings.TrimSpace(b.String()),
		Items:           selectedMemories,
		Rankings:        rankings,
		Conflicts:       detectContextConflicts(selected),
		Citations:       citations,
		EstimatedTokens: used,
		BudgetTokens:    budget,
		Truncated:       truncated,
	}, nil
}

func memoryIDs(items []Memory) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func rankedToMemories(ranked []RankedMemory) []Memory {
	out := make([]Memory, len(ranked))
	for i, r := range ranked {
		out[i] = r.Memory
	}
	return out
}

// detectContextConflicts flags structural conflicts: two or more active memories
// in the context result sharing the same (subject, type). This is a cheap
// invariant check — topic_key supersession should prevent duplicates, but when
// it doesn't, the context caller deserves to know.
func detectContextConflicts(ranked []RankedMemory) []ContextConflict {
	type key struct {
		subject string
		mtype   MemoryType
	}
	groups := map[key][]string{}
	for _, rm := range ranked {
		k := key{rm.Memory.Subject, rm.Memory.Type}
		groups[k] = append(groups[k], rm.Memory.ID)
	}
	var out []ContextConflict
	for k, ids := range groups {
		if len(ids) > 1 {
			out = append(out, ContextConflict{Subject: k.subject, Type: k.mtype, IDs: ids})
		}
	}
	return out
}

func formatContextMemory(m Memory) string {
	tags := ""
	if len(m.Tags) > 0 {
		tags = " tags=" + strings.Join(m.Tags, ",")
	}
	return fmt.Sprintf("- [%s/%s conf=%.2f src=%s:%s%s] %s", m.Type, m.Scope, m.Confidence, m.Source.Kind, m.Source.Ref, tags, compactWhitespace(m.Content))
}

func EstimateTokens(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Cheap, deterministic approximation good enough for budgeting before a provider-specific tokenizer exists.
	return (len([]rune(s)) / 4) + 1
}

func compactWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func (s *Store) contextSessionBlock(ctx context.Context, project string) string {
	lines := []string{}
	if active, err := s.ActiveSession(ctx, project); err == nil {
		lines = append(lines, fmt.Sprintf("- [session/current project=%s id=%s started=%s] active", active.Project, active.ID, active.StartedAt.Format(time.RFC3339)))
	}
	if last, err := s.LastClosedSession(ctx, project); err == nil && strings.TrimSpace(last.Summary) != "" {
		ended := ""
		if last.EndedAt != nil {
			ended = last.EndedAt.Format(time.RFC3339)
		}
		lines = append(lines, fmt.Sprintf("- [session/last project=%s id=%s ended=%s] %s", last.Project, last.ID, ended, compactWhitespace(last.Summary)))
	}
	if len(lines) == 0 {
		return ""
	}
	return "Session context:\n" + strings.Join(lines, "\n")
}
