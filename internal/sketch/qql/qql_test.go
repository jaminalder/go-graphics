package qql

import (
	"bytes"
	"flag"
	"io"
	"math"
	"strings"
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
	return sketch.Context{Width: 64, Height: 80, Seed: seed, Palette: pal}
}

// configured returns a sketch with its options resolved, optionally pinning
// trait flags the way the CLI would.
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
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 42), testCtx(t, 43))
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/qql_seed42_64.png", *update)
}

func TestPlanIsResolutionIndependent(t *testing.T) {
	s := configured(t)
	small := testCtx(t, 42)
	large := small
	large.Width, large.Height = 640, 800

	a, err := s.plan(small)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.plan(large)
	if err != nil {
		t.Fatal(err)
	}
	if a.frame != b.frame || a.stack != b.stack || a.scheme.Background != b.scheme.Background {
		t.Fatal("equal-aspect canvases resolved different plan-level values")
	}
	if schema.Format(a.traits) != schema.Format(b.traits) {
		t.Fatalf("traits differ: %s vs %s", schema.Format(a.traits), schema.Format(b.traits))
	}
	if len(a.dots) != len(b.dots) {
		t.Fatalf("planned %d dots at preview and %d at print", len(a.dots), len(b.dots))
	}
	for i := range a.dots {
		if a.dots[i] != b.dots[i] {
			t.Fatalf("dot %d differs between equal-aspect sizes", i)
		}
	}
}

func TestPaintingAPlanIsDeterministic(t *testing.T) {
	s := configured(t)
	ctx := testCtx(t, 42)
	p, err := s.plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	a := s.paint(ctx, p)
	b := s.paint(ctx, p)
	if !bytes.Equal(sketchtest.ToNRGBA(a).Pix, sketchtest.ToNRGBA(b).Pix) {
		t.Fatal("painting the same plan twice changed pixels")
	}
}

func TestSchemaIsValid(t *testing.T) {
	if err := schema.Validate(); err != nil {
		t.Fatal(err)
	}
	// QQL's own twelve dimensions come first and in the source's order;
	// anything after them is this project's, not the original's.
	qqlDims := []string{
		dimFlowField, dimTurbulence, dimMargin, dimColorVariety, dimColorMode,
		dimStructure, dimBullseye, dimRingThickness, dimRingSize, dimSizeVariety,
		dimPalette, dimSpacing,
	}
	if len(schema) < len(qqlDims) {
		t.Fatalf("schema has %d dimensions, want at least QQL's %d", len(schema), len(qqlDims))
	}
	for i, want := range qqlDims {
		if schema[i].Name != want {
			t.Errorf("dimension %d is %q, want %q — QQL's space must keep its order, "+
				"or every existing seed moves to a different piece", i, schema[i].Name, want)
		}
	}
	// Ours are appendices to that space, not part of it, so no seed may
	// ever land on one: every value they add carries weight 0.
	for _, d := range schema[len(qqlDims):] {
		for _, v := range d.Values[1:] {
			if v.Weight != 0 {
				t.Errorf("dimension %q value %q has weight %v — a seed can land on it, "+
					"which changes what QQL's output space is", d.Name, v.Name, v.Weight)
			}
		}
	}
	// The flat CLI namespace is shared with the generic render flags.
	reserved := map[string]bool{
		"profile": true, "width": true, "height": true, "seed": true,
		"aa": true, "deep": true, "palette": true, "format": true, "out": true,
	}
	for _, d := range schema {
		if reserved[d.Name] {
			t.Errorf("trait %q collides with a generic render flag", d.Name)
		}
	}
}

