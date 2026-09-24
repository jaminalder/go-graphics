package persistence_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/jaminalder/go-graphics/internal/objectstore"
	"github.com/jaminalder/go-graphics/internal/persistence"
	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/studio"
	"github.com/jaminalder/go-graphics/internal/web"
)

func environment(t *testing.T) (*persistence.DB, *objectstore.Store) {
	t.Helper()
	url := os.Getenv("ART_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("run make test-persistence with disposable PostgreSQL and S3")
	}
	// This suite drops its schema: refuse anything except the dedicated disposable fixture.
	if url != "postgres://art:disposable-test-password@127.0.0.1:15439/art_test?sslmode=disable" {
		t.Fatal("refusing non-fixture database URL")
	}
	ctx := context.Background()
	db, err := persistence.Open(ctx, url, "integration")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if _, err = db.Pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckSchema(ctx); err != nil {
		t.Fatal(err)
	}
	objects, err := objectstore.New(objectstore.Config{Endpoint: "http://127.0.0.1:19009", Region: "us-east-1", Bucket: "art-test-" + studio.Token(), Prefix: "images", AccessKey: "disposable-test", SecretKey: "disposable-test-password", Local: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = objects.CreateBucket(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.BindEmptyBucket(ctx, objects); err != nil {
		t.Fatal(err)
	}
	if err = db.Activate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.Enable(ctx, true); err != nil {
		t.Fatal(err)
	}
	return db, objects
}

type actualRenderer struct{}

func (actualRenderer) Render(_ context.Context, q renderjob.Request, w io.Writer) error {
	r, err := publish.Validate(q.Recipe)
	if err != nil {
		return err
	}
	tier, err := publish.Tier(r.ID(), q.Tier)
	if err != nil {
		return err
	}
	return r.Render(w, tier, q.Build)
}

func await(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out")
}

// TestDurableStudioPublishesWithoutWebAndRetainsFavourite exercises actual SQL/S3 and artwork output.
func TestDurableStudioPublishesWithoutWebAndRetainsFavourite(t *testing.T) {
	db, objects := environment(t)
	ctx := context.Background()
	s := &studio.Persistent{DB: db}
	w, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	action := studio.Token()
	id, err := s.Enter(w.Token, "iris", "", "", action)
	if err != nil {
		t.Fatal(err)
	}
	if repeated, e := s.Enter(w.Token, "iris", "", "", action); e != nil || repeated != id {
		t.Fatalf("replay %s %v", repeated, e)
	}
	var count int
	if err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind='render'").Scan(&count); err != nil || count != 4 {
		t.Fatalf("job count %d %v", count, err)
	}
	client, err := persistence.NewWorkerClient(db, objects, actualRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = client.StopAndCancel(c)
	})
	var x studio.Exploration
	await(t, func() bool {
		x, err = s.Get(w.Token, id)
		if err != nil {
			t.Fatal(err)
		}
		return !x.Active
	})
	for _, sample := range x.Samples {
		if sample.Status.State != "ready" {
			t.Fatalf("not ready: %+v", sample.Status)
		}
		a, e := db.Artifact(ctx, sample.Job)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = objects.Get(ctx, a.Key, a.Digest, a.Size); e != nil {
			t.Fatal(e)
		}
	}
	if err = s.Favourite(w.Token, id, x.Samples[0].ID, x.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, "UPDATE art_artifacts SET expires=now()-interval '1 day'"); err != nil {
		t.Fatal(err)
	}
	// A new store instance knows the existing identity and pinned favourite, days later.
	s = &studio.Persistent{DB: db}
	favs, err := s.Favourites(w.Token)
	if err != nil || len(favs) != 1 || favs[0].Sample.Status.State != "ready" {
		t.Fatalf("favourites %+v %v", favs, err)
	}
	if _, err = db.Pool.Exec(ctx, "UPDATE art_workspaces SET expires=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if err = s.Touch(w.Token); !errors.Is(err, studio.ErrExpired) {
		t.Fatalf("revived expired session: %v", err)
	}
}

// TestConcurrentAdmissionRollsBackWholeBatch checks global admission and action replay races.
func TestConcurrentAdmissionRollsBackWholeBatch(t *testing.T) {
	db, _ := environment(t)
	s := &studio.Persistent{DB: db}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for range 3 {
		w, err := s.Create()
		if err != nil {
			t.Fatal(err)
		}
		wg.Go(func() { _, e := s.Enter(w.Token, "iris", "", "", studio.Token()); errs <- e })
	}
	wg.Wait()
	close(errs)
	success, busy := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, renderjob.ErrBusy):
			busy++
		default:
			t.Fatal(err)
		}
	}
	if success != 2 || busy != 1 {
		t.Fatalf("admission success=%d busy=%d", success, busy)
	}
	var jobs, interests int
	if err := db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM river_job WHERE kind='render'").Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM art_interests").Scan(&interests); err != nil {
		t.Fatal(err)
	}
	if jobs != 8 || interests != 8 {
		t.Fatalf("partial admission: %d jobs, %d interests", jobs, interests)
	}
}

