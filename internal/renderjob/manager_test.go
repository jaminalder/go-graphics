package renderjob_test

import (
	"context"
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
