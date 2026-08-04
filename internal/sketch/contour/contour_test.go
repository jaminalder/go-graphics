package contour

import (
	"flag"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("staticart-seven")
	if !ok {
		t.Fatal("staticart-seven palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, New(), testCtx(t, 42), testCtx(t, 43))
}

func TestRejectsTooSmallPalette(t *testing.T) {
	ctx := testCtx(t, 1)
	ctx.Palette = palette.Palette{Slug: "tiny", Colors: []palette.Color{{}, {}}}
	if _, err := New().Render(ctx); err == nil {
		t.Error("expected error for palette with < 3 colors")
	}
}

func TestSamplerIsPure(t *testing.T) {
	p, err := New().plan(testCtx(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.5, 0.5}, {0.9, 0.8}} {
		a := p.At(point[0], point[1])
		b := p.At(point[0], point[1])
		if a != b {
			t.Fatalf("sample at %v changed from %v to %v", point, a, b)
		}
	}
}

func TestPlanIsResolutionIndependent(t *testing.T) {
	s := New()
	small := testCtx(t, 42)
	small.Width, small.Height = 96, 64
	large := small
	large.Width, large.Height = 960, 640

	a, err := s.plan(small)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.plan(large)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.75, 0.5}, {1.4, 0.8}} {
		if ca, cb := a.At(point[0], point[1]), b.At(point[0], point[1]); ca != cb {
			t.Fatalf("sample at %v changed from %v to %v with pixel dimensions", point, ca, cb)
		}
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/contour_seed42_64.png", *update)
}