type raceRenderer struct {
	start   chan struct{}
	release chan struct{}
}

func (r raceRenderer) Render(ctx context.Context, q renderjob.Request, w io.Writer) error {
	close(r.start)
	select {
	case <-r.release:
		return actualRenderer{}.Render(ctx, q, w)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TestRescuedAttemptCannotPublish tests the serializable River attempt guard, not a mock lease.
func TestRescuedAttemptCannotPublish(t *testing.T) {
	db, objects := environment(t)
	ctx := context.Background()
	s := &studio.Persistent{DB: db}
	w, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	r, err := publish.Complete("iris", 42, "diebenkorn-seawall", nil)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	q := &persistence.QueueTx{DB: db, Tx: tx, Context: ctx}
	ids, err := q.Admit(persistence.TokenHash(w.Token), []renderjob.Request{{Version: 1, Build: db.Build, Recipe: r.Bytes(), Tier: "preview"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	render := raceRenderer{make(chan struct{}), make(chan struct{})}
	client, err := persistence.NewWorkerClient(db, objects, render)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = client.StopAndCancel(c)
	})
	select {
	case <-render.start:
	case <-time.After(10 * time.Second):
		t.Fatal("worker not started")
	}
	// Simulate the persisted state transition produced by a rescue and a subsequent claim.
	if _, err = db.Pool.Exec(ctx, "UPDATE river_job SET attempt=attempt+1 WHERE id=(SELECT job_id FROM art_requests WHERE id=$1)", ids[0]); err != nil {
		t.Fatal(err)
	}
	close(render.release)
	await(t, func() bool {
		var n int
		_ = db.Pool.QueryRow(ctx, "SELECT count(*) FROM art_uploads").Scan(&n)
		return n > 0
	})
	time.Sleep(300 * time.Millisecond)
	var n int
	if err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM art_artifacts").Scan(&n); err != nil || n != 0 {
		t.Fatalf("stale artifact count=%d err=%v", n, err)
	}
}

// TestEnqueueRollbackAndSchemaMismatch verifies failure boundaries before runtime wiring.
func TestEnqueueRollbackAndSchemaMismatch(t *testing.T) {
	db, _ := environment(t)
	ctx := context.Background()
	tx, err := db.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.River.InsertTx(ctx, tx, persistence.RenderArgs{Request: "test", Build: db.Build}, &river.InsertOpts{Queue: persistence.Queue(db.Build)}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM river_job").Scan(&n); err != nil || n != 0 {
		t.Fatalf("rollback %d %v", n, err)
	}
	if _, err = db.Pool.Exec(ctx, "UPDATE art_schema SET checksum='wrong'"); err != nil {
		t.Fatal(err)
	}
	if err = db.CheckSchema(ctx); err == nil {
		t.Fatal("accepted altered migration")
	}
}

// TestListenersReconnectAndWebNeverExpiresOnDatabaseFailure protects notification and HTTP failure semantics.
func TestListenersReconnectAndWebNeverExpiresOnDatabaseFailure(t *testing.T) {
	db, objects := environment(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &studio.Persistent{DB: db}
	workspace, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Enter(workspace.Token, "iris", "", "", studio.Token()); err != nil {
		t.Fatal(err)
	}
	events := persistence.NewEvents()
	ch, unsubscribe := events.Subscribe()
	defer unsubscribe()
	done := make(chan struct{})
	go func() { defer close(done); events.Run(ctx, db) }()
	defer func() { cancel(); <-done }()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("missing subscribe snapshot")
	}
	if _, err = db.Pool.Exec(ctx, "SELECT pg_notify('art_results','test')"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("missing committed notification")
	}
	// Terminate only the LISTEN connection in this disposable database, not the pool.
	if _, err = db.Pool.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=current_database() AND query='LISTEN art_results' AND pid<>pg_backend_pid()"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not resynchronize")
	}
	h, err := web.New(web.Config{Origin: "http://example.test", Studio: s, Database: db, Objects: objects, Events: events})
	if err != nil {
		t.Fatal(err)
	}
	// The missing table models a stateful-storage error without killing an unrelated database.
	if _, err = db.Pool.Exec(ctx, "ALTER TABLE art_workspaces RENAME TO offline_workspaces"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "http://example.test/favourites", nil)
	request.AddCookie(&http.Cookie{Name: "art-studio", Value: workspace.Token})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != 503 {
		t.Fatalf("database failure returned %d", response.Code)
	}
	if len(response.Result().Cookies()) != 0 {
		t.Fatal("database error replaced visitor cookie")
	}
	response = httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://example.test/", nil))
	if response.Code != 200 {
		t.Fatal("gallery unavailable")
	}
}

