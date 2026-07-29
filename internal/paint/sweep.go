package paint

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// Sweeps fill a shape by dragging a Brush along a family of paths that
// follow the shape's own flow — concentric rings for anything round.
// Because the coat is laid by the brush rather than flood-filled, the
// silhouette frays and the fill streaks along the sweep direction with
// no extra marks involved. Adding a shape means adding a path family;
// the brush and its texture stay the same.

// Ring returns a closed wobbly ring around (cx, cy) at radius r. turns
// slightly over 1 makes the stroke overlap its own start, the way a hand
// closes a circle. wobble is the radius deviation in canvas units.
func Ring(rng *rand.Rand, cx, cy, r, turns, wobble float64) []Pt {
	start := rng.Float64() * 2 * math.Pi
	pts := Spiral(rng, cx, cy, r, r, turns, wobble)
	sin, cos := math.Sin(start), math.Cos(start)
	for i, p := range pts {
		dx, dy := p.X-cx, p.Y-cy
		pts[i] = Pt{X: cx + dx*cos - dy*sin, Y: cy + dx*sin + dy*cos}
	}
	return pts
}

// SweepRing lays a coat of col over the annulus rIn..rOut by dragging the
// brush along overlapping concentric rings, reloading it between passes
// so bristle gaps do not line up. rIn = 0 fills a solid disc. wobble is
// the per-pass radius deviation as a fraction of the pass radius.
func SweepRing(c *Canvas, rng *rand.Rand, b Brush, cx, cy, rIn, rOut float64, col palette.Color, alpha, wobble float64) {
	if rOut <= rIn {
		return
	}
	w := b.Width()
	// Passes sit half a ferrule inside each rim so the coat lands on the
	// band rather than straddling it, and overlap by ~45%.
	lo := rIn + w/2
	hi := rOut - w/2
	if hi < lo {
		mid := (rIn + rOut) / 2
		lo, hi = mid, mid
	}
	if rIn <= 0 {
		// Seal the centre with a stab of the brush. The innermost pass
		// nominally reaches it, but only through its innermost bristle,
		// which traces a circle a fraction of a ferrule across — short
		// enough that the lift-off wave can hold that one bristle off the
		// surface for the entire revolution and leave a pinhole in the
		// middle of every disc.
		c.Dab(cx, cy, w/2, col, alpha)
	}
	passes := 1
	if hi > lo {
		passes = int(math.Ceil((hi-lo)/(w*0.45))) + 1
	}
	for i := range passes {
		rp := lo
		if passes > 1 {
			rp = lo + (hi-lo)*float64(i)/float64(passes-1)
		}
		if rp < w*0.3 {
			// Too tight to sweep: the brush simply sits on the centre.
			c.Dab(cx, cy, math.Max(rp, w*0.3), col, alpha)
			continue
		}
		// Just over one turn, varied per pass so the lift-off seams
		// scatter around the shape instead of stacking on one radius.
		turns := 1.05 + 0.25*rng.Float64()
		// Cap the grain so the lift-off wave completes a few cycles per
		// revolution. On a ring shorter than its own grain every bristle
		// holds one constant coverage the whole way round, so a lifted
		// bristle drops a complete concentric ring rather than a streak —
		// which is how a small swept disc ends up looking like a target.
		// Reload through Grain, which rearranges the bundle either way.
		pb := b.Grain(rng, math.Min(b.grain, 2*math.Pi*rp*turns/2.5))
		pb.Stroke(c, Ring(rng, cx, cy, rp, turns, wobble*rp), col, alpha)
	}
}
