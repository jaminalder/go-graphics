// Package pools implements sketch 008: watercolour circles on paper
// (spec: docs/sketches/008-pools.md).
//
// Sketch 006 uses the wash model from internal/paint at dot size, where a
// pool is a smudge. This one uses it at the size the model was actually
// built for: few marks, large, near-circular, and overlapping on purpose,
// so the wash has to hold up as a shape — a clean boundary, a rim where
// the water dried, granulation in the middle, and a believable third
// colour wherever two discs cross.
//
// The composition is deliberately plain. Circles at a handful of discrete
// sizes, scattered with blue-noise spacing, each one either a filled disc,
// an open ring, or a small nest of concentric rings. Nothing is asked of
// the layout beyond giving the paint room to be seen.
package pools

import (
	"image"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamLayout = 1 // placement, sizes, structure, colour
	streamPaint  = 2 // per-pool wash deformation
)

// saltPaper salts the granulation lattice so the paper texture is
// independent of the layout.
const saltPaper = 0x706170 // "pap"

// saltGround salts the ground's own granulation lattice, so the sky's
// tooth is not a copy of the marks'.
const saltGround = 0x67726e // "grn"

// Sketch holds the structural knobs. Per-seed variation happens in place.
type Sketch struct {
	Count        int     // anchor circles, before satellites
	Rungs        int     // steps on the size ladder
	Base         float64 // smallest rung, canvas units
	Ratio        float64 // ladder step ratio
	Satellites   float64 // share of anchors given an overlapping companion
	Ragged       float64 // wash edge deviation; shoal's blob is 0.22
	Rings        float64 // share of circles carrying inner rings
	Open         float64 // share painted as annuli rather than discs
	Glaze        float64 // share carrying a second pigment on top
	Banded       float64 // share filled with fine concentric rings
	BandWidth    float64 // ring pitch of a banded circle, canvas units
	BandOverlap  float64 // how far neighbouring rings cross, ×pitch
	MaxBands     int     // most rings a banded circle may be built from
	Alpha        float64 // pool strength
	Pigments     int     // palette colours in play
	Ground       float64 // strength of the painted ground wash; 0 is bare paper
	GroundBlotch float64 // wavelength of the ground's unevenness, canvas units
	Margin       float64 // clear paper at the edge
	Gap          float64 // clearance between anchors, ×radius
	Candidates   int     // darts thrown per anchor

	opts cliOptions
}

// New returns the sketch with its defaults.
func New() *Sketch {
	return &Sketch{
		Count:        22,
		Rungs:        5,
		Base:         0.030,
		Ratio:        1.55,
		Satellites:   0.45,
		Ragged:       0.055,
		Rings:        0.34,
		Open:         0.28,
		Glaze:        0.16,
		Banded:       0.3,
		BandWidth:    0.022,
		BandOverlap:  0.4,
		MaxBands:     5,
		Alpha:        0.74,
		Pigments:     4,
		Ground:       0.5,
		GroundBlotch: 0.34,
		Margin:       0.06,
		Gap:          0.12,
		Candidates:   7,
	}
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "pools" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "watercolour circles on paper: transparent pools and open rings, overlapping"
}

// kind is what a circle is built as.
type kind uint8

const (
	kindPlain  kind = iota // one pool of one pigment
	kindNested             // a pool with concentric rings glazed inside it
	kindOpen               // an annulus, bare paper in the middle
	kindGlaze              // a pool with a second, offset pigment on it
	kindBanded             // a disc made of fine overlapping concentric rings
)

// circle is one placed mark, fully specified before anything is painted.
type circle struct {
	geom.Circle
	kind    kind
	pigment palette.Color
	second  palette.Color // glaze / ring pigment
	rings   int           // inner rings, 0 for none
	band    float64       // annulus thickness, kindOpen only
	offset  float64       // glaze displacement, ×R
	angle   float64       // glaze direction
	alpha   float64
	bands   bandPlan // kindBanded only
}

