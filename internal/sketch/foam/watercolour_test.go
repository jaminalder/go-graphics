package foam

import (
	"image"
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/sketchtest"
)

// schemes is every level of the colour axis, so the tests sweep the whole
// axis rather than whichever one a seed happens to draw.
var schemes = []string{
	schemePassage, schemeAnchor, schemeQuiet,
	schemeWeather, schemeDuet, schemeByScale, schemeFromLum,
}

// painted returns a watercolour sheet with the given flags applied on top.
func painted(t *testing.T, seed uint64, args ...string) (*Sketch, *sheet) {
	t.Helper()
	base := []string{"--fills", "watercolour", "--density", "packed", "--lobes", "few"}
	s := configured(t, append(base, args...)...)
	return s, planned(t, s, seed)
}

// washes lists the cells of a sheet that were actually painted.
func washes(sh *sheet) []int {
	var out []int
	for i, d := range sh.skin {
		if d.style == styleWash && sh.foam.Cells()[i].Area > 0 {
			out = append(out, i)
		}
	}
	return out
}

// TestThePaintDoesNotRegisterWithTheDrawnLine.
//
// Hand-painted work almost never registers: the wash runs a little past the
// line, or stops short and leaves a rind of white paper inside it. The
// failure to register is a large part of why a picture reads as painted
// rather than filled, so a sheet where every wash meets its wall exactly is
// the failure this defends against — and it is the state the fill layer was
// in before, because it took the wall itself as the paint's edge.
func TestThePaintDoesNotRegisterWithTheDrawnLine(t *testing.T) {
	_, sh := painted(t, 7)
	over, short := 0, 0
	for _, i := range washes(sh) {
		switch r := sh.skin[i].water.reach; {
		case r > 0:
			over++
		case r < 0:
			short++
		}
	}
	if over < 5 || short < 5 {
		t.Errorf("%d cells overshoot the line and %d stop short of it; a sheet needs both", over, short)
	}
	// And the error has to be worth seeing: comparable to the line itself,
	// not a hundredth of it.
	worst := 0.0
	for _, i := range washes(sh) {
		worst = math.Max(worst, math.Abs(sh.skin[i].water.reach))
	}
	if worst < sh.level.ink {
		t.Errorf("the worst registration error is %.5f against a %.5f line — invisible", worst, sh.level.ink)
	}
}

