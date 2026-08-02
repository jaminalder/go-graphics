// Package foam implements sketch 009: a sheet divided into curved-walled
// cells by a heavy ink line, each cell filled with something else
// (spec: docs/sketches/009-foam.md).
//
// Every sketch before this one draws marks onto an undivided sheet. This
// one draws a *structure* first — a planar partition, from
// internal/cells — and treats what goes inside each region as a separate
// question with its own answer per cell: a watercolour wash, pencil
// hatching, concentric bands following the cell's own outline, or bare
// paper.
//
// The reference is hand-drawn neurographic art, which is structurally a 2D
// wet foam: three walls meeting at each junction, walls curving rather than
// running straight, and the ink swelling where the walls meet and tapering
// thin between. All three come out of the weighted distance field rather
// than being drawn as special cases.
package foam

import (
	"image"
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamLayout = 1 // where the sites go
	streamDress  = 2 // what each cell is filled with
	streamTraits = 3 // which point of the output space this seed is
	streamLevels = 4 // the trait levels made numeric
	streamTiles  = 5 // where the mosaic's tiles go
)

// saltPaper salts the paper's grain lattice so the tooth of the sheet is
// independent of where the cells landed.
const saltPaper = 0x666f616d // "foam"

// statRes is how finely the cells are measured — see cells.Params.Stat.
// Fixed, not tied to the output size: a cell's centroid decides its colour
// and its inradius decides whether bands can resolve, and both have to be
// the same at preview and at print.
const statRes = 360

// Sketch holds what the walls and the fills are made of. Where the cells go
// is not here: that is the density, lobes, fills and line traits, resolved
// per seed into levels, which pin holds the command-line overrides for.
type Sketch struct {
	Weight  float64 // how strongly a site's radius becomes an additive weight
	Wobble  float64 // hand-drawn wander of the ink line, ×ink width
	Rim     float64 // strength of the wash's dried rim
	RimWide float64 // how far in from the wall the rim reaches, canvas units
	Mottle  float64 // wavelength of the wash's unevenness, canvas units
	Blotch  float64 // strength of that unevenness
	Grain   float64 // paper tooth
	Accent  float64 // share of cells taking a colour from outside their passage
	Passage float64 // wavelength of the colour field, canvas units
	Stroke  float64 // pencil stroke pitch, canvas units
	Flat    float64 // how far a flat fill deepens at full tone; 1 is no deepening
	Shades  float64 // how far a cell's colour wanders from its palette swatch
	Depth   float64 // rise of the relief, ×smallest cell (shade.go)
	Bevel   float64 // run that rise happens over, ×smallest cell
	Light   float64 // the light's bearing, degrees

	// The watercolour (watercolour.go).
	Scatter float64 // light scattered back off the pigment rather than through it
	Tooth   float64 // the paper's tooth for granulation, canvas units

	// pin is where the composition flags land. Only the ones actually given
	// on the command line are read; the rest come from the traits.
	pin levels

	traits *trait.Options
	knobs  *opt.Set
}

// New returns the sketch with its defaults.
func New() *Sketch {
	s := &Sketch{
		Weight:  1.1,
		Wobble:  0.28,
		Rim:     0.9,
		RimWide: 0.014,
		Mottle:  0.22,
		Blotch:  0.24,
		Grain:   0.05,
		Accent:  0.22,
		Passage: 0.75,
		Stroke:  0.0045,
		Flat:    0.76,
		Shades:  0.7,
		Depth:   0.12,
		Bevel:   0.2,
		Light:   135,
		Scatter: 0.28,
		Tooth:   0.003,
		traits:  trait.NewOptions(schema),
	}
	// The pin defaults are only ever shown in --help; a knob left alone is
	// taken from the trait level, not from here.
	s.pin = defaults(rand.New(rand.NewPCG(1, 1)))
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "foam" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "a foam of curved cells in heavy ink, each filled with a wash, pencil, bands or nothing"
}

// Schema implements sketch.Traited.
func (s *Sketch) Schema() trait.Schema { return schema }

// Traits implements sketch.Traited.
func (s *Sketch) Traits(ctx sketch.Context) trait.Set {
	return s.traits.Resolve(ctx.RNG(streamTraits))
}

// TraitSuffix implements sketch.Traited.
func (s *Sketch) TraitSuffix(set trait.Set) string { return s.traits.NameSuffix(set) }

// sheet is one render's structure and dressing: everything settled before a
// pixel is drawn. Kept as a value so the tests can inspect a composition
// without rasterising it.
type sheet struct {
	foam  *cells.Foam
	skin  []dress
	level levels
	field *noise.Perlin
	paper palette.Color
	ink   palette.Color
	ramp  []palette.Color // the pigments, ordered light to dark
	tiles *tiling         // the mosaic layer; nil when nothing is subdivided
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
	paper, ink, ramp := s.inks(palette.ByLuminance(pal.Colors))

	rng := ctx.RNG(streamLayout)
	sites := s.pack(l, rng, aspect)
	group := cells.Merge(rng, sites, l.merge, l.maxLobe)
	foam := cells.New(sites, group, aspect, cells.Params{Node: l.node, Round: l.round, Stat: statRes})

	field := noise.New(ctx.Seed)
	skin := s.dress(foam, l, ctx.RNG(streamDress), aspect, paper, ramp)
	return &sheet{
		foam:  foam,
		skin:  skin,
		level: l,
		field: field,
		paper: paper,
		ink:   ink,
		ramp:  ramp,
		tiles: s.subdivide(foam, l, ctx.RNG(streamTiles), aspect, skin, ramp),
	}, nil
}