func TestBullseyeRings(t *testing.T) {
	tests := []struct {
		value string
		want  []int
	}{
		{"none", []int{2}}, // no selection falls back to two rings
		{"1", []int{1}},
		{"3", []int{3}},
		{"7", []int{7}},
		{"1-3", []int{1, 3}},
		{"1-7", []int{1, 7}},
		{"3-7", []int{3, 7}},
		{"1-3-7", []int{1, 3, 7}},
	}
	for _, tc := range tests {
		t.Run(tc.value, func(t *testing.T) {
			got := bullseyeRings(tc.value)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// The plan is the composition; these bounds are the contract it owes the
// painter, checked across a spread of seeds rather than one.
func TestPlanBoundsAcrossSeeds(t *testing.T) {
	const seeds = 16
	pal, _ := palette.ByName("munch-scream-oil")

	for seed := uint64(0); seed < seeds; seed++ {
		s := configured(t)
		ctx := sketch.Context{Width: 64, Height: 80, Seed: seed, Palette: pal}
		p, err := s.plan(ctx)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if len(p.dots) == 0 {
			t.Errorf("seed %d produced no marks at all (%s)", seed, schema.Format(p.traits))
			continue
		}

		f := p.frame
		margins := newMarginChecker(p.traits, f)
		for i, d := range p.dots {
			if math.IsNaN(d.x) || math.IsNaN(d.y) || math.IsNaN(d.scale) {
				t.Fatalf("seed %d dot %d: NaN in %+v", seed, i, d)
			}
			if d.scale <= 0 {
				t.Fatalf("seed %d dot %d: non-positive scale %v", seed, i, d.scale)
			}
			// Every dot was placed through the margin test, at a spacing at
			// least three quarters of its own radius.
			if !margins.inBounds(d.x, d.y, d.scale*0.75) {
				t.Fatalf("seed %d dot %d at (%v,%v) r=%v breaks the margin",
					seed, i, d.x, d.y, d.scale)
			}
			if d.rings < 1 {
				t.Fatalf("seed %d dot %d: %d rings", seed, i, d.rings)
			}
			if d.density < 0.17 || d.density > 0.93 {
				t.Fatalf("seed %d dot %d: density %v outside [0.17, 0.93]", seed, i, d.density)
			}
			if n := d.drawnRings(f); n < 1 || n > d.rings {
				t.Fatalf("seed %d dot %d: %d drawn rings of %d natural", seed, i, n, d.rings)
			}
			for _, c := range []palette.HSB{d.primary, d.secondary} {
				if c.S < 0 || c.S > 100 || c.B < 0 || c.B > 100 || c.H < 0 || c.H >= 360 {
					t.Fatalf("seed %d dot %d: colour %+v out of range", seed, i, c)
				}
			}
		}
	}
}

// Colours must stay inside the clamp box of the swatch they came from, no
// matter how long the walk runs — that box is what keeps a palette on key.
func TestColoursStayInsideTheirSwatch(t *testing.T) {
	for _, name := range []string{"austin", "berlin", "edinburgh", "fidenza", "miami", "seattle", "seoul"} {
		set, ok := nativeColorSet(name)
		if !ok {
			t.Fatalf("palette %q missing", name)
		}
		if len(set.Seq) == 0 || len(set.Backgrounds) == 0 || len(set.Splatter) == 0 {
			t.Errorf("palette %q is incomplete: %d seq, %d backgrounds, %d splatter",
				name, len(set.Seq), len(set.Backgrounds), len(set.Splatter))
		}
		for _, k := range set.Seq {
			sw := set.swatch(k)
			if sw.Base.H < sw.HMin || sw.Base.H > sw.HMax {
				t.Errorf("%s: swatch %q base hue %v outside [%v,%v]", name, sw.Name, sw.Base.H, sw.HMin, sw.HMax)
			}
			if sw.HStd < 0 || sw.SStd < 0 || sw.BStd < 0 {
				t.Errorf("%s: swatch %q has a negative spread", name, sw.Name)
			}
			// A few colours pin a channel outright — the yellows and
			// oranges hold their hue — but none is frozen on every axis.
			if sw.HStd == 0 && sw.SStd == 0 && sw.BStd == 0 {
				t.Errorf("%s: swatch %q cannot drift at all", name, sw.Name)
			}
		}
	}
}

// Every background's substitutions must leave a usable sequence behind.
func TestBackgroundSubstitutionsLeaveColours(t *testing.T) {
	for _, name := range []string{"austin", "berlin", "edinburgh", "fidenza", "miami", "seattle", "seoul"} {
		set, _ := nativeColorSet(name)
		for _, bg := range set.Backgrounds {
			left := 0
			for _, k := range set.Seq {
				if _, keep := bg.substitute(k); keep {
					left++
				}
			}
			if left == 0 {
				t.Errorf("%s: background %q erases the whole sequence",
					name, set.swatch(bg.Color).Name)
			}
		}
	}
}

func TestColorLisaAdapter(t *testing.T) {
	for _, slug := range palette.Names() {
		p, _ := palette.ByName(slug)
		set, err := colorLisaSet(p)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if len(set.Seq) < 8 {
			t.Errorf("%s: sequence of %d is too short for the high colour-variety traits", slug, len(set.Seq))
		}
		if len(set.Backgrounds) != len(p.Colors) {
			t.Errorf("%s: %d backgrounds for %d colours", slug, len(set.Backgrounds), len(p.Colors))
		}
		for _, bg := range set.Backgrounds {
			if bg.Weight <= 0 {
				t.Errorf("%s: background with weight %v is unreachable", slug, bg.Weight)
			}
			left := 0
			for _, k := range set.Seq {
				if _, keep := bg.substitute(k); keep {
					left++
				}
			}
			if left < len(set.Seq)/2 {
				t.Errorf("%s: a background drops %d of %d sequence colours",
					slug, len(set.Seq)-left, len(set.Seq))
			}
		}
	}

	if _, err := colorLisaSet(palette.Palette{Slug: "tiny", Colors: []palette.Color{{}, {}}}); err == nil {
		t.Error("a two-colour palette should be rejected")
	}
}

// The external palette is reachable only on request: no seed may land on it.
func TestExternalPaletteIsOverrideOnly(t *testing.T) {
	s := configured(t)
	for seed := uint64(0); seed < 3000; seed++ {
		set := s.Traits(sketch.Context{Width: 1, Height: 1, Seed: seed})
		if set.Is(dimPalette, paletteExternal) {
			t.Fatalf("seed %d drew the override-only palette", seed)
		}
	}
	pinned := configured(t, "--qql-palette", paletteExternal)
	if !pinned.Traits(testCtx(t, 1)).Is(dimPalette, paletteExternal) {
		t.Error("pinning the external palette had no effect")
	}
}

func TestTraitSuffixNamesThePalette(t *testing.T) {
	s := configured(t)
	set := s.Traits(testCtx(t, 42))
	suffix := s.TraitSuffix(set)
	if !strings.HasPrefix(suffix, "-p-") {
		t.Errorf("suffix %q should name the palette even when nothing is pinned", suffix)
	}

	pinned := configured(t, "--structure", "shadows")
	got := pinned.TraitSuffix(pinned.Traits(testCtx(t, 42)))
	if !strings.Contains(got, "-str-shadows") {
		t.Errorf("suffix %q should record the pinned structure", got)
	}
}

func TestRejectsDegenerateCanvas(t *testing.T) {
	s := configured(t)
	pal, _ := palette.ByName("munch-scream-oil")
	if _, err := s.Render(sketch.Context{Width: 0, Height: 10, Palette: pal}); err == nil {
		t.Error("a zero-width canvas should be rejected")
	}
}
