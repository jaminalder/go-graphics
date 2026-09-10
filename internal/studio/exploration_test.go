package studio_test

import (
	"context"
	"image"
	"image/png"
	"io"
	"reflect"
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
