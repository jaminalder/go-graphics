package scree

import (
	"flag"
	"io"
	"math"
	"slices"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/scheme"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

var update = flag.Bool("update", false, "regenerate golden files")

// beds is every level of the bed axis, so the tests sweep the whole axis
// rather than whichever one seed 42 happens to draw.
var beds = []string{"boulders", "cobbles", "shingle", "gravel", "grit"}

func testCtx(t *testing.T, seed uint64) sketch.Context {
	t.Helper()
	pal, ok := palette.ByName("cezanne-bathers")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal}
}

// configured returns a sketch with its options resolved, optionally pinning
// flags the way the CLI would. Tests go through the real path rather than
// setting fields, because which flags were *given* is what tells an override
// from what the seed's traits drew.
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

func planned(t *testing.T, s *Sketch, seed uint64) *sheet {
	t.Helper()
	return plannedWithContext(t, s, testCtx(t, seed))
}

func plannedWithContext(t *testing.T, s *Sketch, ctx sketch.Context) *sheet {
	t.Helper()
	sh, err := s.plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return sh
}

func TestDeterminism(t *testing.T) {
	sketchtest.AssertDeterministic(t, configured(t), testCtx(t, 42), testCtx(t, 43))
}

func TestGolden(t *testing.T) {
	got := sketchtest.RenderNRGBA(t, configured(t), testCtx(t, 42))
	sketchtest.Golden(t, got, "testdata/scree_seed42_96.png", *update)
}

// TestEveryBedCoversTheFrame is the guard the pack needs. Sites are placed
// over an *overscan*, so a count that looks generous can still leave the
// frame nearly undivided — the difference never shows in the count, only in
// how many stones have any area on the paper.
func TestEveryBedCoversTheFrame(t *testing.T) {
	want := map[string][2]int{
		"boulders": {5, 24},
		"cobbles":  {14, 50},
		"shingle":  {35, 120},
		"gravel":   {70, 230},
		"grit":     {140, 400},
	}
	for _, b := range beds {
		for seed := uint64(1); seed <= 4; seed++ {
			// Stones pinned worn: merging legitimately lowers the count, and
			// this test is about what the *pack* delivers.
			s := configured(t, "--bed", b, "--stones", "worn")
			n := 0
			for _, c := range planned(t, s, seed).stones.Cells() {
				if c.Area > 0 {
					n++
				}
			}
			lo, hi := want[b][0], want[b][1]
			if n < lo || n > hi {
				t.Errorf("%s seed %d: %d stones on the paper, want %d..%d", b, seed, n, lo, hi)
			}
		}
	}
}

// TestStonesAreGradedInSize pins what separates a river bed from a
// honeycomb. A bed of uniformly sized stones has nothing to read; what makes
// one look like a river bed is that a stone you could sit on sits directly
// against grit, so the largest has to be an order of magnitude above the
// smallest in area.
func TestStonesAreGradedInSize(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		s := configured(t, "--bed", "shingle")
		small, large := math.Inf(1), 0.0
		for _, c := range planned(t, s, seed).stones.Cells() {
			if c.Area <= 0 {
				continue
			}
			small, large = math.Min(small, c.Area), math.Max(large, c.Area)
		}
		if ratio := large / small; ratio < 10 {
			t.Errorf("seed %d: largest stone is only %.1fx the smallest — no grading", seed, ratio)
		}
	}
}

// TestEveryStoneGetsSeveralFacets. The grain is one fineness for the whole
// bed, sized off the *smallest* stone, so the risk runs the other way from
// what it looks like: a boulder is safe by construction and it is the chips
// that can end up with a single face — and a stone with one facet is a stone
// with no surface at all, which is the one thing this sketch cannot deliver.
// The floor in facetRadius is what has to hold, across the whole bed axis.
func TestEveryStoneGetsSeveralFacets(t *testing.T) {
	for _, b := range []string{"boulders", "shingle", "gravel"} {
		for seed := uint64(1); seed <= 3; seed++ {
			sh := planned(t, configured(t, "--bed", b, "--facets", "cut"), seed)
			if sh.facets == nil {
				t.Fatalf("%s seed %d: no facets at all", b, seed)
			}
			count := make([]int, sh.stones.Len())
			for _, owner := range sh.facets.stone {
				count[owner]++
			}
			for i, c := range sh.stones.Cells() {
				// Only stones with a real presence on the paper: a border stone
				// clipped to a sliver honestly has room for one facet.
				if c.Area < 0.004 {
					continue
				}
				if count[i] < 4 {
					t.Errorf("%s seed %d: stone %d covers %.1f%% of the sheet and got %d facets",
						b, seed, i, c.Area*100, count[i])
				}
			}
		}
	}
}

