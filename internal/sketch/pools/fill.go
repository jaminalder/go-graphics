package pools

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// How full the sheet is, as one axis.
//
// Filling a frame is not "more circles": it takes the count up, the size
// ladder down, the clearance in — negative, so discs cross — and the margin
// closed, all together. Raising the count alone gives the same picture with
// more specks in it. Those six numbers were being set together by hand on
// every render, which is the signature of a single decision wearing six
// hats, so they are one trait now.
//
// The level resolves to *ranges*, not to numbers — QQL's idea, and the
// thing that makes a trait worth having. "busy" means a busy sheet, not one
// particular busy sheet: two seeds at the same level differ in how many
// marks, how large and how crowded, while both still read as busy.

const dimFill = "fill"

// schema is the sketch's output space. One dimension so far, weighted
// toward the crowded end because that is where this vocabulary is at its
// best: a sparse sheet is a study, a packed one is a picture.
var schema = trait.Schema{
	{
		Name: dimArrange, Key: "a", InName: true,
		Doc:    "how the marks are laid out on the sheet",
		Values: arrangements,
	},
	{
		Name: dimColourway, Key: "c", InName: true,
		Doc:    "which palette the pigments come from",
		Values: colourways,
	},
	{
		Name: dimSizes, Key: "s", InName: true,
		Doc:    "whether every run is the same size",
		Values: sizeVarieties,
	},
	{
		Name: dimFlow, Key: "w", InName: true,
		Doc:    "the field a run of marks follows",
		Values: flows,
	},
	{
		Name: dimFill, Key: "f", InName: true,
		Doc: "how much of the frame the discs take up",
		Values: []trait.Value{
			{Name: "sparse", Weight: 1},
			{Name: "open", Weight: 2},
			{Name: "medium", Weight: 3},
			{Name: "busy", Weight: 3},
			{Name: "packed", Weight: 2},
		},
	},
}

// layout is everything that decides where the marks go and how big they
// are — the composition, as opposed to what each mark is made of. It is
// resolved once per render from the fill trait, then overridden by whatever
// the caller pinned on the command line.
type layout struct {
	count      int     // anchor circles, before satellites
	rungs      int     // steps on the size ladder
	base       float64 // smallest rung, canvas units
	ratio      float64 // ladder step ratio
	satellites float64 // share of anchors given an overlapping companion
	gap        float64 // clearance between anchors, ×radius; negative crosses
	margin     float64 // clear paper at the edge
	candidates int     // darts thrown per anchor
}

// newFill draws a layout for one fill level. Every rung of the ladder
// reaches a disc of roughly a quarter of the canvas whatever the level, so
// even a packed sheet carries a few large marks among the small ones: a
// packed field of one size is a texture, a packed field with three or four
// big ones in it is a composition.
func newFill(level string, rng *rand.Rand) layout {
	// Darts per anchor is deliberately low at every level. Throwing many
	// maximises the spacing, and perfectly spaced marks are as inert as a
	// grid; a handful leaves passages that crowd and passages of open paper.
	l := layout{candidates: 7}
	switch level {
	case "sparse":
		l.count = 5 + rng.IntN(4)
		l.rungs, l.base = 2, rnd.Uniform(rng, 0.105, 0.125)
		l.ratio, l.gap, l.margin = rnd.Uniform(rng, 1.50, 1.70), rnd.Uniform(rng, 0.06, 0.12), rnd.Uniform(rng, 0.050, 0.062)
		l.satellites = rnd.Uniform(rng, 0.55, 0.75)
	case "open":
		l.count = 10 + rng.IntN(5)
		l.rungs, l.base = 3, rnd.Uniform(rng, 0.078, 0.092)
		l.ratio, l.gap, l.margin = rnd.Uniform(rng, 1.55, 1.70), rnd.Uniform(rng, 0.04, 0.09), rnd.Uniform(rng, 0.040, 0.050)
		l.satellites = rnd.Uniform(rng, 0.55, 0.72)
	case "medium":
		l.count = 18 + rng.IntN(9)
		l.rungs, l.base = 4, rnd.Uniform(rng, 0.060, 0.070)
		l.ratio, l.gap, l.margin = rnd.Uniform(rng, 1.50, 1.60), rnd.Uniform(rng, 0.02, 0.06), rnd.Uniform(rng, 0.030, 0.040)
		l.satellites = rnd.Uniform(rng, 0.50, 0.68)
	case "busy":
		l.count = 32 + rng.IntN(13)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.046, 0.055)
		l.ratio, l.gap, l.margin = rnd.Uniform(rng, 1.45, 1.58), rnd.Uniform(rng, 0.00, 0.03), rnd.Uniform(rng, 0.025, 0.032)
		l.satellites = rnd.Uniform(rng, 0.48, 0.62)
	default: // packed
		l.count = 55 + rng.IntN(16)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.036, 0.045)
		l.ratio, l.gap, l.margin = rnd.Uniform(rng, 1.52, 1.66), rnd.Uniform(rng, -0.04, 0.01), rnd.Uniform(rng, 0.018, 0.026)
		l.satellites = rnd.Uniform(rng, 0.42, 0.58)
	}
	return l
}

