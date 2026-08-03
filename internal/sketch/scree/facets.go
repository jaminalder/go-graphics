package scree

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// The facets: a second, much finer partition laid over the whole sheet, so a
// point's identity is the *pair* (stone, facet).
//
// It is one cells.Foam over the whole canvas rather than a site set per
// stone, and that is the load-bearing choice. Both partitions are read at the
// same warped point and the stone's joint is drawn over the top afterwards —
// so the heavy line clips the fine net for free, with no clipping code
// anywhere. Per-stone site sets would need a foam per stone (a hundred
// measuring passes instead of one) and would still have to solve the same
// clipping problem at every border.
//
// The pack is a variable-radius dart throw, which makes the *grain* of the
// facets a free choice rather than something the geometry forces.
//
// By default the grain is one fineness for the whole bed, sized off the
// smallest stone: a boulder is cut as finely as a chip, so it comes out with
// many facets and the chip with a handful. That is what rock does — the grain
// of a stone is a property of the rock it broke off, not of how big the piece
// is — and it is what the picture wants, because the facets are here to
// describe a *surface*. Scaled with the stone, facet size doubles as a second
// reading of stone size, the two cues confound each other, and a boulder ends
// up looking like a chip photographed from closer.
//
// `--facet-scale` recovers the proportional behaviour, where a dart's radius
// is read from the inradius of the stone it lands in — which is what
// sketch 009's mosaic does, and rightly, because there the tiles carry a
// *colour walk* that needs a comparable number of steps in every cell. It is
// the same mechanism serving a different end.
//
// The facet sites carry weight 0 and their corners are barely rounded, so the
// inner metric is an ordinary Voronoi: straight bisectors, sharp corners,
// flat faces. That is deliberate contrast. The bed is a cluster of worn
// bubbles; making the facets a second cluster of worn bubbles gives one
// texture at two scales and the sheet reads as a blur. Crystal inside organic
// reads as two things — which is what a stone is.

// maxFacets caps the inner pack, and minFacet floors a single facet's radius.
//
// The two are one guard. With the grain held constant across the bed, the
// facet count is set by the *smallest* stone, so a fine bed asks for several
// times what a coarse one does — and a cap that binds is worse than a coarse
// sheet, because the darts stop before the frame is covered and whatever is
// left over falls into the nearest facet already placed, which comes out as a
// few enormous faces in one corner. The floor keeps the count bounded from
// below the cap whatever the bed, so the cap is a backstop and not a limit
// anything reaches.
const (
	maxFacets = 20000
	minFacet  = 0.0035
)

// facetLevels is everything the facets trait resolves to.
type facetLevels struct {
	size float64 // facet radius, ×the smallest stone
	// scale is how far the grain follows the stone: 0 is one fineness for the
	// whole bed, 1 sizes every facet off the stone it lands in. It runs as a
	// power rather than a mix because the quantity being interpolated is a
	// *ratio* of lengths, and the halfway point between "one fineness" and
	// "proportional" is the geometric mean, not the arithmetic one.
	scale float64
	// share is 1 at every level, and the knob is what it is for. A stone left
	// smooth among faceted ones does not read as variety, it reads as one the
	// sheet forgot: the facets are how a stone is described here, so a stone
	// without them is an airbrushed blob with pebbles round it.
	share  float64 // share of stones cut into facets at all
	cut    float64 // random tilt on each face, in slope units
	flake  float64 // random scaling on each face's shade
	crease float64 // how much a face darkens toward its own edge
}

// facetLayer is the built mosaic, with every facet's flat shade already
// worked out. Nothing here is per pixel: a facet is one flat face, so what it
// does with the light is a property of the facet, not of the point.
type facetLayer struct {
	foam *cells.Foam
	on   []bool // per stone: is it cut into facets

	// Per facet, indexed by facet id.
	stone   []int32   // the stone its centroid falls in
	diffuse []float64 // the flat shade, and
	spec    []float64 // the flat highlight

	mean float64 // the mean facet site radius, the fallback facet scale
}

// cut builds the facets. It returns nil when the bed is not faceted, which
// the lighting treats as "shade every pixel smoothly" — so the whole layer
// costs a nil check and nothing else when it is off.
func (s *Sketch) cut(st *cells.Foam, l levels, rng *rand.Rand, aspect float64, seed uint64) *facetLayer {
	if l.fac.share <= 0 || l.fac.size <= 0 {
		return nil
	}
	sites, mean := s.facetSites(st, l, rng, aspect)
	if len(sites) < 2 {
		return nil
	}
	inner := cells.New(sites, cells.Identity(len(sites)), aspect,
		cells.Params{Node: mean * 0.30, Round: mean * 0.06, Stat: statRes})

	f := &facetLayer{foam: inner, mean: mean, on: make([]bool, st.Len())}
	for i := range f.on {
		f.on[i] = rng.Float64() < l.fac.share
	}
	f.light(inner, sites, st, l, seed)
	return f
}