// TestRiverLeaderHandoverRescuesAbandonedWork uses River's actual maintenance, not our own scheduler.
func TestRiverLeaderHandoverRescuesAbandonedWork(t *testing.T) {
	db, objects := environment(t)
	ctx := context.Background()
	a, err := persistence.NewWorkerClient(db, objects, actualRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := persistence.NewWorkerClient(db, objects, actualRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err = b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	stop := func(c *river.Client[pgx.Tx]) {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = c.StopAndCancel(closeCtx)
	}
	t.Cleanup(func() { stop(a); stop(b) })
	var leader string
	await(t, func() bool {
		return db.Pool.QueryRow(ctx, "SELECT leader_id FROM river_leader LIMIT 1").Scan(&leader) == nil
	})
	switch leader {
	case a.ID():
		stop(a)
	case b.ID():
		stop(b)
	default:
		t.Fatalf("unknown leader %s", leader)
	}
	await(t, func() bool {
		var next string
		return db.Pool.QueryRow(ctx, "SELECT leader_id FROM river_leader LIMIT 1").Scan(&next) == nil && next != leader
	})
	// Pause consumption, enqueue, then create a persisted abandoned execution as a killed worker leaves behind.
	if err = db.River.QueuePause(ctx, persistence.Queue(db.Build), nil); err != nil {
		t.Fatal(err)
	}
	s := &studio.Persistent{DB: db}
	ws, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Enter(ws.Token, "iris", "", "", studio.Token())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, `UPDATE river_job SET state='running',attempt=1,attempted_at=now()-interval '3 minutes' WHERE kind='render'`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, "UPDATE art_requests SET first_started=now(),outcome='running'"); err != nil {
		t.Fatal(err)
	}
	if err = db.River.QueueResume(ctx, persistence.Queue(db.Build), nil); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	await(t, func() bool {
		x, e := s.Get(ws.Token, id)
		if e != nil {
			t.Fatal(e)
		}
		for _, sample := range x.Samples {
			if sample.Status.State != "ready" {
				return false
			}
		}
		return true
	})
	t.Logf("River rescued pre-aged abandoned jobs after leader change in %s", time.Since(started))
}

// TestIdleRiverActivityIsMeasured records quiet-system SQL overhead rather than claiming zero polling.
func TestIdleRiverActivityIsMeasured(t *testing.T) {
	db, objects := environment(t)
	ctx := context.Background()
	client, err := persistence.NewWorkerClient(db, objects, actualRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = client.StopAndCancel(c)
	})
	if _, err = db.Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS pg_stat_statements"); err != nil {
		t.Fatal(err)
	}
	// Let startup migrations, periodic catch-up and first leadership election settle.
	time.Sleep(5 * time.Second)
	if _, err = db.Pool.Exec(ctx, "SELECT pg_stat_statements_reset()"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	time.Sleep(12 * time.Second)
	rows, err := db.Pool.Query(ctx, "SELECT left(query,100),calls FROM pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database()) ORDER BY calls DESC LIMIT 8")
	if err != nil {
		t.Fatal(err)
	}
	var calls int64
	for rows.Next() {
		var query string
		var count int64
		if err = rows.Scan(&query, &count); err != nil {
			t.Fatal(err)
		}
		calls += count
		t.Logf("idle query calls=%d: %s", count, query)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	var connections int
	if err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database()").Scan(&connections); err != nil {
		t.Fatal(err)
	}
	t.Logf("idle River top statements incl. measurement: %d calls / %.1fs, %d database connections; configured fetch fallback 30s", calls, time.Since(start).Seconds(), connections)
	if connections > 10 {
		t.Fatalf("unexpected connection growth: %d", connections)
	}
}

