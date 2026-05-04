package memory

import "time"

// MemoryType classifies canonical memories. Keep this LLM-agnostic:
// models consume these records through APIs, but do not own the format.
type MemoryType string

const (
	TypePreference   MemoryType = "preference"
	TypeFact         MemoryType = "fact"
	TypeDecision     MemoryType = "decision"
	TypeTask         MemoryType = "task"
	TypeNote         MemoryType = "note"
	TypeRelationship MemoryType = "relationship"
)

// Scope controls where a memory may be retrieved.
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
	ScopeSession Scope = "session"
	ScopePrivate Scope = "private"
)

// Source preserves provenance. Memory without provenance is weak memory.
type Source struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

// EmbeddingRefs stores references to external/vector indexes.
// Embeddings are deliberately not canonical memory; they are disposable indexes.
type EmbeddingRefs map[string]string

// Memory is the canonical unit.
type Memory struct {
	ID            string        `json:"id"`
	Type          MemoryType    `json:"type"`
	Subject       string        `json:"subject"`
	Content       string        `json:"content"`
	Source        Source        `json:"source"`
	Scope         Scope         `json:"scope"`
	Confidence    float64       `json:"confidence"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	ValidFrom     *time.Time    `json:"valid_from,omitempty"`
	ValidUntil    *time.Time    `json:"valid_until,omitempty"`
	SupersedesID  *string       `json:"supersedes_id,omitempty"`
	SupersededBy  *string       `json:"superseded_by,omitempty"`
	Tags          []string      `json:"tags"`
	EmbeddingRefs EmbeddingRefs `json:"embedding_refs"`
}

// Event is append-only raw history. Canonical memories may be derived from it.
type Event struct {
	ID        string    `json:"id"`
	MemoryID  *string   `json:"memory_id,omitempty"`
	Kind      string    `json:"kind"`
	Payload   string    `json:"payload"`
	Source    Source    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

// Query is intentionally simple. More advanced systems can plug in BM25,
// vector search, graph traversal, or policy filters behind this API.
type Query struct {
	Text           string       `json:"text"`
	Types          []MemoryType `json:"types"`
	Scopes         []Scope      `json:"scopes"`
	Subject        string       `json:"subject"`
	Tags           []string     `json:"tags"`
	Limit          int          `json:"limit"`
	IncludeRanking bool         `json:"include_ranking,omitempty"`
}

// RankingMetadata is derived retrieval state, not canonical memory.
type RankingMetadata struct {
	LexicalScore    *float64 `json:"lexical_score,omitempty"`
	SemanticScore   *float64 `json:"semantic_score,omitempty"`
	RecencyScore    float64  `json:"recency_score"`
	ConfidenceScore float64  `json:"confidence_score"`
	ProvenanceScore float64  `json:"provenance_score"`
	FinalScore      float64  `json:"final_score"`
	RankReason      string   `json:"rank_reason"`
}

type RankedMemory struct {
	Memory  Memory          `json:"memory"`
	Ranking RankingMetadata `json:"ranking"`
}

// Session captures a coding-agent work session for a project.
type Session struct {
	ID        string     `json:"id"`
	Project   string     `json:"project"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Summary   string     `json:"summary"`
}
