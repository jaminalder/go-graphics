package foam

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// Relief: lighting the sheet as though it had a surface.
//
// Everything here rests on one fact about the structure — `Hit.Wall` is a
// real signed distance field, not a mask. A field can be differenced, and a
// difference of a height built from it is a slope; a slope is a normal, and
// a normal can be lit. So the sheet gets a surface without anything ever
// being modelled: height is a function of how deep inside its cell a point
// is, and the cells' own boundaries become the creases of that surface.
//
// The height is taken in *canvas units of rise*, not as an abstract 0..1,
// which is what keeps the lighting resolution-independent: the slope of a
// bevel is rise over run and both are canvas lengths, so preview and print
// light the same picture. The difference step is a canvas length for the
// same reason — a step of "one pixel" would give a bevel that hardened as
// the render got bigger.
//
// One light, one direction, for the whole sheet. Inconsistent lighting is
// what makes fake relief look fake: the eye forgives an impossible surface
// long before it forgives two shadows pointing different ways. The
// direction is drawn per sheet from a narrow band around up-and-left, the
// convention every embossed thing on a page has trained the eye on — a
// sheet lit from below reads as a hole rather than a bump.

const (
	reliefFlat    = iota // no surface at all
	reliefBevel          // a chamfer at every wall: the sheet as cut tiles
	reliefCushion        // a dome per cell and per tile: inflated
	reliefOcclude        // no light, only the dirt that collects in a crease
	reliefTerrace        // cells at different depths, casting on each other
	reliefGlass          // lit from behind: leaded glass
	nreliefs
)

// saltRise salts the per-cell slab height, so which cell stands proud is
// independent of both the paper's tooth and the tiles' own variation.
const saltRise = 0x72697365 // "rise"

// reliefLevels is everything the relief trait resolves to.
type reliefLevels struct {
	mode  int
	lx    float64 // the light's direction across the page, unit length
	ly    float64
	lz    float64 // how high the light stands; low is dramatic, high is flat
	depth float64 // the rise of the surface, canvas units
	width float64 // the run the rise happens over, canvas units
	step  float64 // the finite-difference step, canvas units
	amb   float64 // how much light reaches a face turned away
}

// newRelief draws the lighting for one sheet.
func newRelief(level string, rng *rand.Rand, l *levels) {
	r := reliefLevels{amb: 0.42}
	high := true // how high the light stands, which is a per-mode decision
	switch level {
	case "bevel":
		r.mode = reliefBevel
		r.depth, r.width = l.base*rnd.Uniform(rng, 0.14, 0.22), l.base*rnd.Uniform(rng, 0.10, 0.18)
	case "cushion":
		r.mode = reliefCushion
		r.depth, r.width = l.base*rnd.Uniform(rng, 0.28, 0.42), l.base
		r.amb = 0.36
	case "occlude":
		r.mode = reliefOcclude
		r.depth, r.width = 0, l.base*rnd.Uniform(rng, 0.30, 0.48)
	case "terrace":
		r.mode = reliefTerrace
		// Slabs, not chamfers: the rise between two cells has to be a
		// visible step, because the whole read is that one cell stands above
		// another. The chamfer is only there to give the cliff a face.
		r.depth, r.width = l.base*rnd.Uniform(rng, 0.22, 0.34), l.base*rnd.Uniform(rng, 0.07, 0.12)
		r.amb, high = 0.52, false
	case "glass":
		r.mode = reliefGlass
		r.depth, r.width = 0, l.base*rnd.Uniform(rng, 0.22, 0.34)
	default:
		r.mode = reliefFlat
	}
	// Up and to the left, within twenty degrees — a bearing of 135°, in the
	// same convention the --light knob takes. v grows downward, so "up" is
	// negative and the bearing's sine is flipped.
	a := 3*math.Pi/4 + rnd.Uniform(rng, -0.35, 0.35)
	r.lx, r.ly = math.Cos(a), -math.Sin(a)
	// A high light flatters a bevel and kills a cast shadow: the shadow's
	// length is the rise over the light's slope, so a lamp overhead throws
	// nothing. Terraces want the sun low.
	if high {
		r.lz = rnd.Uniform(rng, 0.55, 0.85)
	} else {
		r.lz = rnd.Uniform(rng, 0.26, 0.40)
	}
	n := math.Sqrt(r.lx*r.lx + r.ly*r.ly + r.lz*r.lz)
	r.lx, r.ly, r.lz = r.lx/n, r.ly/n, r.lz/n
	// Two-thirds of the run the rise happens over: fine enough to resolve
	// the chamfer, coarse enough not to read the field's own hash noise.
	r.step = math.Max(r.width*0.66, 5e-4)
	l.rel = r
}

