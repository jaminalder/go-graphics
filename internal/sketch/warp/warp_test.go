package warp

import (
	"flag"
	"io"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var fixedSeeds = []uint64{1, 2, 3, 5, 8, 13, 21, 34}

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t testing.TB, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("kandinsky-soft-pressure")
	if !ok {
		t.Fatal("kandinsky-soft-pressure palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

// A non-finite field value would poison both domain displacement and colour.
func TestFieldSamplesStayFinite(t *testing.T) {
	for _, seed := range fixedSeeds {
		for _, fieldMode := range []mode{modePlain, modeSingle, modeNested} {
			s := New()
			s.fieldMode = fieldMode
			p, err := s.plan(testCtx(t, seed))
			if err != nil {
				t.Fatal(err)
			}
			for y := range 9 {
				for x := range 9 {
					got := p.sample(float64(x)/8, float64(y)/8)
					values := [...]float64{
						got.value, got.qx, got.qy, got.rx, got.ry, got.activity,
					}
					for _, value := range values {
						if math.IsNaN(value) || math.IsInf(value, 0) {
							t.Fatalf("seed %d mode %d at (%d,%d) produced %v", seed, fieldMode, x, y, got)
						}
					}
				}
			}
		}
	}
}

// Repeated coordinate queries must not consume state or mutate the plan.
func TestSamplingIsPure(t *testing.T) {
	p, err := New().plan(testCtx(t, 13))
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.5, 0.5}, {0.93, 0.81}} {
		first := p.sample(point[0], point[1])
		if second := p.sample(point[0], point[1]); second != first {
			t.Fatalf("sample at %v changed from %v to %v", point, first, second)
		}
	}
}

// The comparison modes must expose real field development, not aliases.
func TestModesAreDistinct(t *testing.T) {
	var plans [3]plan
	for fieldMode := modePlain; fieldMode <= modeNested; fieldMode++ {
		s := New()
		s.fieldMode = fieldMode
		p, err := s.plan(testCtx(t, 13))
		if err != nil {
			t.Fatal(err)
		}
		plans[fieldMode] = p
	}

	plainSingleDiffer := false
	singleNestedDiffer := false
	for y := range 9 {
		for x := range 9 {
			u, v := float64(x)/8, float64(y)/8
			plain := plans[modePlain].sample(u, v).value
			single := plans[modeSingle].sample(u, v).value
			nested := plans[modeNested].sample(u, v).value
			plainSingleDiffer = plainSingleDiffer || plain != single
			singleNestedDiffer = singleNestedDiffer || single != nested
		}
	}
	if !plainSingleDiffer || !singleNestedDiffer {
		t.Fatalf("mode differences: plain/single=%t single/nested=%t", plainSingleDiffer, singleNestedDiffer)
	}
}

func configured(t testing.TB, args ...string) *Sketch {
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

// A render is a deterministic function of its recipe and must vary by seed.
func TestRenderIsDeterministic(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 13), testCtx(t, 21))
}

// Pixel dimensions are output quality, not field configuration.
func TestPlanIgnoresPixelDimensions(t *testing.T) {
	smallCtx := testCtx(t, 13)
	smallCtx.Width, smallCtx.Height = 96, 64
	largeCtx := smallCtx
	largeCtx.Width, largeCtx.Height = 960, 640

	s := configured(t, "--appearance", "structure")
	small, err := s.plan(smallCtx)
	if err != nil {
		t.Fatal(err)
	}
	large, err := s.plan(largeCtx)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]float64{{0.1, 0.2}, {0.75, 0.5}, {1.4, 0.8}} {
		if a, b := small.sample(point[0], point[1]), large.sample(point[0], point[1]); a != b {
			t.Fatalf("field sample at %v changed from %v to %v", point, a, b)
		}
		if a, b := small.At(point[0], point[1]), large.At(point[0], point[1]); a != b {
			t.Fatalf("color at %v changed from %v to %v", point, a, b)
		}
	}
}

// Both mappings must stay inside finite sRGB for every field mode.
func TestBothAppearancesProduceFiniteColors(t *testing.T) {
	for _, appearance := range []string{"gradient", "structure"} {
		for _, fieldMode := range []string{"plain", "single", "nested"} {
			p, err := configured(t, "--appearance", appearance, "--warp", fieldMode).plan(testCtx(t, 13))
			if err != nil {
				t.Fatal(err)
			}
			for y := range 9 {
				for x := range 9 {
					color := p.At(float64(x)/8, float64(y)/8)
					for _, component := range [...]float64{color.R, color.G, color.B} {
						if math.IsNaN(component) || math.IsInf(component, 0) || component < 0 || component > 1 {
							t.Fatalf("%s/%s at (%d,%d) produced %v", appearance, fieldMode, x, y, color)
						}
					}
				}
			}
		}
	}
}

// Every public boundary is accepted and applies to the resolved sketch.
func TestOptionsAcceptBoundariesAndChoices(t *testing.T) {
	for _, args := range [][]string{
		{"--scale", "0.25"},
		{"--scale", "12"},
		{"--octaves", "1"},
		{"--octaves", "8"},
		{"--gain", "0.2"},
		{"--gain", "0.85"},
		{"--lacunarity", "1.2"},
		{"--lacunarity", "3.5"},
		{"--warp-strength", "0"},
		{"--warp-strength", "8"},
		{"--nested-strength", "0"},
		{"--nested-strength", "8"},
		{"--warp", "plain"},
		{"--warp", "single"},
		{"--warp", "nested"},
		{"--appearance", "gradient"},
		{"--appearance", "structure"},
	} {
		configured(t, args...)
	}
}

// Values outside documented ranges and unknown names must fail Configure.
func TestOptionsRejectInvalidValues(t *testing.T) {
	for _, args := range [][]string{
		{"--scale", "0.24"},
		{"--scale", "12.1"},
		{"--octaves", "0"},
		{"--octaves", "9"},
		{"--gain", "0.19"},
		{"--gain", "0.86"},
		{"--lacunarity", "1.1"},
		{"--lacunarity", "3.6"},
		{"--warp-strength", "-0.1"},
		{"--warp-strength", "8.1"},
		{"--nested-strength", "-0.1"},
		{"--nested-strength", "8.1"},
		{"--warp", "triple"},
		{"--appearance", "rainbow"},
	} {
		s := New()
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		s.Flags(fs)
		if err := fs.Parse(args); err != nil {
			continue
		}
		if _, err := s.Configure(); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
}

func TestRejectsTooSmallPalette(t *testing.T) {
	ctx := testCtx(t, 13)
	ctx.Palette = palette.Palette{Slug: "tiny", Colors: []palette.Color{{}, {}}}
	if _, err := configured(t).Render(ctx); err == nil {
		t.Error("expected error for palette with fewer than three colors")
	}
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, New(), testCtx(t, 13))
	sketchtest.Golden(t, got, "testdata/warp_seed13_64.png", *update)
}

var benchmarkSample fieldSample

func BenchmarkSample(b *testing.B) {
	p, err := configured(b).plan(testCtx(b, 13))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		benchmarkSample = p.sample(float64(i%97)/97, float64(i%89)/89)
	}
}
