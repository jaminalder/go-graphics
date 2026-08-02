package paint

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// disc is the simplest region there is.
type disc struct{ cx, cy, r float64 }

func (d disc) Depth(u, v float64) float64 { return d.r - math.Hypot(u-d.cx, v-d.cy) }

// crescent is a disc with a bite taken out of it — the shape a radial pool
// cannot describe, and the reason Fill exists.
type crescent struct{ outer, bite disc }

func (c crescent) Depth(u, v float64) float64 {
	return math.Min(c.outer.Depth(u, v), -c.bite.Depth(u, v))
}

func boxOf(d disc) Box {
	return Box{MinU: d.cx - d.r, MinV: d.cy - d.r, MaxU: d.cx + d.r, MaxV: d.cy + d.r}
}

func rngFor(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 5)) }

// TestOneRoundRegionIsOneTouch. A brush filling a round shape puts one load
// down; several overlapping ones leave a cluster of blobs with the corners
// bare, which is exactly what the first version of the covering produced.
func TestOneRoundRegionIsOneTouch(t *testing.T) {
	d := disc{0.5, 0.5, 0.2}
	got := cover(rngFor(1), d, boxOf(d))
	if len(got) != 1 {
		t.Fatalf("a disc took %d touches, want 1", len(got))
	}
	// And that touch has to be the inscribed circle, near enough. Darts
	// alone never land on the deepest point; without the hill climb every
	// pool came out a fraction of the size it should have been.
	if math.Abs(got[0].r-d.r) > 0.02*d.r {
		t.Errorf("the touch has radius %.4f against the region's %.4f — it did not find the deepest point", got[0].r, d.r)
	}
	if math.Hypot(got[0].u-d.cx, got[0].v-d.cy) > 0.02*d.r {
		t.Errorf("the touch sits at (%.3f,%.3f), not at the centre", got[0].u, got[0].v)
	}
}

// TestAConcaveRegionTakesSeveralTouches is the claim the package is built
// on: a Wash pool is radial and cannot be a crescent, but several round
// touches sized to the region's own depth can cover one between them. If
// this fails, an irregular cell gets a circle painted in the middle of it.
func TestAConcaveRegionTakesSeveralTouches(t *testing.T) {
	c := crescent{outer: disc{0.5, 0.5, 0.25}, bite: disc{0.62, 0.5, 0.17}}
	got := cover(rngFor(2), c, boxOf(c.outer))
	if len(got) < 2 {
		t.Fatalf("a crescent took %d touches, want at least 2", len(got))
	}
	// Every touch has to be inside the region, or the paint is landing in
	// the bite.
	for i, p := range got {
		if c.Depth(p.u, p.v) <= 0 {
			t.Errorf("touch %d at (%.3f,%.3f) is outside the region", i, p.u, p.v)
		}
	}
	// And between them they have to cover it. Sampled over the region: a
	// point deep inside with no touch reaching it is bare paper in the
	// middle of a painted shape.
	miss, total := 0, 0
	for i := range 60 {
		for j := range 60 {
			u, v := 0.25+0.5*float64(i)/60, 0.25+0.5*float64(j)/60
			d := c.Depth(u, v)
			if d < 0.02 {
				continue // near the edge, where a round touch legitimately falls short
			}
			total++
			covered := false
			for _, p := range got {
				if math.Hypot(p.u-u, p.v-v) <= p.r*1.3 {
					covered = true
					break
				}
			}
			if !covered {
				miss++
			}
		}
	}
	if total == 0 {
		t.Fatal("no interior samples")
	}
	if share := float64(miss) / float64(total); share > 0.08 {
		t.Errorf("%.0f%% of the crescent's interior has no touch over it", share*100)
	}
}

// TestTheCoveringIsResolutionIndependent — invariant 2. The touches decide
// the composition, so if any of them depended on the pixel grid a print
// would be a different picture from its preview. The first version had a
// minimum radius in pixels for exactly the reason it looks harmless: it
// stopped sub-pixel pools. It also made the covering resolution dependent.
func TestTheCoveringIsResolutionIndependent(t *testing.T) {
	c := crescent{outer: disc{0.5, 0.5, 0.22}, bite: disc{0.6, 0.44, 0.12}}
	a := cover(rngFor(3), c, boxOf(c.outer))
	b := cover(rngFor(3), c, boxOf(c.outer))
	if len(a) != len(b) {
		t.Fatalf("two identical calls gave %d and %d touches", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("touch %d: %v then %v", i, a[i], b[i])
		}
	}
	// Nothing in cover's signature can see the canvas, which is the
	// structural half of the same claim: with no scale to consult it cannot
	// accidentally regain a dependence on the pixel grid.
	_ = cover
}

// TestAnEmptyRegionPaintsNothing — a caller may hand over a cell a heavier
// neighbour swallowed, and it must not become a stray blob.
func TestAnEmptyRegionPaintsNothing(t *testing.T) {
	outside := disc{5, 5, 0.1}
	if got := cover(rngFor(4), outside, Box{MinU: 0, MinV: 0, MaxU: 0.2, MaxV: 0.2}); len(got) != 0 {
		t.Errorf("a region outside its own box took %d touches", len(got))
	}
	if got := cover(rngFor(4), disc{0.5, 0.5, 0.1}, Box{}); len(got) != 0 {
		t.Errorf("an empty box took %d touches", len(got))
	}
}

// TestASliverIsStillPainted. A packed sheet is mostly slivers, and a region
// too thin for the darts to land in has to get paint anyway or the picture
// comes out full of holes where its smallest shapes were.
func TestASliverIsStillPainted(t *testing.T) {
	tiny := disc{0.5, 0.5, 0.0008}
	if got := cover(rngFor(5), tiny, boxOf(tiny)); len(got) == 0 {
		t.Error("a sliver got no paint at all")
	}
}

// TestFillActuallyPutsPaintDown — the end to end claim, that a region handed
// to Fill comes back covered in pigment and the paper outside it does not.
func TestFillActuallyPutsPaintDown(t *testing.T) {
	paper := palette.Color{R: 1, G: 1, B: 1}
	cv := NewCanvas(200, 200, paper)
	d := disc{0.5, 0.5, 0.2}
	w := DefaultWash(9)
	w.Fill(cv, rngFor(6), d, boxOf(d), palette.Color{R: 0.1, G: 0.2, B: 0.6}, 0.6, 1.05)

	img := cv.Image()
	at := func(u, v float64) float64 {
		o := img.PixOffset(int(u*200), int(v*200))
		return (float64(img.Pix[o]) + float64(img.Pix[o+1]) + float64(img.Pix[o+2])) / (3 * 255)
	}
	if in := at(0.5, 0.5); in > 0.7 {
		t.Errorf("the middle of the region is %.2f bright — no paint went down", in)
	}
	if out := at(0.05, 0.05); out < 0.95 {
		t.Errorf("the corner of the canvas is %.2f bright — paint escaped the region", out)
	}
}
