package tapestry

import (
	"bytes"
	"flag"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var update = flag.Bool("update", false, "regenerate golden files")

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("kandinsky-soft-pressure")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 64, Height: 64, Seed: seed, Palette: pal}
}

func renderNRGBA(t *testing.T, ctx sketch.Context) *image.NRGBA {
	t.Helper()
	img, err := New().Render(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return img.(*image.NRGBA)
}

func TestDeterminism(t *testing.T) {
	a := renderNRGBA(t, testCtx(t, 42))
	b := renderNRGBA(t, testCtx(t, 42))
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("same seed produced different images")
	}
	c := renderNRGBA(t, testCtx(t, 43))
	if bytes.Equal(a.Pix, c.Pix) {
		t.Error("different seeds produced identical images")
	}
}

func TestRejectsTooSmallPalette(t *testing.T) {
	ctx := testCtx(t, 1)
	ctx.Palette = palette.Palette{Slug: "tiny", Colors: []palette.Color{{}, {}, {}}}
	if _, err := New().Render(ctx); err == nil {
		t.Error("expected error for palette with < 4 colors")
	}
}

func TestPlanBounds(t *testing.T) {
	s := New()
	for seed := uint64(0); seed < 200; seed++ {
		p := s.plan(testCtx(t, seed))
		if p.freq < 4 || p.freq > 6 {
			t.Fatalf("seed %d: freq %v out of range", seed, p.freq)
		}
		if p.bands < 20 || p.bands > 40 {
			t.Fatalf("seed %d: %d bands out of range", seed, p.bands)
		}
		for b, g := range p.grads {
			// Terrace widths are floored at 1/bands, so a band has at most
			// bands+1 levels and at least 2.
			if g.Len() < 2 || g.Len() > p.bands+1 {
				t.Fatalf("seed %d: band %d has %d terraces (bands=%d)", seed, b, g.Len(), p.bands)
			}
		}
		// Band edges must be strictly ordered within the mapped span.
		if !(-p.span < p.cuts[0] && p.cuts[0] < p.cuts[1] && p.cuts[1] < 0 &&
			0 < p.cuts[2] && p.cuts[2] < p.cuts[3] && p.cuts[3] < p.span) {
			t.Fatalf("seed %d: band cuts %v not ordered within ±%v", seed, p.cuts, p.span)
		}
		if len(p.stripes) < 8 {
			t.Fatalf("seed %d: only %d stripes on a square canvas", seed, len(p.stripes))
		}
		if last := p.stripes[len(p.stripes)-1].end; last < 1 {
			t.Fatalf("seed %d: stripes end at %v, canvas not covered", seed, last)
		}
		for _, st := range p.stripes {
			if st.amount < 0 || st.amount > 0.30 {
				t.Fatalf("seed %d: stripe amount %v out of range", seed, st.amount)
			}
		}
	}
}

