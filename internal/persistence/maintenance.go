package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/riverqueue/river"

	"github.com/jaminalder/go-graphics/internal/objectstore"
)

// MaintenanceArgs schedules bounded application cleanup independently of render pause.
type MaintenanceArgs struct{}

// Kind identifies application maintenance, distinct from River's own maintenance.
func (MaintenanceArgs) Kind() string { return "art_maintenance" }

// Maintainer reconciles application ownership and immutable bucket objects.
type Maintainer struct {
	river.WorkerDefaults[MaintenanceArgs]
	DB      *DB
	Objects *objectstore.Store
}

// Work uses an advisory transaction lock for duplicate periodic invocations, not leader election.
func (m *Maintainer) Work(ctx context.Context, _ *river.Job[MaintenanceArgs]) error {
	if err := m.DB.CheckBucket(ctx, m.Objects); err != nil {
		return err
	}
	tx, err := m.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer Rollback(tx)
	var locked bool
	if err = tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock(7192402)").Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	if _, err = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); err != nil {
		return err
	}
	// Cancel expired owners' unfinished interests before deleting ownership.
	rows, err := tx.Query(ctx, "SELECT i.workspace,i.request FROM art_interests i JOIN art_workspaces w ON w.id=i.workspace WHERE w.expires<=now() LIMIT 100")
	if err != nil {
		return err
	}
	var interests [][2]string
	for rows.Next() {
		var i [2]string
		if err = rows.Scan(&i[0], &i[1]); err != nil {
			rows.Close()
			return err
		}
		interests = append(interests, i)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	q := &QueueTx{DB: m.DB, Tx: tx, Context: ctx}
	for _, i := range interests {
		q.Cancel(i[0], i[1])
	}
	if q.Err != nil {
		return q.Err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM art_workspaces WHERE id IN(SELECT id FROM art_workspaces WHERE expires<=now() AND NOT EXISTS(SELECT 1 FROM art_interests WHERE workspace=art_workspaces.id) LIMIT 100)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE art_requests r SET outcome=CASE WHEN j.state='discarded' THEN 'failed' WHEN r.outcome IN ('failed','expired') THEN r.outcome ELSE 'cancelled' END FROM river_job j WHERE j.id=r.job_id AND j.state IN ('cancelled','discarded') AND r.outcome IN ('queued','running')`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `WITH expired AS(DELETE FROM art_artifacts a WHERE a.request IN(SELECT a.request FROM art_artifacts a WHERE a.expires<=now() AND NOT EXISTS(SELECT 1 FROM art_pins p JOIN art_workspaces w ON w.id=p.workspace WHERE p.request=a.request AND w.expires>now()) LIMIT 100) RETURNING object_key)
      UPDATE art_uploads SET state='deleting',delete_after=now()+interval '1 hour' WHERE object_key IN(SELECT object_key FROM expired)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE art_uploads u SET state='deleting',delete_after=now() WHERE state='pending' AND created<now()-interval '1 hour' AND NOT EXISTS(SELECT 1 FROM art_artifacts a WHERE a.object_key=u.object_key)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM art_rate_buckets WHERE touched<now()-interval '5 minutes'"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM art_renderers WHERE touched<now()-interval '1 day'"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM art_uploads WHERE state='deleting' AND deleted_at<now()-interval '7 days' AND created<now()-interval '8 days'"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM art_requests r WHERE r.id IN(SELECT r.id FROM art_requests r LEFT JOIN river_job j ON j.id=r.job_id WHERE r.created<now()-interval '1 day' AND (j.id IS NULL OR j.state IN ('completed','discarded','cancelled')) AND NOT EXISTS(SELECT 1 FROM art_interests WHERE request=r.id) AND NOT EXISTS(SELECT 1 FROM art_pins WHERE request=r.id) AND NOT EXISTS(SELECT 1 FROM art_artifacts WHERE request=r.id) AND NOT EXISTS(SELECT 1 FROM art_uploads WHERE request=r.id) LIMIT 100)`); err != nil {
		return err
	}
	if err = Notify(ctx, tx, "maintenance"); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	rows, err = m.DB.Pool.Query(ctx, "SELECT object_key FROM art_uploads WHERE state='deleting' AND delete_after<=now() ORDER BY delete_after LIMIT 100")
	if err != nil {
		return err
	}
	var keys []string
	for rows.Next() {
		var k string
		if err = rows.Scan(&k); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, key := range keys {
		if err = m.Objects.Delete(ctx, key); err != nil {
			return err
		}
		if _, err = m.DB.Pool.Exec(ctx, "UPDATE art_uploads SET deleted_at=coalesce(deleted_at,now()),delete_after=now()+interval '1 hour' WHERE object_key=$1 AND state='deleting'", key); err != nil {
			return err
		}
	}
	return m.reconcile(ctx)
}

func (m *Maintainer) reconcile(ctx context.Context) error {
	var bound bool
	var cursor string
	var location string
	if err := m.DB.Pool.QueryRow(ctx, "SELECT bucket_bound,object_cursor,bucket_location FROM art_control").Scan(&bound, &cursor, &location); err != nil {
		return err
	}
	if !bound {
		return nil
	}
	if location != m.Objects.Location() {
		return errors.New("image storage location differs from bound database")
	}
	objects, next, err := m.Objects.List(ctx, cursor)
	if err != nil {
		return err
	}
	for _, o := range objects {
		if time.Since(o.Modified) < time.Hour {
			continue
		}
		var known bool
		if err = m.DB.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM art_uploads WHERE object_key=$1)", o.Key).Scan(&known); err != nil {
			return err
		}
		if !known {
			if err = m.Objects.Delete(ctx, o.Key); err != nil {
				return err
			}
		}
	}
	_, err = m.DB.Pool.Exec(ctx, "UPDATE art_control SET object_cursor=$1", next)
	return err
}

// BindEmptyBucket authorizes reconciliation only after explicit verification of an empty prefix.
func (d *DB) BindEmptyBucket(ctx context.Context, s *objectstore.Store) error {
	var bound bool
	var location string
	if err := d.Pool.QueryRow(ctx, "SELECT bucket_bound,bucket_location FROM art_control").Scan(&bound, &location); err != nil {
		return err
	}
	if bound {
		if location != s.Location() {
			return errors.New("image storage location differs from bound database")
		}
		return nil
	}
	objects, _, err := s.List(ctx, "")
	if err != nil {
		return err
	}
	if len(objects) != 0 {
		return errors.New("refusing to bind nonempty image prefix to an empty database")
	}
	_, err = d.Pool.Exec(ctx, "UPDATE art_control SET bucket_bound=true,bucket_location=$1", s.Location())
	return err
}

// CheckBucket binds runtime object operations to the explicitly initialized location.
func (d *DB) CheckBucket(ctx context.Context, s *objectstore.Store) error {
	var valid bool
	if err := d.Pool.QueryRow(ctx, "SELECT bucket_bound AND bucket_location=$1 FROM art_control", s.Location()).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return errors.New("image bucket has not been bound to this database")
	}
	return nil
}
