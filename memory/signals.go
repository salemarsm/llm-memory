package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrSignalNotFound = errors.New("signal not found")

// CreateSignal persists a new AgentSignal. Leases must supply ExpiresAt.
func (s *Store) CreateSignal(ctx context.Context, sig AgentSignal) (AgentSignal, error) {
	if err := validateSignal(sig); err != nil {
		return AgentSignal{}, err
	}
	if sig.ID == "" {
		sig.ID = newID("sig")
	}
	if sig.CreatedAt.IsZero() {
		sig.CreatedAt = time.Now().UTC()
	}
	if sig.Status == "" {
		sig.Status = SignalStatusActive
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_signals
			(id, project, topic_key, kind, status, owner_agent, target_agent,
			 payload, created_at, expires_at, resolved_at, memory_id, session_id)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		sig.ID, sig.Project, nullableStringVal(sig.TopicKey), string(sig.Kind),
		string(sig.Status), sig.OwnerAgent, nullableStringVal(sig.TargetAgent),
		sig.Payload, formatTime(sig.CreatedAt),
		nullableTime(sig.ExpiresAt), nullableTime(sig.ResolvedAt),
		sig.MemoryID, sig.SessionID,
	)
	return sig, err
}

// GetSignal returns a single signal by ID.
func (s *Store) GetSignal(ctx context.Context, id string) (AgentSignal, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+signalColumns+` FROM agent_signals WHERE id = ?`, id)
	sig, err := scanSignal(row)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentSignal{}, ErrSignalNotFound
	}
	return sig, err
}

// ListSignals returns signals matching the query. Status defaults to "active".
func (s *Store) ListSignals(ctx context.Context, q SignalQuery) ([]AgentSignal, error) {
	where := []string{}
	args := []any{}

	if q.Project != "" {
		where = append(where, "project = ?")
		args = append(args, q.Project)
	}
	if q.Kind != "" {
		where = append(where, "kind = ?")
		args = append(args, string(q.Kind))
	}
	status := q.Status
	if status == "" {
		status = SignalStatusActive
	}
	if status != "*" {
		where = append(where, "status = ?")
		args = append(args, string(status))
	}
	if q.Agent != "" {
		where = append(where, "(owner_agent = ? OR target_agent = ?)")
		args = append(args, q.Agent, q.Agent)
	}

	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := fmt.Sprintf(`SELECT %s FROM agent_signals %s ORDER BY created_at DESC LIMIT %d`, signalColumns, clause, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AgentSignal
	for rows.Next() {
		sig, err := scanSignal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sig)
	}
	return out, rows.Err()
}

// UpdateSignalStatus transitions a signal to a new status and records resolved_at when appropriate.
func (s *Store) UpdateSignalStatus(ctx context.Context, id string, status SignalStatus) (AgentSignal, error) {
	var resolvedAt *time.Time
	if status == SignalStatusResolved || status == SignalStatusCancelled {
		t := time.Now().UTC()
		resolvedAt = &t
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE agent_signals SET status = ?, resolved_at = ? WHERE id = ?`,
		string(status), nullableTime(resolvedAt), id,
	)
	if err != nil {
		return AgentSignal{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return AgentSignal{}, ErrSignalNotFound
	}
	return s.GetSignal(ctx, id)
}

// ExpireStaleSignals sets status=expired on all active signals whose expires_at is in the past.
// Returns the number of signals expired.
func (s *Store) ExpireStaleSignals(ctx context.Context) (int, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE agent_signals SET status = 'expired'
		 WHERE status = 'active' AND expires_at IS NOT NULL AND expires_at < ?`,
		formatTime(time.Now().UTC()),
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// --- helpers ---

const signalColumns = `id, project, COALESCE(topic_key,''), kind, status,
	owner_agent, COALESCE(target_agent,''), payload,
	created_at, expires_at, resolved_at, memory_id, session_id`

type signalScanner interface {
	Scan(dest ...any) error
}

func scanSignal(row signalScanner) (AgentSignal, error) {
	var sig AgentSignal
	var createdAt string
	var expiresAt, resolvedAt sql.NullString
	var memoryID, sessionID sql.NullString
	err := row.Scan(
		&sig.ID, &sig.Project, &sig.TopicKey,
		(*string)(&sig.Kind), (*string)(&sig.Status),
		&sig.OwnerAgent, &sig.TargetAgent, &sig.Payload,
		&createdAt, &expiresAt, &resolvedAt, &memoryID, &sessionID,
	)
	if err != nil {
		return AgentSignal{}, err
	}
	sig.CreatedAt, _ = parseTime(createdAt)
	if expiresAt.Valid {
		t, _ := parseTime(expiresAt.String)
		sig.ExpiresAt = &t
	}
	if resolvedAt.Valid {
		t, _ := parseTime(resolvedAt.String)
		sig.ResolvedAt = &t
	}
	if memoryID.Valid {
		sig.MemoryID = &memoryID.String
	}
	if sessionID.Valid {
		sig.SessionID = &sessionID.String
	}
	return sig, nil
}

func validateSignal(sig AgentSignal) error {
	if sig.Project == "" {
		return errors.New("signal project is required")
	}
	if sig.Kind == "" {
		return errors.New("signal kind is required")
	}
	if sig.OwnerAgent == "" {
		return errors.New("signal owner_agent is required")
	}
	if sig.Kind == SignalKindLease && sig.ExpiresAt == nil {
		return errors.New("lease signals must have expires_at; no infinite locks")
	}
	return nil
}