// TestAFacetIsOneFlatShade is the sketch. The shade is computed once per
// facet and held constant across it, which is what puts a hard step at every
// facet edge; computed per pixel — one line's difference — the same surface
// and the same light give a sheet of soft blobs. If this ever starts varying
// within a facet, the picture has quietly become the one this sketch exists
// to replace.
func TestAFacetIsOneFlatShade(t *testing.T) {
	s := configured(t, "--bed", "cobbles", "--facets", "plates")
	sh := planned(t, s, 3)
	if sh.facets == nil {
		t.Fatal("no facets")
	}
	const res = 40
	seen := make(map[int]float64)
	for i := range res * res {
		u := (float64(i%res) + 0.5) / res
		v := (float64(i/res) + 0.5) / res
		wu, wv := s.warp(sh.field, sh.level, u, v)
		id := sh.facets.foam.At(wu, wv).Cell
		d := sh.facets.diffuse[id]
		if prev, ok := seen[id]; ok && prev != d {
			t.Fatalf("facet %d shaded %.4f at one point and %.4f at another", id, prev, d)
		}
		seen[id] = d
	}
	if len(seen) < 20 {
		t.Errorf("only %d distinct facets sampled — the test is not exercising anything", len(seen))
	}
}

// TestTheLitSideIsBrighterThanTheShadowedSide. One lamp for the whole sheet
// is the single cue that makes fake relief read as relief, and the whole
// chain — bearing, elevation, gain, Lambert — has to preserve the sign. A
// face tilted toward the lamp must come out brighter than a flat one, and a
// flat one brighter than a face tilted away.
func TestTheLitSideIsBrighterThanTheShadowedSide(t *testing.T) {
	for _, level := range []string{"raking", "morning", "noon", "overcast"} {
		sh := planned(t, configured(t, "--light", level), 5)
		l := sh.level.lit
		// A height field's normal is (−∂h/∂x, −∂h/∂y, 1), so a gradient
		// pointing *away* from the lamp tilts the face *toward* it.
		toward, _ := l.litFor(-l.lx*0.6, -l.ly*0.6)
		flat, _ := l.litFor(0, 0)
		away, _ := l.litFor(l.lx*0.6, l.ly*0.6)
		if !(toward >= flat && flat > away) {
			t.Errorf("--light %s: toward %.3f, flat %.3f, away %.3f — not monotonic",
				level, toward, flat, away)
		}
		if away <= 0 {
			t.Errorf("--light %s: a face turned away is unlit (%.3f) — no ambient", level, away)
		}
	}
}

// TestAShadowedStoneStaysLighterThanTheJoint. The joint is the water between
// the stones and it has to stay the darkest thing on the sheet, because it is
// what tells the eye where one stone ends. This is the failure the shadow
// lean caused and the reason atValue exists: the diffuse had already
// multiplied the colour down, and mixing in a sky colour that was dark in its
// own right took the light away twice, so the shadowed half of every stone
// sank into the water it was lying in.
//
// The check is on the composed result, not on the diffuse — the ambient floor
// makes the diffuse alone impossible to fail.
func TestAShadowedStoneStaysLighterThanTheJoint(t *testing.T) {
	for _, w := range []string{"dry", "damp", "wet", "sunk"} {
		for _, g := range []string{"raking", "morning", "noon", "overcast"} {
			for seed := uint64(1); seed <= 3; seed++ {
				s := configured(t, "--wet", w, "--light", g)
				sh := planned(t, s, seed)
				joint := sh.ink.joint.Luminance()
				for i, d := range sh.skin {
					// The deepest shadow this stone can reach: the ambient, and
					// the full lean toward the sky.
					dark := s.applyLight(sh, d.pigment, sh.level.lit.amb, 0)
					if lum := dark.Luminance(); lum < joint*1.35 {
						t.Errorf("--wet %s --light %s seed %d: stone %d in shadow is %.3f against a joint of %.3f",
							w, g, seed, i, lum, joint)
					}
				}
			}
		}
	}
}