// Render implements sketch.Sketch. Painting is stamp-based, so Context.AA
// and Deep are unused — the wash anti-aliases itself.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	aspect := float64(ctx.Width) / float64(ctx.Height)
	rng := ctx.RNG(streamLayout)

	paper, tint, bag, ramp := s.inks(byLuminance(ctx.Palette.Colors), rng)

	circles := s.plan(rng, aspect, bag, ramp)

	// The canvas starts as bare paper; the ground is glazed onto it, on its
	// own stream so that changing the marks never repaints the sky.
	cv := paint.NewCanvas(ctx.Width, ctx.Height, paper)
	if s.Ground > 0 {
		groundWash(ctx.Seed^saltGround).Ground(cv, tint, s.Ground, s.GroundBlotch)
	}

	wash := paint.DefaultWash(ctx.Seed ^ saltPaper)
	wash.Ragged = s.Ragged
	wash.GrainScale = paperTooth
	prng := ctx.RNG(streamPaint)
	for _, c := range circles {
		s.paint(cv, prng, wash, c)
	}
	return cv.Image(), nil
}

// byLuminance copies a palette into darkest-first order.
func byLuminance(cols []palette.Color) []palette.Color {
	out := append([]palette.Color(nil), cols...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Luminance() < out[j].Luminance() })
	return out
}

// inks decides the paper and the pigment draw pile. The pile is weighted
// so one pigment dominates and the rest are progressively rarer: drawing
// uniformly gives every colour equal presence, which reads as a sampler
// rather than as a picture.
func (s *Sketch) inks(byLum []palette.Color, rng *rand.Rand) (paper, tint palette.Color, bag, ramp []palette.Color) {
	lightest := byLum[len(byLum)-1]
	// Bare paper: warm, close to white, and never one of the pigments —
	// every mark has to be able to sit on it transparently. The ground
	// colour is a single tint glazed over it, the palette's own lightest
	// colour softened, so the sky belongs to the same set of paints as
	// everything standing on it.
	paper = lightest.Lighten(0.86).Desaturate(0.7)
	tint = groundTint(byLum)

	n := min(max(s.Pigments, 1), len(byLum))
	// Take the darkest end of the palette: a transparent glaze of a pale
	// colour on pale paper deposits nothing anyone can see.
	// Two views of the same pigments. The bag is shuffled and weighted, so
	// drawing from it gives one dominant colour and rare accents. The ramp
	// keeps them in luminance order, because a banded mark graduates
	// between neighbours on it — see banded.go.
	ramp = append([]palette.Color(nil), byLum[:n]...)
	pick := append([]palette.Color(nil), ramp...)
	rng.Shuffle(len(pick), func(i, j int) { pick[i], pick[j] = pick[j], pick[i] })

	weights := []int{10, 6, 3, 2, 1, 1}
	for i, c := range pick {
		w := 1
		if i < len(weights) {
			w = weights[i]
		}
		for range w {
			bag = append(bag, c)
		}
	}
	return paper, tint, bag, ramp
}

// groundTint picks the colour the sheet is washed with: the palette's own
// lightest, softened. A few palettes have a near-white as their lightest —
// a paper colour rather than a paint — and it cannot tint anything: the
// ground comes out as bare paper with a rumour of colour on it and none of
// the wash's structure shows. Those borrow from the next colour down until
// the tint has body. The threshold is set above every ordinary palette's
// lightest, so this is a rescue for the handful that need it and not a
// correction applied to the rest.
func groundTint(byLum []palette.Color) palette.Color {
	c := byLum[len(byLum)-1]
	const tooPale, target = 0.88, 0.85
	if l := c.Luminance(); l > tooPale && len(byLum) > 1 {
		next := byLum[len(byLum)-2]
		if span := l - next.Luminance(); span > 1e-6 {
			c = palette.Lerp(c, next, mathx.Clamp01((l-target)/span))
		}
	}
	return c.Desaturate(0.3)
}

