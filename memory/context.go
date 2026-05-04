package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

type ContextResponse struct {
	ContextID       string   `json:"context_id"`
	Context         string   `json:"context"`
	Items           []Memory `json:"items"`
	EstimatedTokens int      `json:"estimated_tokens"`
	BudgetTokens    int      `json:"budget_tokens"`
	Truncated       bool     `json:"truncated"`
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
			log.Printf("llm-memory: ensure active session failed: %v", err)
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

	items, err := s.Search(ctx, Query{
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
	if len(items) == 0 && strings.TrimSpace(req.Query) != "" && req.Subject != "" {
		// FTS is candidate generation, not truth. Conversational prompts can miss
		// durable subject memories entirely, so fall back to high-confidence active
		// subject memories and let budget/type/scope filters keep context compact.
		items, err = s.Search(ctx, Query{Types: req.Types, Scopes: req.Scopes, Subject: req.Subject, Tags: req.Tags, Limit: limit})
		if err != nil {
			return ContextResponse{}, err
		}
	}

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
	selected := make([]Memory, 0, len(items))
	truncated := false
	for _, m := range items {
		if m.Confidence < 0.5 {
			continue
		}
		line := formatContextMemory(m)
		cost := EstimateTokens(line)
		if used+cost > budget {
			truncated = true
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
		used += cost
		selected = append(selected, m)
	}

	contextID := newID("ctx")
	if len(selected) > 0 {
		payload, _ := json.Marshal(map[string]any{
			"context_id":       contextID,
			"query":            req.Query,
			"project":          project,
			"subject":          req.Subject,
			"memory_ids":       memoryIDs(selected),
			"estimated_tokens": used,
			"budget_tokens":    budget,
			"truncated":        truncated,
		})
		if err := s.AppendEvent(ctx, Event{Kind: "context.built", Payload: string(payload), Source: Source{Kind: "retrieval", Ref: "BuildContext"}, CreatedAt: time.Now().UTC()}); err != nil {
			log.Printf("llm-memory: append context.built event failed: %v", err)
		}
	}

	return ContextResponse{
		ContextID:       contextID,
		Context:         strings.TrimSpace(b.String()),
		Items:           selected,
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
