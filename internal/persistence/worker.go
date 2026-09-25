package persistence

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image/png"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/jaminalder/go-graphics/internal/objectstore"
	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
)

// Worker publishes completed render results without involving the web process.
type Worker struct {
	river.WorkerDefaults[RenderArgs]
	DB       *DB
	Objects  *objectstore.Store
	Renderer renderjob.Renderer
}

// NextRetry bounds infrastructure retry delays; deterministic failures cancel the job.
func (w *Worker) NextRetry(j *river.Job[RenderArgs]) time.Time {
	return time.Now().Add(time.Duration(j.Attempt*j.Attempt) * time.Second)
}

func executionID() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// Eligible checks the current attempt using River's public transactional read API.
// Call in SERIALIZABLE transactions which subsequently update the job with JobCompleteTx:
// a concurrent rescue changes that row and PostgreSQL aborts the stale writer.
func Eligible(ctx context.Context, d *DB, tx pgx.Tx, job *river.Job[RenderArgs]) error {
	current, err := d.River.JobGetTx(ctx, tx, job.ID)
	if err != nil {
		return err
	}
	if current.State != rivertype.JobStateRunning || current.Attempt != job.Attempt {
		return errors.New("render attempt superseded")
	}
	return nil
}

// Work owns the render/upload attempt; external effects use a unique object key.
func (w *Worker) Work(ctx context.Context, job *river.Job[RenderArgs]) error {
	if job.Args.Build != w.DB.Build {
		return river.JobCancel(errors.New("unsupported renderer build"))
	}
	if err := w.DB.CheckBucket(ctx, w.Objects); err != nil {
		return err
	}
	if err := w.Objects.Health(ctx); err != nil {
		return err
	}
	generation := executionID()
	var request renderjob.Request
	var epoch int64
	err := retrySerialization(ctx, func() error { var err error; request, epoch, err = w.begin(ctx, job, generation); return err })
	if err != nil {
		return err
	}
	var data bytes.Buffer
	if err = w.Renderer.Render(ctx, request, &data); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return w.terminal(ctx, job, generation, err)
	}
	recipe, err := publish.Validate(request.Recipe)
	if err != nil {
		return w.terminal(ctx, job, generation, err)
	}
	tier, err := publish.Tier(recipe.ID(), request.Tier)
	if err != nil {
		return w.terminal(ctx, job, generation, err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data.Bytes()))
	if err != nil || int64(data.Len()) > objectstore.MaxImage || config.Width != tier.Width || config.Height != tier.Height {
		return w.terminal(ctx, job, generation, errors.New("invalid renderer output"))
	}
	if _, err = png.Decode(bytes.NewReader(data.Bytes())); err != nil {
		return w.terminal(ctx, job, generation, err)
	}
	key := job.Args.Request + "/" + generation + ".png"
	if err = w.reserve(ctx, job, generation, key, int64(data.Len())); err != nil {
		return err
	}
	if err = w.Objects.Put(ctx, key, data.Bytes()); err != nil {
		return err
	}
	sum := sha256.Sum256(data.Bytes())
	return retrySerialization(ctx, func() error {
		return w.complete(ctx, job, generation, epoch, key, hex.EncodeToString(sum[:]), int64(data.Len()))
	})
}