// TestPinningTheStoneSizeKeepsTheWear. Every length that describes a stone —
// how far its corners are worn, how far a junction's swelling reaches, how
// thick the joint is — is a fraction of the smallest stone, so pinning
// --base has to carry all of them with it and change nothing else. An earlier
// version re-derived the wear and the swelling from fixed fractions after the
// traits had spoken, so `--base` quietly overrode `--stones`: a `worn` bed
// pinned to a hand-set size came out with the corners of a `broken` one.
func TestPinningTheStoneSizeKeepsTheWear(t *testing.T) {
	for _, level := range []string{"worn", "rolled", "broken"} {
		free := planned(t, configured(t, "--bed", "gravel", "--stones", level), 4).level
		pinned := planned(t, configured(t, "--bed", "gravel", "--stones", level, "--base", "0.021"), 4).level

		if pinned.base != 0.021 {
			t.Fatalf("--base 0.021 gave a base of %v", pinned.base)
		}
		// The ratios are what the trait decided; only the length they scale
		// against was pinned, so every one of them must survive untouched.
		for _, r := range []struct {
			name       string
			free, pinn float64
		}{
			{"round", free.round / free.base, pinned.round / pinned.base},
			{"node", free.node / free.base, pinned.node / pinned.base},
			{"ink", free.ink / free.base, pinned.ink / pinned.base},
		} {
			if math.Abs(r.free-r.pinn) > 1e-9 {
				t.Errorf("--stones %s: pinning --base moved %s from %.4f to %.4f x base",
					level, r.name, r.free, r.pinn)
			}
		}
	}
}

// TestPreviewAndPrintAgree — invariant 2. Every length the surface is built
// from is a canvas length, so the plan at one output size must be the plan at
// another: the stones sit in the same places, the domes are the same height,
// and the facets are cut the same way.
func TestPreviewAndPrintAgree(t *testing.T) {
	pal, ok := palette.ByName("cezanne-bathers")
	if !ok {
		t.Fatal("palette missing")
	}
	s := configured(t)
	small, err := s.plan(sketch.Context{Width: 96, Height: 96, Seed: 11, Palette: pal})
	if err != nil {
		t.Fatal(err)
	}
	large, err := s.plan(sketch.Context{Width: 600, Height: 600, Seed: 11, Palette: pal})
	if err != nil {
		t.Fatal(err)
	}
	if small.stones.Len() != large.stones.Len() {
		t.Fatalf("%d stones at preview, %d at print", small.stones.Len(), large.stones.Len())
	}
	for i, a := range small.stones.Cells() {
		b := large.stones.Cells()[i]
		if math.Abs(a.Inradius-b.Inradius) > 1e-12 || math.Abs(a.CX-b.CX) > 1e-12 {
			t.Fatalf("stone %d differs between sizes: inradius %.9f vs %.9f", i, a.Inradius, b.Inradius)
		}
	}
	if small.facets.foam.Len() != large.facets.foam.Len() {
		t.Errorf("%d facets at preview, %d at print", small.facets.foam.Len(), large.facets.foam.Len())
	}
}

// TestSmoothCutsNoFacets. The smooth level is the control the acceptance
// checklist leans on — the same surface and the same light with the flat
// shading turned off — and it only means anything if it really builds no
// facet layer, so the lighting takes the per-pixel path everywhere.
func TestSmoothCutsNoFacets(t *testing.T) {
	if sh := planned(t, configured(t, "--facets", "smooth"), 7); sh.facets != nil {
		t.Error("--facets smooth still built a facet layer")
	}
	if sh := planned(t, configured(t, "--facets", "cut"), 7); sh.facets == nil {
		t.Error("--facets cut built no facet layer")
	}
}

// TestEveryStoneIsPaintedWithinRange. The load is what the wash actually
// lays down; outside [0,1] paint.FlatWash either does nothing or is clamped
// silently, and a stone that took no pigment reads as a hole in the bed
// rather than as a pale stone.
func TestEveryStoneIsPaintedWithinRange(t *testing.T) {
	for seed := uint64(1); seed <= 10; seed++ {
		for _, d := range planned(t, configured(t), seed).skin {
			if d.load < 0.2 || d.load > 1 {
				t.Errorf("seed %d: a stone took a load of %.3f", seed, d.load)
			}
		}
	}
}

