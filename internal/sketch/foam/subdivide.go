package foam

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// The mosaic layer: a second, much finer partition laid over the whole
// sheet, so a point's identity is the *pair* (outer cell, inner tile).
//
// It is one `cells.Foam` over the whole canvas rather than a site set per
// cell, and that is the load-bearing choice. A point is looked up in both
// partitions at the same warped coordinate, the tile decides the colour and
// the outer ink is drawn on top afterwards — so the heavy line clips the
// fine net for free, with no clipping code anywhere. Per-cell site sets
// would need a foam per cell (forty measuring passes instead of one) and
// would still have to solve the same clipping problem at every border.
//
// What a single global foam apparently cannot do is give a big lobe and a
// sliver comparable tile counts, since one site spacing serves the whole
// sheet. It can: the pack is a *variable-radius* dart throw, and a dart's
// radius is read from the inradius of whichever outer cell it lands in. The
// spacing therefore follows the outer structure while the partition stays
// global. That is why cells.Cell carries an inradius — the same measurement
// the band fill uses to decide whether it can resolve.
//
// The inner sites carry weight 0, so the inner metric is an ordinary
// Voronoi: straight bisectors, sharp corners. That is deliberate contrast.
// The outer foam is a bubble cluster; making the tiles a second bubble
// cluster gives one texture at two scales, and the sheet reads as a blur.
// Crystal inside organic reads as two things. Both are bent by the same
// warp, so the tiles still lean with the cells they sit in.

// The colour schemes. Each is a different answer to "where does a tile get
// its colour from", and they are what the mosaic trait resolves to.
const (
	schemeNone      = iota // no subdivision at all
	schemeFamily           // a walk along the ramp within each outer cell
	schemeStrata           // hue by position on the sheet, value by tile area
	schemeTonal            // one hue per outer cell, tiles vary in lightness
	schemeSolo             // one cell in full colour, the rest near-neutral
	schemeNeighbour        // a tile wears the colour of the cell over its wall
	nschemes
)

// sublevels is everything the mosaic trait resolves to.
type sublevels struct {
	scheme int
	tile   float64 // inner site radius, × the outer cell's inradius
	share  float64 // share of outer cells that are subdivided at all
	fine   float64 // inner line width, × the outer ink
	spread float64 // how far a scheme travels, in ramp steps across a cell
}

// maxTiles caps the inner pack. A sheet of ten thousand tiles is a texture,
// not a mosaic, and every tile past a few thousand costs a measuring sample
// without changing what the picture says.
const maxTiles = 4500

// saltTile salts the per-tile hash so the tiles' own variation does not move
// when the paper's tooth does.
const saltTile = 0x74696c65 // "tile"

// piece is the mosaic dressing of one *outer* cell.
type piece struct {
	on       bool    // is this cell subdivided, or left as a plain fill
	anchor   int     // where on the ramp this cell's family of tiles starts
	cos, sin float64 // the direction a family walks across the cell
}

// tiling is the built mosaic layer.
type tiling struct {
	foam  *cells.Foam
	piece []piece

	// Per tile, indexed by tile id. Measured tiles that fall outside the
	// frame have no area or centroid, so both fall back to the site itself:
	// the warp can pull an off-frame tile into view, and a tile with a
	// centroid at the origin would take its colour from the corner.
	cx, cy []float64
	area   []float64
	inrad  []float64
	near   []int32   // the outer cell over this tile's nearest outer wall
	edge   []float64 // how near the tile's centre is to an outer wall, 0..1

	ref  float64 // the median tile area — the reference for "big" and "small"
	mean float64 // the mean inner site radius, the fallback tile scale
	ink  float64 // the inner line's width, canvas units
	hero int     // the cell the soloist scheme gives full colour to
}

// subdivide builds the mosaic. It returns nil when the sheet is not
// subdivided, which every caller downstream treats as "draw the plain
// fills" — so the mosaic costs a nil check and nothing else when it is off.
func (s *Sketch) subdivide(f *cells.Foam, l levels, rng *rand.Rand, aspect float64, skin []dress, ramp []palette.Color) *tiling {
	if l.sub.scheme == schemeNone || l.sub.share <= 0 {
		return nil
	}
	sites, mean := s.tileSites(f, l, rng, aspect)
	if len(sites) < 2 {
		return nil
	}
	// Corners rounded only slightly, and a short node range: the tiles are
	// meant to read as cut facets, not as a second crop of bubbles.
	inner := cells.New(sites, cells.Identity(len(sites)), aspect,
		cells.Params{Node: mean * 0.35, Round: mean * 0.10, Stat: statRes})

	t := &tiling{foam: inner, mean: mean, ink: l.ink * l.sub.fine, hero: -1}
	t.measure(inner, sites, f, l)
	t.dress(f, l, rng, aspect, skin, ramp)
	return t
}