// facetSites throws variable-radius darts over the pack's whole overscanned
// rectangle, sizing each one from the stone it lands in.
func (s *Sketch) facetSites(st *cells.Foam, l levels, rng *rand.Rand, aspect float64) (sites []cells.Site, mean float64) {
	minX, minY := -l.over, -l.over
	maxX, maxY := aspect+l.over, 1+l.over

	// The index's own cell size is a performance knob only (see
	// geom.NewIndexIn), so it is sized to the typical facet rather than the
	// largest one.
	typical := math.Max(l.base*l.fac.size, minFacet)
	index := geom.NewIndexIn(minX, minY, maxX, maxY, typical)

	sum := 0.0
	for tries := 0; tries < 600000 && len(sites) < maxFacets; tries++ {
		x := minX + rng.Float64()*(maxX-minX)
		y := minY + rng.Float64()*(maxY-minY)
		r := facetRadius(st, l, x, y)
		c := geom.Circle{X: x, Y: y, R: r}
		if !index.FitsWithGap(c, 0) {
			continue
		}
		index.Insert(c)
		// Weight 0: the inner metric is an ordinary Voronoi, straight-walled
		// against the bed's arcs.
		sites = append(sites, cells.Site{X: x, Y: y})
		sum += r
	}
	if len(sites) == 0 {
		return nil, typical
	}
	return sites, sum / float64(len(sites))
}

// facetRadius is the grain of the rock at a point.
//
// At scale 0 — the default — it is one length for the whole bed, a fraction of
// the *smallest* stone. Since the smallest stones have an inradius of about
// that, they are cut exactly as finely as they always were, and every larger
// stone is now cut to the same grain instead of to its own size.
func facetRadius(st *cells.Foam, l levels, x, y float64) float64 {
	r := l.base * l.fac.size
	if l.fac.scale > 0 {
		span := st.Cells()[st.At(x, y).Cell].Inradius
		if span <= 0 {
			// A stone measured entirely outside the frame has no inradius. Its
			// facets are off-paper anyway; the smallest stone stands in.
			span = l.base
		}
		// Clamped from below before the power is taken: a border stone cut down
		// to a sliver by the frame has an inradius near zero, and a ratio near
		// zero raised to a fractional power is still near zero — one stone in
		// the bed rendered as sandpaper.
		r *= math.Pow(math.Max(span, l.base*0.25)/l.base, l.fac.scale)
	}
	return math.Max(r, minFacet)
}

// light works out each facet's one flat shade, at its centroid.
func (f *facetLayer) light(inner *cells.Foam, sites []cells.Site, st *cells.Foam, l levels, seed uint64) {
	n := inner.Len()
	f.stone = make([]int32, n)
	f.diffuse = make([]float64, n)
	f.spec = make([]float64, n)

	for i, c := range inner.Cells() {
		cx, cy := c.CX, c.CY
		if c.Area <= 0 {
			// A facet measured entirely outside the frame has no centroid, and
			// the warp can still pull part of it into view. Its site is the
			// only place it is known to be; a centroid left at the origin
			// would take its shade from the corner of the page.
			cx, cy = sites[i].X, sites[i].Y
		}
		f.stone[i] = int32(st.At(cx, cy).Cell) //nolint:gosec // stone counts are small

		hx, hy := slope(st, l, cx, cy)
		// The cut: a small random tilt on each face, so the stone reads as
		// broken rather than moulded. Small on purpose — pushed up, the tilt
		// stops describing a surface and becomes noise, the faces stop
		// agreeing about where the light is, and the stone goes flat again.
		// The tilt is in the units slope() works in, so the two are
		// comparable: a tilt of 1 turns a face through 45°.
		hx += l.fac.cut * (2*noise.Hash01(seed^saltFacet, int64(i), 0) - 1)
		hy += l.fac.cut * (2*noise.Hash01(seed^saltFacet, int64(i), 1) - 1)

		diffuse, spec := l.lit.litFor(hx, hy)
		// And a small scaling, so two faces on the same slope still differ.
		// Real stone is not one material all the way through.
		diffuse *= 1 + l.fac.flake*(2*noise.Hash01(seed^saltFacet, int64(i), 2)-1)
		f.diffuse[i], f.spec[i] = mathx.Clamp01(diffuse), spec
	}
}