// relief lights one point. u, v are the *unwarped* canvas coordinates: the
// surface is differenced in screen space and each sample is warped on its
// own way in, so the displacement bends the lighting exactly as it bends
// the walls.
func (s *Sketch) relief(sh *sheet, oh cells.Hit, u, v float64, col palette.Color, seed uint64) palette.Color {
	r := sh.level.rel
	switch r.mode {
	case reliefFlat:
		return col
	case reliefOcclude:
		return s.occlude(sh, oh, u, v, col)
	case reliefGlass:
		return s.glass(sh, oh, u, v, col)
	}

	e := r.step
	hx := (s.surface(sh, u+e, v, seed) - s.surface(sh, u-e, v, seed)) / (2 * e)
	hy := (s.surface(sh, u, v+e, seed) - s.surface(sh, u, v-e, seed)) / (2 * e)
	// The surface normal of a height field is (−∂h/∂x, −∂h/∂y, 1),
	// normalised. Nothing more is needed: the creases of the partition are
	// already in h, so they come out as edges of the surface for free.
	n := math.Sqrt(hx*hx + hy*hy + 1)
	lambert := (-hx*r.lx - hy*r.ly + r.lz) / n
	lit := r.amb + (1-r.amb)*mathx.Clamp01(lambert)

	// A narrow specular on the faces square to the light, which is what
	// tells the eye the surface is smooth rather than matte paper.
	spec := math.Pow(mathx.Clamp01(lambert), 24) * 0.16

	out := shade(col, lit)
	if r.mode == reliefTerrace {
		// Two cues, not one. The cast shadow says which cell is *behind*
		// another; the tone says which is *above*, because a card nearer the
		// lamp catches more of it. Lambert alone says neither — the top of a
		// slab is flat, so every slab is lit identically however high it
		// stands, and the sheet reads as outlines until one of these runs.
		lvl := s.slab(oh.Cell, r, seed) / math.Max(r.depth, 1e-9)
		out = shade(out, (0.84+0.055*lvl)*s.castShadow(sh, u, v, seed))
	}
	return palette.Color{R: out.R + spec, G: out.G + spec, B: out.B + spec}.Clamp()
}

// surface is the height of the sheet at a point, in canvas units of rise.
//
// It is the composition of two fields, and that is the point of building it
// here rather than inside the partition: the outer cells give the large
// form — a dome, a slab — and the tiles of the mosaic give the facets on
// top of it, so one foam field carries relief at two scales.
func (s *Sketch) surface(sh *sheet, u, v float64, seed uint64) float64 {
	r := sh.level.rel
	wu, wv := s.warp(sh.field, sh.level, u, v)
	oh := sh.foam.At(wu, wv)

	h := 0.0
	switch r.mode {
	case reliefBevel:
		h = r.depth * chamfer(oh.Wall, r.width)
	case reliefCushion:
		span := math.Max(sh.foam.Cells()[oh.Cell].Inradius, sh.level.base)
		h = r.depth * dome(oh.Wall, span)
	case reliefTerrace:
		// A cell sits at one of four depths, drawn from its id, with a
		// chamfered edge so the cliff has a face to catch the light.
		h = s.slab(oh.Cell, r, seed) + r.depth*0.35*chamfer(oh.Wall, r.width)
	}

	// The tiles. A subdivided cell's facets ride on the cell's own form,
	// at a fraction of its rise — the small structure must modulate the
	// large one, not compete with it.
	if t := sh.tiles; t != nil && t.piece[oh.Cell].on {
		ih := t.foam.At(wu, wv)
		switch r.mode {
		case reliefCushion:
			h += r.depth * 0.45 * dome(ih.Wall, math.Max(t.inrad[ih.Cell], t.mean))
		default:
			h += r.depth * 0.5 * chamfer(ih.Wall, math.Max(t.mean*0.35, r.width*0.5))
		}
	}
	return h
}

// chamfer is a plateau with a soft edge: 0 on the wall, 1 once the point is
// a full width inside. Smoothstep rather than a straight ramp because a
// ramp meets the plateau at a corner, and a corner in the height is a bright
// line in the lighting.
func chamfer(wall, width float64) float64 {
	if math.IsInf(wall, 1) {
		return 1
	}
	return mathx.Smoothstep(0, math.Max(width, 1e-9), wall)
}

// dome is the profile of a half-cylinder over the cell: 0 at the wall,
// 1 at the deepest point, with the steep shoulder near the edge that makes
// an inflated thing look inflated. A smoothstep here gives a pillow with
// flat sides, which reads as a button.
func dome(wall, span float64) float64 {
	if math.IsInf(wall, 1) {
		return 1
	}
	t := mathx.Clamp01(wall / math.Max(span, 1e-9))
	return math.Sqrt(t * (2 - t))
}