// tileSites throws variable-radius darts over the pack's whole overscanned
// rectangle, sizing each one from the outer cell it lands in.
func (s *Sketch) tileSites(f *cells.Foam, l levels, rng *rand.Rand, aspect float64) (sites []cells.Site, mean float64) {
	minX, minY := -l.over, -l.over
	maxX, maxY := aspect+l.over, 1+l.over

	// The index's cell size is a performance knob only (see geom.NewIndexIn),
	// so it is sized to the typical tile rather than the largest one.
	typical := math.Max(l.base*l.sub.tile, 1e-3)
	index := geom.NewIndexIn(minX, minY, maxX, maxY, typical)

	sum := 0.0
	for tries := 0; tries < 90000 && len(sites) < maxTiles; tries++ {
		x := minX + rng.Float64()*(maxX-minX)
		y := minY + rng.Float64()*(maxY-minY)
		r := s.tileRadius(f, l, x, y)
		c := geom.Circle{X: x, Y: y, R: r}
		if !index.FitsWithGap(c, 0) {
			continue
		}
		index.Insert(c)
		// Weight 0: the inner metric is an ordinary Voronoi, straight-walled
		// against the outer foam's arcs.
		sites = append(sites, cells.Site{X: x, Y: y})
		sum += r
	}
	if len(sites) == 0 {
		return nil, typical
	}
	return sites, sum / float64(len(sites))
}

// tileRadius is the whole of "every cell gets a comparable tile count": the
// spacing at a point is a fixed fraction of the inradius of the outer cell
// that owns it, so a quarter-canvas lobe and a sliver come out with roughly
// the same number of tiles rather than the lobe getting fifty and the
// sliver one.
func (s *Sketch) tileRadius(f *cells.Foam, l levels, x, y float64) float64 {
	c := f.Cells()[f.At(x, y).Cell]
	r := c.Inradius
	if r <= 0 {
		// A cell measured entirely outside the frame has no inradius. Its
		// tiles are off-paper anyway; the smallest site is a safe stand-in.
		r = l.base
	}
	return math.Max(r*l.sub.tile, l.base*0.07)
}

// measure copies out what each tile knows, with the site as the fallback for
// anything the measuring pass could not see, and asks the outer foam what
// each tile's neighbourhood looks like.
func (t *tiling) measure(inner *cells.Foam, sites []cells.Site, outer *cells.Foam, l levels) {
	n := inner.Len()
	t.cx, t.cy = make([]float64, n), make([]float64, n)
	t.area, t.inrad = make([]float64, n), make([]float64, n)
	t.near, t.edge = make([]int32, n), make([]float64, n)

	var areas []float64
	for i, c := range inner.Cells() {
		t.cx[i], t.cy[i] = c.CX, c.CY
		t.area[i], t.inrad[i] = c.Area, c.Inradius
		if c.Area <= 0 {
			t.cx[i], t.cy[i] = sites[i].X, sites[i].Y
			t.inrad[i] = t.mean
		} else {
			areas = append(areas, c.Area)
		}
		// Which outer cell is over this tile's nearest outer wall, and how
		// near that wall the tile sits. Asked once per tile rather than once
		// per pixel: a tile is one flat piece of colour, so its relationship
		// to the outer structure is a property of the tile, not of the point.
		oh := outer.At(t.cx[i], t.cy[i])
		t.near[i] = int32(oh.Cell) //nolint:gosec // cell counts are small
		if oh.Near >= 0 {
			t.near[i] = int32(oh.Near) //nolint:gosec // cell counts are small
		}
		// Measured against the *containing cell's* own size, not against a
		// fixed length. A fixed one is longer than a small cell is wide, so
		// every tile of every small cell counts as being at the wall and the
		// scheme collapses to a uniform half-and-half mix of two pigments —
		// which was the first version, and it read as mud.
		span := outer.Cells()[oh.Cell].Inradius
		if span <= 0 {
			span = l.base
		}
		if !math.IsInf(oh.Wall, 1) {
			t.edge[i] = 1 - mathx.Smoothstep(0, span*0.65, oh.Wall)
		}
	}
	sort.Float64s(areas)
	if len(areas) > 0 {
		t.ref = areas[len(areas)/2]
	}
}

