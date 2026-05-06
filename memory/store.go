package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("memory not found")

type Store struct {
	db         *sql.DB
	embedder   EmbeddingAdapter // nil = lexical-only mode
	llmAdapter LLMAdapter       // nil = heuristic extraction only
}

// SetEmbeddingAdapter wires in an optional semantic retrieval adapter.
func (s *Store) SetEmbeddingAdapter(a EmbeddingAdapter) { s.embedder = a }

// SetLLMAdapter wires in an optional LLM adapter used for intelligent memory extraction.
func (s *Store) SetLLMAdapter(a LLMAdapter) { s.llmAdapter = a }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Keep a single connection until write concurrency policy is explicit. This avoids
	// SQLITE_BUSY surprises with local SQLite writes; WAL is enabled in migrations for readers.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			memory_id TEXT,
			kind TEXT NOT NULL,
			payload TEXT NOT NULL,
			source_kind TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS context_feedback (
			id TEXT PRIMARY KEY,
			context_id TEXT NOT NULL,
			useful INTEGER NOT NULL CHECK(useful IN (0, 1)),
			memory_ids_json TEXT NOT NULL DEFAULT '[]',
			source_kind TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_context_feedback_context ON context_feedback(context_id, created_at DESC);`,
		`ALTER TABLE events ADD COLUMN memory_id TEXT;`,
		`CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			subject TEXT NOT NULL,
			content TEXT NOT NULL,
			source_kind TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			scope TEXT NOT NULL,
			confidence REAL NOT NULL CHECK(confidence >= 0 AND confidence <= 1),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			valid_from TEXT,
			valid_until TEXT,
			supersedes_id TEXT,
			superseded_by TEXT,
			tags_json TEXT NOT NULL DEFAULT '[]',
			embedding_refs_json TEXT NOT NULL DEFAULT '{}'
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(id UNINDEXED, content, subject, tags);`,
		`CREATE INDEX IF NOT EXISTS idx_memories_type_scope ON memories(type, scope);`,
		`CREATE INDEX IF NOT EXISTS idx_memories_subject ON memories(subject);`,
		`CREATE TABLE IF NOT EXISTS memory_tags (
			memory_id TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
			tag TEXT NOT NULL,
			PRIMARY KEY(memory_id, tag)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_memory_tags_tag ON memory_tags(tag);`,
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			title TEXT NOT NULL,
			source_kind TEXT NOT NULL,
			source_ref TEXT NOT NULL,
			sha256 TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS ingestion_runs (
			id TEXT PRIMARY KEY,
			source_path TEXT NOT NULL,
			recursive INTEGER NOT NULL CHECK(recursive IN (0, 1)),
			parser TEXT NOT NULL,
			status TEXT NOT NULL,
			files_seen INTEGER NOT NULL DEFAULT 0,
			documents_created INTEGER NOT NULL DEFAULT 0,
			chunks_created INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			completed_at TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_ingestion_runs_created ON ingestion_runs(created_at DESC);`,
		`ALTER TABLE documents ADD COLUMN ingestion_run_id TEXT;`,
		`CREATE INDEX IF NOT EXISTS idx_documents_ingestion_run ON documents(ingestion_run_id);`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
			ordinal INTEGER NOT NULL,
			heading_path TEXT NOT NULL,
			content TEXT NOT NULL,
			token_count INTEGER NOT NULL,
			page_from INTEGER,
			page_to INTEGER,
			embedding_refs_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			UNIQUE(document_id, ordinal)
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(id UNINDEXED, document_id UNINDEXED, heading_path, content);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_document ON chunks(document_id, ordinal);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			summary TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_project_started ON sessions(project, started_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_project_active ON sessions(project, ended_at);`,
		// v0.4 governance fields
		`ALTER TABLE memories ADD COLUMN topic_key TEXT;`,
		`ALTER TABLE memories ADD COLUMN status TEXT NOT NULL DEFAULT 'active';`,
		`CREATE INDEX IF NOT EXISTS idx_memories_topic_key ON memories(topic_key) WHERE topic_key IS NOT NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_memories_status ON memories(status);`,
		// cross-cutting: persistent inter-agent coordination
		`CREATE TABLE IF NOT EXISTS agent_signals (
			id          TEXT PRIMARY KEY,
			project     TEXT NOT NULL,
			topic_key   TEXT,
			kind        TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'active',
			owner_agent TEXT NOT NULL,
			target_agent TEXT,
			payload     TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			expires_at  TEXT,
			resolved_at TEXT,
			memory_id   TEXT REFERENCES memories(id),
			session_id  TEXT REFERENCES sessions(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_signals_project_status ON agent_signals(project, status, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_signals_kind ON agent_signals(kind, status);`,
		`CREATE INDEX IF NOT EXISTS idx_signals_expires ON agent_signals(expires_at) WHERE expires_at IS NOT NULL;`,
		// v0.6: retrieval evaluation harness
		`CREATE TABLE IF NOT EXISTS retrieval_eval_runs (
			id         TEXT PRIMARY KEY,
			label      TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS retrieval_eval_items (
			id          TEXT PRIMARY KEY,
			run_id      TEXT NOT NULL REFERENCES retrieval_eval_runs(id) ON DELETE CASCADE,
			query       TEXT NOT NULL,
			subject     TEXT NOT NULL DEFAULT '',
			memory_id   TEXT NOT NULL,
			rank        INTEGER NOT NULL,
			relevant    INTEGER NOT NULL CHECK(relevant IN (0,1)),
			final_score REAL,
			rank_reason TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_eval_items_run ON retrieval_eval_items(run_id, rank);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations(version, name, applied_at) VALUES (1, 'bootstrap', ?)`, formatTime(time.Now().UTC()))
	return err
}

func (s *Store) AppendEvent(ctx context.Context, e Event) error {
	if e.ID == "" {
		e.ID = newID("evt")
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO events(id, memory_id, kind, payload, source_kind, source_ref, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, nullableString(e.MemoryID), e.Kind, e.Payload, e.Source.Kind, e.Source.Ref, formatTime(e.CreatedAt))
	return err
}

func (s *Store) UpsertMemory(ctx context.Context, m Memory) (Memory, error) {
	result, err := s.UpsertMemoryFull(ctx, m)
	return result.Memory, err
}

// UpsertMemoryFull saves the memory and returns conflict candidates for the same subject+type.
func (s *Store) UpsertMemoryFull(ctx context.Context, m Memory) (UpsertResult, error) {
	m.Content = StripPrivateTags(m.Content)
	if err := validateMemory(m); err != nil {
		return UpsertResult{}, err
	}
	if err := DetectSensitiveData(m.Content); err != nil {
		return UpsertResult{}, err
	}

	// topic_key: auto-supersede existing active memory with same topic_key
	if m.TopicKey != "" {
		if err := s.supersedeByTopicKey(ctx, &m); err != nil {
			return UpsertResult{}, err
		}
	}

	m = prepareMemoryForWrite(m, nil)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UpsertResult{}, err
	}
	defer tx.Rollback()
	if err := upsertMemoryTx(ctx, tx, m); err != nil {
		return UpsertResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return UpsertResult{}, err
	}

	conflicts, _ := s.FindConflicts(ctx, m)
	duplicates, _ := s.FindDuplicates(ctx, m)
	return UpsertResult{Memory: m, Conflicts: conflicts, Duplicates: duplicates}, nil
}

// supersedeByTopicKey marks any existing active memory with the same topic_key as superseded.
func (s *Store) supersedeByTopicKey(ctx context.Context, m *Memory) error {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM memories WHERE topic_key = ? AND (status IS NULL OR status = 'active') AND superseded_by IS NULL`,
		m.TopicKey)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()

	now := formatTime(time.Now().UTC())
	for _, id := range ids {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE memories SET superseded_by = ?, updated_at = ? WHERE id = ?`,
			m.ID, now, id); err != nil {
			return err
		}
	}
	return nil
}

// FindDuplicates returns active memories with the same subject and highly similar
// content (FTS5 match on key terms). Excludes m itself.
func (s *Store) FindDuplicates(ctx context.Context, m Memory) ([]Memory, error) {
	if m.Subject == "" || m.Content == "" {
		return nil, nil
	}
	// Build a short FTS query from the first ~60 chars of content
	terms := ftsQuery(m.Content)
	if terms == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT m.id, m.type, m.subject, m.content, m.source_kind, m.source_ref, m.scope, m.confidence,
		 m.created_at, m.updated_at, m.valid_from, m.valid_until, m.supersedes_id, m.superseded_by,
		 m.tags_json, m.embedding_refs_json, COALESCE(m.topic_key,''), COALESCE(m.status,'active')
		 FROM memories m
		 JOIN memories_fts ON memories_fts.id = m.id
		 WHERE memories_fts MATCH ? AND m.subject = ? AND m.id != ?
		   AND m.superseded_by IS NULL AND (m.status IS NULL OR m.status = 'active')
		 ORDER BY bm25(memories_fts) ASC LIMIT 3`,
		terms, m.Subject, m.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		mem, err := scanMemoryFull(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, mem)
	}
	return out, rows.Err()
}

// FindConflicts returns active memories with the same subject and type as m,
// excluding m itself. Used to surface potential contradictions after a write.
func (s *Store) FindConflicts(ctx context.Context, m Memory) ([]Memory, error) {
	if m.Subject == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, subject, content, source_kind, source_ref, scope, confidence,
		 created_at, updated_at, valid_from, valid_until, supersedes_id, superseded_by,
		 tags_json, embedding_refs_json, COALESCE(topic_key,''), COALESCE(status,'active')
		 FROM memories
		 WHERE subject = ? AND type = ? AND id != ?
		   AND superseded_by IS NULL AND (status IS NULL OR status = 'active')
		 ORDER BY updated_at DESC LIMIT 5`,
		m.Subject, string(m.Type), m.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		mem, err := scanMemoryFull(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, mem)
	}
	return out, rows.Err()
}

func prepareMemoryForWrite(m Memory, supersedes *string) Memory {
	m.Content = StripPrivateTags(m.Content)
	if m.ID == "" {
		m.ID = newID("mem")
	}
	now := time.Now().UTC()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if supersedes != nil {
		m.SupersedesID = supersedes
	}
	if m.Tags == nil {
		m.Tags = []string{}
	}
	if m.EmbeddingRefs == nil {
		m.EmbeddingRefs = EmbeddingRefs{}
	}
	if m.Status == "" {
		m.Status = StatusActive
	}
	return m
}

func upsertMemoryTx(ctx context.Context, tx *sql.Tx, m Memory) error {
	tags, err := json.Marshal(m.Tags)
	if err != nil {
		return err
	}
	embeds, err := json.Marshal(m.EmbeddingRefs)
	if err != nil {
		return err
	}
	status := string(m.Status)
	if status == "" {
		status = string(StatusActive)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO memories(
		id, type, subject, content, source_kind, source_ref, scope, confidence,
		created_at, updated_at, valid_from, valid_until, supersedes_id, superseded_by,
		tags_json, embedding_refs_json, topic_key, status
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		type=excluded.type,
		subject=excluded.subject,
		content=excluded.content,
		source_kind=excluded.source_kind,
		source_ref=excluded.source_ref,
		scope=excluded.scope,
		confidence=excluded.confidence,
		updated_at=excluded.updated_at,
		valid_from=excluded.valid_from,
		valid_until=excluded.valid_until,
		supersedes_id=excluded.supersedes_id,
		superseded_by=excluded.superseded_by,
		tags_json=excluded.tags_json,
		embedding_refs_json=excluded.embedding_refs_json,
		topic_key=excluded.topic_key,
		status=excluded.status`,
		m.ID, m.Type, m.Subject, m.Content, m.Source.Kind, m.Source.Ref, m.Scope, m.Confidence,
		formatTime(m.CreatedAt), formatTime(m.UpdatedAt), nullableTime(m.ValidFrom), nullableTime(m.ValidUntil), nullableString(m.SupersedesID), nullableString(m.SupersededBy),
		string(tags), string(embeds), nullableStringVal(m.TopicKey), status)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE id = ?`, m.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO memories_fts(id, content, subject, tags) VALUES (?, ?, ?, ?)`, m.ID, m.Content, m.Subject, strings.Join(m.Tags, " ")); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_tags WHERE memory_id = ?`, m.ID); err != nil {
		return err
	}
	for _, tag := range m.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO memory_tags(memory_id, tag) VALUES (?, ?)`, m.ID, tag); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetMemory(ctx context.Context, id string) (Memory, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, type, subject, content, source_kind, source_ref, scope, confidence, created_at, updated_at, valid_from, valid_until, supersedes_id, superseded_by, tags_json, embedding_refs_json FROM memories WHERE id = ?`, id)
	if err != nil {
		return Memory{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Memory{}, ErrNotFound
	}
	return scanMemory(rows)
}

func (s *Store) Search(ctx context.Context, q Query) ([]Memory, error) {
	rows, err := s.SearchRanked(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]Memory, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Memory)
	}
	return out, nil
}

func (s *Store) SearchRanked(ctx context.Context, q Query) ([]RankedMemory, error) {
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	statusFilter := "(m.status IS NULL OR m.status = 'active')"
	if q.Status != "" {
		statusFilter = "m.status = '" + string(q.Status) + "'"
	}
	where := []string{"m.superseded_by IS NULL", statusFilter}
	args := []any{}
	join := ""
	orderBy := "m.updated_at DESC"
	lexicalExpr := "NULL"
	if strings.TrimSpace(q.Text) != "" {
		fts := ftsQuery(q.Text)
		if fts != "" {
			join = "JOIN memories_fts ON memories_fts.id = m.id"
			where = append(where, "memories_fts MATCH ?")
			args = append(args, fts)
			orderBy = "bm25(memories_fts) ASC, m.confidence DESC, m.updated_at DESC"
			lexicalExpr = "bm25(memories_fts)"
		}
	}
	if q.Subject != "" {
		where = append(where, "m.subject = ?")
		args = append(args, q.Subject)
	}
	if len(q.Types) > 0 {
		where = append(where, "m.type IN ("+placeholders(len(q.Types))+")")
		for _, v := range q.Types {
			args = append(args, v)
		}
	}
	if len(q.Scopes) > 0 {
		where = append(where, "m.scope IN ("+placeholders(len(q.Scopes))+")")
		for _, v := range q.Scopes {
			args = append(args, v)
		}
	}
	for _, tag := range q.Tags {
		where = append(where, "EXISTS (SELECT 1 FROM memory_tags mt WHERE mt.memory_id = m.id AND mt.tag = ?)")
		args = append(args, tag)
	}
	args = append(args, limit)

	sqlq := fmt.Sprintf(`SELECT m.id, m.type, m.subject, m.content, m.source_kind, m.source_ref, m.scope, m.confidence, m.created_at, m.updated_at, m.valid_from, m.valid_until, m.supersedes_id, m.superseded_by, m.tags_json, m.embedding_refs_json, %s AS lexical_score FROM memories m %s WHERE %s ORDER BY %s LIMIT ?`, lexicalExpr, join, strings.Join(where, " AND "), orderBy)
	rows, err := s.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RankedMemory
	for rows.Next() {
		m, lexical, err := scanMemoryWithLexicalScore(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, RankedMemory{Memory: m, Ranking: rankMemory(m, lexical)})
	}
	return out, rows.Err()
}

func (s *Store) Supersede(ctx context.Context, oldID string, newer Memory) (Memory, error) {
	newer.Content = StripPrivateTags(newer.Content)
	if err := validateMemory(newer); err != nil {
		return Memory{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `SELECT id, type, subject, content, source_kind, source_ref, scope, confidence, created_at, updated_at, valid_from, valid_until, supersedes_id, superseded_by, tags_json, embedding_refs_json FROM memories WHERE id = ?`, oldID)
	old, err := scanMemory(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Memory{}, ErrNotFound
	}
	if err != nil {
		return Memory{}, err
	}

	newer = prepareMemoryForWrite(newer, &old.ID)
	if err := upsertMemoryTx(ctx, tx, newer); err != nil {
		return Memory{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE memories SET superseded_by = ?, updated_at = ? WHERE id = ?`, newer.ID, formatTime(time.Now().UTC()), oldID); err != nil {
		return Memory{}, err
	}
	return newer, tx.Commit()
}

// Forget soft-deletes a memory (status = 'deleted'). Use HardDelete to remove permanently.
func (s *Store) Forget(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE memories SET status = 'deleted', updated_at = ? WHERE id = ? AND (status IS NULL OR status != 'deleted')`,
		formatTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SupersessionTimelineEntry represents one supersession event.
type SupersessionTimelineEntry struct {
	OldID     string    `json:"old_id"`
	NewID     string    `json:"new_id"`
	Subject   string    `json:"subject"`
	OldType   string    `json:"old_type"`
	CreatedAt time.Time `json:"created_at"`
}

// SupersessionTimeline returns recent supersession events ordered by time desc.
func (s *Store) SupersessionTimeline(ctx context.Context, limit int) ([]SupersessionTimelineEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT old.id, new.id, new.subject, old.type, new.created_at
		 FROM memories old
		 JOIN memories new ON new.supersedes_id = old.id
		 ORDER BY new.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SupersessionTimelineEntry
	for rows.Next() {
		var e SupersessionTimelineEntry
		var created string
		if err := rows.Scan(&e.OldID, &e.NewID, &e.Subject, &e.OldType, &created); err != nil {
			return nil, err
		}
		t, err := parseTime(created)
		if err != nil {
			return nil, err
		}
		e.CreatedAt = t
		out = append(out, e)
	}
	return out, rows.Err()
}

// ApproveMemory transitions a pending memory to active status.
func (s *Store) ApproveMemory(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE memories SET status = 'active', updated_at = ? WHERE id = ? AND status = 'pending'`,
		formatTime(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDelete permanently removes a memory and its FTS index entry.
func (s *Store) HardDelete(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM memories_fts WHERE id = ?`, id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func (s *Store) ListEvents(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, memory_id, kind, payload, source_kind, source_ref, created_at FROM events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func validateMemory(m Memory) error {
	if m.Type == "" || m.Subject == "" || m.Content == "" || m.Source.Kind == "" || m.Scope == "" {
		return errors.New("type, subject, content, source.kind, and scope are required")
	}
	if m.Confidence < 0 || m.Confidence > 1 {
		return errors.New("confidence must be between 0 and 1")
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

func scanMemoryFull(rows scanner) (Memory, error) {
	var m Memory
	var created, updated string
	var validFrom, validUntil, supersedes, supersededBy sql.NullString
	var tagsJSON, embedsJSON, topicKey, status string
	if err := rows.Scan(&m.ID, &m.Type, &m.Subject, &m.Content, &m.Source.Kind, &m.Source.Ref, &m.Scope, &m.Confidence, &created, &updated, &validFrom, &validUntil, &supersedes, &supersededBy, &tagsJSON, &embedsJSON, &topicKey, &status); err != nil {
		return Memory{}, err
	}
	m.TopicKey = topicKey
	m.Status = MemoryStatus(status)
	return finishScanMemory(m, created, updated, validFrom, validUntil, supersedes, supersededBy, tagsJSON, embedsJSON)
}

func scanMemory(rows scanner) (Memory, error) {
	var m Memory
	var created, updated string
	var validFrom, validUntil, supersedes, supersededBy sql.NullString
	var tagsJSON, embedsJSON string
	if err := rows.Scan(&m.ID, &m.Type, &m.Subject, &m.Content, &m.Source.Kind, &m.Source.Ref, &m.Scope, &m.Confidence, &created, &updated, &validFrom, &validUntil, &supersedes, &supersededBy, &tagsJSON, &embedsJSON); err != nil {
		return Memory{}, err
	}
	return finishScanMemory(m, created, updated, validFrom, validUntil, supersedes, supersededBy, tagsJSON, embedsJSON)
}

func finishScanMemory(m Memory, created, updated string, validFrom, validUntil, supersedes, supersededBy sql.NullString, tagsJSON, embedsJSON string) (Memory, error) {
	var err error
	m.CreatedAt, err = parseTime(created)
	if err != nil {
		return Memory{}, err
	}
	m.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return Memory{}, err
	}
	if validFrom.Valid {
		t, err := parseTime(validFrom.String)
		if err != nil {
			return Memory{}, err
		}
		m.ValidFrom = &t
	}
	if validUntil.Valid {
		t, err := parseTime(validUntil.String)
		if err != nil {
			return Memory{}, err
		}
		m.ValidUntil = &t
	}
	if supersedes.Valid {
		m.SupersedesID = &supersedes.String
	}
	if supersededBy.Valid {
		m.SupersededBy = &supersededBy.String
	}
	if err := json.Unmarshal([]byte(tagsJSON), &m.Tags); err != nil {
		return Memory{}, err
	}
	if err := json.Unmarshal([]byte(embedsJSON), &m.EmbeddingRefs); err != nil {
		return Memory{}, err
	}
	if m.Status == "" {
		m.Status = StatusActive
	}
	return m, nil
}

func placeholders(n int) string {
	p := make([]string, n)
	for i := range p {
		p[i] = "?"
	}
	return strings.Join(p, ",")
}

func newID(prefix string) string            { return fmt.Sprintf("%s_%s", prefix, uuid.NewString()) }
func formatTime(t time.Time) string         { return t.UTC().Format(time.RFC3339Nano) }
func parseTime(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}
func nullableString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func nullableStringVal(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanMemoryWithLexicalScore(rows scanner) (Memory, *float64, error) {
	var m Memory
	var created, updated string
	var validFrom, validUntil, supersedes, supersededBy sql.NullString
	var tagsJSON, embedsJSON string
	var lexical sql.NullFloat64
	if err := rows.Scan(&m.ID, &m.Type, &m.Subject, &m.Content, &m.Source.Kind, &m.Source.Ref, &m.Scope, &m.Confidence, &created, &updated, &validFrom, &validUntil, &supersedes, &supersededBy, &tagsJSON, &embedsJSON, &lexical); err != nil {
		return Memory{}, nil, err
	}
	var err error
	m.CreatedAt, err = parseTime(created)
	if err != nil {
		return Memory{}, nil, err
	}
	m.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return Memory{}, nil, err
	}
	if validFrom.Valid {
		t, err := parseTime(validFrom.String)
		if err != nil {
			return Memory{}, nil, err
		}
		m.ValidFrom = &t
	}
	if validUntil.Valid {
		t, err := parseTime(validUntil.String)
		if err != nil {
			return Memory{}, nil, err
		}
		m.ValidUntil = &t
	}
	if supersedes.Valid {
		m.SupersedesID = &supersedes.String
	}
	if supersededBy.Valid {
		m.SupersededBy = &supersededBy.String
	}
	if err := json.Unmarshal([]byte(tagsJSON), &m.Tags); err != nil {
		return Memory{}, nil, err
	}
	if err := json.Unmarshal([]byte(embedsJSON), &m.EmbeddingRefs); err != nil {
		return Memory{}, nil, err
	}
	if lexical.Valid {
		v := lexical.Float64
		return m, &v, nil
	}
	return m, nil, nil
}

func rankMemory(m Memory, lexical *float64) RankingMetadata {
	confidence := m.Confidence
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	recency := recencyScore(m.UpdatedAt)
	provenance := provenanceScore(m.Source)
	lexicalComponent := 0.0
	reason := []string{}
	if lexical != nil {
		lexicalComponent = lexicalScore(*lexical)
		reason = append(reason, "lexical FTS5/BM25 match")
	}
	if confidence >= 0.8 {
		reason = append(reason, "high confidence")
	}
	if provenance >= 0.8 {
		reason = append(reason, "strong provenance")
	}
	if recency >= 0.75 {
		reason = append(reason, "recent")
	}
	final := 0.45*lexicalComponent + 0.30*confidence + 0.15*provenance + 0.10*recency
	if lexical == nil {
		final = 0.45*confidence + 0.35*provenance + 0.20*recency
	}
	return RankingMetadata{LexicalScore: lexical, RecencyScore: recency, ConfidenceScore: confidence, ProvenanceScore: provenance, FinalScore: final, RankReason: strings.Join(reason, " + ")}
}

func lexicalScore(bm25 float64) float64 {
	if bm25 < 0 {
		bm25 = -bm25
	}
	return 1 / (1 + bm25)
}

func recencyScore(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	days := time.Since(t).Hours() / 24
	switch {
	case days <= 7:
		return 1
	case days >= 365:
		return 0.1
	default:
		return 1 - (days-7)*(0.9/(365-7))
	}
}

func provenanceScore(src Source) float64 {
	switch strings.TrimSpace(src.Kind) {
	case "conversation", "api", "gui", "memctl", "chunk", "file:docling":
		if strings.TrimSpace(src.Ref) != "" {
			return 0.9
		}
		return 0.7
	case "suggestion":
		return 0.6
	case "":
		return 0.2
	default:
		if strings.TrimSpace(src.Ref) != "" {
			return 0.75
		}
		return 0.5
	}
}
