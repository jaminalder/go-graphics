package paint

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func washRNG(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 99)) }

var (
	yellow = palette.Color{R: 0.95, G: 0.85, B: 0.15}
	cyan   = palette.Color{R: 0.15, G: 0.6, B: 0.85}
)

func at(c *Canvas, u, v float64) palette.Color {
	x := int(u * c.scale)
	y := int(v * c.scale)
	return c.pix[y*c.w+x]
}

// TestWashesMixWhereTheyCross is the point of the whole model: two
// transparent pigments crossing must produce a third colour, not
// whichever was laid second.
func TestWashesMixWhereTheyCross(t *testing.T) {
	c := NewCanvas(400, 400, white)
	w := DefaultWash(1)
	w.Grain, w.Ragged = 0, 0.02 // isolate the blend from shape and tooth
	w.Pool(c, washRNG(1), 0.4, 0.5, 0.16, yellow, 0.85)
	w.Pool(c, washRNG(2), 0.6, 0.5, 0.16, cyan, 0.85)

	onlyY := at(c, 0.3, 0.5)
	onlyC := at(c, 0.7, 0.5)
	both := at(c, 0.5, 0.5)

	// The crossing is not simply the second wash repainted over the first.
	if math.Abs(both.R-onlyC.R) < 0.02 && math.Abs(both.B-onlyC.B) < 0.02 {
		t.Errorf("crossing %v matches the upper wash %v — pigment is replacing, not mixing", both, onlyC)
	}
	// Two absorbing layers pass less light than either alone.
	if both.Luminance() >= onlyY.Luminance() || both.Luminance() >= onlyC.Luminance() {
		t.Errorf("crossing (lum %.3f) is not darker than either wash alone (%.3f, %.3f)",
			both.Luminance(), onlyY.Luminance(), onlyC.Luminance())
	}
	// Yellow absorbs blue and cyan absorbs red, so the crossing keeps
	// green: it must not simply be a darker copy of either parent.
	if both.G <= both.R || both.G <= both.B {
		t.Errorf("yellow over cyan gave %v, want green dominant", both)
	}
}

// TestWashOrderShiftsCrossingOnlySlightly bounds the one thing that
// stops pools being order-independent, and records why this file does
// not implement the interleaved layer ordering the technique is usually
// described with — five layers of blob A, five of B, five of A again, so
// that neither ends up wholly on top.
//
// Absorption alone commutes exactly: multiplying transmittances gives
// the same answer either way round. The back-scattering floor breaks
// that, and correctly so, since the upper glaze scatters light before
// the lower one ever sees it. The case below is the worst one available
// — two saturated complementary pigments, both at full strength, fully
// crossed — and it moves the crossing by around 4%. On the muted
// palettes these sketches use it is far less. That is the trade being
// made: pools stay independent and cheap, at the cost of a shift this
// test keeps honest.
func TestWashOrderShiftsCrossingOnlySlightly(t *testing.T) {
	// Each pool keeps its own colour, position and seed; only the order
	// they are laid in changes. (Swapping the colours between the two
	// positions instead would compare two different pictures.)
	paint := func(swap bool) *Canvas {
		c := NewCanvas(300, 300, white)
		w := DefaultWash(3)
		layY := func() { w.Pool(c, washRNG(11), 0.42, 0.5, 0.16, yellow, 0.8) }
		layC := func() { w.Pool(c, washRNG(12), 0.58, 0.5, 0.16, cyan, 0.8) }
		if swap {
			layC()
			layY()
		} else {
			layY()
			layC()
		}
		return c
	}
	a, b := paint(false), paint(true)
	got, want := at(a, 0.5, 0.5), at(b, 0.5, 0.5)
	worst := 0.0
	for _, d := range []float64{got.R - want.R, got.G - want.G, got.B - want.B} {
		worst = math.Max(worst, math.Abs(d))
	}
	if worst > 12.0/255 {
		t.Errorf("swapping paint order moved the crossing by %.1f/255: %v vs %v — too much to ignore ordering",
			worst*255, got, want)
	}
}

// TestPoolRimIsDarkerThanItsMiddle pins the most recognisable watercolour
// cue. Sampling many angles because the rim is deliberately partial —
// only the crisp arcs carry one.
func TestPoolRimIsDarkerThanItsMiddle(t *testing.T) {
	c := NewCanvas(600, 600, white)
	w := DefaultWash(7)
	// Isolate the rim: granulation and pooling both vary the interior,
	// and either can out-darken the rim on an individual ray.
	w.Grain, w.Mottle = 0, 0
	const cx, cy, r = 0.5, 0.5, 0.3
	w.Pool(c, washRNG(5), cx, cy, r, cyan, 0.7)

	// Walk out along each ray and find where the pigment is heaviest. In
	// a pool that has a rim, the darkest point on most rays lies in the
	// outer part rather than at the centre. Scanning rather than probing
	// a fixed radius matters because the boundary is ragged: the rim sits
	// wherever that ray's edge happens to fall.
	const samples, steps = 64, 60
	outerPeak := 0
	for i := range samples {
		a := float64(i) * 2 * math.Pi / samples
		cos, sin := math.Cos(a), math.Sin(a)
		best, bestAt := 2.0, 0.0
		for k := 1; k <= steps; k++ {
			rr := r * 1.4 * float64(k) / steps
			if lum := at(c, cx+rr*cos, cy+rr*sin).Luminance(); lum < best {
				best, bestAt = lum, rr
			}
		}
		if bestAt > 0.6*r {
			outerPeak++
		}
	}
	if outerPeak < samples/3 {
		t.Errorf("pigment peaks in the outer part of only %d/%d rays; the pool has no rim", outerPeak, samples)
	}
}

