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
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamLayout = 1 // placement, sizes, structure, colour
	streamPaint  = 2 // per-pool wash deformation
	streamTraits = 3 // which point of the output space this seed is
	streamFill   = 4 // the fill level made numeric
)

// saltPaper salts the granulation lattice so the paper texture is
// independent of the layout.
const saltPaper = 0x706170 // "pap"

// saltGround salts the ground's own granulation lattice, so the sky's
// tooth is not a copy of the marks'.
const saltGround = 0x67726e // "grn"

// ringFloor is the radius below which a circle gets one inner ring instead
// of two or three, in canvas units.
const ringFloor = 0.055

// Sketch holds what every mark is made of and painted with. Where the
// marks go is not here: that is the fill trait, resolved per seed into a
// layout (fill.go), which pin holds the command-line overrides for.
type Sketch struct {
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

	// pin is where the composition flags land. Only the ones actually given
	// on the command line are read; the rest come from the fill level.
	pin layout

	traits *trait.Options
	knobs  *opt.Set
}

// New returns the sketch with its defaults.
func New() *Sketch {
	s := &Sketch{
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
		traits:       trait.NewOptions(schema),
	}
	// The pin defaults are only ever shown in --help; a knob left alone is
	// taken from the fill level, not from here.
	s.pin = newFill("medium", rand.New(rand.NewPCG(1, 1)))
	s.declare()
	return s
}

// Schema implements sketch.Traited.
func (s *Sketch) Schema() trait.Schema { return schema }

// Traits implements sketch.Traited.
func (s *Sketch) Traits(ctx sketch.Context) trait.Set {
	return s.traits.Resolve(ctx.RNG(streamTraits))
}

