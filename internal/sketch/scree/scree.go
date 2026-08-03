// Package scree implements sketch 010: a river bed seen from above — worn
// stones packed edge to edge in dark water, each cut into flat facets and
// each facet given one shade by a single light standing over the whole sheet
// (spec: docs/sketches/010-scree.md).
//
// Sketch 009 ends on a sheet that already reads as stones, and it gets there
// by hatching: marks that thin toward the light and crowd away from it, so a
// flat tile comes up off the page. That is an engraver's trick, and it has
// an engraver's limit — the tone is carried by marks, so the stone is a
// drawing of a stone.
//
// This one gives the sheet a surface instead. Each stone is a dome over its
// own wall-distance field; the dome is cut into facets by a second, finer
// partition; and each facet takes *one* flat shade from its own normal
// against one light. The hard step at every facet edge is the whole point: a
// smooth dome shaded per pixel reads as an airbrushed blob, and the same
// surface shaded per facet reads as rock, because rock is what breaks into
// flat faces.
package scree

import (
	"image"
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamLayout = 1 // where the stones go
	streamDress  = 2 // what colour each stone is
	streamTraits = 3 // which point of the output space this seed is
	streamLevels = 4 // the trait levels made numeric
	streamFacets = 5 // where the facets are cut
	streamGold   = 6 // which small-to-medium stones hold gold
)

// saltPaper salts the paper's grain lattice, so the tooth of the sheet does
// not move when the stones do.
const saltPaper = 0x73637265 // "scre"

// saltFacet salts the per-facet tilt and shade jitter, so the cut of the
// stone is independent of the paper.
const saltFacet = 0x66616365 // "face"

// tooth is the paper's grain, in canvas units.
const tooth = 0.0035

// statRes is how finely both partitions are measured — see cells.Params.
// Fixed rather than tied to the output size: a stone's inradius sets the
// height of its own dome and the size of its facets, so it has to be the
// same at preview and at print (invariant 2).
const statRes = 360

// Sketch holds what the stones and the water are made of. Where the stones
// go is not here — that is the bed, stones, facets, light, wet and joint
// traits, resolved per seed into levels, which pin holds the overrides for.
type Sketch struct {
	Weight float64 // how strongly a stone's size bends its walls
	Wobble float64 // hand wander of the joint, ×its width
	Grain  float64 // paper tooth

	// The paint (internal/paint.FlatWash).
	Load   float64 // pigment in a stone at full tone
	Pool   float64 // wavelength of the pigment's pooling, canvas units
	Uneven float64 // how strongly it pools

	// The colour scheme's spread (internal/scheme).
	Accent  float64 // share of stones taking a colour from outside their passage
	Passage float64 // wavelength of the colour field, canvas units
	Shades  float64 // how far a stone wanders from its palette swatch
	Sat     float64 // lift on every pigment's saturation
	Gold    bool    // reserve yellow for two or three rare gold nuggets

	// The surface and the lamp (light.go).
	Rise      float64 // how proud a stone stands, ×its own inradius
	Light     float64 // the light's bearing in degrees; 90 is from the top
	Elevation float64 // how high the lamp stands; low is dramatic
	Ambient   float64 // how much light reaches a face turned away
	Gloss     float64 // strength of the specular
	Sharp     float64 // how tight it is
	Warmth    float64 // how far the lit side leans toward the light's colour
	Coolness  float64 // how far the shadowed side leans toward the sky's

	// The water (traits.go).
	Soak  float64 // how much the water darkens a stone
	Sheen float64 // how much it polishes it
	Deep  float64 // how far the colour goes toward the water's own

	// pin is where the composition flags land. Only the ones actually given
	// on the command line are read; the rest come from the traits.
	pin levels

	traits *trait.Options
	knobs  *opt.Set
}

// New returns the sketch with its defaults.
func New() *Sketch {
	s := &Sketch{
		Weight:  1.05,
		Wobble:  0.24,
		Grain:   0.05,
		Load:    0.95,
		Pool:    0.09,
		Uneven:  0.6,
		Accent:  0.2,
		Passage: 0.8,
		Shades:  0.75,
		Sat:     0,
		// A rise measured against the stone's own inradius, not against the
		// page: pebbles are roughly self-similar, so a big stone stands
		// proportionally as proud as a small one. Fixed in canvas units the
		// small stones come out as domes and the large ones as puddles.
		Rise:      0.58,
		Light:     135,
		Elevation: 0.62,
		Ambient:   0.40,
		Gloss:     0.16,
		Sharp:     26,
		Warmth:    0.30,
		Coolness:  0.34,
		Soak:      0.5,
		Sheen:     1,
		Deep:      0.12,
		traits:    trait.NewOptions(schema),
	}
	// The pin defaults are only ever shown in --help; a knob left alone is
	// taken from the trait level, not from here.
	s.pin = defaults(rand.New(rand.NewPCG(1, 1)))
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "scree" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "a river bed of worn stones, each cut into flat facets and lit by one lamp"
}