func TestPoolIsDeterministicAndBounded(t *testing.T) {
	build := func() *Canvas {
		c := NewCanvas(240, 240, white)
		DefaultWash(9).Pool(c, washRNG(4), 0.5, 0.5, 0.2, cyan, 0.8)
		return c
	}
	a, b := build(), build()
	for i := range a.pix {
		if a.pix[i] != b.pix[i] {
			t.Fatal("same seed must give the same pool")
		}
	}
	// Nothing may land beyond the raggedness budget.
	lim := 0.2 * 1.5
	for y := range 240 {
		for x := range 240 {
			if a.pix[y*240+x] == white {
				continue
			}
			dx := (float64(x)+0.5)/240 - 0.5
			dy := (float64(y)+0.5)/240 - 0.5
			if d := math.Hypot(dx, dy); d > lim {
				t.Fatalf("pigment at distance %v, beyond the %v budget", d, lim)
			}
		}
	}
}

func TestPoolIgnoresDegenerateInput(t *testing.T) {
	c := NewCanvas(64, 64, white)
	blank := NewCanvas(64, 64, white)
	w := DefaultWash(2)
	w.Pool(c, washRNG(1), 0.5, 0.5, 0, cyan, 0.8)     // no radius
	w.Pool(c, washRNG(1), 0.5, 0.5, 0.2, cyan, 0)     // no pigment
	w.Pool(c, washRNG(1), 0.5, 0.5, 0.0001, cyan, .8) // sub-pixel
	for i := range c.pix {
		if c.pix[i] != blank.pix[i] {
			t.Fatal("a degenerate pool painted something")
		}
	}
}

// TestToothHasNoLatticeAlignment guards the bug that made large pools
// look pixelated: the paper grain was square-lattice value noise, whose
// extrema sit exactly on the lattice nodes. Sampled on the nodes it
// spans the full range, sampled between them it is an average of four
// neighbours and barely varies at all — so the interior resolved into a
// regular array of soft blobs once cells grew past a pixel or two. Cell
// noise scatters its feature points, so the two agree.
func TestToothHasNoLatticeAlignment(t *testing.T) {
	const cell, n = 8.0, 48
	spread := func(offset float64) float64 {
		var vals []float64
		mean := 0.0
		for i := range n {
			for j := range n {
				v := tooth(5, (float64(i)+offset)*cell, (float64(j)+offset)*cell, cell)
				vals = append(vals, v)
				mean += v
			}
		}
		mean /= float64(len(vals))
		varr := 0.0
		for _, v := range vals {
			varr += (v - mean) * (v - mean)
		}
		return varr / float64(len(vals))
	}
	onNode, betweenNodes := spread(0), spread(0.5)
	if onNode == 0 || betweenNodes == 0 {
		t.Fatalf("tooth is constant (on %v, between %v)", onNode, betweenNodes)
	}
	if r := onNode / betweenNodes; r < 0.5 || r > 2 {
		t.Errorf("grain varies %.2fx more on the lattice than between it — the texture is grid-aligned", r)
	}
}

// TestMottleScalesWithThePool pins the other half of that fix. Paper
// tooth is physically absolute, but the pooling of pigment *within* a
// wash belongs to the wash, so its scale has to follow the radius.
// Pinned to an absolute wavelength instead, a large pool shows many
// cycles of it and a small one almost none, so the two read as
// different materials.
//
// Counting blotches rather than measuring their depth: an absolute
// wavelength changes how *many* there are across a pool, while their
// amplitude stays much the same either way.
func TestMottleScalesWithThePool(t *testing.T) {
	blotchesAcross := func(r float64) float64 {
		const px = 700
		c := NewCanvas(px, px, white)
		w := DefaultWash(11)
		w.Grain, w.Edge = 0, 0 // isolate pooling from tooth and rim
		w.Pool(c, washRNG(3), 0.5, 0.5, r, cyan, 0.75)

		total, lines := 0, 0
		for _, off := range []float64{-0.22, 0, 0.22} {
			y := int((0.5 + off*r) * px)
			// Always the same number of samples across the interior, so
			// the transect itself is measured in pool-radii, not pixels.
			const n = 121
			vals := make([]float64, n)
			mean := 0.0
			for k := range n {
				f := -0.45 + 0.9*float64(k)/(n-1)
				x := int((0.5 + f*r) * px)
				vals[k] = c.pix[y*px+x].Luminance()
				mean += vals[k]
			}
			mean /= n
			cross := 0
			for k := 1; k < n; k++ {
				if (vals[k-1]-mean)*(vals[k]-mean) < 0 {
					cross++
				}
			}
			total += cross
			lines++
		}
		return float64(total) / float64(lines)
	}
	small, large := blotchesAcross(0.06), blotchesAcross(0.22)
	if small == 0 && large == 0 {
		t.Fatal("no interior variation at all")
	}
	if d := math.Abs(large - small); d > 3 {
		t.Errorf("a large pool shows %.1f blotches across and a small one %.1f — pooling is not following the pool", large, small)
	}
}
