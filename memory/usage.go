package memory

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

type MemoryUsage struct {
	MemoryID     string     `json:"memory_id"`
	ContextUses  int        `json:"context_uses"`
	UsefulVotes  int        `json:"useful_votes"`
	UselessVotes int        `json:"useless_votes"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

func (s *Store) MemoryUsageStats(ctx context.Context, limit int) (map[string]MemoryUsage, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `SELECT kind, payload, created_at FROM events WHERE kind IN ('context.built', 'context.feedback') ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := map[string]MemoryUsage{}
	for rows.Next() {
		var kind, payload, created string
		if err := rows.Scan(&kind, &payload, &created); err != nil {
			return nil, err
		}
		createdAt, err := parseTime(created)
		if err != nil {
			return nil, err
		}
		switch kind {
		case "context.built":
			var p struct {
				MemoryIDs []string `json:"memory_ids"`
			}
			if err := json.Unmarshal([]byte(payload), &p); err != nil {
				continue
			}
			for _, id := range p.MemoryIDs {
				st := stats[id]
				st.MemoryID = id
				st.ContextUses++
				if st.LastUsedAt == nil || createdAt.After(*st.LastUsedAt) {
					t := createdAt
					st.LastUsedAt = &t
				}
				stats[id] = st
			}
		case "context.feedback":
			var p struct {
				Useful        bool     `json:"useful"`
				MemoryIDsUsed []string `json:"memory_ids_used"`
			}
			if err := json.Unmarshal([]byte(payload), &p); err != nil {
				continue
			}
			for _, id := range p.MemoryIDsUsed {
				st := stats[id]
				st.MemoryID = id
				if p.Useful {
					st.UsefulVotes++
				} else {
					st.UselessVotes++
				}
				stats[id] = st
			}
		}
	}
	return stats, rows.Err()
}

type MemoryUsageRow struct {
	Usage        MemoryUsage `json:"usage"`
	Item         Memory      `json:"memory"`
	QualityScore float64     `json:"quality_score"`
}

// QualityScore computes a 0–1 score combining confidence, usage, feedback, and recency.
// Higher = more valuable, more used, more recent.
func QualityScore(m Memory, u MemoryUsage) float64 {
	score := m.Confidence * 0.30

	// Usage contribution (capped at 10 uses = full credit)
	usageScore := float64(u.ContextUses) / 10.0
	if usageScore > 1.0 {
		usageScore = 1.0
	}
	score += usageScore * 0.35

	// Feedback (useful vs useless votes)
	totalVotes := u.UsefulVotes + u.UselessVotes
	if totalVotes > 0 {
		score += (float64(u.UsefulVotes) / float64(totalVotes)) * 0.20
	}

	// Provenance bonus (non-trivial source)
	if m.Source.Kind != "" && m.Source.Kind != "unknown" && m.Source.Kind != "gui" {
		score += 0.10
	}

	// Recency penalty: lose up to 0.05 per 90 days of age
	ageDays := time.Since(m.UpdatedAt).Hours() / 24
	recencyPenalty := (ageDays / 90.0) * 0.05
	if recencyPenalty > 0.15 {
		recencyPenalty = 0.15
	}
	score -= recencyPenalty

	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func (s *Store) ListMemoryUsage(ctx context.Context, q Query, eventLimit int) ([]MemoryUsageRow, error) {
	items, err := s.Search(ctx, q)
	if err != nil {
		return nil, err
	}
	stats, err := s.MemoryUsageStats(ctx, eventLimit)
	if err != nil {
		return nil, err
	}
	rows := make([]MemoryUsageRow, 0, len(items))
	for _, item := range items {
		st := stats[item.ID]
		st.MemoryID = item.ID
		rows = append(rows, MemoryUsageRow{
			Usage:        st,
			Item:         item,
			QualityScore: QualityScore(item, st),
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Usage.ContextUses != rows[j].Usage.ContextUses {
			return rows[i].Usage.ContextUses > rows[j].Usage.ContextUses
		}
		return rows[i].Item.UpdatedAt.After(rows[j].Item.UpdatedAt)
	})
	return rows, nil
}