// ladder is the size ladder: a geometric run of radii, weighted toward the
// small end. Discrete rungs rather than a continuous range are the whole
// point — a scatter of continuously varying radii has no size hierarchy to
// read, only a spread.
func (s *Sketch) ladder() (radii []float64, weights []float64) {
	r := s.Base
	w := 12.0
	for range max(s.Rungs, 1) {
		radii = append(radii, r)
		weights = append(weights, w)
		r *= s.Ratio
		// A gentle falloff. Steeper and the top rungs never come up, which
		// leaves a field of same-sized specks with no hierarchy to read.
		w *= 0.7
	}
	return radii, weights
}

// plan places every circle and settles what it is made of, largest first
// so the paint order lets small marks settle on top.
func (s *Sketch) plan(rng *rand.Rand, aspect float64, bag, ramp []palette.Color) []circle {
	radii, weights := s.ladder()
	maxR := radii[len(radii)-1]
	index := geom.NewIndex(aspect, 1, maxR)

	var out []circle
	for range s.Count {
		rung := weightedPick(rng, weights)
		r := radii[rung]
		x, y, ok := s.bestCandidate(rng, index, aspect, r)
		if !ok {
			continue
		}
		c := geom.Circle{X: x, Y: y, R: r}
		index.Insert(c)
		out = append(out, s.build(rng, c, bag, ramp))

		// A companion, deliberately placed to cross its parent. Left to
		// chance the overlaps either never happen or arrive as a pile-up;
		// the crossings are the subject, so they are placed on purpose.
		//
		// It comes from one rung down, not from the bottom of the ladder:
		// a speck crossing a large disc shows nothing, and the mixing only
		// reads when both parties to it are worth looking at.
		if rng.Float64() >= s.Satellites {
			continue
		}
		sr := radii[max(rung-1, 0)]
		if rung == 0 {
			sr = r * 0.72
		}
		th := rng.Float64() * 2 * math.Pi
		d := (r + sr) * (0.42 + 0.32*rng.Float64())
		sc := geom.Circle{X: x + d*math.Cos(th), Y: y + d*math.Sin(th), R: sr}
		if !s.inPaper(sc, aspect) {
			continue
		}
		index.Insert(sc)
		out = append(out, s.build(rng, sc, bag, ramp))
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].R > out[j].R })
	return out
}

