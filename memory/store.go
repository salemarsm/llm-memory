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
	db *sql.DB
}

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
	if err := validateMemory(m); err != nil {
		return Memory{}, err
	}
	m = prepareMemoryForWrite(m, nil)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, err
	}
	defer tx.Rollback()
	if err := upsertMemoryTx(ctx, tx, m); err != nil {
		return Memory{}, err
	}
	return m, tx.Commit()
}

func prepareMemoryForWrite(m Memory, supersedes *string) Memory {
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
	_, err = tx.ExecContext(ctx, `INSERT INTO memories(
		id, type, subject, content, source_kind, source_ref, scope, confidence,
		created_at, updated_at, valid_from, valid_until, supersedes_id, superseded_by,
		tags_json, embedding_refs_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		embedding_refs_json=excluded.embedding_refs_json`,
		m.ID, m.Type, m.Subject, m.Content, m.Source.Kind, m.Source.Ref, m.Scope, m.Confidence,
		formatTime(m.CreatedAt), formatTime(m.UpdatedAt), nullableTime(m.ValidFrom), nullableTime(m.ValidUntil), nullableString(m.SupersedesID), nullableString(m.SupersededBy),
		string(tags), string(embeds))
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
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	where := []string{"m.superseded_by IS NULL"}
	args := []any{}
	join := ""
	orderBy := "m.updated_at DESC"
	if strings.TrimSpace(q.Text) != "" {
		fts := ftsQuery(q.Text)
		if fts != "" {
			join = "JOIN memories_fts ON memories_fts.id = m.id"
			where = append(where, "memories_fts MATCH ?")
			args = append(args, fts)
			orderBy = "bm25(memories_fts) ASC, m.confidence DESC, m.updated_at DESC"
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

	sqlq := fmt.Sprintf(`SELECT m.id, m.type, m.subject, m.content, m.source_kind, m.source_ref, m.scope, m.confidence, m.created_at, m.updated_at, m.valid_from, m.valid_until, m.supersedes_id, m.superseded_by, m.tags_json, m.embedding_refs_json FROM memories m %s WHERE %s ORDER BY %s LIMIT ?`, join, strings.Join(where, " AND "), orderBy)
	rows, err := s.db.QueryContext(ctx, sqlq, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Supersede(ctx context.Context, oldID string, newer Memory) (Memory, error) {
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

func (s *Store) Forget(ctx context.Context, id string) error {
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
		var e Event
		var created string
		var memoryID sql.NullString
		if err := rows.Scan(&e.ID, &memoryID, &e.Kind, &e.Payload, &e.Source.Kind, &e.Source.Ref, &created); err != nil {
			return nil, err
		}
		if memoryID.Valid {
			e.MemoryID = &memoryID.String
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

func scanMemory(rows scanner) (Memory, error) {
	var m Memory
	var created, updated string
	var validFrom, validUntil, supersedes, supersededBy sql.NullString
	var tagsJSON, embedsJSON string
	if err := rows.Scan(&m.ID, &m.Type, &m.Subject, &m.Content, &m.Source.Kind, &m.Source.Ref, &m.Scope, &m.Confidence, &created, &updated, &validFrom, &validUntil, &supersedes, &supersededBy, &tagsJSON, &embedsJSON); err != nil {
		return Memory{}, err
	}
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