// Schema implements sketch.Traited.
func (s *Sketch) Schema() trait.Schema { return schema }

// Traits implements sketch.Traited.
func (s *Sketch) Traits(ctx sketch.Context) trait.Set {
	return s.traits.Resolve(ctx.RNG(streamTraits))
}

// TraitSuffix implements sketch.Traited.
func (s *Sketch) TraitSuffix(set trait.Set) string { return s.traits.NameSuffix(set) }

// sheet is one render's bed and its dressing: everything settled before a
// pixel is drawn. Kept as a value so the tests can inspect a composition
// without rasterising it.
type sheet struct {
	stones *cells.Foam
	facets *facetLayer // nil when nothing is faceted
	skin   []stone
	level  levels
	field  *noise.Perlin
	wash   paint.FlatWash
	ink    inks
}

// plan builds the sheet for one context.
func (s *Sketch) plan(ctx sketch.Context) (*sheet, error) {
	aspect := float64(ctx.Width) / float64(ctx.Height)

	tr := s.Traits(ctx)
	l := s.levelsFor(tr, ctx.RNG(streamLevels))

	pal, err := colours(tr.Get(dimColourway), ctx.Palette)
	if err != nil {
		return nil, err
	}
	if s.Gold {
		pal, err = withoutYellow(pal)
		if err != nil {
			return nil, err
		}
	}
	ink := s.inks(palette.ByLuminance(pal.Colors))

	rng := ctx.RNG(streamLayout)
	sites := s.pack(l, rng, aspect)
	group := cells.Merge(rng, sites, l.merge, l.maxLobe)
	stones := cells.New(sites, group, aspect, cells.Params{Node: l.node, Round: l.round, Stat: statRes})

	wash := paint.NewFlatWash(ctx.Seed ^ saltPaper)
	wash.Blotch, wash.Mottle = s.Pool, s.Uneven
	wash.Tooth, wash.Grain = tooth*1.8, s.Grain*6
	field := noise.New(ctx.Seed)

	skin := s.dress(stones, l, ctx.RNG(streamDress), aspect, ink)
	if s.Gold {
		muteOrdinaryYellows(skin)
		s.addNuggets(skin, stones, field, l, ink, ctx.RNG(streamGold), aspect)
	}
	ink.joint = sink(ink.joint, darkestShadow(skin, l))

	return &sheet{
		stones: stones,
		facets: s.cut(stones, l, ctx.RNG(streamFacets), aspect, ctx.Seed),
		skin:   skin,
		level:  l,
		field:  field,
		wash:   wash,
		ink:    ink,
	}, nil
}

// Render implements sketch.Sketch. One pure pixel function: look the point
// up in the warped bed, paint its stone, light it, then lay the joint over
// the top.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	sh, err := s.plan(ctx)
	if err != nil {
		return nil, err
	}
	return sketch.Raster(ctx, func(u, v float64) palette.Color {
		wu, wv := s.warp(sh.field, sh.level, u, v)
		h := sh.stones.At(wu, wv)
		// Paint first, light second. The light multiplies the painted colour
		// rather than being mixed into it, so a dark pigment stays dark and
		// the scheme's value structure survives being lit.
		col := s.paint(sh, h, u, v)
		col = s.illuminate(sh, h, wu, wv, col)
		if s.Gold && sh.skin[h.Cell].nugget {
			col = gild(col)
		}
		// The joint goes on top of everything and is never itself lit: it is
		// the water between the stones, not part of any stone's surface.
		col = s.lay(sh, col, h, u, v, ctx.Seed)
		if s.Gold && !sh.skin[h.Cell].nugget {
			col = muteYellow(col)
		}
		return col
	}), nil
}

// paint lays the stone's own wash over the paper — an even body of pigment
// that pools broadly and catches in the paper's tooth, and nothing else. All
// the modelling happens afterwards, in the light.
func (s *Sketch) paint(sh *sheet, h cells.Hit, u, v float64) palette.Color {
	d := sh.skin[h.Cell]
	return sh.wash.Over(sh.ink.paper, d.pigment, d.load, u, v)
}