// layoutFor resolves the composition for one render: what the seed's fill
// level drew, with any pinned flag laid over it. This is the whole point of
// the trait — a caller can move one number without restating the other
// seven, and a caller who says nothing still gets a coherent sheet.
func (s *Sketch) layoutFor(tr trait.Set, rng *rand.Rand) layout {
	l := newFill(tr.Get(dimFill), rng)
	for _, o := range []struct {
		name string
		set  func()
	}{
		{"count", func() { l.count = s.pin.count }},
		{"rungs", func() { l.rungs = s.pin.rungs }},
		{"base", func() { l.base = s.pin.base }},
		{"ratio", func() { l.ratio = s.pin.ratio }},
		{"satellites", func() { l.satellites = s.pin.satellites }},
		{"gap", func() { l.gap = s.pin.gap }},
		{"margin", func() { l.margin = s.pin.margin }},
	} {
		if s.knobs.WasSet(o.name) {
			o.set()
		}
	}
	return l
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

	tr := s.Traits(ctx)
	l := s.layoutFor(tr, ctx.RNG(streamFill))

	pal, err := colours(tr.Get(dimColourway), ctx.Palette)
	if err != nil {
		return nil, err
	}
	paper, tint, ramp := s.inks(palette.ByLuminance(pal.Colors))

	circles := s.plan(l, tr.Get(dimArrange), tr.Get(dimFlow), rng, aspect, ramp)

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

// inks decides the paper and the pigment draw pile. The pile is weighted
// so one pigment dominates and the rest are progressively rarer: drawing
// uniformly gives every colour equal presence, which reads as a sampler
// rather than as a picture.
func (s *Sketch) inks(byLum []palette.Color) (paper, tint palette.Color, ramp []palette.Color) {
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
	// Kept in luminance order, because everything that reads colour here
	// walks it as a sequence: a banded mark graduates between neighbours on
	// it, and the colour walk steps along it (arrange.go). A shuffled,
	// weighted draw pile was the older mechanism and is gone with it — the
	// walk is what gives the piece its dominant colour now, and it does so
	// in passages rather than uniformly.
	ramp = append([]palette.Color(nil), byLum[:n]...)
	return paper, tint, ramp
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

// plan places every circle and settles what it is made of, largest first
// so the paint order lets small marks settle on top.
func (s *Sketch) plan(l layout, arrange, flow string, rng *rand.Rand, aspect float64, ramp []palette.Color) []circle {
	radii, weights := l.ladder()
	index := geom.NewIndex(aspect, 1, radii[len(radii)-1])
	walk := newColorWalk(rng, len(ramp))

	var out []circle
	place := func(c geom.Circle, sc scheme) bool {
		if !l.inPaper(c, aspect) || !index.FitsWithGap(c, c.R*l.gap) {
			return false
		}
		index.Insert(c)
		out = append(out, s.build(rng, c, sc, ramp))
		return true
	}

	groups := startGroups(arrange, rng, aspect, l.spacing(radii))
	if groups == nil {
		return s.scatter(l, rng, aspect, ramp, radii, weights, index, walk, place, &out)
	}

	f := newField(flow, rng, aspect)
	for _, g := range groups {
		if len(out) >= l.count {
			break
		}
		// One size and one colour for the whole run. This is the rule the
		// look turns on: per mark they come out as salt and pepper and no
		// strand reads as a strand.
		rung := rnd.PickIndex(rng, weights)
		r := radii[rung]
		sc := walk.next(rng)

		for _, start := range g {
			if len(out) >= l.count {
				break
			}
			// Both ways from the seed, so a run is a whole stretch of the
			// field through its start rather than a tail hanging off it.
			for _, dir := range [2]float64{1, -1} {
				x, y := start.x, start.y
				for step := 0; step < maxWalk; step++ {
					if placed := place(geom.Circle{X: x, Y: y, R: r}, sc); placed {
						s.satellite(l, rng, index, aspect, geom.Circle{X: x, Y: y, R: r}, rung, radii, sc, ramp, &out)
					}
					// Advance by this mark's own diameter, which is what
					// makes consecutive marks touch. A fixed grid cannot:
					// its step would have to be the size of the mark being
					// laid, and that changes with every run.
					adv := 2*r + r*math.Max(l.gap, 0)
					th := f.at(x, y)
					x += dir * adv * math.Cos(th)
					y += dir * adv * math.Sin(th)
					if !inBleed(pt{x, y}, aspect) {
						break
					}
				}
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].R > out[j].R })
	return out
}

// maxWalk bounds one direction of one run, so a field that circles forever
// cannot spin in place.
const maxWalk = 400

// satellite sometimes lays a companion across a mark. Left to chance the
// overlaps either never happen or arrive as a pile-up; the crossings are
// part of the subject, so they are placed on purpose — and from one rung
// down, because a speck crossing a large disc shows nothing.
func (s *Sketch) satellite(l layout, rng *rand.Rand, index *geom.Index, aspect float64,
	c geom.Circle, rung int, radii []float64, sc scheme, ramp []palette.Color, out *[]circle,
) {
	if rng.Float64() >= l.satellites {
		return
	}
	sr := radii[max(rung-1, 0)]
	if rung == 0 {
		sr = c.R * 0.72
	}
	th := rng.Float64() * 2 * math.Pi
	d := (c.R + sr) * (0.42 + 0.32*rng.Float64())
	comp := geom.Circle{X: c.X + d*math.Cos(th), Y: c.Y + d*math.Sin(th), R: sr}
	if !l.inPaper(comp, aspect) {
		return
	}
	index.Insert(comp)
	*out = append(*out, s.build(rng, comp, sc, ramp))
}

// scatter is the structureless arrangement: darts rather than walks, and
// the only one that leaves the sheet with no direction in it at all.
func (s *Sketch) scatter(l layout, rng *rand.Rand, aspect float64, ramp []palette.Color,
	radii, weights []float64, index *geom.Index, walk *colorWalk,
	place func(geom.Circle, scheme) bool, out *[]circle,
) []circle {
	// Attempts, not placements: a dart that lands on an occupied patch has
	// not spent a mark, and counting it as one leaves a crowded sheet
	// far short of its own budget.
	for tries := 0; len(*out) < l.count && tries < l.count*4; tries++ {
		rung := rnd.PickIndex(rng, weights)
		r := radii[rung]
		x, y, ok := l.bestCandidate(rng, index, aspect, r)
		if !ok {
			continue
		}
		sc := walk.next(rng)
		c := geom.Circle{X: x, Y: y, R: r}
		if place(c, sc) {
			s.satellite(l, rng, index, aspect, c, rung, radii, sc, ramp, out)
		}
	}
	sort.SliceStable(*out, func(i, j int) bool { return (*out)[i].R > (*out)[j].R })
	return *out
}

// build settles what one placed circle is made of.
func (s *Sketch) build(rng *rand.Rand, g geom.Circle, sc scheme, ramp []palette.Color) circle {
	// The walk decides the hue and the bag decides the accents: a mark
	// takes its pigment from where the walk stands, and its second colour
	// from a neighbour on the ramp, so a passage holds together while no
	// two marks in it are quite the same.
	c := circle{
		Circle:  g,
		pigment: ramp[sc.at],
		second:  ramp[sc.second],
		// Strength varies a little per mark, the way a loaded brush does,
		// and stays well below 1 so a crossing is still readable as two
		// pigments rather than as one opaque patch.
		alpha: s.Alpha * (0.85 + 0.3*rng.Float64()),
	}

	switch {
	case rng.Float64() < s.Banded:
		c.kind = kindBanded
		c.bands = s.planBands(rng, g.R, sc, ramp)
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
	//
	// The threshold is absolute, not a multiple of the ladder's smallest
	// rung. Whether three rings can be told apart inside a disc is a fact
	// about the disc and the paint, not about how crowded the sheet it sits
	// on is — measured against the ladder, a sparse sheet (whose smallest
	// circle is already large) would deny rings to every mark on it.
	if c.kind == kindNested || (c.kind == kindOpen && rng.Float64() < s.Rings) {
		c.rings = 2 + rng.IntN(2)
		if g.R < ringFloor {
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