// TestPigmentCrossesAWetWallAndNotADryOne.
//
// Two cells painted while both are wet mix across the wall between them;
// two painted at different times do not. Both are the same mechanism —
// the neighbour's paint evaluated at the mirrored wall distance — so what
// this pins is that the wetness of a *pair* is what switches it on, and
// that a dry sheet keeps its pigment inside its own cells.
func TestPigmentCrossesAWetWallAndNotADryOne(t *testing.T) {
	sWet, wet := painted(t, 7, "--bleed", "1", "--seep", "0.9")
	sDry, dry := painted(t, 7, "--bleed", "0", "--overshoot", "0")

	crossed, checked := 0, 0
	for j := range 200 {
		for i := range 200 {
			u, v := (float64(i)+0.5)/200, (float64(j)+0.5)/200
			h := dry.foam.At(u, v)
			if math.IsInf(h.Wall, 1) || h.Next < 0 {
				continue
			}
			if dry.skin[h.Cell].style != styleWash || dry.skin[h.Next].style != styleWash {
				continue
			}
			checked++
			own := sDry.fill(dry.skin[h.Cell], h, dry.field, 7, u, v, dry.paper)
			if got := sDry.paint(dry, h, 7, u, v); got != own {
				t.Fatalf("with no bleed and no overshoot, paint at (%.3f, %.3f) still differs from the cell's own fill", u, v)
			}
			ownWet := sWet.fill(wet.skin[h.Cell], h, wet.field, 7, u, v, wet.paper)
			if sWet.paint(wet, h, 7, u, v) != ownWet {
				crossed++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no point of the sheet had a washed cell on both sides of a wall")
	}
	if share := float64(crossed) / float64(checked); share < 0.05 {
		t.Errorf("only %.1f%% of the sheet took pigment from across a wall at --bleed 1", share*100)
	}
}

// TestTheRimIsTheDarkestPartOfAWash.
//
// Pigment dries denser in the last stretch before the boundary — the single
// most recognisable watercolour cue there is. It has to be a *ridge* just
// inside the paint's edge and not a ramp climbing to it: coverage is already
// falling away at the boundary, so a rim that only rises toward the edge
// multiplies a number on its way to zero and never shows.
//
// The wall distance is swept with the position held still, which is not a
// physical path across a cell but is exactly what isolates the rim from
// every other thing that varies over one.
func TestTheRimIsTheDarkestPartOfAWash(t *testing.T) {
	s, sh := painted(t, 7, "--water", waterPlain)
	list := washes(sh)
	if len(list) == 0 {
		t.Fatal("nothing was painted")
	}
	tested := 0
	for _, i := range list[:min(len(list), 12)] {
		d := sh.skin[i]
		w := d.water
		if w.rimWide <= 0 {
			continue
		}
		darkest, at := math.Inf(1), 0.0
		deep := 0.0
		for k := range 200 {
			wall := float64(k) / 200 * 6 * w.rimWide
			lum := s.watercolour(d, wall, sh.field, 7, d.water.cx, d.water.cy, sh.paper).Luminance()
			if lum < darkest {
				darkest, at = lum, wall
			}
			deep = lum
		}
		if at > 2.5*w.rimWide {
			t.Errorf("cell %d is darkest %.4f in, against a %.4f rim — that is not a rim", i, at, w.rimWide)
		}
		if deep-darkest < 0.01 {
			t.Errorf("cell %d has no rim at all: its edge is %.4f and its middle %.4f", i, darkest, deep)
		}
		tested++
	}
	if tested == 0 {
		t.Fatal("no cell carried a rim to test")
	}
}

// TestABackrunIsPaleInsideAHardRing.
//
// A backrun is not a soft blob. The returning water dissolves the settled
// pigment, carries it outward and abandons it at the front where it stopped,
// so the shape has a *pale* interior and a hard ridge at its boundary. The
// pale part alone reads as a lifted highlight and the ridge alone as a
// second rim; it is the pair that reads as a cauliflower.
func TestABackrunIsPaleInsideAHardRing(t *testing.T) {
	field := noise.New(9)
	w := waterDress{scale: 0.05, blevel: 0.02, bamp: 0.008, tilt: 0.3, cos: 1, cx: 0.4, cy: 0.4}
	peak, at, deep := 0.0, 0.0, 0.0
	for k := range 400 {
		wall := float64(k) / 400 * 0.06
		f := bloom(w, wall, field, 0.4, 0.4)
		if f > peak {
			peak, at = f, wall
		}
		deep = f
	}
	if peak < 1.5 {
		t.Errorf("the backrun's ridge only reaches %.2f× the surrounding wash — too soft to read", peak)
	}
	if deep > 0.7 {
		t.Errorf("the backrun's interior is %.2f× the surrounding wash — it is not pale", deep)
	}
	if at > 0.045 {
		t.Errorf("the ridge sits %.4f in, well past the %.4f front it should mark", at, w.blevel)
	}
}

// TestEveryWaterLevelPaintsInItsOwnManner. A level is a claim about what the
// paint did, and a level whose manner weights do not actually dominate is a
// name with nothing behind it.
func TestEveryWaterLevelPaintsInItsOwnManner(t *testing.T) {
	want := map[string]int{waterPlain: mannerFlat, waterCharge: mannerCharged, waterBloom: mannerBloom, waterGlaze: mannerGlaze}
	for level, manner := range want {
		hit, total := 0, 0
		for seed := uint64(1); seed <= 4; seed++ {
			_, sh := painted(t, seed, "--water", level)
			for _, i := range washes(sh) {
				total++
				if sh.skin[i].water.manner == manner {
					hit++
				}
			}
		}
		if total == 0 {
			t.Fatalf("--water %s painted nothing", level)
		}
		if share := float64(hit) / float64(total); share < 0.5 {
			t.Errorf("--water %s gave its own manner to only %.0f%% of the cells", level, share*100)
		}
	}
}

// TestEachColourSchemeOrganisesTheSheetDifferently.
//
// The schemes are the point of scheme.go: at a hundred and fifty cells, how
// colour is *distributed* matters more than what any one cell is painted
// with. Two things have to hold — that a scheme actually changes the sheet,
// and that the near-monochrome one really is near-monochrome, since that is
// the level most easily left as a synonym for the default.
func TestEachColourSchemeOrganisesTheSheetDifferently(t *testing.T) {
	sheets := map[string][]palette.Color{}
	for _, sc := range schemes {
		_, sh := painted(t, 7, "--scheme", sc)
		var got []palette.Color
		for _, i := range washes(sh) {
			got = append(got, sh.skin[i].pigment)
		}
		sheets[sc] = got
	}
	for _, sc := range schemes {
		if sc == schemePassage {
			continue
		}
		same := 0
		for i := range sheets[sc] {
			if i < len(sheets[schemePassage]) && sheets[sc][i] == sheets[schemePassage][i] {
				same++
			}
		}
		if share := float64(same) / float64(len(sheets[sc])); share > 0.8 {
			t.Errorf("--scheme %s gives %.0f%% of its cells the same pigment as passage does", sc, share*100)
		}
	}
	// Measured as how tightly the sheet huddles round one hue, not as a
	// count of distinct colours. Every cell is a shade of its scheme's
	// answer — a tint or a deepening, the way real paint has tints and
	// shades of every pigment — so no two cells hold exactly the same
	// colour, and counting them would say a monochrome sheet uses a hundred.
	// What "near-monochrome" means is one hue at many strengths.
	huddle := func(cs []palette.Color) float64 {
		hs := make([]float64, 0, len(cs))
		for _, c := range cs {
			if h, sat, _ := c.HSL(); sat >= 0.1 {
				hs = append(hs, h)
			}
		}
		if len(hs) == 0 {
			return 1
		}
		best := 0
		for _, centre := range hs {
			n := 0
			for _, h := range hs {
				d := math.Mod(math.Abs(h-centre), 360)
				if math.Min(d, 360-d) <= 20 {
					n++
				}
			}
			best = max(best, n)
		}
		return float64(best) / float64(len(hs))
	}
	if q, p := huddle(sheets[schemeQuiet]), huddle(sheets[schemePassage]); q <= p {
		t.Errorf("the quiet sheet huddles %.0f%% within one hue and the passage sheet %.0f%% — it is not near-monochrome", q*100, p*100)
	}
}

// TestASheetKeepsCellsAtBothEndsOfTheValueRange. A scheme's job is not only
// hue: a sheet whose cells are all loaded the same has no value structure
// and nothing to read from across the room. Every scheme has to put some
// cells at each end.
func TestASheetKeepsCellsAtBothEndsOfTheValueRange(t *testing.T) {
	for _, sc := range schemes {
		lo, hi := math.Inf(1), 0.0
		for seed := uint64(1); seed <= 3; seed++ {
			_, sh := painted(t, seed, "--scheme", sc)
			for _, i := range washes(sh) {
				lo = math.Min(lo, sh.skin[i].water.load)
				hi = math.Max(hi, sh.skin[i].water.load)
			}
		}
		if hi/lo < 2.5 {
			t.Errorf("--scheme %s spans loads %.2f..%.2f, a factor of %.1f — no value structure", sc, lo, hi, hi/lo)
		}
	}
}

// TestTheWatercolourIsTheSamePictureAtEverySize is invariant 2 for the layer
// that most easily breaks it. Every wash cue here is a length — the rim
// width, the registration error, the paper's tooth — and any one of them
// expressed in pixels would give a print that is a different picture from
// its preview, which is not something a golden at 96px would ever catch.
func TestTheWatercolourIsTheSamePictureAtEverySize(t *testing.T) {
	s := configured(t, "--fills", "watercolour", "--density", "medium", "--water", waterStudio)

	// Compared against a 3× render averaged down, at three sizes. The claim
	// is *convergence*, not a number: a picture whose cues are all lengths
	// disagrees with a finer render only where a pixel straddles an edge,
	// and that disagreement halves every time the resolution doubles. A cue
	// expressed in pixels would hold its error flat or grow it, however
	// small the constant.
	//
	// An absolute threshold at one size is the wrong test and was the first
	// version of this one: it passed or failed on which numbers the seed's
	// water level happened to roll, and appending a trait dimension ahead of
	// the water draw was enough to tip it.
	var got []float64
	for _, n := range []int{60, 120, 240} {
		small, large := render(t, s, n), render(t, s, n*3)
		sum := 0.0
		for y := range n {
			for x := range n {
				sum += math.Abs(lumAt(small, x, y, 1) - lumAt(large, x*3, y*3, 3))
			}
		}
		got = append(got, sum/float64(n*n))
	}
	for i := 1; i < len(got); i++ {
		if got[i] > got[i-1]*0.8 {
			t.Errorf("disagreement went %.4f → %.4f as the render doubled; a length-based cue should nearly halve — something is measured in pixels",
				got[i-1], got[i])
		}
	}
	if got[0] > 0.06 {
		t.Errorf("even the coarsest pair disagrees by %.4f, far past edge quantisation", got[0])
	}
}

func render(t *testing.T, s *Sketch, size int) *image.NRGBA {
	t.Helper()
	pal, ok := palette.ByName("tchelitchew-hide-and-seek")
	if !ok {
		t.Fatal("palette missing")
	}
	return sketchtest.RenderNRGBA(t, s, sketch.Context{Width: size, Height: size, Seed: 7, Palette: pal})
}

// lumAt is the mean luminance of an n×n block with its corner at (x, y).
func lumAt(img *image.NRGBA, x, y, n int) float64 {
	sum := 0.0
	for j := range n {
		for i := range n {
			o := img.PixOffset(x+i, y+j)
			c := palette.Color{
				R: float64(img.Pix[o]) / 255,
				G: float64(img.Pix[o+1]) / 255,
				B: float64(img.Pix[o+2]) / 255,
			}
			sum += c.Luminance()
		}
	}
	return sum / float64(n*n)
}

// TestABleedNeverReachesFurtherThanItWasGiven. The neighbour's paint is
// evaluated at the mirrored wall distance, which is the same arithmetic the
// cell's own paint uses — so a sign error there would not crash or look
// obviously wrong, it would quietly flood whole cells with their
// neighbours' pigment. This pins the depth.
func TestABleedNeverReachesFurtherThanItWasGiven(t *testing.T) {
	s, sh := painted(t, 7, "--bleed", "1", "--seep", "0.6")
	for j := range 150 {
		for i := range 150 {
			u, v := (float64(i)+0.5)/150, (float64(j)+0.5)/150
			h := sh.foam.At(u, v)
			if math.IsInf(h.Wall, 1) || h.Next < 0 {
				continue
			}
			nb, ok := crossing(sh, 7, h.Cell, h.Next)
			if !ok {
				continue
			}
			if h.Wall <= nb.water.depth() {
				continue
			}
			own := s.fill(sh.skin[h.Cell], h, sh.field, 7, u, v, sh.paper)
			if got := s.paint(sh, h, 7, u, v); got != own {
				t.Fatalf("cell %d took pigment from cell %d at %.4f from the wall, past its %.4f reach",
					h.Cell, h.Next, h.Wall, nb.water.depth())
			}
		}
	}
}

// TestTheWatercolourFillLevelPaintsAlmostEveryCell. `--fills watercolour` is
// the level this whole layer exists for; it has to be a painted sheet, with
// only the handful of bare cells the composition needs to rest on.
func TestTheWatercolourFillLevelPaintsAlmostEveryCell(t *testing.T) {
	washed, bare, other := 0, 0, 0
	for seed := uint64(1); seed <= 6; seed++ {
		_, sh := painted(t, seed)
		for _, d := range sh.skin {
			switch d.style {
			case styleWash:
				washed++
			case styleEmpty:
				bare++
			default:
				other++
			}
		}
	}
	if other != 0 {
		t.Errorf("%d cells were drawn rather than painted on a watercolour sheet", other)
	}
	if share := float64(bare) / float64(washed+bare); share < 0.03 || share > 0.3 {
		t.Errorf("%.0f%% of the sheet is bare paper, want a handful", share*100)
	}
}

// TestNoSeedIsMovedByAddingTheWatercolour. The water and scheme dimensions
// are appended to the schema and `watercolour` is added to `fills` at weight
// 0, both so that an existing seed still draws the piece it drew before —
// Derive consumes one draw per dimension in schema order, and a weight-0
// value does not change a dimension's total. If either ever stopped being
// true, every seed of sketch 009 would silently become a different picture.
func TestNoSeedIsMovedByAddingTheWatercolour(t *testing.T) {
	fills, ok := schema.Dim(dimFills)
	if !ok {
		t.Fatal("no fills dimension")
	}
	if fills.Weight("watercolour") != 0 {
		t.Error("the watercolour fill level carries a weight, which moves every seed's fills draw")
	}
	if schema[len(schema)-1].Name != dimScheme || schema[len(schema)-2].Name != dimWater {
		t.Errorf("the watercolour dimensions are not last in the schema: %s, %s",
			schema[len(schema)-2].Name, schema[len(schema)-1].Name)
	}
	// And no seed lands on the level, so it stays a deliberate act.
	s := configured(t)
	for seed := uint64(1); seed <= 80; seed++ {
		if got := s.Traits(testCtx(t, seed)).Get(dimFills); got == "watercolour" {
			t.Fatalf("seed %d drew the watercolour fill level, which carries weight 0", seed)
		}
	}
}

// TestABareCellTakesNoPigmentAtAll — granulation and pooling both modulate
// how much pigment is there, so on bare paper they must modulate nothing. A
// texture that survives onto the unpainted cells reads as film grain laid
// over the picture rather than as grit inside the paint.
func TestABareCellTakesNoPigmentAtAll(t *testing.T) {
	s, sh := painted(t, 7, "--water", waterSed, "--granulate", "2")
	for _, i := range washes(sh) {
		d := sh.skin[i]
		// Far outside the cell: the paint cannot have reached here.
		got := s.watercolour(d, -10*d.water.scale, sh.field, 7, 0.3, 0.6, sh.paper)
		if got != sh.paper {
			t.Fatalf("cell %d put %v on paper ten cell-widths outside itself", i, got)
		}
	}
	// And a hit with no second cell in range is fully covered, not blank.
	var h cells.Hit
	h.Wall = math.Inf(1)
	if got := s.watercolour(sh.skin[washes(sh)[0]], h.Wall, sh.field, 7, 0.3, 0.6, sh.paper); got == sh.paper {
		t.Error("a point with no wall in range was left unpainted")
	}
}
