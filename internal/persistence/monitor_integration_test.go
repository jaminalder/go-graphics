package persistence_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/jaminalder/go-graphics/internal/limits"
	"github.com/jaminalder/go-graphics/internal/persistence"
	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/studio"
)

func TestMonitoringAttributesProducerRendererAndIdleBoots(t *testing.T) {
	db, objects := environment(t)
	ctx := context.Background()
	t.Setenv("ART_INSTANCE_NAME", "art-persistent-79509-web-1")
	if err := db.Identify("web"); err != nil {
		t.Fatal(err)
	}
	if err := db.Heartbeat(ctx, nil); err != nil {
		t.Fatal(err)
	}
	producer := db.Instance
	s := &studio.Persistent{DB: db}
	ws, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.Enter(ws.Token, "iris", "", "", studio.Token())
	if err != nil {
		t.Fatal(err)
	}
	snap, err := db.Monitor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Queue.Waiting != 4 || len(snap.Flow) != 1 || snap.Flow[0].Producer.ID != producer.ID || snap.Flow[0].Renderer.ID != "" {
		t.Fatalf("unexpected initial snapshot %+v", snap)
	}
	if _, err = s.Enter(ws.Token, "iris", "", "", studio.Token()); err == nil {
		t.Fatal("expected active batch rejection")
	}
	// Independent process/client on the same database, with the same River and heartbeat ID.
	renderDB := *db
	t.Setenv("ART_INSTANCE_NAME", "art-persistent-79509-renderer-1")
	if err = renderDB.Identify("renderer"); err != nil {
		t.Fatal(err)
	}
	ok := true
	if err = renderDB.Heartbeat(ctx, &ok); err != nil {
		t.Fatal(err)
	}
	client, err := persistence.NewWorkerClient(&renderDB, objects, actualRenderer{})
	if err != nil {
		t.Fatal(err)
	}
	if client.ID() != renderDB.Instance.ID {
		t.Fatal("River ID and heartbeat differ")
	}
	if err = client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = client.StopAndCancel(c)
	})
	await(t, func() bool { x, e := s.Get(ws.Token, id); return e == nil && !x.Active })
	snap, err = db.Monitor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Queue.Completed != 4 || snap.Queue.Waiting != 0 || len(snap.Flow) != 1 || snap.Flow[0].Renderer.ID != renderDB.Instance.ID || snap.Flow[0].Jobs != 4 {
		t.Fatalf("bad completed snapshot %+v", snap)
	}
	stats, e := db.LoadStats(ctx, time.Now().Add(-time.Minute), time.Now().Add(time.Second))
	if e != nil {
		t.Fatal(e)
	}
	if len(stats) != 1 || stats[0].Jobs != 4 || stats[0].State != "completed" || stats[0].TotalP95 == nil || stats[0].ExecutionSum == nil {
		t.Fatalf("bad server timing %+v", stats)
	}
	if _, e = db.LoadStats(ctx, time.Now(), time.Now().Add(-time.Second)); e == nil {
		t.Fatal("accepted reversed window")
	}
	if err = renderDB.StopInstance(ctx); err != nil {
		t.Fatal(err)
	}
	old := renderDB.Instance.ID
	if err = renderDB.Identify("renderer"); err != nil {
		t.Fatal(err)
	}
	if err = renderDB.Heartbeat(ctx, &ok); err != nil {
		t.Fatal(err)
	}
	snap, err = db.Monitor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var stopped, live bool
	for _, i := range snap.Instances {
		if i.ID == old {
			stopped = i.Status == "stopped" && i.Completed == 4
		}
		if i.ID == renderDB.Instance.ID {
			live = i.Status == "live" && i.Completed == 0
		}
	}
	if !stopped || !live {
		t.Fatalf("restart merged distinct boots %+v", snap.Instances)
	}
	if _, err = db.Pool.Exec(ctx, "UPDATE art_instances SET touched=now()-interval '50 seconds' WHERE id=$1", producer.ID); err != nil {
		t.Fatal(err)
	}
	snap, err = db.Monitor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range snap.Instances {
		if i.ID == producer.ID && i.Status != "stale" {
			t.Fatal("dead producer reported live")
		}
	}
}

