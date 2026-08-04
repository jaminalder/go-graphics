package shallows

import (
	"bytes"
	"flag"
	"io"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/riffle"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("kandinsky-soft-pressure")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal}
}

func configured(t *testing.T, args ...string) *Sketch {
	t.Helper()
	s := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Configure(); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 12), testCtx(t, 13))
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t), testCtx(t, 12))
	sketchtest.Golden(t, got, "testdata/shallows_seed12_96.png", *update)
}

func TestSurfaceChangesTheBedInsideOneRender(t *testing.T) {
	ctx := testCtx(t, 12)
	active := sketchtest.RenderNRGBA(t, configured(t), ctx)
	still := sketchtest.RenderNRGBA(t, configured(t,
		"--ripple-strength", "0",
		"--ripple-shadow", "0",
		"--ripple-light", "0",
		"--refraction", "0",
		"--dapple-shadow", "0",
	), ctx)
	if bytes.Equal(active.Pix, still.Pix) {
		t.Fatal("surface controls did not change the integrated render")
	}

	changed := 0
	for i := 0; i < len(active.Pix); i += 4 {
		if active.Pix[i] != still.Pix[i] || active.Pix[i+1] != still.Pix[i+1] || active.Pix[i+2] != still.Pix[i+2] {
			changed++
		}
	}
	if share := float64(changed) / float64(len(active.Pix)/4); share < 0.55 {
		t.Fatalf("surface changed %.0f%% of pixels, want a clearly legible field", 100*share)
	}
}

func TestWaterSeedDoesNotChangeTheBed(t *testing.T) {
	s := configured(t)
	ctx := testCtx(t, 12)
	bedA, err := s.bed.NewBed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bedB, err := s.bed.NewBed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waterA, err := riffle.NewSurface(ctx, riffle.DefaultSurfaceConfig(41))
	if err != nil {
		t.Fatal(err)
	}
	waterB, err := riffle.NewSurface(ctx, riffle.DefaultSurfaceConfig(42))
	if err != nil {
		t.Fatal(err)
	}

	waterChanged := false
	for _, point := range [][2]float64{{0.1, 0.2}, {0.5, 0.5}, {0.9, 0.8}} {
		if a, b := bedA.At(point[0], point[1]), bedB.At(point[0], point[1]); a != b {
			t.Fatalf("bed changed at %v while only constructing another water seed", point)
		}
		if waterA.At(point[0], point[1]) != waterB.At(point[0], point[1]) {
			waterChanged = true
		}
	}
	if !waterChanged {
		t.Fatal("different water seeds produced identical sampled surfaces")
	}
}

func TestBedSchemaIsPreserved(t *testing.T) {
	if err := configured(t).Schema().Validate(); err != nil {
		t.Fatal(err)
	}
}