// castShadow is what makes a terrace read as depth rather than as outlines.
// A cell is in shadow if a *taller* cell stands between it and the light, so
// the sheet is marched a few steps toward the light and each step asks
// whether the ground there is higher than the ray has climbed.
func (s *Sketch) castShadow(sh *sheet, u, v float64, seed uint64) float64 {
	r := sh.level.rel
	// The ray's slope: how much height it gains per canvas unit travelled
	// toward the light. Straight from the light direction, so the shadow
	// length and the shading agree about where the light is.
	slope := r.lz / math.Max(math.Hypot(r.lx, r.ly), 1e-6)
	// The march is as long as the tallest step can reach and no longer. Set
	// as a multiple of the *line* instead — which is what the first version
	// did — the taps never get far enough for a step to overtake the ray,
	// and the shadow silently does nothing.
	reach := 3 * r.depth / slope
	here := s.terraceHeight(sh, u, v, seed)
	dark := 0.0
	const taps = 12
	for k := 1; k <= taps; k++ {
		d := reach * float64(k) / taps
		there := s.terraceHeight(sh, u+r.lx*d, v+r.ly*d, seed)
		// Averaged over the taps, not maximised over them. A max is the
		// physically right answer and the wrong picture: the occluder enters
		// one tap at a time as the point moves, so each tap's whole
		// contribution switches on at once and the shadow's edge comes out
		// as a staircase with a tread the length of one tap. Averaging turns
		// that switch into a twelfth of the shadow's depth, which reads as
		// the penumbra a low sun would give anyway.
		dark += mathx.Smoothstep(0, r.depth*0.7, there-here-d*slope)
	}
	return 1 - 0.62*dark/taps
}

// terraceHeight is the slab height alone — no chamfer, no tiles. The shadow
// test wants the *step* between cells and nothing else; including the edge
// treatment puts a thin shadow along every wall, which is a drawn outline
// wearing a shadow's clothes.
func (s *Sketch) terraceHeight(sh *sheet, u, v float64, seed uint64) float64 {
	wu, wv := s.warp(sh.field, sh.level, u, v)
	return s.slab(sh.foam.At(wu, wv).Cell, sh.level.rel, seed)
}

// slab is which of four depths a cell stands at. Four rather than a
// continuum: the sheet has to read as a stack of cut cards, and a cell that
// is a hair above its neighbour throws a shadow that reads as a drawing
// error rather than as height.
func (s *Sketch) slab(cell int, r reliefLevels, seed uint64) float64 {
	return r.depth * math.Floor(noise.Hash01(seed^saltRise, int64(cell), 0)*4)
}

// occlude is the cheapest of the five and the one that survives the most
// seeds: no light at all, only the darkening that collects where two
// surfaces meet. It needs no normals, so it costs one extra lookup instead
// of ten, and because it never brightens anything it cannot blow a pale
// pigment out.
func (s *Sketch) occlude(sh *sheet, oh cells.Hit, u, v float64, col palette.Color) palette.Color {
	r := sh.level.rel
	ao := chamfer(oh.Wall, r.width)
	if t := sh.tiles; t != nil && t.piece[oh.Cell].on {
		wu, wv := s.warp(sh.field, sh.level, u, v)
		// The tiles' own creases, at half strength: a facet is a shallower
		// fold than a cell wall and should collect less dirt.
		ao *= 0.5 + 0.5*chamfer(t.foam.At(wu, wv).Wall, math.Max(t.mean*0.4, 1e-4))
	}
	return shade(col, 0.5+0.5*ao)
}

// glass lights the sheet from behind. Pigment that a lamp is shining
// through is at its most saturated where the glass is thin and goes to the
// colour of the lead where it thickens, so the transmission is read off the
// same wall distance everything else is — inverted.
func (s *Sketch) glass(sh *sheet, oh cells.Hit, u, v float64, col palette.Color) palette.Color {
	r := sh.level.rel
	through := chamfer(oh.Wall, r.width)
	if t := sh.tiles; t != nil && t.piece[oh.Cell].on {
		wu, wv := s.warp(sh.field, sh.level, u, v)
		ih := t.foam.At(wu, wv)
		through = math.Min(through, 0.25+0.75*chamfer(ih.Wall, math.Max(t.mean*0.5, 1e-4)))
	}
	// Toward the lead at the edges, toward a lit version of the pigment in
	// the middle. Lighten alone washes out; the mix keeps the hue and adds
	// the glow on top of it.
	lead := shade(col, 0.42)
	lit := palette.Lerp(col, col.Lighten(0.42), 0.7)
	return palette.Lerp(lead, lit, through)
}