func TestConfiguredCapacityChangesAdmissionWithoutRemovingVisitorBounds(t *testing.T) {
	for _, profile := range []string{"production", "local-capacity"} {
		t.Run(profile, func(t *testing.T) {
			db, _ := environment(t)
			db.Limits, _ = limits.Defaults(profile)
			s := &studio.Persistent{DB: db}
			for i := 0; i < db.Limits.Outstanding/4; i++ {
				ws, e := s.Create()
				if e != nil {
					t.Fatal(e)
				}
				if _, e = s.Enter(ws.Token, "iris", "", "", studio.Token()); e != nil {
					t.Fatalf("early rejection at %d: %v", i, e)
				}
			}
			ws, e := s.Create()
			if e != nil {
				t.Fatal(e)
			}
			if _, e = s.Enter(ws.Token, "iris", "", "", studio.Token()); e == nil {
				t.Fatal("outstanding cap bypassed")
			}
			var count int
			if e = db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM river_job WHERE kind='render'").Scan(&count); e != nil || count != db.Limits.Outstanding {
				t.Fatalf("jobs %d err %v", count, e)
			}
		})
	}
}

type failingMonitorRenderer struct{}

func (failingMonitorRenderer) Render(context.Context, renderjob.Request, io.Writer) error {
	return errors.New("controlled rendering failure")
}

func TestMonitorCountsActualDeterministicRenderFailures(t *testing.T) {
	db, objects := environment(t)
	ctx := context.Background()
	if err := db.Identify("web"); err != nil {
		t.Fatal(err)
	}
	s := &studio.Persistent{DB: db}
	ws, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Enter(ws.Token, "iris", "", "", studio.Token()); err != nil {
		t.Fatal(err)
	}
	renderDB := *db
	if err = renderDB.Identify("renderer"); err != nil {
		t.Fatal(err)
	}
	client, err := persistence.NewWorkerClient(&renderDB, objects, failingMonitorRenderer{})
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
	await(t, func() bool {
		snapshot, e := db.Monitor(ctx)
		if e != nil {
			t.Fatal(e)
		}
		return snapshot.Queue.Failed == 4
	})
	snapshot, err := db.Monitor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Flow) != 1 || snapshot.Flow[0].State != "failed" || snapshot.Flow[0].Jobs != 4 {
		t.Fatalf("terminal classification %+v", snapshot.Flow)
	}
}

func TestMonitorKeepsOldOutstandingJobsAndLabelsLegacyData(t *testing.T) {
	db, _ := environment(t)
	ctx := context.Background()
	s := &studio.Persistent{DB: db}
	ws, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Enter(ws.Token, "iris", "", "", studio.Token()); err != nil {
		t.Fatal(err)
	}
	_, err = db.Pool.Exec(ctx, "UPDATE river_job SET created_at=now()-interval '2 hours',metadata='{}' WHERE kind='render'")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := db.Monitor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Queue.Waiting != 4 || snap.Queue.OldestSeconds < 7200 || len(snap.Flow) != 1 || snap.Flow[0].Producer.ID != "" {
		t.Fatalf("legacy/outstanding missing %+v", snap)
	}
	// Deterministic render failures use River cancellation plus a durable application marker.
	_, err = db.Pool.Exec(ctx, "UPDATE river_job SET state='cancelled',finalized_at=now(),metadata=$1 WHERE kind='render'", json.RawMessage(`{"art_terminal_outcome":"failed"}`))
	if err != nil {
		t.Fatal(err)
	}
	snap, err = db.Monitor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Queue.Failed != 4 || snap.Flow[0].State != "failed" {
		t.Fatalf("terminal failures hidden %+v", snap)
	}
}

func TestMonitoringMigrationPreservesVersionOneState(t *testing.T) {
	db, _ := environment(t)
	ctx := context.Background()
	s := &studio.Persistent{DB: db}
	ws, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	// Return this disposable fixture to the previous committed application schema.
	_, err = db.Pool.Exec(ctx, "DROP TABLE art_instances; DROP INDEX art_render_jobs_recent; DELETE FROM art_schema WHERE version=2")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.CheckSchema(ctx); err == nil {
		t.Fatal("old schema accepted before migration")
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
	if _, err = s.Session(ws.Token); err != nil {
		t.Fatal("migration lost identity", err)
	}
	if _, err = db.Monitor(ctx); err != nil {
		t.Fatal(err)
	}
}
