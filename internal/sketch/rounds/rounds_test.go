package rounds

import (
	"bytes"
	"flag"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("munch-scream-oil")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, New(), testCtx(t, 42), testCtx(t, 43))
}

func TestHumanDialChangesOutput(t *testing.T) {
	machine := New()
	machine.Human = 0
	human := New()
	human.Human = 1
	a := sketchtest.RenderNRGBA(t, machine, testCtx(t, 42))
	b := sketchtest.RenderNRGBA(t, human, testCtx(t, 42))
	if bytes.Equal(a.Pix, b.Pix) {
		t.Error("human dial changed nothing")
	}
	// Machine mode is itself deterministic and distinct from default.
	a2 := sketchtest.RenderNRGBA(t, machine, testCtx(t, 42))
	if !bytes.Equal(a.Pix, a2.Pix) {
		t.Error("machine mode not deterministic")
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/rounds_seed42_64.png", *update)
}