// Render implements sketch.Sketch. One pure pixel function: look the point
// up in the warped partition, ask its cell's dressing for a colour, then lay
// the ink over the top.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	sh, err := s.plan(ctx)
	if err != nil {
		return nil, err
	}
	return sketch.Raster(ctx, func(u, v float64) palette.Color {
		wu, wv := s.warp(sh.field, sh.level, u, v)
		h := sh.foam.At(wu, wv)
		// paint, not fill: a wash may cross the wall into this cell, either
		// because it failed to register with the line or because the two
		// were painted while both were wet (watercolour.go).
		c := s.paint(sh, h, ctx.Seed, u, v)
		// The mosaic replaces what the paint laid down wherever a cell was
		// subdivided; the relief lights whatever came out. Both run *before*
		// the ink, so the heavy outer line clips the fine net and is never
		// itself lit.
		c = s.tile(sh, h, u, v, wu, wv, c, ctx.Seed)
		c = s.relief(sh, h, u, v, c, ctx.Seed)
		return s.lay(c, h, sh.level, sh.field, ctx.Seed, sh.ink, u, v)
	}), nil
}

// warp bends the whole partition through a smooth displacement field before
// it is ever looked up.
//
// The weighted metric curves its walls, but only where two neighbouring
// sites differ sharply in weight; most neighbours are near enough in size
// that their shared wall comes out nearly straight, and a sheet of those
// reads as a cracked pane. Bending the plane curves every wall at once, and
// costs two noise samples.
//
// The displacement is a *curl* field, which is divergence-free: it shears
// the plane without compressing it, so a cell comes out bent rather than
// squeezed to nothing. Cells are measured before the warp — the field
// neither creates nor destroys area, so a cell's share of the sheet
// survives it, and its centroid moves by less than the warp itself.
func (s *Sketch) warp(field *noise.Perlin, l levels, u, v float64) (float64, float64) {
	if l.warp <= 0 {
		return u, v
	}
	// The wavelength is many cells long, and the displacement a fraction of
	// one. Bending at cell scale instead does not curve the walls, it
	// dissolves them: every cell comes out as an amoeba and the junctions
	// stop being junctions.
	w := l.base * l.swirl
	dx, dy := field.Curl(u/w, v/w, 2)
	dx, dy = dx*l.base*l.warp, dy*l.base*l.warp

	// Curl is a gradient, and a gradient has no bound: the tail of its
	// distribution is several times its typical length. Unchecked, those
	// few points are displaced clean out of the packed region, where there
	// are no sites left to be near and the border cells smear outward. The
	// limiter is tanh rather than a clamp because a clamp creases the field
	// exactly where it bites, and a crease in the displacement is a visible
	// straight edge cutting across the cells.
	m := math.Hypot(dx, dy)
	if m < 1e-12 {
		return u, v
	}
	k := l.over * math.Tanh(m/l.over) / m
	return u + dx*k, v + dy*k
}

// inks decides the paper, the line and the pigments.
func (s *Sketch) inks(byLum []palette.Color) (paper, ink palette.Color, ramp []palette.Color) {
	darkest, lightest := byLum[0], byLum[len(byLum)-1]
	// Bare paper: the palette's lightest taken almost to white and most of
	// the way to grey. The reference is drawn on cartridge paper, and every
	// cell has to read as a colour laid *on* it.
	paper = lightest.Lighten(0.8).Desaturate(0.55)
	// The line is graphite, not a pigment: the palette's darkest pushed down
	// until it stops being a colour. Drawn in a palette colour the walls
	// join the composition and the structure stops reading as a structure.
	ink = shade(darkest.Desaturate(0.45), 0.42)
	// Everything from the palette is available as a pigment — the reference
	// is polychrome, and a restricted ramp turns a foam into a diagram.
	ramp = append([]palette.Color(nil), byLum...)
	return paper, ink, ramp
}

// pack scatters the sites the cells are grown from. Best-candidate darts
// with a size ladder, the vocabulary sketch 008 places its marks with —
// what is different here is the overscan: the pack covers a rectangle
// larger than the frame, so the cells at the border are *cut* by the edge.
// Packed inside the frame instead, the border cells run out to nothing and
// the sheet reads as an object floating on paper rather than as a fragment
// of something larger.
func (s *Sketch) pack(l levels, rng *rand.Rand, aspect float64) []cells.Site {
	radii, weights := rnd.Ladder(l.base, l.ratio, l.rungs, 0.7)
	minX, minY := -l.over, -l.over
	maxX, maxY := aspect+l.over, 1+l.over
	index := geom.NewIndexIn(minX, minY, maxX, maxY, radii[len(radii)-1])

	var out []cells.Site
	for tries := 0; len(out) < l.count && tries < l.count*8; tries++ {
		r := radii[rnd.PickIndex(rng, weights)]
		// Best-candidate: a handful of darts, keep the one furthest from
		// what is already down. Many darts would maximise the spacing, and
		// evenly spaced sites give an even foam — the reference's character
		// is in the slivers wedged between the big lobes.
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
		// A foam of one cell is not a foam. Only reachable by pinning the
		// count to nothing, and cells.New needs at least one site.
		out = append(out, cells.Site{X: aspect / 2, Y: 0.5, W: l.base})
	}
	return out
}

// nearest is the distance from a point to the closest site placed so far,
// or a large number while the sheet is empty.
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

// shade multiplies a colour's channels, which is the one operation the
// palette package does not have: Lighten and Desaturate both go through
// HSL, and darkening a pigment toward graphite wants neither.
func shade(c palette.Color, f float64) palette.Color {
	return palette.Color{R: c.R * f, G: c.G * f, B: c.B * f}.Clamp()
}