func TestGoldModeRemovesYellowFromTheOrdinaryPalette(t *testing.T) {
	pal, ok := palette.ByName("avery-bicycle-rider")
	if !ok {
		t.Fatal("palette missing")
	}
	if !yellowLike(palette.MustHex("#F3C937")) {
		t.Fatal("Avery gold was not recognized as yellow")
	}
	for _, hex := range []string{"#7B533E", "#BFA588", "#604847", "#552723"} {
		if yellowLike(palette.MustHex(hex)) {
			t.Errorf("%s was classified as yellow", hex)
		}
	}
	filtered, err := withoutYellow(pal)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Colors) != 4 {
		t.Fatalf("filtered palette has %d colours, want 4", len(filtered.Colors))
	}
	for i, want := range pal.Colors[1:] {
		if filtered.Colors[i] != want {
			t.Errorf("colour %d changed from %s to %s", i, want.Hex(), filtered.Colors[i].Hex())
		}
	}
}

func TestGoldModeChoosesTwoOrThreeSmallToMediumNuggets(t *testing.T) {
	pal, ok := palette.ByName("avery-bicycle-rider")
	if !ok {
		t.Fatal("palette missing")
	}
	for seed := uint64(1); seed <= 12; seed++ {
		s := configured(t, "--gold", "--colourway", "avery-bicycle-rider", "--bed", "gravel")
		ctx := sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal}
		sh := plannedWithContext(t, s, ctx)
		allowed := make(map[int]bool)
		for _, id := range s.nuggetCandidates(sh.stones, sh.field, sh.level, 1) {
			allowed[id] = true
		}

		count := 0
		for id, d := range sh.skin {
			if !d.nugget {
				if yellowLike(d.pigment) {
					t.Errorf("seed %d: ordinary stone %d is yellow (%s)", seed, id, d.pigment.Hex())
				}
				continue
			}
			count++
			if !allowed[id] {
				t.Errorf("seed %d: nugget %d is outside the candidate set", seed, id)
			}
			_, sat, _ := d.pigment.HSL()
			if math.Abs(sat-1) > 1e-12 {
				t.Errorf("seed %d: nugget %d saturation is %.3f, want 1", seed, id, sat)
			}
		}
		if count != 2 && count != 3 {
			t.Errorf("seed %d: got %d nuggets, want 2 or 3", seed, count)
		}
	}
}

func TestGoldModeIsDeterministicAndResolutionIndependent(t *testing.T) {
	pal, ok := palette.ByName("avery-bicycle-rider")
	if !ok {
		t.Fatal("palette missing")
	}
	ids := func(width, height int) []int {
		s := configured(t, "--gold", "--colourway", "avery-bicycle-rider", "--bed", "gravel")
		sh := plannedWithContext(t, s, sketch.Context{Width: width, Height: height, Seed: 8, Palette: pal})
		var out []int
		for id, d := range sh.skin {
			if d.nugget {
				out = append(out, id)
			}
		}
		return out
	}

	a, b, print := ids(96, 96), ids(96, 96), ids(600, 600)
	if !slices.Equal(a, b) {
		t.Fatalf("same render selected %v then %v", a, b)
	}
	if !slices.Equal(a, print) {
		t.Fatalf("preview selected %v, print selected %v", a, print)
	}

	s := configured(t, "--gold", "--colourway", "avery-bicycle-rider", "--bed", "gravel")
	sketchtest.AssertDeterministic(t, s,
		sketch.Context{Width: 48, Height: 48, Seed: 8, Palette: pal},
		sketch.Context{Width: 48, Height: 48, Seed: 9, Palette: pal},
	)
}

func TestEveryChosenNuggetAppearsInTheRenderedBed(t *testing.T) {
	pal, ok := palette.ByName("avery-bicycle-rider")
	if !ok {
		t.Fatal("palette missing")
	}
	for seed := uint64(1); seed <= 30; seed++ {
		s := configured(t, "--gold", "--colourway", "avery-bicycle-rider", "--bed", "gravel")
		sh := plannedWithContext(t, s, sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal})
		seen := make(map[int]bool)
		for y := range 96 {
			for x := range 96 {
				u, v := (float64(x)+0.5)/96, (float64(y)+0.5)/96
				wu, wv := s.warp(sh.field, sh.level, u, v)
				seen[sh.stones.At(wu, wv).Cell] = true
			}
		}
		for id, d := range sh.skin {
			if d.nugget && !seen[id] {
				t.Errorf("seed %d: selected nugget %d does not appear in the rendered bed", seed, id)
			}
		}
	}
}