// bestCandidate throws darts and keeps the one furthest from what is
// already on the paper, which spaces the marks without the regularity a
// grid would impose.
//
// Few darts on purpose. Throwing many maximises the spacing, and perfectly
// spaced marks are as inert as a grid; a handful leaves the sheet with
// passages that crowd and passages of open paper.
func (s *Sketch) bestCandidate(rng *rand.Rand, index *geom.Index, aspect, r float64) (x, y float64, ok bool) {
	bestD := -1.0
	for range max(s.Candidates, 1) {
		cx := s.Margin + r + rng.Float64()*(aspect-2*(s.Margin+r))
		cy := s.Margin + r + rng.Float64()*(1-2*(s.Margin+r))
		c := geom.Circle{X: cx, Y: cy, R: r}
		if !index.FitsWithGap(c, r*s.Gap) {
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

func (s *Sketch) inPaper(c geom.Circle, aspect float64) bool {
	return c.X-c.R > s.Margin*0.5 && c.X+c.R < aspect-s.Margin*0.5 &&
		c.Y-c.R > s.Margin*0.5 && c.Y+c.R < 1-s.Margin*0.5
}

// build settles what one placed circle is made of.
func (s *Sketch) build(rng *rand.Rand, g geom.Circle, bag, ramp []palette.Color) circle {
	c := circle{
		Circle:  g,
		pigment: bag[rng.IntN(len(bag))],
		second:  bag[rng.IntN(len(bag))],
		// Strength varies a little per mark, the way a loaded brush does,
		// and stays well below 1 so a crossing is still readable as two
		// pigments rather than as one opaque patch.
		alpha: s.Alpha * (0.85 + 0.3*rng.Float64()),
	}

	switch {
	case rng.Float64() < s.Banded:
		c.kind = kindBanded
		c.bands = s.planBands(rng, g.R, ramp)
	case rng.Float64() < s.Open:
		c.kind = kindOpen
		// Bands run from a fat annulus to a drawn-looking hoop; the fat
		// end keeps the open marks in the same family as the filled ones.
		c.band = g.R * (0.18 + 0.42*rng.Float64())
	case rng.Float64() < s.Rings:
		c.kind = kindNested
	case rng.Float64() < s.Glaze:
		c.kind = kindGlaze
		c.offset = 0.15 + 0.3*rng.Float64()
		c.angle = rng.Float64() * 2 * math.Pi
	default:
		c.kind = kindPlain
	}

	// Rings only where they can resolve: on a small circle three of them
	// merge into a dark disc, which is a worse mark than a plain one.
	if c.kind == kindNested || (c.kind == kindOpen && rng.Float64() < s.Rings) {
		c.rings = 2 + rng.IntN(2)
		if g.R < s.Base*2 {
			c.rings = 1
		}
	}
	return c
}

// paint lays one circle. Rings are glazed rather than drawn: laying the
// same pigment again over a pool is what a painter does to deepen a
// passage, and because the wash is transparent it deepens here too.
func (s *Sketch) paint(cv *paint.Canvas, rng *rand.Rand, w paint.Wash, c circle) {
	switch c.kind {
	case kindBanded:
		s.paintBands(cv, rng, w, c)
	case kindOpen:
		mid := c.R - c.band/2
		w.Ring(cv, rng, c.X, c.Y, mid, c.band, c.pigment, c.alpha)
		s.paintRings(cv, rng, w, c, mid-c.band/2)
	case kindGlaze:
		w.Pool(cv, rng, c.X, c.Y, c.R, c.pigment, c.alpha)
		r2 := c.R * (0.45 + 0.25*rng.Float64())
		d := c.R * c.offset
		w.Pool(cv, rng, c.X+d*math.Cos(c.angle), c.Y+d*math.Sin(c.angle), r2, c.second, c.alpha*0.9)
	default:
		w.Pool(cv, rng, c.X, c.Y, c.R, c.pigment, c.alpha)
		s.paintRings(cv, rng, w, c, c.R)
	}
}

// paintRings glazes concentric rings inside radius limit.
func (s *Sketch) paintRings(cv *paint.Canvas, rng *rand.Rand, w paint.Wash, c circle, limit float64) {
	if c.rings < 1 || limit <= 0 {
		return
	}
	// Rings sit on evenly spaced radii inside the limit, thin enough that
	// the paper (or the pool) still shows between them.
	for i := 1; i <= c.rings; i++ {
		rr := limit * float64(i) / float64(c.rings+1)
		band := math.Min(limit*0.11, rr*0.9)
		w.Ring(cv, rng, c.X, c.Y, rr, band, c.second, c.alpha*0.85)
	}
}

// weightedPick draws an index with probability proportional to weight.
func weightedPick(rng *rand.Rand, weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	bisect := rng.Float64() * total
	cum := 0.0
	for i, w := range weights {
		cum += w
		if cum > bisect {
			return i
		}
	}
	return len(weights) - 1
}

// paperTooth is the grain of the sheet, in canvas units. Ground and marks
// share it because they are on one piece of paper: the default is fine
// enough to read as noise across an empty ground, and a ground and a mark
// granulating at different scales are two different papers in one picture.
const paperTooth = 0.007

// groundWash is the wash the sheet's ground is laid with. It is the one
// wash in the picture that is meant to be featureless, so everything that
// gives a pool its character — the stack, the ragged edge, the rim — plays
// no part. What is left is the unevenness of the pigment and the tooth of
// the paper, which is all a flat wash has, and both are turned up: spread
// over a whole sheet they are the only thing there is to see.
func groundWash(seed uint64) paint.Wash {
	w := paint.DefaultWash(seed)
	w.Mottle = 1.45
	w.Grain = 0.42
	w.GrainScale = paperTooth
	return w
}