// dress settles the per-outer-cell half of the mosaic: which cells are
// subdivided at all, and where each one's family of tiles sits on the ramp.
func (t *tiling) dress(f *cells.Foam, l levels, rng *rand.Rand, aspect float64, skin []dress, ramp []palette.Color) {
	t.piece = make([]piece, f.Len())
	for i := range t.piece {
		p := piece{on: rng.Float64() < l.sub.share}
		// The family starts where the cell's own pigment already sits, so
		// the passages the plain sheet has — a green corner, a brown corner
		// — survive the subdivision instead of being overwritten by it.
		p.anchor = rampIndex(ramp, skin[i].pigment)
		a := rnd.Uniform(rng, 0, 2*math.Pi)
		p.cos, p.sin = math.Cos(a), math.Sin(a)
		t.piece[i] = p
	}
	// The soloist's one cell: large, and near the middle of the sheet. Area
	// alone picks a corner cell as often as not, and a corner cell is cut in
	// half by the frame — the sheet then reads as a grey field with
	// something coloured escaping out of the edge, rather than as a subject
	// with a ground around it.
	best := 0.0
	for i, c := range f.Cells() {
		if !t.piece[i].on || c.Area <= 0 {
			continue
		}
		d := mathx.Clamp01(math.Hypot((c.CX-aspect/2)/(aspect/2), (c.CY-0.5)/0.5))
		if score := c.Area * (1 - 0.75*d); score > best {
			t.hero, best = i, score
		}
	}
}

// rampIndex finds a pigment's place on the ramp. Pigments are copied out of
// the ramp, so equality is exact; a colour from anywhere else falls in the
// middle, which is the least opinionated place to start a walk from.
func rampIndex(ramp []palette.Color, c palette.Color) int {
	for i, r := range ramp {
		if r == c {
			return i
		}
	}
	return len(ramp) / 2
}

// tile replaces the plain fill with the mosaic wherever a cell was
// subdivided, and lays the fine inner net over it.
//
// (wu, wv) is the *warped* point, the same one the outer lookup used: both
// partitions have to be read through one displacement or the tiles slide out
// of the cells they belong to.
func (s *Sketch) tile(sh *sheet, oh cells.Hit, u, v, wu, wv float64, base palette.Color, seed uint64) palette.Color {
	t := sh.tiles
	if t == nil || !t.piece[oh.Cell].on {
		return base
	}
	ih := t.foam.At(wu, wv)
	col := s.tileColour(sh, t, oh.Cell, ih.Cell, seed)

	// Each tile dries like a small flat wash: a rim where it meets its own
	// wall, and the paper's tooth through it. Without the rim the sheet is a
	// chart; with it the tiles read as laid rather than filled.
	if !math.IsInf(ih.Wall, 1) {
		rim := 1 - mathx.Smoothstep(0, math.Max(t.mean*0.35, s.RimWide*0.5), ih.Wall)
		col = palette.Lerp(col, shade(col, 0.78), rim*s.Rim*0.7)
	}
	col = shade(col, 1+s.grain(seed, u, v))
	return t.lay(col, ih, sh.ink)
}

// lay draws the inner net: the same taper-and-swell rule as the outer line,
// at a fraction of its width and let down toward the paper. Drawn at full
// strength in the same graphite it competes with the structure, and the
// sheet stops having a hierarchy — which is the one thing subdividing must
// not cost.
func (t *tiling) lay(col palette.Color, h cells.Hit, ink palette.Color) palette.Color {
	if t.ink <= 0 || math.IsInf(h.Wall, 1) {
		return col
	}
	half := t.ink / 2 * (1 + 0.9*h.Node)
	soft := math.Max(t.ink*0.4, 0.0003)
	a := 1 - mathx.Smoothstep(half-soft, half+soft, h.Wall)
	if a <= 0 {
		return col
	}
	return palette.Lerp(col, ink, a*0.72)
}

