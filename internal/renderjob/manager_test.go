package renderjob_test

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/trait"
)

type blocked struct {
	started chan struct{}
	once    sync.Once
}

func (b *blocked) Render(ctx context.Context, _ renderjob.Request, _ io.Writer) error {
	b.once.Do(func() { close(b.started) })
	<-ctx.Done()
	return ctx.Err()
}

// TestBatchAdmissionIsAtomicAndCancellationReleasesSlots defends global resource reservations.
func TestBatchAdmissionIsAtomicAndCancellationReleasesSlots(t *testing.T) {
	b := &blocked{started: make(chan struct{})}
	m, e := renderjob.New(renderjob.Config{Directory: t.TempDir(), Build: "test", Renderer: b, Queue: 4})
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	var qs []renderjob.Request
	for i := uint64(1); i <= 4; i++ {
		r, e := publish.Complete("iris", i, "diebenkorn-seawall", trait.Set{})
		if e != nil {
			t.Fatal(e)
		}
		qs = append(qs, renderjob.Request{Version: 1, Build: "test", Tier: "preview", Recipe: r.Bytes()})
	}
	ids, e := m.Admit("a", qs)
	if e != nil {
		t.Fatal(e)
	}
	<-b.started
	if _, e = m.Admit("b", append(qs, qs[0])); e == nil {
		t.Fatal("accepted oversized batch")
	}
	for _, id := range ids {
		m.Cancel("a", id)
	}
	time.Sleep(10 * time.Millisecond)
	if _, e = m.Admit("b", qs); e != nil {
		t.Fatal(e)
	}
}

type solid struct{}

func (solid) Render(_ context.Context, q renderjob.Request, w io.Writer) error {
	r, e := publish.Validate(q.Recipe)
	if e != nil {
		return e
	}
	tier, e := publish.Tier(r.ID(), q.Tier)
	if e != nil {
		return e
	}
	return png.Encode(w, image.NewRGBA(image.Rect(0, 0, tier.Width, tier.Height)))
}

func request(t *testing.T, seed uint64) renderjob.Request {
	t.Helper()
	r, e := publish.Complete("iris", seed, "diebenkorn-seawall", trait.Set{})
	if e != nil {
		t.Fatal(e)
	}
	return renderjob.Request{Version: 1, Build: "test", Tier: "preview", Recipe: r.Bytes()}
}

func ready(t *testing.T, m *renderjob.Manager, owner, id string) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if m.Status(owner, id).State == "ready" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal(m.Status(owner, id))
}

// TestRejectedMixedCachedBatchHasNoSideEffects defends atomic subscriber and job admission.
func TestRejectedMixedCachedBatchHasNoSideEffects(t *testing.T) {
	m, e := renderjob.New(renderjob.Config{Directory: t.TempDir(), Build: "test", Renderer: solid{}})
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	q := request(t, 42)
	ids, e := m.Admit("owner0", []renderjob.Request{q})
	if e != nil {
		t.Fatal(e)
	}
	ready(t, m, "owner0", ids[0])
	for i := 1; i < 24; i++ {
		if _, e := m.Admit(fmt.Sprint("owner", i), []renderjob.Request{q}); e != nil {
			t.Fatal(e)
		}
	}
	if m.Status("owner0", ids[0]).State != "ready" {
		t.Fatal("cache hit stole previous subscriber")
	}
	if _, e := m.Admit("overflow", []renderjob.Request{request(t, 43), q}); e == nil {
		t.Fatal("accepted excess subscribers")
	}
	queued, running, _, _ := m.Counts()
	if queued != 0 || running != 0 {
		t.Fatal("partial rejected batch was admitted")
	}
	m.Cancel("owner0", ids[0])
	if _, e := m.Admit("replacement", []renderjob.Request{q}); e != nil {
		t.Fatal("ready subscriber was not released", e)
	}
}

// TestJoiningSharedWorkCannotExceedWorkspaceCapacity closes a coalescing bypass.
func TestJoiningSharedWorkCannotExceedWorkspaceCapacity(t *testing.T) {
	b := &blocked{started: make(chan struct{})}
	m, e := renderjob.New(renderjob.Config{Directory: t.TempDir(), Build: "test", Renderer: b})
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	var qs []renderjob.Request
	for i := uint64(1); i < 5; i++ {
		qs = append(qs, request(t, i))
	}
	if _, e := m.Admit("a", qs); e != nil {
		t.Fatal(e)
	}
	<-b.started
	if _, e := m.Admit("b", []renderjob.Request{request(t, 5)}); e != nil {
		t.Fatal(e)
	}
	if _, e := m.Admit("a", []renderjob.Request{request(t, 5)}); e == nil {
		t.Fatal("joined fifth active job")
	}
}

// TestOpenDownloadPinsTheArtifactUntilClose protects physical cache accounting.
func TestOpenDownloadPinsTheArtifactUntilClose(t *testing.T) {
	m, e := renderjob.New(renderjob.Config{Directory: t.TempDir(), Build: "test", Renderer: solid{}, CacheCount: 1})
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	ids, e := m.Admit("a", []renderjob.Request{request(t, 42)})
	if e != nil {
		t.Fatal(e)
	}
	ready(t, m, "a", ids[0])
	lease, _, e := m.Open(ids[0])
	if e != nil {
		t.Fatal(e)
	}
	defer lease.Close()
	next, e := m.Admit("b", []renderjob.Request{request(t, 43)})
	if e != nil {
		t.Fatal(e)
	}
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if m.Status("b", next[0]).State == "failed" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if m.Status("b", next[0]).State != "failed" {
		t.Fatal("leased artifact was evicted")
	}
	if _, e := lease.Seek(0, 0); e != nil {
		t.Fatal(e)
	}
}

// TestIdleCacheExpiresWithoutAnotherAdmission keeps retention independent of traffic.
func TestIdleCacheExpiresWithoutAnotherAdmission(t *testing.T) {
	m, err := renderjob.New(renderjob.Config{Directory: t.TempDir(), Build: "test", Renderer: solid{}, MaxAge: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	ids, err := m.Admit("owner", []renderjob.Request{request(t, 42)})
	if err != nil {
		t.Fatal(err)
	}
	ready(t, m, "owner", ids[0])
	m.Enable(false)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, _, count, size := m.Counts()
		if count == 0 && size == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("expired artifact still occupies the idle cache")
}
