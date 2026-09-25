package persistence

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jaminalder/go-graphics/internal/limits"
)

// QueueSummary counts execution jobs, not HTTP requests or unique visitors.
type QueueSummary struct {
	Waiting       int     `json:"waiting"`
	Running       int     `json:"running"`
	Retrying      int     `json:"retrying"`
	Failed        int     `json:"failed_last_hour"`
	Completed     int     `json:"completed_last_hour"`
	OldestSeconds float64 `json:"oldest_waiting_seconds"`
}

// InstanceStatus includes idle, draining/stopped and stale process boots.
type InstanceStatus struct {
	Instance
	AgeSeconds float64 `json:"age_seconds"`
	Status     string  `json:"status"`
	StorageOK  *bool   `json:"storage_ok"`
	Running    int     `json:"running"`
	Completed  int     `json:"completed_last_hour"`
}

// Flow groups jobs by their original producer and last claimant. Retried/coalesced work is not duplicated.
type Flow struct {
	Producer Instance `json:"producer"`
	Renderer Instance `json:"renderer"`
	State    string   `json:"state"`
	Jobs     int      `json:"jobs"`
}

// MonitorSnapshot is a coherent, bounded, read-only operational snapshot.
type MonitorSnapshot struct {
	Limits             limits.Policy    `json:"limits"`
	At                 time.Time        `json:"at"`
	Enabled            bool             `json:"admission_enabled"`
	Build              string           `json:"build"`
	Queue              QueueSummary     `json:"queue"`
	Instances          []InstanceStatus `json:"instances"`
	Flow               []Flow           `json:"flow"`
	InstancesTruncated bool             `json:"instances_truncated"`
	FlowTruncated      bool             `json:"flow_truncated"`
}

// Monitor queries retained jobs and presence without keeping a transaction open between refreshes.
func (d *DB) Monitor(ctx context.Context) (MonitorSnapshot, error) {
	s := MonitorSnapshot{Instances: []InstanceStatus{}, Flow: []Flow{}, Limits: d.Limits}
	tx, err := d.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return s, err
	}
	defer Rollback(tx)
	if err = tx.QueryRow(ctx, "SELECT now(),enabled,build FROM art_control").Scan(&s.At, &s.Enabled, &s.Build); err != nil {
		return s, err
	}
	if err = tx.QueryRow(ctx, `SELECT
      count(*) FILTER(WHERE state IN ('available','scheduled','pending')),
      count(*) FILTER(WHERE state='running'),
      count(*) FILTER(WHERE state='retryable'),
      count(*) FILTER(WHERE (state='discarded' OR (state='cancelled' AND metadata->>'art_terminal_outcome'='failed')) AND finalized_at>=now()-interval '1 hour'),
      count(*) FILTER(WHERE state='completed' AND finalized_at>=now()-interval '1 hour'),
      coalesce(extract(epoch FROM now()-min(created_at) FILTER(WHERE state IN ('available','scheduled','pending','retryable'))),0)
      FROM river_job WHERE kind='render'`).Scan(&s.Queue.Waiting, &s.Queue.Running, &s.Queue.Retrying, &s.Queue.Failed, &s.Queue.Completed, &s.Queue.OldestSeconds); err != nil {
		return s, err
	}
	rows, err := tx.Query(ctx, `WITH counts AS(
      SELECT attempted_by[array_upper(attempted_by,1)] AS worker,
        count(*) FILTER(WHERE state='running') AS running,
        count(*) FILTER(WHERE state='completed' AND finalized_at>=now()-interval '1 hour') AS completed
      FROM river_job WHERE kind='render' AND (state='running' OR finalized_at>=now()-interval '1 hour') GROUP BY 1)
      SELECT i.id,i.role,i.hostname,i.name,i.build,extract(epoch FROM now()-i.touched)::float8,
        CASE WHEN i.stopped IS NOT NULL THEN 'stopped' WHEN i.touched<now()-interval '45 seconds' THEN 'stale' ELSE 'live' END,
        i.storage_ok,coalesce(c.running,0),coalesce(c.completed,0)
      FROM art_instances i LEFT JOIN counts c ON c.worker=i.id
      WHERE i.touched>=now()-interval '1 hour' OR coalesce(c.running,0)>0
      ORDER BY (i.stopped IS NULL AND i.touched>=now()-interval '45 seconds') DESC,i.role,i.name,i.started DESC LIMIT 201`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var i InstanceStatus
		if err = rows.Scan(&i.ID, &i.Role, &i.Hostname, &i.Name, &i.Build, &i.AgeSeconds, &i.Status, &i.StorageOK, &i.Running, &i.Completed); err != nil {
			rows.Close()
			return s, err
		}
		s.Instances = append(s.Instances, i)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return s, err
	}
	if len(s.Instances) > 200 {
		s.Instances = s.Instances[:200]
		s.InstancesTruncated = true
	}
	rows, err = tx.Query(ctx, `WITH jobs AS(
      SELECT metadata->>'producer_id' AS producer_id,metadata->>'producer_name' AS producer_name,
        metadata->>'producer_hostname' AS producer_hostname,metadata->>'producer_build' AS producer_build,
        attempted_by[array_upper(attempted_by,1)] AS worker,
        CASE WHEN state='cancelled' AND metadata->>'art_terminal_outcome'='failed' THEN 'failed' ELSE state::text END AS state
      FROM river_job WHERE kind='render' AND (state IN ('available','running','retryable','scheduled','pending') OR finalized_at>=now()-interval '1 hour'))
      SELECT coalesce(producer_id,''),coalesce(producer_hostname,''),coalesce(producer_name,''),coalesce(producer_build,''),
        coalesce(worker,''),coalesce(i.hostname,''),coalesce(i.name,worker,''),coalesce(i.build,''),j.state,count(*)
      FROM jobs j LEFT JOIN art_instances i ON i.id=j.worker GROUP BY 1,2,3,4,5,6,7,8,9
      ORDER BY count(*) DESC,1,5,9 LIMIT 201`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		f := Flow{Producer: Instance{Role: "web"}, Renderer: Instance{Role: "renderer"}}
		if err = rows.Scan(&f.Producer.ID, &f.Producer.Hostname, &f.Producer.Name, &f.Producer.Build, &f.Renderer.ID, &f.Renderer.Hostname, &f.Renderer.Name, &f.Renderer.Build, &f.State, &f.Jobs); err != nil {
			rows.Close()
			return s, err
		}
		s.Flow = append(s.Flow, f)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return s, err
	}
	if len(s.Flow) > 200 {
		s.Flow = s.Flow[:200]
		s.FlowTruncated = true
	}
	if err = tx.Commit(ctx); err != nil {
		return s, err
	}
	return s, nil
}