// tileColour is the mosaic's point: five different answers to where a
// tile's pigment comes from.
func (s *Sketch) tileColour(sh *sheet, t *tiling, cell, id int, seed uint64) palette.Color {
	ramp := sh.ramp
	p := t.piece[cell]
	// One value per (cell, tile) pair rather than per tile: a tile straddling
	// an outer wall is cut in two by the ink, and the two halves belong to
	// different cells, so they are entitled to differ.
	jitter := noise.Hash01(seed^saltTile, int64(cell), int64(id))

	switch sh.level.sub.scheme {
	case schemeFamily:
		// A walk along the ramp *within* the cell: the tile's centroid
		// projected onto a per-cell axis, in units of the cell's own size.
		// Every tile of a cell therefore comes from a short run of adjacent
		// pigments, and the cell reads as one family rather than as confetti
		// — which is what happens when each tile draws freely.
		c := sh.foam.Cells()[cell]
		span := math.Max(c.Inradius, sh.level.base)
		w := ((t.cx[id]-c.CX)*p.cos + (t.cy[id]-c.CY)*p.sin) / (2 * span)
		step := int(math.Round(w * sh.level.sub.spread))
		return ramp[wrap(p.anchor+step, len(ramp))]

	case schemeStrata:
		// Hue from where the tile is on the *sheet*, ignoring the cells
		// entirely, and value from how big the tile is. The colour field
		// crosses the walls, so the structure and the colour are two
		// independent readings of the same picture.
		w := s.Passage * 1.6
		f := mathx.Clamp01(0.5 + 1.6*sh.field.FBM(t.cx[id]/w, t.cy[id]/w, 2))
		col := ramp[min(int(f*float64(len(ramp))), len(ramp)-1)]
		big := mathx.Clamp01(t.area[id] / math.Max(2*t.ref, 1e-9))
		return palette.Lerp(shade(col, 0.72), col.Lighten(0.22), big)

	case schemeTonal:
		// One hue for the cell; the tiles differ only in how much light is
		// in them. The most restrained of the five, and the one that keeps
		// the outer shapes most legible.
		col := ramp[p.anchor]
		lift := (jitter - 0.5) * 2
		if lift >= 0 {
			return col.Lighten(lift * 0.55)
		}
		return shade(col, 1+lift*0.42)

	case schemeSolo:
		// One cell in full colour and the sheet near-neutral around it.
		// The hero's tiles walk its family; everything else is drained to
		// paper, keeping just enough pigment to tell the tiles apart.
		if cell == t.hero {
			c := sh.foam.Cells()[cell]
			span := math.Max(c.Inradius, sh.level.base)
			w := ((t.cx[id]-c.CX)*p.cos + (t.cy[id]-c.CY)*p.sin) / (2 * span)
			step := int(math.Round(w * sh.level.sub.spread))
			return ramp[wrap(p.anchor+step, len(ramp))]
		}
		quiet := ramp[p.anchor].Desaturate(0.88).Lighten(0.32)
		return shade(quiet, 0.94+0.12*jitter)

	default: // schemeNeighbour
		// A tile takes the pigment of the cell across its nearest *outer*
		// wall, in proportion to how near that wall it is. A cell's rim
		// therefore wears its neighbours' colours and only its core keeps
		// its own, so the cells bleed into one another while the ink still
		// says exactly where the boundary is.
		own := ramp[p.anchor]
		n := int(t.near[id])
		if n < 0 || n >= len(t.piece) || n == cell {
			return own
		}
		other := ramp[t.piece[n].anchor]
		// Committed, not hedged: a tile at the wall is *its neighbour's*
		// colour. Blended halfway it reads as a third pigment nobody chose,
		// and the whole point — that a cell's rim belongs to the cell it is
		// touching — is lost. The jitter only roughens the boundary between
		// the two territories so it does not fall on a contour line.
		w := mathx.Clamp01(t.edge[id] * (0.75 + 0.5*jitter))
		return palette.Lerp(own, other, mathx.Smoothstep(0.15, 0.85, w))
	}
}

// wrap folds an index into the ramp. Reflection rather than modulo: the
// ramp is ordered by luminance, so wrapping round jumps from the darkest
// pigment to the lightest and a walk that was a gradient becomes a cliff.
func wrap(i, n int) int {
	if n <= 1 {
		return 0
	}
	period := 2 * (n - 1)
	i = ((i % period) + period) % period
	if i >= n {
		i = period - i
	}
	return i
}

