package persistence

import (
	"context"
	"errors"
	"time"
)

// LoadTiming measures one server-side cohort; durations are seconds, not client polling time.
type LoadTiming struct {
	Tier             string   `json:"tier"`
	Renderer         string   `json:"last_renderer"`
	State            string   `json:"state"`
	Jobs             int      `json:"jobs"`
	Retried          int      `json:"retried_jobs"`
	QueueAverage     *float64 `json:"enqueue_to_last_attempt_avg_seconds"`
	ExecutionAverage *float64 `json:"last_attempt_to_final_avg_seconds"`
	TotalP95         *float64 `json:"enqueue_to_final_p95_seconds"`
	ExecutionSum     *float64 `json:"last_attempt_execution_sum_seconds"`
}

// LoadStats queries jobs enqueued in a half-open time window; no run header authorizes or bypasses work.
func (d *DB) LoadStats(ctx context.Context, since, until time.Time) ([]LoadTiming, error) {
	if since.IsZero() || !until.After(since) || until.Sub(since) > 25*time.Hour {
		return nil, errors.New("invalid load window (maximum 25 hours)")
	}
	rows, err := d.Pool.Query(ctx, `SELECT coalesce(r.tier,'unknown'),coalesce(j.attempted_by[array_upper(j.attempted_by,1)],''),j.state::text,count(*),count(*) FILTER(WHERE j.attempt>1),
      avg(extract(epoch FROM j.attempted_at-j.created_at))::float8,
      avg(extract(epoch FROM j.finalized_at-j.attempted_at))::float8,
      percentile_cont(0.95) WITHIN GROUP(ORDER BY extract(epoch FROM j.finalized_at-j.created_at))::float8,
      sum(extract(epoch FROM j.finalized_at-j.attempted_at))::float8
      FROM river_job j LEFT JOIN art_requests r ON r.job_id=j.id WHERE j.kind='render' AND j.created_at >= $1 AND j.created_at < $2
      GROUP BY 1,2,3 ORDER BY 1,2,3`, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LoadTiming{}
	for rows.Next() {
		var t LoadTiming
		if err = rows.Scan(&t.Tier, &t.Renderer, &t.State, &t.Jobs, &t.Retried, &t.QueueAverage, &t.ExecutionAverage, &t.TotalP95, &t.ExecutionSum); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