func TestFold(t *testing.T) {
	const span = 0.6
	tests := []struct{ in, want float64 }{
		{0, 0},
		{0.5, 0.5},
		{-0.5, -0.5},
		{0.6, 0.6},
		{-0.6, -0.6},
		{0.7, 0.5},   // reflects at +span
		{-0.7, -0.5}, // reflects at -span
		{1.3, -0.1},  // continues down past the reflection
	}
	for _, tt := range tests {
		if got := fold(tt.in, span); math.Abs(got-tt.want) > 1e-12 {
			t.Errorf("fold(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
	// Continuity across the boundary.
	if math.Abs(fold(span-1e-9, span)-fold(span+1e-9, span)) > 1e-8 {
		t.Error("fold is discontinuous at +span")
	}
}

func TestStripeAt(t *testing.T) {
	p := &plan{stripes: []stripe{{end: 0.2, amount: 0.1}, {end: 0.5, amount: 0.2}, {end: 1.1, amount: 0.3}}}
	tests := []struct {
		u    float64
		want float64
	}{{0, 0.1}, {0.19, 0.1}, {0.2, 0.2}, {0.49, 0.2}, {0.5, 0.3}, {1.0, 0.3}}
	for _, tt := range tests {
		if got := p.stripeAt(tt.u).amount; got != tt.want {
			t.Errorf("stripeAt(%v).amount = %v, want %v", tt.u, got, tt.want)
		}
	}
}

func TestDisableStripes(t *testing.T) {
	striped := renderNRGBA(t, testCtx(t, 42))

	s := New()
	s.DisableStripes = true
	img, err := s.Render(testCtx(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	plain := img.(*image.NRGBA)

	if bytes.Equal(striped.Pix, plain.Pix) {
		t.Error("disabling stripes changed nothing")
	}
	// The underlying composition must survive: pixels in pass-through
	// stripes differ only by the per-stripe grain multiplier, so a
	// meaningful share must still be byte-identical. (Zone crossfading
	// makes coincidental matches rarer than in earlier versions, hence
	// the modest 2.5% bar.)
	same := 0
	for i := 0; i < len(plain.Pix); i += 4 {
		if plain.Pix[i] == striped.Pix[i] && plain.Pix[i+1] == striped.Pix[i+1] &&
			plain.Pix[i+2] == striped.Pix[i+2] {
			same++
		}
	}
	if total := len(plain.Pix) / 4; same < total/40 {
		t.Errorf("only %d/%d pixels unchanged — stripe stream leaked into the base composition", same, total)
	}
}

func TestTerraceGrain(t *testing.T) {
	plain := renderNRGBA(t, testCtx(t, 42))

	s := New()
	s.TerraceGrain = true
	img, err := s.Render(testCtx(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	grained := img.(*image.NRGBA)

	if bytes.Equal(plain.Pix, grained.Pix) {
		t.Error("terrace grain changed nothing")
	}
	// Grain draws use their own stream, so the base composition must
	// survive: most terraces are unboosted and within boosted ones the
	// change is only an amplitude boost of the same grain values — a large
	// share of pixels must be byte-identical.
	same := 0
	for i := 0; i < len(plain.Pix); i += 4 {
		if plain.Pix[i] == grained.Pix[i] && plain.Pix[i+1] == grained.Pix[i+1] &&
			plain.Pix[i+2] == grained.Pix[i+2] {
			same++
		}
	}
	if total := len(plain.Pix) / 4; same < total/4 {
		t.Errorf("only %d/%d pixels unchanged — grain draws leaked into the base composition", same, total)
	}

	// Only wide terraces may be boosted, with bounded strength.
	for seed := uint64(0); seed < 100; seed++ {
		p := s.plan(testCtx(t, seed))
		wMin := 1.0 / float64(p.bands)
		for b, boosts := range p.grainBoost {
			if boosts == nil {
				continue
			}
			for i, boost := range boosts {
				if boost == 0 {
					continue
				}
				if w := p.terraceWidths[b][i]; w < 2.5*wMin {
					t.Fatalf("seed %d: narrow terrace (band %d level %d, width %v) boosted", seed, b, i, w)
				}
				if boost < 2.5 || boost > 5.5 {
					t.Fatalf("seed %d: boost %v out of range", seed, boost)
				}
			}
		}
	}

	// A different grain seed relays out the grain on the same image.
	s2 := New()
	s2.TerraceGrain = true
	s2.GrainSeed = 99
	img2, err := s2.Render(testCtx(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(grained.Pix, img2.(*image.NRGBA).Pix) {
		t.Error("different grain seeds produced identical grain layouts")
	}
}

func TestTerraceCrackle(t *testing.T) {
	plain := renderNRGBA(t, testCtx(t, 42))

	s := New()
	s.TerraceCrackle = true
	img, err := s.Render(testCtx(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	crackled := img.(*image.NRGBA)

	if bytes.Equal(plain.Pix, crackled.Pix) {
		t.Error("crackle changed nothing")
	}
	// Cracks are thin lines within selected terraces — the vast majority
	// of pixels must be untouched.
	same := 0
	for i := 0; i < len(plain.Pix); i += 4 {
		if plain.Pix[i] == crackled.Pix[i] && plain.Pix[i+1] == crackled.Pix[i+1] &&
			plain.Pix[i+2] == crackled.Pix[i+2] {
			same++
		}
	}
	if total := len(plain.Pix) / 4; same < total/2 {
		t.Errorf("only %d/%d pixels unchanged — crackle affects too much", same, total)
	}

	// Only wide terraces may crackle, with bounded strength.
	for seed := uint64(0); seed < 100; seed++ {
		p := s.plan(testCtx(t, seed))
		if p.crackleRes < 150 || p.crackleRes > 250 {
			t.Fatalf("seed %d: crackle res %v out of range", seed, p.crackleRes)
		}
		wMin := 1.0 / float64(p.bands)
		for b, boosts := range p.crackleBoost {
			if boosts == nil {
				continue
			}
			for i, boost := range boosts {
				if boost == 0 {
					continue
				}
				if w := p.terraceWidths[b][i]; w < 5*wMin {
					t.Fatalf("seed %d: narrow terrace (band %d level %d) crackled", seed, b, i)
				}
				if boost < 0.5 || boost > 0.9 {
					t.Fatalf("seed %d: crackle strength %v out of range", seed, boost)
				}
			}
		}
	}
}

func TestRelief(t *testing.T) {
	plain := renderNRGBA(t, testCtx(t, 42))

	s := New()
	s.Relief = true
	a, err := s.Render(testCtx(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Render(testCtx(t, 42))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.(*image.NRGBA).Pix, b.(*image.NRGBA).Pix) {
		t.Error("relief render is not deterministic")
	}
	if bytes.Equal(a.(*image.NRGBA).Pix, plain.Pix) {
		t.Error("relief shading changed nothing")
	}
}

func TestReliefPresets(t *testing.T) {
	base, ok := ReliefPreset("baseline")
	if !ok || base != DefaultReliefParams() {
		t.Error("baseline preset must equal DefaultReliefParams")
	}
	names := ReliefPresetNames()
	if len(names) != 10 {
		t.Errorf("expected 10 presets, got %d", len(names))
	}
	for _, n := range names {
		if _, ok := ReliefPreset(n); !ok {
			t.Errorf("preset %q not resolvable", n)
		}
	}
	if _, ok := ReliefPreset("nope"); ok {
		t.Error("unknown preset should not resolve")
	}
}

func TestGolden(t *testing.T) {
	got := renderNRGBA(t, testCtx(t, 42))
	golden := filepath.Join("testdata", "tapestry_seed42_64.png")

	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(golden)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, got); err != nil {
			t.Fatal(err)
		}
		t.Log("golden regenerated — eyeball it before committing")
		return
	}

	f, err := os.Open(golden)
	if err != nil {
		t.Fatalf("missing golden (run with -update): %v", err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := want.Bounds()
	wantN := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			wantN.Set(x, y, want.At(x, y))
		}
	}
	if !bytes.Equal(got.Pix, wantN.Pix) {
		t.Error("render differs from golden (intentional change? re-run with -update and eyeball)")
	}
}
