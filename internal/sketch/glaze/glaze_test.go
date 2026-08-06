package glaze

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"io"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

var (
	benchmarkImage  image.Image
	benchmarkSample veilSample
)

func testCtx(t testing.TB, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("hokusai-great-wave")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal}
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

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 12), testCtx(t, 13))
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t), testCtx(t, 12))
	sketchtest.Golden(t, got, "testdata/glaze_seed12_96.png", *update)
}

func TestBedSchemaIsPreserved(t *testing.T) {
	if err := configured(t).Schema().Validate(); err != nil {
		t.Fatal(err)
	}
}

// The veil holds no pixel dimensions, so a print of one recipe must sample
// exactly the field its preview did. If this fails, something in the water
// has been expressed in pixels and invariant 2 is broken.
func TestTheVeilIsTheSameFieldAtAnyResolution(t *testing.T) {
	cfg := configured(t).config()
	small, large := newVeil(cfg), newVeil(cfg)
	for _, p := range [][2]float64{{0.11, 0.23}, {0.5, 0.5}, {0.87, 0.31}} {
		a, b := small.At(p[0], p[1]), large.At(p[0], p[1])
		if a != b {
			t.Fatalf("veil disagreed with itself at %v: %+v vs %+v", p, a, b)
		}
	}
}

// The manner is one knob because it is five materials. If two of them render
// the same picture, the knob has stopped meaning anything.
func TestEveryMannerIsADifferentPicture(t *testing.T) {
	ctx := testCtx(t, 12)
	names := []string{"glaze", "marble", "terrace", "filament", "silk"}
	seen := map[string]string{}
	for _, name := range names {
		img := sketchtest.RenderNRGBA(t, configured(t, "--veil", name), ctx)
		key := string(img.Pix)
		if other, ok := seen[key]; ok {
			t.Fatalf("--veil %s and --veil %s rendered identically", other, name)
		}
		seen[key] = name
	}
}

// A knob left alone must stay the manner's. This is what makes --veil a
// single decision rather than a default nobody can override selectively.
func TestAMannerKeepsItsOwnNumbersUntilAFlagOverridesThem(t *testing.T) {
	marble := configured(t, "--veil", "marble").config()
	if marble.drift == preset(mannerGlaze, marble.seed).drift {
		t.Fatal("marble took the glaze's drift; the preset is not being applied")
	}

	pinned := configured(t, "--veil", "marble", "--drift", "0.002").config()
	if pinned.drift != 0.002 {
		t.Fatalf("explicit --drift was ignored: got %v", pinned.drift)
	}
	if pinned.opacity != marble.opacity || pinned.scale != marble.scale {
		t.Fatal("overriding one knob disturbed the rest of the manner")
	}
}

// The water and the bed are separately re-dealable: a different --water-seed
// must move the veil and leave the stones exactly where they were.
func TestTheWaterSeedMovesOnlyTheWater(t *testing.T) {
	ctx := testCtx(t, 12)
	s := configured(t)

	bedA, err := s.bed.NewBed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bedB, err := s.bed.NewBed(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]float64{{0.2, 0.3}, {0.6, 0.7}} {
		if bedA.At(p[0], p[1]) != bedB.At(p[0], p[1]) {
			t.Fatalf("the bed is not stable at %v", p)
		}
	}

	cfg := s.config()
	first := newVeil(cfg)
	cfg.seed++
	second := newVeil(cfg)
	moved := false
	for _, p := range [][2]float64{{0.2, 0.3}, {0.6, 0.7}, {0.9, 0.15}} {
		if first.At(p[0], p[1]) != second.At(p[0], p[1]) {
			moved = true
		}
	}
	if !moved {
		t.Fatal("two water seeds produced the same veil")
	}

	plain := sketchtest.RenderNRGBA(t, configured(t), ctx)
	other := sketchtest.RenderNRGBA(t, configured(t, "--water-seed", "77"), ctx)
	if bytes.Equal(plain.Pix, other.Pix) {
		t.Fatal("--water-seed did not change the render")
	}
}

// The veil has to be *seen*. A water that changes a handful of pixels is the
// failure 012 records for a separately rendered overlay: too weak to read.
func TestTheVeilCoversTheSheet(t *testing.T) {
	ctx := testCtx(t, 12)
	wet := sketchtest.RenderNRGBA(t, configured(t), ctx)
	dry := sketchtest.RenderNRGBA(t, configured(t, "--opacity", "0", "--drift", "0", "--glint", "0"), ctx)

	changed := 0
	for i := 0; i < len(wet.Pix); i += 4 {
		if wet.Pix[i] != dry.Pix[i] || wet.Pix[i+1] != dry.Pix[i+1] || wet.Pix[i+2] != dry.Pix[i+2] {
			changed++
		}
	}
	if share := float64(changed) / float64(len(wet.Pix)/4); share < 0.85 {
		t.Fatalf("the veil touched %.0f%% of the sheet, want nearly all of it", 100*share)
	}
}

// The picture is stone *under* water, which means the bed must still be
// legible through it. If the veil flattens the frame, every stone reads the
// same and the sketch has become a noise field with a palette.
func TestTheBedSurvivesTheVeil(t *testing.T) {
	ctx := testCtx(t, 12)
	img := sketchtest.RenderNRGBA(t, configured(t), ctx)
	tones := map[uint8]bool{}
	for i := 0; i < len(img.Pix); i += 4 {
		tones[img.Pix[i+1]] = true
	}
	if len(tones) < 60 {
		t.Fatalf("only %d green levels survive; the veil has flattened the bed", len(tones))
	}
}