// newMosaic draws how the sheet is subdivided and how the tiles are
// coloured. The two belong on one axis because they are one decision: a
// scheme that walks the ramp across a cell needs enough tiles in that cell
// for the walk to be a walk, and a scheme that colours by tile area needs
// tiles whose areas differ. Choosing the subdivision and the colouring
// independently gives combinations that say nothing.
func newMosaic(level string, rng *rand.Rand, l *levels) {
	var m sublevels
	switch level {
	case "family":
		m.scheme = schemeFamily
		m.tile, m.share = rnd.Uniform(rng, 0.24, 0.34), rnd.Uniform(rng, 0.55, 0.82)
		// Two or three neighbours on the ramp, not five. The ramp is ordered
		// by luminance, so adjacent entries agree about lightness and can
		// disagree wildly about hue — a walk that crosses five of them is
		// the whole palette in one cell, which is the confetti the passages
		// exist to avoid.
		m.spread = rnd.Uniform(rng, 1.4, 2.8)
	case "strata":
		m.scheme = schemeStrata
		m.tile, m.share = rnd.Uniform(rng, 0.19, 0.27), rnd.Uniform(rng, 0.75, 1)
	case "tonal":
		m.scheme = schemeTonal
		m.tile, m.share = rnd.Uniform(rng, 0.26, 0.40), rnd.Uniform(rng, 0.5, 0.8)
	case "soloist":
		m.scheme = schemeSolo
		m.tile, m.share = rnd.Uniform(rng, 0.22, 0.32), rnd.Uniform(rng, 0.85, 1)
		m.spread = rnd.Uniform(rng, 1.6, 3.2)
	case "neighbour":
		m.scheme = schemeNeighbour
		m.tile, m.share = rnd.Uniform(rng, 0.22, 0.32), rnd.Uniform(rng, 0.8, 1)
	default: // plain
		l.sub = sublevels{scheme: schemeNone}
		return
	}
	// A third of the outer line. Any heavier and the sheet has two
	// structures arguing with each other; any lighter and the tiles stop
	// being tiles and read as a gradient.
	m.fine = rnd.Uniform(rng, 0.26, 0.40)
	l.sub = m
}

// declareLook names the mosaic and relief knobs. They are declared here
// rather than in declare() so that the whole subdivision is one file to
// read and one file to remove.
func (s *Sketch) declareLook(o *opt.Set) {
	o.Float("tile", "inner tile size, x the outer cell's inradius", "ti", 0.06, 0.9, &s.pin.sub.tile)
	o.Float("tiled", "share of cells subdivided into tiles", "tl", 0, 1, &s.pin.sub.share)
	o.Float("fine", "inner net width, x the outer ink", "fn", 0, 1.5, &s.pin.sub.fine)
	o.Float("spread", "ramp steps a cell's family of tiles walks", "sp", 0, 14, &s.pin.sub.spread)
	o.Float("depth", "rise of the relief, x the smallest cell", "dp", 0, 1.5, &s.Depth)
	o.Float("bevel", "run that rise happens over, x the smallest cell", "bv", 0.02, 3, &s.Bevel)
	o.Float("light", "the light's bearing in degrees; 90 is from the top", "lg", 0, 360, &s.Light)
}

// overrideLook applies whichever of those knobs the caller actually gave.
// depth and bevel are multiples of the smallest cell, as the ink is, so
// they resolve against the pack this render ended up with rather than
// against the page: a rise that reads as a chamfer on an open sheet is a
// cliff on a packed one.
func (s *Sketch) overrideLook(l *levels) {
	s.override([]knob{
		{"tile", func() { l.sub.tile = s.pin.sub.tile }},
		{"tiled", func() { l.sub.share = s.pin.sub.share }},
		{"fine", func() { l.sub.fine = s.pin.sub.fine }},
		{"spread", func() { l.sub.spread = s.pin.sub.spread }},
		{"depth", func() { l.rel.depth = l.base * s.Depth }},
		{"bevel", func() {
			l.rel.width = l.base * s.Bevel
			l.rel.step = math.Max(l.rel.width*0.66, 5e-4)
		}},
		{"light", func() {
			// Screen coordinates grow downward, so a bearing read the way a
			// compass rose is read has to be flipped in v.
			a := s.Light * math.Pi / 180
			lx, ly, lz := math.Cos(a), -math.Sin(a), l.rel.lz
			n := math.Sqrt(lx*lx + ly*ly + lz*lz)
			l.rel.lx, l.rel.ly, l.rel.lz = lx/n, ly/n, lz/n
		}},
	})
}