// TestClearAndMaintenanceKeepPinnedImages protects ownership and cleanup races.
func TestClearAndMaintenanceKeepPinnedImages(t *testing.T) {
	db, objects := environment(t)
	ctx := context.Background()
	s := &studio.Persistent{DB: db}
	ws, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Enter(ws.Token, "iris", "", "", studio.Token())
	if err != nil {
		t.Fatal(err)
	}
	client, err := persistence.NewWorkerClient(db, objects, actualRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = client.StopAndCancel(closeCtx)
	})
	var x studio.Exploration
	await(t, func() bool { x, err = s.Get(ws.Token, id); return err == nil && !x.Active })
	if err = s.Favourite(ws.Token, id, x.Samples[0].ID, x.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Pool.Exec(ctx, "UPDATE art_artifacts SET expires=now()-interval '1 day'"); err != nil {
		t.Fatal(err)
	}
	maint := &persistence.Maintainer{DB: db, Objects: objects}
	if err = maint.Work(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Artifact(ctx, x.Samples[0].Job); err != nil {
		t.Fatal("pinned image evicted", err)
	}
	if _, err = db.Artifact(ctx, x.Samples[1].Job); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unfavoured artifact remained: %v", err)
	}
	if err = s.Clear(ws.Token); err != nil {
		t.Fatal(err)
	}
	if err = maint.Work(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Session(ws.Token); !errors.Is(err, studio.ErrExpired) {
		t.Fatalf("clear retained session %v", err)
	}
	if _, err = db.Pool.Exec(ctx, "UPDATE art_uploads SET delete_after=now()-interval '1 minute' WHERE state='deleting'"); err != nil {
		t.Fatal(err)
	}
	if err = maint.Work(ctx, nil); err != nil {
		t.Fatal(err)
	}
	var bytes int64
	if err = db.Pool.QueryRow(ctx, "SELECT coalesce(sum(size),0) FROM art_uploads WHERE deleted_at IS NULL").Scan(&bytes); err != nil || bytes != 0 {
		t.Fatalf("leaked bytes %d %v", bytes, err)
	}
}

// TestSharedInterestCancellationAndReplayRace checks two independent web owners and concurrent retries.
func TestSharedInterestCancellationAndReplayRace(t *testing.T) {
	db, _ := environment(t)
	ctx := context.Background()
	s := &studio.Persistent{DB: db}
	a, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	r, err := publish.Complete("iris", 43, "diebenkorn-seawall", nil)
	if err != nil {
		t.Fatal(err)
	}
	request := renderjob.Request{Version: 1, Build: db.Build, Recipe: r.Bytes(), Tier: "preview"}
	var id string
	for _, owner := range []string{a.Token, b.Token} {
		tx, e := db.Pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		q := &persistence.QueueTx{DB: db, Tx: tx, Context: ctx}
		ids, e := q.Admit(persistence.TokenHash(owner), []renderjob.Request{request})
		if e != nil {
			t.Fatal(e)
		}
		if id != "" && id != ids[0] {
			t.Fatal("did not coalesce")
		}
		id = ids[0]
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
	}
	for i, owner := range []string{a.Token, b.Token} {
		tx, e := db.Pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(ctx, "SELECT singleton FROM art_control FOR UPDATE"); e != nil {
			t.Fatal(e)
		}
		q := &persistence.QueueTx{DB: db, Tx: tx, Context: ctx}
		q.Cancel(persistence.TokenHash(owner), id)
		if q.Err != nil {
			t.Fatal(q.Err)
		}
		if e = tx.Commit(ctx); e != nil {
			t.Fatal(e)
		}
		var state string
		if e = db.Pool.QueryRow(ctx, "SELECT state FROM river_job WHERE id=(SELECT job_id FROM art_requests WHERE id=$1)", id).Scan(&state); e != nil {
			t.Fatal(e)
		}
		if i == 0 && state != "available" {
			t.Fatal("cancelled remaining subscriber")
		}
		if i == 1 && state != "cancelled" {
			t.Fatal("last-interest cancellation missing")
		}
	}
	action := studio.Token()
	results := make(chan string, 2)
	failures := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { v, e := s.Enter(a.Token, "iris", "", "", action); results <- v; failures <- e })
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	first := <-results
	second := <-results
	if first != second {
		t.Fatal("concurrent replay created two explorations")
	}
	var count int
	if err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM river_job WHERE state='available' AND kind='render'").Scan(&count); err != nil || count != 4 {
		t.Fatalf("replayed jobs=%d err=%v", count, err)
	}
}