// warp bends the whole bed through a smooth displacement before it is ever
// looked up. Both partitions are read at the warped point, so the facets
// lean with the stones they sit in.
//
// The weighted metric curves a wall only where the two stones either side of
// it differ sharply in size, and most neighbours do not, so without a warp
// the typical wall comes out straight and the bed reads as crazy paving.
// Bending the plane curves every wall at once, for two noise samples.
//
// Curl rather than a plain gradient because it is divergence-free: it shears
// the plane without compressing it, so a stone comes out bent rather than
// squeezed to nothing. Its length is unbounded, so the displacement is held
// inside the pack's overscan by a tanh limiter — a hard clamp would crease
// the field exactly where it bites, and a crease in the displacement is a
// straight edge cutting across the stones.
func (s *Sketch) warp(field *noise.Perlin, l levels, u, v float64) (float64, float64) {
	if l.warp <= 0 {
		return u, v
	}
	w := l.base * l.swirl
	dx, dy := field.Curl(u/w, v/w, 2)
	dx, dy = dx*l.base*l.warp, dy*l.base*l.warp
	m := math.Hypot(dx, dy)
	if m < 1e-12 {
		return u, v
	}
	k := l.over * math.Tanh(m/l.over) / m
	return u + dx*k, v + dy*k
}

// pack scatters the sites the stones are grown from: best-candidate darts
// over a geometric size ladder, with an overscan so that a stone at the
// border is *cut* by the frame. Packed inside the frame instead, the border
// stones run out to nothing and the bed reads as an object floating on
// paper rather than as a fragment of a riverbed that carries on past it.
func (s *Sketch) pack(l levels, rng *rand.Rand, aspect float64) []cells.Site {
	radii, weights := rnd.Ladder(l.base, l.ratio, l.rungs, 0.7)
	minX, minY := -l.over, -l.over
	maxX, maxY := aspect+l.over, 1+l.over
	index := geom.NewIndexIn(minX, minY, maxX, maxY, radii[len(radii)-1])

	var out []cells.Site
	for tries := 0; len(out) < l.count && tries < l.count*8; tries++ {
		r := radii[rnd.PickIndex(rng, weights)]
		// Best-candidate, and few darts on purpose. Many darts would maximise
		// the spacing, and evenly spaced sites give an evenly graded bed —
		// a river sorts its stones, but it does not sort them that well.
		bx, by, bd, ok := 0.0, 0.0, -1.0, false
		for range max(l.darts, 1) {
			x := minX + rng.Float64()*(maxX-minX)
			y := minY + rng.Float64()*(maxY-minY)
			c := geom.Circle{X: x, Y: y, R: r}
			if !index.FitsWithGap(c, r*l.gap) {
				continue
			}
			d := nearest(index, x, y)
			if d > bd {
				bx, by, bd, ok = x, y, d, true
			}
		}
		if !ok {
			continue
		}
		index.Insert(geom.Circle{X: bx, Y: by, R: r})
		out = append(out, cells.Site{X: bx, Y: by, W: r * s.Weight})
	}
	if len(out) < 2 {
		// A bed of one stone is not a bed. Only reachable by pinning the count
		// to nothing, and cells.New needs at least one site.
		out = append(out, cells.Site{X: aspect / 2, Y: 0.5, W: l.base})
	}
	return out
}

// nearest is the distance from a point to the closest site placed so far,
// or a large number while the bed is empty.
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

// lay puts the joint over whatever the stone was painted and lit with.
//
// The taper and the swollen junction are one rule, not two: the half-width
// grows with Node, which is 1 where three stones meet and 0 halfway along a
// wall, so the fillets between three stones come out concave because Node
// falls off smoothly.
func (s *Sketch) lay(sh *sheet, col palette.Color, h cells.Hit, u, v float64, seed uint64) palette.Color {
	l := sh.level
	if math.IsInf(h.Wall, 1) || l.ink <= 0 {
		return col
	}
	half := l.ink / 2 * (1 + l.swell*h.Node)
	// The wander of a hand. In canvas units, so preview and print wobble
	// identically (invariant 2).
	half *= 1 + s.Wobble*sh.field.At(u/0.035, v/0.035)
	// The softness of the edge is a fraction of the *unswollen* line. Tied to
	// the local half-width instead it grows with the swelling, so exactly
	// where the joint is heaviest — the knot where three stones meet — it
	// dissolves into a halo.
	soft := math.Max(l.ink*0.16, 0.0004)
	a := 1 - mathx.Smoothstep(half-soft, half+soft, h.Wall)
	if a <= 0 {
		return col
	}
	a *= mathx.Clamp01(0.92 + 2*s.grain(seed, u, v))
	return palette.Lerp(col, sh.ink.joint, a)
}

// grain is the paper's tooth: a signed nudge from a hash lattice, salted so
// the texture does not move when the stones do.
func (s *Sketch) grain(seed uint64, u, v float64) float64 {
	ix, iy := int64(math.Floor(u/tooth)), int64(math.Floor(v/tooth))
	return (noise.Hash01(seed^saltPaper, ix, iy) - 0.5) * s.Grain
}

// shade multiplies a colour's channels, which is the one operation the
// palette package does not have: Lighten and Desaturate both go through HSL,
// and turning a face away from the light wants neither.
func shade(c palette.Color, f float64) palette.Color {
	return palette.Color{R: c.R * f, G: c.G * f, B: c.B * f}.Clamp()
}
