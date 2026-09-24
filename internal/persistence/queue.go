package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
)

// RenderArgs references immutable canonical application data, not a JSON-normalized recipe.
type RenderArgs struct {
	Request string `json:"request"`
	Build   string `json:"build"`
}

// Kind identifies the River handler.
func (RenderArgs) Kind() string { return "render" }

// QueueTx binds studio domain operations to its current transaction.
// Err records errors from the legacy domain's read/cancellation interface; the caller must check it before committing.
type QueueTx struct {
	DB      *DB
	Tx      pgx.Tx
	Context context.Context
	Err     error
}

const activeStates = "('available','running','retryable','scheduled','pending')"

// Admit atomically reserves up to four jobs with global and per-owner budgets.
func (q *QueueTx) Admit(owner string, requests []renderjob.Request) ([]string, error) {
	ctx, tx := q.Context, q.Tx
	if len(requests) == 0 || len(requests) > 4 {
		return nil, renderjob.ErrBusy
	}
	var enabled bool
	var build string
	var epoch int64
	if err := tx.QueryRow(ctx, "SELECT enabled,build,epoch FROM art_control WHERE singleton FOR UPDATE").Scan(&enabled, &build, &epoch); err != nil {
		return nil, err
	}
	if !enabled || build != q.DB.Build {
		return nil, renderjob.ErrBusy
	}
	ids := make([]string, len(requests))
	for i, r := range requests {
		if err := r.Validate(build); err != nil {
			return nil, err
		}
		recipe, err := publish.Validate(r.Recipe)
		if err != nil {
			return nil, err
		}
		tier, err := publish.Tier(recipe.ID(), r.Tier)
		if err != nil {
			return nil, err
		}
		id := recipe.Key(tier, build)
		ids[i] = id
		var existing int64
		var ready, active bool
		err = tx.QueryRow(ctx, `SELECT coalesce(r.job_id,0),
          EXISTS(SELECT 1 FROM art_artifacts a WHERE a.request=r.id AND (a.expires>now() OR EXISTS(SELECT 1 FROM art_pins p JOIN art_workspaces w ON w.id=p.workspace WHERE p.request=r.id AND w.expires>now()))),
          coalesce(j.state IN `+activeStates+`,false)
          FROM art_requests r LEFT JOIN river_job j ON j.id=r.job_id WHERE r.id=$1`, id).Scan(&existing, &ready, &active)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		var owned, subscribers int
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM art_interests WHERE request=$1 AND workspace<>$2", id, owner).Scan(&subscribers); err != nil {
			return nil, err
		}
		if subscribers >= 24 {
			return nil, renderjob.ErrBusy
		}
		if !ready {
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM art_interests i JOIN art_requests r ON r.id=i.request JOIN river_job j ON j.id=r.job_id WHERE i.workspace=$1 AND j.state IN `+activeStates+` AND i.request<>$2`, owner, id).Scan(&owned); err != nil {
				return nil, err
			}
			if owned >= 4 {
				return nil, renderjob.ErrBusy
			}
		}
		if !ready && !active {
			var outstanding int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind='render' AND state IN `+activeStates).Scan(&outstanding); err != nil {
				return nil, err
			}
			if outstanding >= 9 {
				return nil, renderjob.ErrBusy
			}
			instance := q.DB.Instance
			metadata, err := json.Marshal(map[string]string{"producer_id": instance.ID, "producer_name": instance.Name, "producer_hostname": instance.Hostname, "producer_build": build})
			if err != nil {
				return nil, err
			}
			job, err := q.DB.River.InsertTx(ctx, tx, RenderArgs{id, build}, &river.InsertOpts{Queue: Queue(build), MaxAttempts: 3, Metadata: metadata})
			if err != nil {
				return nil, err
			}
			_, err = tx.Exec(ctx, `INSERT INTO art_requests(id,recipe,tier,build,job_id,generation,epoch) VALUES($1,$2,$3,$4,$5,$6,$7)
            ON CONFLICT(id) DO UPDATE SET job_id=excluded.job_id,generation=excluded.generation,epoch=excluded.epoch,outcome='queued',created=now(),first_started=NULL`, id, r.Recipe, r.Tier, build, job.Job.ID, executionID(), epoch)
			if err != nil {
				return nil, err
			}
		}
		if _, err = tx.Exec(ctx, "INSERT INTO art_interests VALUES($1,$2) ON CONFLICT DO NOTHING", owner, id); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// Status derives execution state from River and availability from retained artifacts.
func (q *QueueTx) Status(owner, id string) renderjob.Status {
	s := renderjob.Status{ID: id, State: "expired"}
	if id == "" {
		return s
	}
	var state, outcome string
	var ready bool
	err := q.Tx.QueryRow(q.Context, `SELECT coalesce(j.state::text,''),r.outcome,
      EXISTS(SELECT 1 FROM art_artifacts a WHERE a.request=r.id AND (a.expires>now() OR EXISTS(SELECT 1 FROM art_pins p JOIN art_workspaces w ON w.id=p.workspace WHERE p.request=r.id AND w.expires>now())))
      FROM art_requests r LEFT JOIN river_job j ON j.id=r.job_id JOIN art_interests i ON i.request=r.id WHERE r.id=$1 AND i.workspace=$2`, id, owner).Scan(&state, &outcome, &ready)
	if errors.Is(err, pgx.ErrNoRows) {
		return s
	}
	if err != nil {
		q.Err = err
		return s
	}
	s.State = outcome
	if ready {
		s.State = "ready"
		return s
	}
	switch state {
	case "available", "pending", "scheduled", "retryable":
		s.State = "queued"
	case "running":
		s.State = "running"
	case "cancelled":
		if outcome != "expired" && outcome != "failed" {
			s.State = "cancelled"
		}
	case "discarded":
		s.State = "failed"
	case "completed":
		s.State = "expired"
	}
	if s.State == "ready" {
		s.State = "expired"
	}
	if s.State == "failed" {
		s.Message = "This sample could not finish. Try again when you are ready."
	}
	return s
}

// Cancel releases one interest and fences publication before requesting River cancellation.
func (q *QueueTx) Cancel(owner, id string) {
	if id == "" || q.Err != nil {
		return
	}
	_, err := q.Tx.Exec(q.Context, "DELETE FROM art_interests WHERE workspace=$1 AND request=$2", owner, id)
	if err != nil {
		q.Err = err
		return
	}
	var job int64
	err = q.Tx.QueryRow(q.Context, `UPDATE art_requests SET generation=md5(generation||clock_timestamp()::text),outcome=CASE WHEN outcome IN ('queued','running') THEN 'cancelled' ELSE outcome END
      WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM art_interests WHERE request=$1) RETURNING coalesce(job_id,0)`, id).Scan(&job)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		q.Err = err
		return
	}
	if job != 0 {
		_, err = q.DB.River.JobCancelTx(q.Context, q.Tx, job)
		if err != nil && !errors.Is(err, river.ErrNotFound) {
			q.Err = err
		}
	}
}

// Notify is a transactional wake-up; records remain authoritative.
func Notify(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx, "SELECT pg_notify('art_results',$1)", id)
	return err
}
