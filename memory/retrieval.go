package memory

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var ftsTokenRE = regexp.MustCompile(`[\pL\pN_]+`)

var ftsStopwords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "but": {}, "by": {}, "for": {}, "from": {}, "how": {}, "i": {}, "in": {}, "is": {}, "it": {}, "me": {}, "my": {}, "of": {}, "on": {}, "or": {}, "should": {}, "that": {}, "the": {}, "this": {}, "to": {}, "what": {}, "when": {}, "where": {}, "who": {}, "why": {}, "with": {},
	"como": {}, "de": {}, "da": {}, "das": {}, "do": {}, "dos": {}, "e": {}, "em": {}, "eu": {}, "isso": {}, "minha": {}, "meu": {}, "o": {}, "os": {}, "para": {}, "por": {}, "que": {}, "um": {}, "uma": {},
}

// ftsQuery turns conversational prompts into a permissive FTS5 expression.
// Plain whitespace in FTS5 behaves like AND; memory retrieval usually wants
// candidate generation, so we OR normalized prefix terms and let BM25 rank them.
func ftsQuery(text string) string {
	seen := map[string]struct{}{}
	terms := []string{}
	for _, raw := range ftsTokenRE.FindAllString(strings.ToLower(text), -1) {
		if len([]rune(raw)) < 3 {
			continue
		}
		if _, ok := ftsStopwords[raw]; ok {
			continue
		}
		if _, ok := seen[raw]; ok {
			continue
		}
		seen[raw] = struct{}{}
		terms = append(terms, raw+"*")
		if len(terms) >= 12 {
			break
		}
	}
	return strings.Join(terms, " OR ")
}

type ContextFeedback struct {
	ContextID     string   `json:"context_id"`
	Useful        bool     `json:"useful"`
	MemoryIDsUsed []string `json:"memory_ids_used"`
	Source        Source   `json:"source"`
}

func (s *Store) RecordContextFeedback(ctx context.Context, fb ContextFeedback) error {
	if fb.ContextID == "" {
		return ErrInvalidFeedback("context_id is required")
	}
	if fb.Source.Kind == "" {
		fb.Source.Kind = "api"
	}
	ids, err := json.Marshal(fb.MemoryIDsUsed)
	if err != nil {
		return err
	}
	useful := 0
	if fb.Useful {
		useful = 1
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `INSERT INTO context_feedback(id, context_id, useful, memory_ids_json, source_kind, source_ref, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		newID("cfb"), fb.ContextID, useful, string(ids), fb.Source.Kind, fb.Source.Ref, formatTime(now))
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"context_id": fb.ContextID, "useful": fb.Useful, "memory_ids_used": fb.MemoryIDsUsed})
	return s.AppendEvent(ctx, Event{Kind: "context.feedback", Payload: string(payload), Source: fb.Source, CreatedAt: now})
}

type ErrInvalidFeedback string

func (e ErrInvalidFeedback) Error() string { return string(e) }
