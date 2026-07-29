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

// TestWashOrderBarelyMatters records why this file does not bother with
// the interleaved layer ordering the technique is usually described
// with — paint five layers of blob A, five of B, five of A again, so
// neither ends up wholly on top.
//
// Absorption alone commutes exactly: multiplying transmittances gives
// the same answer either way round. The back-scattering floor breaks
// that, and correctly so, since the upper glaze scatters light before
// the lower one ever sees it. But it breaks it by only a hair, so
// interleaving would buy nothing a viewer could see, and pools can be
// painted one at a time.
func TestWashOrderBarelyMatters(t *testing.T) {
	paint := func(first, second palette.Color, s1, s2 uint64) *Canvas {
		c := NewCanvas(300, 300, white)
		w := DefaultWash(3)
		w.Pool(c, washRNG(s1), 0.42, 0.5, 0.16, first, 0.8)
		w.Pool(c, washRNG(s2), 0.58, 0.5, 0.16, second, 0.8)
		return c
	}
	a := paint(yellow, cyan, 11, 12)
	b := paint(cyan, yellow, 12, 11)
	got, want := at(a, 0.5, 0.5), at(b, 0.5, 0.5)
	worst := 0.0
	for _, d := range []float64{got.R - want.R, got.G - want.G, got.B - want.B} {
		worst = math.Max(worst, math.Abs(d))
	}
	// A couple of levels out of 255: present, but far below anything the
	// eye picks out of a field of overlapping marks.
	if worst > 8.0/255 {
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
	w.Grain = 0
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
