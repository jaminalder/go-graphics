package drift

import (
	"flag"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("feeley-april-15")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, New(), testCtx(t, 42), testCtx(t, 43))
}

func TestPlacement(t *testing.T) {
	s := New()
	for seed := uint64(0); seed < 10; seed++ {
		dots := s.place(testCtx(t, seed), 1)
		if len(dots) < 150 {
			t.Fatalf("seed %d: only %d dots placed", seed, len(dots))
		}
		for i := range dots {
			a := &dots[i]
			if a.R < s.MinR-1e-12 || a.R > s.MaxR+1e-12 {
				t.Fatalf("seed %d: radius %v out of range", seed, a.R)
			}
			if a.X-a.R < 0 || a.X+a.R > 1 || a.Y-a.R < 0 || a.Y+a.R > 1 {
				t.Fatalf("seed %d: dot %d leaves the canvas", seed, i)
			}
			for j := i + 1; j < len(dots); j++ {
				b := &dots[j]
				dx, dy := a.X-b.X, a.Y-b.Y
				if math.Sqrt(dx*dx+dy*dy) < a.R+b.R+s.Gap-1e-9 {
					t.Fatalf("seed %d: dots %d and %d overlap", seed, i, j)
				}
			}
		}
	}
}

func TestForcedStyleDiffers(t *testing.T) {
	rings := New()
	rings.Style = StyleRings
	gouache := New()
	gouache.Style = StyleGouache
	a := sketchtest.RenderNRGBA(t, rings, testCtx(t, 42))
	b := sketchtest.RenderNRGBA(t, gouache, testCtx(t, 42))
	same := 0
	for i := range a.Pix {
		if a.Pix[i] == b.Pix[i] {
			same++
		}
	}
	if same == len(a.Pix) {
		t.Error("forcing different styles changed nothing")
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/drift_seed42_64.png", *update)
}
