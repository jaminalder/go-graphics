package persistence

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

// Artifact is a verified pointer, never image bytes stored in SQL.
type Artifact struct {
	Key, Digest string
	Size        int64
	Created     time.Time
}

// Artifact resolves a live disposable or pinned image without creating work.
func (d *DB) Artifact(ctx context.Context, id string) (Artifact, error) {
	var a Artifact
	err := d.Pool.QueryRow(ctx, `SELECT object_key,digest,size,created FROM art_artifacts a WHERE request=$1 AND (expires>now() OR EXISTS(SELECT 1 FROM art_pins p JOIN art_workspaces w ON w.id=p.workspace WHERE p.request=a.request AND w.expires>now()))`, id).Scan(&a.Key, &a.Digest, &a.Size, &a.Created)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, os.ErrNotExist
	}
	if err != nil {
		return a, ErrUnavailable
	}
	return a, nil
}

// Missing invalidates a confirmed missing pointer; never called on a storage outage.
func (d *DB) Missing(ctx context.Context, id, key string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer Rollback(tx)
	if _, err = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM art_artifacts WHERE request=$1 AND object_key=$2", id, key); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE art_uploads SET state='deleting',delete_after=now() WHERE object_key=$1", key); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Counts returns operational queue and storage counters.
func (d *DB) Counts(ctx context.Context) (queued, running, files int, size int64, err error) {
	err = d.Pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE state IN ('available','scheduled','retryable','pending')),count(*) FILTER(WHERE state='running') FROM river_job WHERE kind='render'`).Scan(&queued, &running)
	if err == nil {
		err = d.Pool.QueryRow(ctx, "SELECT count(*),coalesce(sum(size),0) FROM art_artifacts").Scan(&files, &size)
	}
	return
}

// Ready checks a matching recently alive renderer through SQL, without renderer RPC.
func (d *DB) Ready(ctx context.Context) error {
	if err := d.CheckSchema(ctx); err != nil {
		return err
	}
	var ready bool
	err := d.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM art_renderers r JOIN art_control c ON c.build=r.build WHERE r.build=$1 AND r.touched>now()-interval '45 seconds' AND r.storage_ok)", d.Build).Scan(&ready)
	if err != nil {
		return err
	}
	if !ready {
		return errors.New("matching renderer unavailable")
	}
	return nil
}

// Allow shares expensive-operation token buckets across web replicas.
func (d *DB) Allow(ctx context.Context, key string, rate, burst float64) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, ErrUnavailable
	}
	defer Rollback(tx)
	// Global lock bounds cardinality even under concurrent first-time callers.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7192403)"); err != nil {
		return false, ErrUnavailable
	}
	var count int
	if err = tx.QueryRow(ctx, "SELECT count(*) FROM art_rate_buckets").Scan(&count); err != nil {
		return false, ErrUnavailable
	}
	if count >= 10000 {
		if _, err = tx.Exec(ctx, "DELETE FROM art_rate_buckets WHERE touched<now()-interval '5 minutes'"); err != nil {
			return false, ErrUnavailable
		}
		if err = tx.QueryRow(ctx, "SELECT count(*) FROM art_rate_buckets").Scan(&count); err != nil {
			return false, ErrUnavailable
		}
		if count >= 10000 {
			return false, nil
		}
	}
	var tokens float64
	err = tx.QueryRow(ctx, `INSERT INTO art_rate_buckets(id,tokens,touched) VALUES($1,$3,now()) ON CONFLICT(id) DO UPDATE SET tokens=least($3,art_rate_buckets.tokens+extract(epoch FROM now()-art_rate_buckets.touched)*$2/60),touched=now() RETURNING tokens`, TokenHash(key), rate, burst).Scan(&tokens)
	if err != nil {
		return false, ErrUnavailable
	}
	allowed := tokens >= 1
	if allowed {
		if _, err = tx.Exec(ctx, "UPDATE art_rate_buckets SET tokens=tokens-1 WHERE id=$1", TokenHash(key)); err != nil {
			return false, ErrUnavailable
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return false, ErrUnavailable
	}
	return allowed, nil
}