// ladder is the size ladder this sheet's marks come off — see rnd.Ladder
// for why the rungs are discrete.
func (l layout) ladder() (radii, weights []float64) {
	return rnd.Ladder(l.base, l.ratio, l.rungs, 0.7)
}

// bestCandidate throws darts and keeps the one furthest from what is
// already on the paper, which spaces the marks without the regularity a
// grid would impose.
func (l layout) bestCandidate(rng *rand.Rand, index *geom.Index, aspect, r float64) (x, y float64, ok bool) {
	bestD := -1.0
	for range max(l.candidates, 1) {
		cx := l.margin + r + rng.Float64()*(aspect-2*(l.margin+r))
		cy := l.margin + r + rng.Float64()*(1-2*(l.margin+r))
		c := geom.Circle{X: cx, Y: cy, R: r}
		if !index.FitsWithGap(c, r*l.gap) {
			continue
		}
		d := nearest(index, cx, cy)
		if d > bestD {
			bestD, x, y, ok = d, cx, cy, true
		}
	}
	return x, y, ok
}

// nearest is the distance from a point to the closest circle centre placed
// so far, or a large number when the paper is still empty.
func nearest(index *geom.Index, x, y float64) float64 {
	best := math.Inf(1)
	for _, c := range index.Circles() {
		best = math.Min(best, math.Hypot(c.X-x, c.Y-y))
	}
	if math.IsInf(best, 1) {
		return 1e6
	}
	return best
}

// inPaper reports whether a satellite still sits on the sheet. Companions
// are allowed the tighter half-margin — they are placed against a parent
// rather than into open space — but nothing may be clipped by the edge.
func (l layout) inPaper(c geom.Circle, aspect float64) bool {
	return c.X-c.R > l.margin*0.5 && c.X+c.R < aspect-l.margin*0.5 &&
		c.Y-c.R > l.margin*0.5 && c.Y+c.R < 1-l.margin*0.5
}

// spacing is the distance between *walks*, not between marks. A walk fills
// in along its own length, so seeding at mark spacing lays every strand on
// top of the last one; seeding much wider leaves the sheet in stripes.
//
// It comes off the smallest rung, not the mean. The ladder reaches a disc
// of a quarter of the canvas, so its mean is dragged up by a rung that
// comes up one time in ten — seeding at that spacing puts a handful of
// walks on the whole sheet and most of it stays bare. Three of the
// commonest mark is the width at which strands read as strands.
func (l layout) spacing(radii []float64) float64 {
	return radii[0] * 3
}

// lattice is the seed spacing when every run is the same size: exactly one
// mark-pitch, so consecutive walks land shoulder to shoulder and the rows
// register into a grid. Any wider and the rows read as stripes; any
// narrower and every second walk is rejected wholesale.
func (l layout) lattice(r float64) float64 {
	return 2*r + r*math.Max(l.gap, 0)
}
