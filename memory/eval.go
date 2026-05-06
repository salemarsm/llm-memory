package memory

import (
	"context"
	"math"
	"sort"
	"time"
)

// EvalFixture is one annotated retrieval query with ground-truth relevant IDs.
// RelevantIDs are ordered by decreasing relevance for nDCG computation.
type EvalFixture struct {
	Label       string
	Query       string
	Subject     string
	Scopes      []Scope
	RelevantIDs []string // ground truth, ordered best-first
}

// FixtureResult holds retrieval output and computed metrics for one fixture.
type FixtureResult struct {
	Label         string             `json:"label"`
	Query         string             `json:"query"`
	RetrievedIDs  []string           `json:"retrieved_ids"`
	RelevantIDs   []string           `json:"relevant_ids"`
	PrecisionAt5  float64            `json:"precision_at_5"`
	NDCG10        float64            `json:"ndcg_at_10"`
	Rankings      []RankingMetadata  `json:"rankings,omitempty"`
}

// EvalReport summarizes retrieval quality across all fixtures in a run.
type EvalReport struct {
	Fixtures   []FixtureResult `json:"fixtures"`
	MeanP5     float64         `json:"mean_precision_at_5"`
	MeanNDCG10 float64         `json:"mean_ndcg_at_10"`
}

// RunEval executes each fixture against SearchRanked (re-sorted by final_score),
// then computes precision@5 and nDCG@10. The run is persisted in retrieval_eval_runs
// and retrieval_eval_items for historical comparison.
func (s *Store) RunEval(ctx context.Context, label string, fixtures []EvalFixture) (EvalReport, error) {
	runID := newID("evr")
	now := formatTime(time.Now().UTC())
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO retrieval_eval_runs(id, label, created_at) VALUES (?,?,?)`,
		runID, label, now,
	); err != nil {
		return EvalReport{}, err
	}

	var results []FixtureResult
	for _, f := range fixtures {
		ranked, err := s.SearchRanked(ctx, Query{
			Text:    f.Query,
			Subject: f.Subject,
			Scopes:  f.Scopes,
			Limit:   10,
		})
		if err != nil {
			return EvalReport{}, err
		}
		sort.Slice(ranked, func(i, j int) bool {
			return ranked[i].Ranking.FinalScore > ranked[j].Ranking.FinalScore
		})

		retrieved := make([]string, len(ranked))
		rankings := make([]RankingMetadata, len(ranked))
		for i, r := range ranked {
			retrieved[i] = r.Memory.ID
			rankings[i] = r.Ranking
		}

		p5 := precisionAtK(retrieved, f.RelevantIDs, 5)
		ndcg := ndcgAtK(retrieved, f.RelevantIDs, 10)

		for rank, r := range ranked {
			relevant := 0
			for _, id := range f.RelevantIDs {
				if id == r.Memory.ID {
					relevant = 1
					break
				}
			}
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO retrieval_eval_items(id,run_id,query,subject,memory_id,rank,relevant,final_score,rank_reason,created_at)
				 VALUES (?,?,?,?,?,?,?,?,?,?)`,
				newID("evi"), runID, f.Query, f.Subject, r.Memory.ID, rank+1, relevant,
				r.Ranking.FinalScore, r.Ranking.RankReason, now,
			)
		}

		results = append(results, FixtureResult{
			Label:        f.Label,
			Query:        f.Query,
			RetrievedIDs: retrieved,
			RelevantIDs:  f.RelevantIDs,
			PrecisionAt5: p5,
			NDCG10:       ndcg,
			Rankings:     rankings,
		})
	}

	report := EvalReport{Fixtures: results}
	for _, r := range results {
		report.MeanP5 += r.PrecisionAt5
		report.MeanNDCG10 += r.NDCG10
	}
	if n := float64(len(results)); n > 0 {
		report.MeanP5 /= n
		report.MeanNDCG10 /= n
	}
	return report, nil
}

// precisionAtK computes precision@k given a ranked list and a relevant set.
func precisionAtK(retrieved, relevant []string, k int) float64 {
	rel := make(map[string]bool, len(relevant))
	for _, id := range relevant {
		rel[id] = true
	}
	if k > len(retrieved) {
		k = len(retrieved)
	}
	hits := 0
	for _, id := range retrieved[:k] {
		if rel[id] {
			hits++
		}
	}
	if k == 0 {
		return 0
	}
	return float64(hits) / float64(k)
}

// ndcgAtK computes nDCG@k using binary relevance. Relevant IDs are unordered
// for grading purposes (any relevant doc at rank i contributes gain 1/log2(i+2)).
func ndcgAtK(retrieved, relevant []string, k int) float64 {
	rel := make(map[string]bool, len(relevant))
	for _, id := range relevant {
		rel[id] = true
	}
	if k > len(retrieved) {
		k = len(retrieved)
	}
	dcg := 0.0
	for i := 0; i < k; i++ {
		if rel[retrieved[i]] {
			dcg += 1.0 / math.Log2(float64(i+2))
		}
	}
	// IDCG: place all relevant docs at top positions
	idealK := len(relevant)
	if idealK > k {
		idealK = k
	}
	idcg := 0.0
	for i := 0; i < idealK; i++ {
		idcg += 1.0 / math.Log2(float64(i+2))
	}
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}
