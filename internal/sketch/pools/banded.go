package pools

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// A banded circle is filled edge to centre with fine concentric rings that
// overlap their neighbours slightly. It is a different kind of object from
// the other marks here: not a pool with rings drawn inside it, but a disc
// that is *made of* rings, the way a section through a tree is.
//
// Two things do the work, and both are properties of transparent paint
// rather than anything drawn.
//
// The overlap is the first. Where two neighbouring bands cross, the pigment
// of each passes through the other, so every seam darkens on its own — a
// fine contour line at every ring boundary that nobody had to draw and that
// no opaque medium would produce. Butt the bands together instead and the
// mark is a flat gradient with hairline gaps in it.
//
// The colour ramp is the second. Bands step along an interpolation between
// a few palette anchors from rim to centre, so the mark reads as one object
// that changes colour through its depth rather than as a stack of unrelated
// rings. Drawing each band's colour independently gives confetti; a ramp
// with the seams on top gives strata.

// rampAnchors is how many palette colours a mark's ramp passes through.
// Two is a plain graduation and four is a rainbow; three lets a mark turn
// once on its way in, which is what reads as depth.
const rampAnchors = 3

// bandPlan is the ring stack of one banded circle, resolved in the layout
// so the composition can be tested without painting it.
type bandPlan struct {
	mid    []float64       // band centre radii, outermost first
	width  []float64       // band thicknesses, before overlap
	colors []palette.Color // one per band
}

// planBands cuts a disc of radius r into rings of roughly bandWidth and
// gives each one a colour from the ramp.
func (s *Sketch) planBands(rng *rand.Rand, r float64, sc scheme, ramp []palette.Color) bandPlan {
	// Band count comes from a width in canvas units, not from a fixed
	// number, so a mark twice the radius gets twice the rings rather than
	// rings twice as fat and the ring texture keeps its weight across the
	// size ladder — up to a point.
	//
	// That point is MaxBands, and it is where the rule inverts. Past it a
	// large disc would go on accumulating rings until it read as a target
	// rather than as a few concentric washes, and the thing that makes the
	// mark — a ring wide enough to be a band of colour in its own right,
	// with its own wet edge and rim — would be lost to a count. So the
	// biggest discs keep the ring count and widen the rings instead, which
	// is the trade a painter makes for the same reason.
	n := min(max(int(math.Round(r/s.BandWidth)), 2), max(s.MaxBands, 2))
	step := r / float64(n)

	// Anchors are consecutive entries of the luminance-ordered pigments,
	// walked in one direction, not free draws from the whole set. A ramp
	// between two colours from opposite ends of a palette spends most of
	// its length in the muddy middle between them, and the mark comes out
	// grey whatever its endpoints were; neighbours graduate cleanly, and
	// walking one way means the mark also darkens or lightens as it goes
	// inward instead of doubling back on itself.
	// Where the ramp starts and which way it runs both come from the run's
	// scheme, so every banded mark in a strand graduates identically and
	// the strand reads as one repeated mark rather than as a queue of
	// related ones.
	anchors := make([]palette.Color, rampAnchors)
	for i := range anchors {
		anchors[i] = ramp[((sc.at+i*sc.dir)%len(ramp)+len(ramp))%len(ramp)]
	}

	p := bandPlan{
		mid:    make([]float64, n),
		width:  make([]float64, n),
		colors: make([]palette.Color, n),
	}
	for i := range n {
		// Bands are laid outermost first, so the ramp runs rim to centre.
		outer := r - float64(i)*step
		// Widths vary a little: a mark whose rings are all the same
		// thickness reads as a printed target rather than as brushwork.
		w := step * (1 + s.BandOverlap) * (0.85 + 0.3*rng.Float64())
		mid := outer - step/2
		if i == n-1 {
			// The innermost band has to swallow the centre. Left to the
			// overlap it usually does, but at a low overlap it can finish a
			// hair short and leave a pinhole of bare paper in the middle of
			// an otherwise filled disc, which reads as a mistake rather than
			// as a decision. A thickness of twice the radius has no hole
			// left, and the wash draws it as the pool it has become.
			w = math.Max(w, 2*mid)
		}
		p.mid[i] = mid
		p.width[i] = w

		t := float64(i) / float64(max(n-1, 1)) * float64(rampAnchors-1)
		k := min(int(t), rampAnchors-2)
		p.colors[i] = palette.LerpHSL(anchors[k], anchors[k+1], t-float64(k))
	}
	return p
}

// paintBands lays the stack. The innermost band closes over the centre on
// its own: once its thickness reaches its diameter the ring has no hole
// left and the wash draws it as a pool, so the mark fills completely
// without a special case for the middle.
func (s *Sketch) paintBands(cv *paint.Canvas, rng *rand.Rand, w paint.Wash, c circle) {
	for i := range c.bands.mid {
		// Strength falls slightly toward the centre, so the rim stays the
		// strongest statement and the interior does not silt up into a
		// solid disc once the innermost bands start overlapping heavily.
		fade := 1 - 0.25*float64(i)/float64(len(c.bands.mid))
		w.Ring(cv, rng, c.X, c.Y,
			c.bands.mid[i], c.bands.width[i], c.bands.colors[i], c.alpha*fade)
	}
}