func TestEveryChosenNuggetHasVisibleGold(t *testing.T) {
	pal, ok := palette.ByName("avery-bicycle-rider")
	if !ok {
		t.Fatal("palette missing")
	}
	for _, seed := range []uint64{8, 50} {
		s := configured(t, "--gold", "--colourway", "avery-bicycle-rider", "--bed", "gravel")
		ctx := sketch.Context{Width: 96, Height: 96, Seed: seed, Palette: pal}
		sh := plannedWithContext(t, s, ctx)
		img := sketchtest.RenderNRGBA(t, s, ctx)
		gold := make([]int, sh.stones.Len())
		for y := range 96 {
			for x := range 96 {
				u, v := (float64(x)+0.5)/96, (float64(y)+0.5)/96
				wu, wv := s.warp(sh.field, sh.level, u, v)
				id := sh.stones.At(wu, wv).Cell
				px := img.NRGBAAt(x, y)
				col := palette.Color{R: float64(px.R) / 255, G: float64(px.G) / 255, B: float64(px.B) / 255}
				if readsYellow(col) {
					gold[id]++
				}
			}
		}
		for id, d := range sh.skin {
			if d.nugget && gold[id] == 0 {
				t.Errorf("seed %d: nugget %d has no visible gold pixels", seed, id)
			}
		}
	}
}

func TestOrdinaryRenderedStonesDoNotReadYellowInGoldMode(t *testing.T) {
	for _, name := range []string{"avery-bicycle-rider", "cezanne-bathers"} {
		pal, ok := palette.ByName(name)
		if !ok {
			t.Fatal("palette missing")
		}
		for seed := uint64(1); seed <= 12; seed++ {
			s := configured(t, "--gold", "--colourway", name, "--bed", "gravel", "--scheme", scheme.Passage)
			ctx := sketch.Context{Width: 48, Height: 48, Seed: seed, Palette: pal}
			sh := plannedWithContext(t, s, ctx)
			img := sketchtest.RenderNRGBA(t, s, ctx)
			for y := range 48 {
				for x := range 48 {
					u, v := (float64(x)+0.5)/48, (float64(y)+0.5)/48
					wu, wv := s.warp(sh.field, sh.level, u, v)
					h := sh.stones.At(wu, wv)
					if sh.skin[h.Cell].nugget {
						continue
					}
					px := img.NRGBAAt(x, y)
					col := palette.Color{R: float64(px.R) / 255, G: float64(px.G) / 255, B: float64(px.B) / 255}
					if readsYellow(col) {
						t.Fatalf("%s seed %d: ordinary stone %d renders yellow at (%d,%d): %s", name, seed, h.Cell, x, y, col.Hex())
					}
				}
			}
		}
	}
}

func TestWithoutGoldNoStoneIsANugget(t *testing.T) {
	for id, d := range planned(t, configured(t), 8).skin {
		if d.nugget {
			t.Errorf("stone %d became a nugget without --gold", id)
		}
	}
}

func TestGoldAppearsInTheOutputName(t *testing.T) {
	s := New()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	s.Flags(fs)
	if err := fs.Parse([]string{"--gold"}); err != nil {
		t.Fatal(err)
	}
	suffix, err := s.Configure()
	if err != nil {
		t.Fatal(err)
	}
	if suffix != "-gold" {
		t.Fatalf("--gold suffix is %q, want -gold", suffix)
	}
}

func TestGoldModeSupportsDuetWhenFilteringLeavesOneColour(t *testing.T) {
	pal, ok := palette.ByName("quidor-leatherstocking")
	if !ok {
		t.Fatal("palette missing")
	}
	s := configured(t, "--gold", "--colourway", fromFlag, "--scheme", scheme.Duet, "--bed", "boulders")
	plannedWithContext(t, s, sketch.Context{Width: 48, Height: 48, Seed: 1, Palette: pal})
}
