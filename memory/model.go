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

// MemoryStatus tracks the lifecycle state of a memory.
type MemoryStatus string

const (
	StatusActive  MemoryStatus = "active"
	StatusPending MemoryStatus = "pending"  // awaiting approval
	StatusDeleted MemoryStatus = "deleted"  // soft-deleted
)

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
	TopicKey      string        `json:"topic_key,omitempty"`
	Status        MemoryStatus  `json:"status,omitempty"`
}

// UpsertResult is returned by UpsertMemoryFull. It includes the saved memory,
// potential conflicts (same subject+type), and near-duplicates (similar content).
type UpsertResult struct {
	Memory     Memory   `json:"memory"`
	Conflicts  []Memory `json:"conflicts,omitempty"`
	Duplicates []Memory `json:"duplicates,omitempty"`
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
// Query filters memories for search/context operations.
type Query struct {
	Text           string       `json:"text"`
	Types          []MemoryType `json:"types"`
	Scopes         []Scope      `json:"scopes"`
	Subject        string       `json:"subject"`
	Tags           []string     `json:"tags"`
	Limit          int          `json:"limit"`
	IncludeRanking bool         `json:"include_ranking,omitempty"`
	Status         MemoryStatus `json:"status,omitempty"` // filter by status; empty = active only
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

// SignalKind classifies coordination signals between agents.
type SignalKind string

const (
	SignalKindNotice        SignalKind = "notice"         // durable announcement for other agents
	SignalKindLease         SignalKind = "lease"          // soft lock over a file, topic, or task
	SignalKindHandoff       SignalKind = "handoff"        // "agent A stopped here; agent B continue there"
	SignalKindConflict      SignalKind = "conflict"       // reviewable contradiction or competing claim
	SignalKindReviewRequest SignalKind = "review_request" // request for another agent or human to inspect
	SignalKindBlocker       SignalKind = "blocker"        // durable explanation of why progress is paused
)

// SignalStatus tracks the lifecycle of a coordination signal.
type SignalStatus string

const (
	SignalStatusActive       SignalStatus = "active"
	SignalStatusAcknowledged SignalStatus = "acknowledged"
	SignalStatusExpired      SignalStatus = "expired"
	SignalStatusResolved     SignalStatus = "resolved"
	SignalStatusCancelled    SignalStatus = "cancelled"
)

// AgentSignal is a durable, low-frequency coordination record shared by agents
// through the same local SQLite database. Signals are not canonical knowledge —
// they are workflow-oriented interaction state (leases, handoffs, blockers, etc.).
// Leases must always carry ExpiresAt; no infinite locks.
type AgentSignal struct {
	ID          string       `json:"id"`
	Project     string       `json:"project"`
	TopicKey    string       `json:"topic_key,omitempty"`
	Kind        SignalKind   `json:"kind"`
	Status      SignalStatus `json:"status"`
	OwnerAgent  string       `json:"owner_agent"`
	TargetAgent string       `json:"target_agent,omitempty"`
	Payload     string       `json:"payload,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	ResolvedAt  *time.Time   `json:"resolved_at,omitempty"`
	MemoryID    *string      `json:"memory_id,omitempty"`
	SessionID   *string      `json:"session_id,omitempty"`
}

// SignalQuery filters signal list operations.
type SignalQuery struct {
	Project string
	Kind    SignalKind
	Status  SignalStatus // empty = active only
	Agent   string      // owner or target
	Limit   int
}