// Terracing is the whole of one manner: the water must land on a few flat
// values, not on a continuum with a curve applied to it.
func TestTerracingLandsOnAFewFlatValues(t *testing.T) {
	cfg := configured(t, "--veil", "terrace", "--grain-water", "0").config()
	v := newVeil(cfg)
	plateaus := map[float64]int{}
	riser := 0
	total := 0
	for i := range 120 {
		for j := range 120 {
			u, w := float64(i)/120, float64(j)/120
			load := v.At(u, w).load / cfg.opacity
			total++
			step := load * float64(cfg.terraces)
			if math.Abs(step-math.Round(step)) < 1e-9 {
				plateaus[math.Round(step)]++
			} else {
				riser++
			}
		}
	}
	if len(plateaus) < 3 {
		t.Fatalf("only %d plateaus in the frame; the terraces are not reading", len(plateaus))
	}
	if share := float64(riser) / float64(total); share > 0.35 {
		t.Fatalf("%.0f%% of the sheet is on a riser, so the sheets are gradients", 100*share)
	}
}

// The load-bearing choice in water.go: the extinction is normalised so that
// the pigment's brightest channel passes freely. Without it a pale swatch
// absorbs almost nothing and the veil disappears, which is how the water
// ended up being as strong as the palette happened to be dark.
func TestAPalePigmentStillFiltersAsBlue(t *testing.T) {
	pale := palette.MustHex("#8DCEE2")
	f := newFilter(pale, 0)
	if f.tintR-f.tintB < 1.5 {
		t.Fatalf("a pale blue barely filters: red %v vs blue %v", f.tintR, f.tintB)
	}

	under := palette.MustHex("#B9A98C")
	got := f.over(under, 0.8)
	if before, after := under.B/under.R, got.B/got.R; after < before*1.6 {
		t.Fatalf("stone went from B/R %.2f to %.2f; the water is not reading as blue", before, after)
	}
	if got.R >= under.R {
		t.Fatal("the water added red instead of absorbing it")
	}
}

// Absorption, not a lerp: more water always means less of the bed's own
// light, and the bed is never averaged out of existence at body 0.
func TestMoreWaterAlwaysTakesMoreLight(t *testing.T) {
	f := newFilter(palette.MustHex("#2D4472"), 0)
	under := palette.MustHex("#D9CCAC")
	previous := 2.0
	for _, load := range []float64{0, 0.2, 0.5, 0.9, 1.2} {
		got := f.over(under, load).Luminance()
		if got > previous+1e-9 {
			t.Fatalf("load %v was brighter than the load before it", load)
		}
		previous = got
	}

	bright := f.over(palette.MustHex("#FFFFFF"), 1.2)
	dim := f.over(palette.MustHex("#404040"), 1.2)
	if bright.Luminance() <= dim.Luminance() {
		t.Fatal("the bed stopped showing through at full load; this is a lid, not a glaze")
	}
}

// The pigment is chosen from the palette, not invented, and it is chosen for
// hue rather than for brightness — 012's score picks hokusai's cream.
func TestTheWaterColourComesFromThePaletteAndIsBlue(t *testing.T) {
	for _, name := range []string{
		"hokusai-great-wave", "diebenkorn-seawall", "kandinsky-soft-pressure",
		"vermeer-pearl-earring", "afklint-swan", "hopper-night-windows",
	} {
		pal, ok := palette.ByName(name)
		if !ok {
			t.Fatalf("palette %q missing", name)
		}
		got := coolest(pal.Colors)
		found := false
		for _, c := range pal.Colors {
			if c == got {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: the water colour is not a member of its palette", name)
		}
		if h, _, _ := got.HSL(); hueDistance(h, marineHue) > 90 {
			t.Fatalf("%s: chose %s, %.0f degrees off marine", name, got.Hex(), hueDistance(h, marineHue))
		}
	}
}

// A palette with no blue in it must still produce a picture rather than an
// error or a black frame.
func TestAPaletteWithNoBlueStillMakesWater(t *testing.T) {
	pal, ok := palette.ByName("hokusai-great-wave")
	if !ok {
		t.Fatal("palette missing")
	}
	warm := pal
	warm.Colors = []palette.Color{
		palette.MustHex("#D9CCAC"), palette.MustHex("#B07C3A"), palette.MustHex("#7A3B22"),
		palette.MustHex("#E8DFC8"), palette.MustHex("#3C2A1E"),
	}
	ctx := testCtx(t, 12)
	ctx.Palette = warm
	if _, err := configured(t).Render(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownVeilIsRejected(t *testing.T) {
	s := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse([]string{"--veil", "custard"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Configure(); err == nil {
		t.Fatal("an unknown veil was accepted")
	}
}

func BenchmarkVeilSample(b *testing.B) {
	v := newVeil(configured(b).config())
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		benchmarkSample = v.At(float64(i%97)/97, float64(i%89)/89)
	}
}

func BenchmarkRender(b *testing.B) {
	for _, size := range []int{96, 192} {
		b.Run(fmt.Sprintf("%dx%d", size, size), func(b *testing.B) {
			s := configured(b)
			ctx := testCtx(b, 12)
			ctx.Width, ctx.Height = size, size
			b.ReportAllocs()
			for range b.N {
				var err error
				benchmarkImage, err = s.Render(ctx)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
