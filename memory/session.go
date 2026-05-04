package memory

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type SessionStartRequest struct {
	Project string `json:"project"`
}

type SessionEndRequest struct {
	Project string `json:"project"`
	Summary string `json:"summary"`
}

type SessionSummaryRequest struct {
	Project   string `json:"project"`
	SessionID string `json:"session_id"`
}

func (s *Store) StartSession(ctx context.Context, project string) (Session, error) {
	project = normalizeSessionProject(project)
	if active, err := s.ActiveSession(ctx, project); err == nil {
		return active, nil
	} else if err != ErrNotFound {
		return Session{}, err
	}
	now := time.Now().UTC()
	sess := Session{ID: newID("ses"), Project: project, StartedAt: now}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(id, project, started_at, summary) VALUES (?, ?, ?, '')`, sess.ID, sess.Project, formatTime(sess.StartedAt))
	if err != nil {
		return Session{}, err
	}
	_ = s.AppendEvent(ctx, Event{Kind: "session.started", Payload: sess.ID, Source: Source{Kind: "session", Ref: project}, CreatedAt: now})
	return sess, nil
}

func (s *Store) EndActiveSession(ctx context.Context, project, summary string) (Session, error) {
	project = normalizeSessionProject(project)
	active, err := s.ActiveSession(ctx, project)
	if err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE sessions SET ended_at = ?, summary = ? WHERE id = ?`, formatTime(now), strings.TrimSpace(summary), active.ID)
	if err != nil {
		return Session{}, err
	}
	active.EndedAt = &now
	active.Summary = strings.TrimSpace(summary)
	_ = s.AppendEvent(ctx, Event{Kind: "session.ended", Payload: active.ID, Source: Source{Kind: "session", Ref: project}, CreatedAt: now})
	return active, nil
}

func (s *Store) ActiveSession(ctx context.Context, project string) (Session, error) {
	project = normalizeSessionProject(project)
	return s.scanSession(s.db.QueryRowContext(ctx, `SELECT id, project, started_at, ended_at, summary FROM sessions WHERE project = ? AND ended_at IS NULL ORDER BY started_at DESC LIMIT 1`, project))
}

func (s *Store) LastClosedSession(ctx context.Context, project string) (Session, error) {
	project = normalizeSessionProject(project)
	return s.scanSession(s.db.QueryRowContext(ctx, `SELECT id, project, started_at, ended_at, summary FROM sessions WHERE project = ? AND ended_at IS NOT NULL ORDER BY ended_at DESC LIMIT 1`, project))
}

func (s *Store) GetSession(ctx context.Context, id string) (Session, error) {
	return s.scanSession(s.db.QueryRowContext(ctx, `SELECT id, project, started_at, ended_at, summary FROM sessions WHERE id = ?`, id))
}

func (s *Store) SessionSummary(ctx context.Context, req SessionSummaryRequest) (Session, error) {
	if strings.TrimSpace(req.SessionID) != "" {
		return s.GetSession(ctx, req.SessionID)
	}
	if sess, err := s.ActiveSession(ctx, req.Project); err == nil {
		return sess, nil
	} else if err != ErrNotFound {
		return Session{}, err
	}
	return s.LastClosedSession(ctx, req.Project)
}

func (s *Store) EnsureActiveSession(ctx context.Context, project string) (Session, error) {
	return s.StartSession(ctx, project)
}

func (s *Store) scanSession(row *sql.Row) (Session, error) {
	var sess Session
	var started string
	var ended sql.NullString
	if err := row.Scan(&sess.ID, &sess.Project, &started, &ended, &sess.Summary); err != nil {
		if err == sql.ErrNoRows {
			return Session{}, ErrNotFound
		}
		return Session{}, err
	}
	sess.StartedAt, _ = parseTime(started)
	if ended.Valid && strings.TrimSpace(ended.String) != "" {
		t, _ := parseTime(ended.String)
		sess.EndedAt = &t
	}
	return sess, nil
}

func normalizeSessionProject(project string) string {
	project = NormalizeProject(project)
	if project == "" {
		return "default"
	}
	return project
}