// Retry only PostgreSQL serialization/deadlock aborts, never an ambiguous network commit.
func retrySerialization(ctx context.Context, fn func() error) error {
	for attempt := 0; ; attempt++ {
		err := fn()
		var pgErr *pgconn.PgError
		if attempt >= 2 || !errors.As(err, &pgErr) || (pgErr.Code != "40001" && pgErr.Code != "40P01") {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
}

func (w *Worker) begin(ctx context.Context, job *river.Job[RenderArgs], generation string) (renderjob.Request, int64, error) {
	tx, err := w.DB.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return renderjob.Request{}, 0, err
	}
	defer Rollback(tx)
	var build string
	var epoch int64
	if err = tx.QueryRow(ctx, "SELECT build,epoch FROM art_control FOR UPDATE").Scan(&build, &epoch); err != nil {
		return renderjob.Request{}, 0, err
	}
	if err = Eligible(ctx, w.DB, tx, job); err != nil {
		return renderjob.Request{}, 0, river.JobCancel(err)
	}
	var r renderjob.Request
	var fresh bool
	err = tx.QueryRow(ctx, `SELECT recipe,tier,(first_started IS NULL AND created<now()-make_interval(secs=>$4)) OR (first_started IS NOT NULL AND first_started<now()-interval '5 minutes')
      FROM art_requests WHERE id=$1 AND job_id=$2 AND epoch=$3 AND EXISTS(SELECT 1 FROM art_interests i JOIN art_workspaces ws ON ws.id=i.workspace WHERE i.request=$1 AND ws.expires>now()) FOR UPDATE`, job.Args.Request, job.ID, epoch, w.DB.Limits.QueueAgeSeconds).Scan(&r.Recipe, &r.Tier, &fresh)
	if errors.Is(err, pgx.ErrNoRows) || build != job.Args.Build {
		return r, epoch, river.JobCancel(errors.New("render request no longer active"))
	}
	if err != nil {
		return r, epoch, err
	}
	if fresh {
		_, err = tx.Exec(ctx, "UPDATE art_requests SET outcome='expired' WHERE id=$1", job.Args.Request)
		if err != nil {
			return r, epoch, err
		}
		if err = Notify(ctx, tx, job.Args.Request); err != nil {
			return r, epoch, err
		}
		if err = tx.Commit(ctx); err != nil {
			return r, epoch, err
		}
		return r, epoch, river.JobCancel(errors.New("render request expired"))
	}
	_, err = tx.Exec(ctx, "UPDATE art_requests SET generation=$2,outcome='running',first_started=coalesce(first_started,now()) WHERE id=$1", job.Args.Request, generation)
	if err != nil {
		return r, epoch, err
	}
	if err = Notify(ctx, tx, job.Args.Request); err != nil {
		return r, epoch, err
	}
	if err = tx.Commit(ctx); err != nil {
		return r, epoch, err
	}
	r.Version = 1
	r.Build = build
	return r, epoch, nil
}

func (w *Worker) terminal(ctx context.Context, j *river.Job[RenderArgs], generation string, cause error) error {
	if err := river.MetadataSet(ctx, "art_terminal_outcome", "failed"); err != nil {
		return err
	}
	tx, err := w.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer Rollback(tx)
	_, err = tx.Exec(ctx, "UPDATE art_requests SET outcome='failed' WHERE id=$1 AND generation=$2", j.Args.Request, generation)
	if err != nil {
		return err
	}
	if err = Notify(ctx, tx, j.Args.Request); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return river.JobCancel(cause)
}

func (w *Worker) reserve(ctx context.Context, j *river.Job[RenderArgs], generation, key string, size int64) error {
	tx, err := w.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer Rollback(tx)
	if _, err = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); err != nil {
		return err
	}
	var valid bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM art_requests WHERE id=$1 AND generation=$2 AND job_id=$3)", j.Args.Request, generation, j.ID).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return river.JobCancel(errors.New("superseded upload"))
	}
	var used int64
	var count int
	if err = tx.QueryRow(ctx, "SELECT coalesce(sum(size),0),count(*) FROM art_uploads WHERE deleted_at IS NULL").Scan(&used, &count); err != nil {
		return err
	}
	// Includes retained favourites and pending/deleting bytes. Never evict a pinned favourite to accept new work.
	if used+size > 2<<30 || count >= 5000 {
		return errors.New("image storage capacity reached")
	}
	_, err = tx.Exec(ctx, "INSERT INTO art_uploads(object_key,request,generation,size,state) VALUES($1,$2,$3,$4,'pending')", key, j.Args.Request, generation, size)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (w *Worker) complete(ctx context.Context, j *river.Job[RenderArgs], generation string, epoch int64, key, digest string, size int64) error {
	tx, err := w.DB.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer Rollback(tx)
	if _, err = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); err != nil {
		return err
	}
	if err = Eligible(ctx, w.DB, tx, j); err != nil {
		return err
	}
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM art_requests r JOIN art_control c ON c.epoch=r.epoch AND c.build=r.build WHERE r.id=$1 AND r.job_id=$2 AND r.generation=$3 AND r.epoch=$4 AND EXISTS(SELECT 1 FROM art_interests i JOIN art_workspaces ws ON ws.id=i.workspace WHERE i.request=r.id AND ws.expires>now()))`, j.Args.Request, j.ID, generation, epoch).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return river.JobCancel(errors.New("publication no longer authorized"))
	}
	tag, err := tx.Exec(ctx, "UPDATE art_uploads SET state='published' WHERE object_key=$1 AND state='pending' AND generation=$2", key, generation)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("upload no longer publishable")
	}
	// Replacement never overwrites a winner's object; queue the previous immutable key for deletion.
	_, err = tx.Exec(ctx, `UPDATE art_uploads SET state='deleting',delete_after=now()+interval '1 hour' WHERE object_key IN(SELECT object_key FROM art_artifacts WHERE request=$1) AND object_key<>$2`, j.Args.Request, key)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO art_artifacts(request,object_key,digest,size,expires) VALUES($1,$2,$3,$4,now()+interval '24 hours') ON CONFLICT(request) DO UPDATE SET object_key=excluded.object_key,digest=excluded.digest,size=excluded.size,created=now(),expires=excluded.expires`, j.Args.Request, key, digest, size)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE art_requests SET outcome='ready' WHERE id=$1", j.Args.Request); err != nil {
		return err
	}
	completed, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, j)
	if err != nil {
		return err
	}
	if completed.State != rivertype.JobStateCompleted {
		return errors.New("river refused completion")
	}
	if err = Notify(ctx, tx, j.Args.Request); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// NewWorkerClient configures River's existing consumer and maintenance election.
func NewWorkerClient(d *DB, objects *objectstore.Store, renderer renderjob.Renderer) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &Worker{DB: d, Objects: objects, Renderer: renderer})
	river.AddWorker(workers, &Maintainer{DB: d, Objects: objects})
	client, err := river.NewClient(riverpgxv5.New(d.Pool), &river.Config{
		ID:      d.Instance.ID,
		Workers: workers, Queues: map[string]river.QueueConfig{Queue(d.Build): {MaxWorkers: 1}, "maintenance": {MaxWorkers: 1}},
		JobTimeout: 90 * time.Second, RescueStuckJobsAfter: 2 * time.Minute, MaxAttempts: 3, FetchPollInterval: 30 * time.Second,
		ReindexerIndexNames: []string{}, CompletedJobRetentionPeriod: 24 * time.Hour, CancelledJobRetentionPeriod: 24 * time.Hour, DiscardedJobRetentionPeriod: 7 * 24 * time.Hour,
		PeriodicJobs: []*river.PeriodicJob{river.NewPeriodicJob(river.PeriodicInterval(time.Minute), func() (river.JobArgs, *river.InsertOpts) {
			return MaintenanceArgs{}, &river.InsertOpts{Queue: "maintenance", MaxAttempts: 3, UniqueOpts: river.UniqueOpts{ByPeriod: time.Minute}}
		}, &river.PeriodicJobOpts{RunOnStart: true})},
	})
	return client, err
}