// crease darkens a face toward its own edge — the little shadow a cut plane
// collects where it meets the next one. It is what makes the facets read as
// faces of a solid rather than as a chart of coloured regions, and it stays
// small: a heavy crease is an inner line drawn round every facet, and the
// sheet then has two structures arguing about which one is the drawing.
func (f *facetLayer) crease(h cells.Hit, l levels) float64 {
	if l.fac.crease <= 0 || math.IsInf(h.Wall, 1) {
		return 1
	}
	// A narrow band. Wide, the crease is a gradient filling most of the face
	// and every facet reads as a small bubble — which is the soft shading the
	// flat shading exists to avoid, reintroduced one facet down.
	w := math.Max(f.mean*0.16, 1e-4)
	return 1 - l.fac.crease*(1-mathx.Smoothstep(0, w, h.Wall))
}

// newFacets draws how the stones are cut.
//
// The facet size, the share of stones cut, the tilt, the shade jitter and the
// crease are one axis because they are one decision. A tilt that reads as
// knapping on a big plate is noise on a chip, and a crease sized for a coarse
// cut swallows a fine one whole — choosing them independently gives
// combinations that say nothing.
func newFacets(level string, rng *rand.Rand, l *levels) {
	var m facetLevels
	switch level {
	case "plates":
		m.size, m.share = rnd.Uniform(rng, 0.34, 0.46), 1
		m.cut, m.flake = rnd.Uniform(rng, 0.05, 0.09), rnd.Uniform(rng, 0.02, 0.05)
		m.crease = rnd.Uniform(rng, 0.10, 0.18)
	case "crazed":
		m.size, m.share = rnd.Uniform(rng, 0.17, 0.23), 1
		m.cut, m.flake = rnd.Uniform(rng, 0.07, 0.13), rnd.Uniform(rng, 0.03, 0.06)
		m.crease = rnd.Uniform(rng, 0.07, 0.13)
	case "shattered":
		m.size, m.share = rnd.Uniform(rng, 0.11, 0.16), 1
		m.cut, m.flake = rnd.Uniform(rng, 0.11, 0.19), rnd.Uniform(rng, 0.04, 0.08)
		m.crease = rnd.Uniform(rng, 0.05, 0.10)
	case "smooth":
		// No facets at all: the same surface and the same light, shaded per
		// pixel. Weight 0 in the schema, because a smoothly shaded dome is the
		// picture the flat shading exists to replace — but it is one flag
		// away, and it is the only way to see what the facets are doing.
		l.fac = facetLevels{}
		return
	default: // cut
		m.size, m.share = rnd.Uniform(rng, 0.24, 0.32), 1
		m.cut, m.flake = rnd.Uniform(rng, 0.06, 0.11), rnd.Uniform(rng, 0.03, 0.06)
		m.crease = rnd.Uniform(rng, 0.08, 0.15)
	}
	// One fineness for the whole bed at every level. It is not drawn per level
	// because it is not a matter of degree: either the grain is a property of
	// the rock or it is a property of the piece, and only one of those is true
	// of stone.
	m.scale = 0
	l.fac = m
}

// declareCut names the facet and lamp knobs. They are declared here rather
// than in declare() so that the whole faceting is one file to read and one
// file to remove.
func (s *Sketch) declareCut(o *opt.Set) {
	o.Float("facet", "facet size, x the smallest stone", "fc", 0.05, 1.2, &s.pin.fac.size)
	o.Float("facet-scale", "how far the grain follows the stone; 0 is one fineness for the bed", "fs", 0, 1, &s.pin.fac.scale)
	o.Float("faceted", "share of stones cut into facets", "fd", 0, 1, &s.pin.fac.share)
	o.Float("cut", "random tilt on each face; 1 is 45 degrees", "ct", 0, 1.5, &s.pin.fac.cut)
	o.Float("flake", "random scaling on each face's shade", "fk", 0, 0.6, &s.pin.fac.flake)
	o.Float("crease", "how far a face darkens toward its own edge", "cr", 0, 0.8, &s.pin.fac.crease)

	o.Float("rise", "how proud a stone stands, x its own inradius", "rs", 0, 1.5, &s.Rise)
	// Not "light": that name belongs to the trait dimension, which decides the
	// weather. This is one number inside it.
	o.Float("bearing", "the lamp's bearing in degrees; 90 is from the top", "lg", 0, 360, &s.Light)
	o.Float("elevation", "how high the lamp stands; low is dramatic", "el", 0.05, 3, &s.Elevation)
	o.Float("ambient", "how much light reaches a face turned away", "am", 0, 1, &s.Ambient)
	o.Float("gloss", "strength of the specular", "gl", 0, 0.8, &s.Gloss)
	o.Float("sharp", "how tight the specular is", "sp", 2, 200, &s.Sharp)
	o.Float("warmth", "how far the lit side leans toward the lamp's colour", "wm", 0, 1, &s.Warmth)
	o.Float("coolness", "how far the shadowed side leans toward the sky's", "cl", 0, 1, &s.Coolness)
}
