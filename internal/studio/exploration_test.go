package studio_test

import (
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jaminalder/go-graphics/internal/publish"
	"github.com/jaminalder/go-graphics/internal/renderjob"
	"github.com/jaminalder/go-graphics/internal/studio"
)

type blankRenderer struct{}

func (blankRenderer) Render(_ context.Context, q renderjob.Request, w io.Writer) error {
	r, err := publish.Validate(q.Recipe)
	if err != nil {
		return err
	}
	tier, err := publish.Tier(r.ID(), q.Tier)
	if err != nil {
		return err
	}
	return png.Encode(w, image.NewRGBA(image.Rect(0, 0, tier.Width, tier.Height)))
}

func workingStudio(t *testing.T) (*studio.Store, *renderjob.Manager, studio.Workspace) {
	t.Helper()
	jobs, err := renderjob.New(renderjob.Config{Directory: t.TempDir(), Build: "test", Renderer: blankRenderer{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jobs.Close)
	store := studio.New(jobs, "test")
	workspace, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	return store, jobs, workspace
}

func completed(t *testing.T, store *studio.Store, token, id string) studio.Exploration {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		x, err := store.Get(token, id)
		if err != nil {
			t.Fatal(err)
		}
		if !x.Active {
			return x
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("images did not complete")
	return studio.Exploration{}
}

// TestEnteringCreatesFourOnceAndKeepsFavouritesDuringRepeatedPlay defends the
// complete initial interaction, including rollback when admission is disabled.
func TestEnteringCreatesFourOnceAndKeepsFavouritesDuringRepeatedPlay(t *testing.T) {
	store, jobs, w := workingStudio(t)
	action := studio.Token()
	id, err := store.Enter(w.Token, "iris", "fine", "hopper-night-windows", action)
	if err != nil {
		t.Fatal(err)
	}
	x := completed(t, store, w.Token, id)
	if len(x.Samples) != 4 || len(x.Batches) != 1 {
		t.Fatal("enter did not admit four images")
	}
	if again, err := store.Enter(w.Token, "iris", "fine", "hopper-night-windows", action); err != nil || again != id {
		t.Fatal("repeated entry was not idempotent", err)
	}
	replayed, _ := store.Get(w.Token, id)
	if len(replayed.Batches) != 1 || len(replayed.Samples) != 4 {
		t.Fatal("entry replay admitted another batch")
	}
	if err := store.Favourite(w.Token, id, x.Samples[0].ID, x.Revision, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		again, err := store.Enter(w.Token, "iris", "winding", "cezanne-bathers", studio.Token())
		if err != nil || again != id {
			t.Fatal("repeated play failed", err)
		}
		completed(t, store, w.Token, id)
	}
	favourites, err := store.Favourites(w.Token)
	if err != nil || len(favourites) != 1 || favourites[0].Sample.ID != x.Samples[0].ID {
		t.Fatal("favourite disappeared", err)
	}
	jobs.Enable(false)
	if _, err := store.Enter(w.Token, "iris", "layered", "", studio.Token()); err == nil {
		t.Fatal("disabled rendering admitted work")
	}
	latest, err := store.Get(w.Token, id)
	if err != nil || latest.Style != "winding" || latest.Colour != "cezanne-bathers" {
		t.Fatal("failed entry changed the chosen direction")
	}
}

// TestSimilarityUsesOneUnfavouritedImageAndKeepsItsVisualFamily prevents an
// unrelated fourth image or a hidden favourite requirement in the detail action.
func TestSimilarityUsesOneUnfavouritedImageAndKeepsItsVisualFamily(t *testing.T) {
	store, _, w := workingStudio(t)
	id, err := store.Enter(w.Token, "iris", "layered", "tchelitchew-hide-and-seek", studio.Token())
	if err != nil {
		t.Fatal(err)
	}
	x := completed(t, store, w.Token, id)
	parent := x.Samples[0]
	action := studio.Token()
	batch, err := store.Generate(w.Token, id, x.Revision, action, []string{parent.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := store.Generate(w.Token, id, x.Revision, action, []string{parent.ID}, false); err != nil || again != batch {
		t.Fatal("similarity replay changed admission", err)
	}
	latest := completed(t, store, w.Token, id)
	if len(latest.Batches) != 2 || len(latest.Samples) != 8 || len(latest.Batches[1].Samples) != 4 {
		t.Fatal("similarity replay admitted another batch")
	}
	for _, sample := range latest.Samples[4:] {
		if sample.Recipe.Seed() == parent.Recipe.Seed() || sample.Recipe.Palette() != parent.Recipe.Palette() || !reflect.DeepEqual(sample.Recipe.Traits(), parent.Recipe.Traits()) {
			t.Fatal("similar image left the chosen visual family")
		}
	}
	if len(latest.Favourites) != 0 {
		t.Fatal("generating similar images implicitly favourited the parent")
	}
	if _, err := store.Generate(w.Token, id, latest.Revision, studio.Token(), []string{parent.ID, latest.Samples[1].ID}, false); err == nil {
		t.Fatal("accepted multiple parents")
	}
	if _, err := store.Generate(w.Token, id, latest.Revision, studio.Token(), []string{studio.Token()}, false); err == nil {
		t.Fatal("accepted an unowned parent")
	}
}

type failingRenderer struct{ fail atomic.Bool }

func (r *failingRenderer) Render(ctx context.Context, q renderjob.Request, w io.Writer) error {
	if r.fail.Load() {
		return errors.New("render failed")
	}
	return (blankRenderer{}).Render(ctx, q, w)
}

// TestRetryKeepsTheFailedSimilarityParent prevents a temporary renderer failure
// from silently replacing the chosen visual family with base-space samples.
func TestRetryKeepsTheFailedSimilarityParent(t *testing.T) {
	renderer := &failingRenderer{}
	jobs, err := renderjob.New(renderjob.Config{Directory: t.TempDir(), Build: "test", Renderer: renderer})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jobs.Close)
	store := studio.New(jobs, "test")
	workspace, _ := store.Create()
	id, err := store.Enter(workspace.Token, "iris", "layered", "tchelitchew-hide-and-seek", studio.Token())
	if err != nil {
		t.Fatal(err)
	}
	initial := completed(t, store, workspace.Token, id)
	parent := initial.Samples[0]
	renderer.fail.Store(true)
	failedID, err := store.Generate(workspace.Token, id, initial.Revision, studio.Token(), []string{parent.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	failed := completed(t, store, workspace.Token, id)
	for _, sample := range failed.Samples[4:] {
		if sample.Status.State != "failed" {
			t.Fatal("expected the renderer failure")
		}
	}
	renderer.fail.Store(false)
	action := studio.Token()
	if _, err := store.Retry("another workspace", id, failed.Revision, action, failedID); err == nil {
		t.Fatal("accepted an unowned retry")
	}
	if _, err := store.Retry(workspace.Token, id, failed.Revision, action, studio.Token()); err == nil {
		t.Fatal("accepted an unknown batch")
	}
	retryID, err := store.Retry(workspace.Token, id, failed.Revision, action, failedID)
	if err != nil {
		t.Fatal(err)
	}
	if replay, err := store.Retry(workspace.Token, id, failed.Revision, action, failedID); err != nil || replay != retryID {
		t.Fatal("retry was not idempotent", err)
	}
	retried := completed(t, store, workspace.Token, id)
	if len(retried.Batches) != 3 || len(retried.Samples) != 12 {
		t.Fatal("retry admitted more than four images")
	}
	if retried.Batches[2].Parent != parent.ID {
		t.Fatal("retry forgot the parent")
	}
	for _, sample := range retried.Samples[8:] {
		if sample.Status.State != "ready" || sample.Recipe.Palette() != parent.Recipe.Palette() || !reflect.DeepEqual(sample.Recipe.Traits(), parent.Recipe.Traits()) {
			t.Fatal("retry left the parent's visual family")
		}
	}
	if _, err := store.Retry(workspace.Token, id, retried.Revision, studio.Token(), retryID); err == nil {
		t.Fatal("ready batch was offered as a failure retry")
	}
}
