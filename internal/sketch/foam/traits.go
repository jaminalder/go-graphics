package foam

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The output space. Four axes decide the sheet and one decides its colour,
// and each resolves to *ranges* rather than to numbers — QQL's idea, and
// the reason a trait is worth having over a flag: "busy" means a busy
// sheet, not one particular busy sheet.
const (
	dimDensity = "density"
	dimLobes   = "lobes"
	dimFills   = "fills"
	dimLine    = "line"
	dimMosaic  = "mosaic"
	dimRelief  = "relief"
	dimWater   = "water"
	dimScheme  = "scheme"
)

var schema = trait.Schema{
	{
		Name: dimColourway, Key: "c", InName: true,
		Doc:    "which palette the sheet is drawn from",
		Values: colourways,
	},
	{
		Name: dimDensity, Key: "d", InName: true,
		Doc: "how finely the sheet is divided",
		Values: []trait.Value{
			{Name: "sparse", Weight: 1},
			{Name: "open", Weight: 2},
			{Name: "medium", Weight: 3},
			{Name: "busy", Weight: 3},
			{Name: "packed", Weight: 2},
			// Finer than packed, and weight 0 so no seed draws it: at this
			// size a cell is a chip rather than a shape, which is a texture
			// to reach for on purpose rather than one to land on.
			{Name: "fine", Weight: 0},
		},
	},
	{
		Name: dimLobes, Key: "l", InName: true,
		Doc: "how many cells are lobes rather than bubbles",
		Values: []trait.Value{
			{Name: "tidy", Weight: 1},
			{Name: "few", Weight: 3},
			{Name: "many", Weight: 3},
			{Name: "most", Weight: 1},
		},
	},
	{
		Name: dimFills, Key: "f", InName: true,
		Doc: "what the cells are filled with",
		Values: []trait.Value{
			{Name: "washed", Weight: 3},
			{Name: "mixed", Weight: 3},
			{Name: "drawn", Weight: 2},
			{Name: "airy", Weight: 2},
			{Name: "net", Weight: 0},
			// Weight 0, appended: see the note on the dimension below.
			{Name: "watercolour", Weight: 0},
			{Name: "flat", Weight: 0},
		},
	},
	{
		Name: dimLine, Key: "n", InName: true,
		Doc: "the weight of the ink line",
		Values: []trait.Value{
			{Name: "fine", Weight: 2},
			{Name: "drawn", Weight: 3},
			{Name: "bold", Weight: 2},
		},
	},
	// Appended, and appended deliberately. Derive draws in schema order, so
	// the five dimensions above are untouched and every existing seed still
	// lands on the sheet it always did (decision 21 in ARCHITECTURE.md made
	// the same argument for QQL's wash medium).
	{
		Name: dimMosaic, Key: "m",
		Doc: "how the cells are subdivided into tiles, and where a tile's colour comes from",
		Values: []trait.Value{
			{Name: "plain", Weight: 1},
			{Name: "family", Weight: 0},
			{Name: "strata", Weight: 0},
			{Name: "tonal", Weight: 0},
			{Name: "soloist", Weight: 0},
			{Name: "neighbour", Weight: 0},
		},
	},
	{
		Name: dimRelief, Key: "r",
		Doc: "how the sheet is lit",
		Values: []trait.Value{
			{Name: "flat", Weight: 1},
			{Name: "bevel", Weight: 0},
			{Name: "cushion", Weight: 0},
			{Name: "occlude", Weight: 0},
			{Name: "terrace", Weight: 0},
			{Name: "glass", Weight: 0},
		},
	},
	// The two watercolour dimensions are *appended*, which is what keeps
	// every existing seed drawing the piece it drew before: Derive consumes
	// one draw per dimension in schema order, so the dimensions above are
	// untouched by adding to the end (the argument is decision 21 in
	// docs/ARCHITECTURE.md, made for QQL's wash medium).
	//
	// `watercolour` is added to `fills` at weight 0 for the other half of
	// the same argument: a value with weight 0 does not change the
	// dimension's total, so no seed's `fills` draw moves either.
	{
		Name: dimWater, Key: "w", InName: true,
		Doc: "what the paint did while it dried",
		Values: []trait.Value{
			{Name: waterPlain, Weight: 2},
			{Name: waterCharge, Weight: 3},
			{Name: waterBloom, Weight: 2},
			{Name: waterGlaze, Weight: 2},
			{Name: waterSed, Weight: 2},
			{Name: waterBled, Weight: 2},
			{Name: waterStudio, Weight: 3},
		},
	},
	{
		Name: dimScheme, Key: "s", InName: true,
		Doc: "how colour is organised over the sheet",
		Values: []trait.Value{
			// Weight 0 on everything but passage, which is what every sheet
			// was implicitly painted with before the schemes existed. The
			// scheme drives pencil, hatch and band cells as well as
			// watercolour ones, so giving the others weight here would
			// change what every existing seed looks like — the one thing
			// appending a dimension is supposed to avoid. Promoting them is
			// this block, and nothing else.
			{Name: schemePassage, Weight: 1},
			{Name: schemeGradient, Weight: 0},
			{Name: schemeSequence, Weight: 0},
			{Name: schemeInherit, Weight: 0},
			{Name: schemeDominance, Weight: 0},
			{Name: schemeComplement, Weight: 0},
			{Name: schemeAnalogous, Weight: 0},
			{Name: schemeTriad, Weight: 0},
			{Name: schemeQuiet, Weight: 0},
			{Name: schemeNotan, Weight: 0},
			{Name: schemeAnchor, Weight: 0},
			{Name: schemeWeather, Weight: 0},
			{Name: schemeDuet, Weight: 0},
			{Name: schemeByScale, Weight: 0},
			{Name: schemeFromLum, Weight: 0},
		},
	},
}

// levels is everything the traits resolve to: the site pack, the merging,
// the ink and the mix of fills. It is drawn once per render and then
// overridden by whatever the caller pinned on the command line.
type levels struct {
	// the pack
	count int     // sites, before the pack gives up
	rungs int     // steps on the size ladder
	base  float64 // smallest site radius, canvas units
	ratio float64 // ladder step ratio
	gap   float64 // clearance between sites, ×radius
	over  float64 // overscan beyond the frame, canvas units
	darts int     // candidates thrown per site

	// the bending
	merge   float64 // share of sites that reach for a neighbour
	maxLobe int     // most sites one lobe may absorb
	warp    float64 // how far the partition is bent, ×smallest cell
	swirl   float64 // wavelength of that bending, ×smallest cell

	// the line
	ink   float64 // wall thickness, canvas units
	swell float64 // extra thickness at a junction, ×ink
	node  float64 // distance over which a third cell counts as near
	round float64 // radius a cell's corners are rounded over, canvas units

	// the fills
	styles [nstyles]float64 // relative weight of each style

	// the mosaic and the light (subdivide.go, shade.go)
	sub sublevels
	rel reliefLevels
	// the paint (watercolour.go, scheme.go)
	water  waterLevels
	scheme string
}

// defaults are the levels shown in --help. What a render actually uses is
// drawn from the seed's traits.
func defaults(rng *rand.Rand) levels {
	l := newDensity("medium", rng)
	newLobes("few", rng, &l)
	newLine("drawn", rng, &l)
	newFills("mixed", &l)
	newMosaic("family", rng, &l)
	newRelief("bevel", rng, &l)
	newWater(waterStudio, rng, &l)
	l.scheme = schemePassage
	return l
}

// newDensity draws the pack for one density level.
//
// The ladder always reaches a site several times the radius of the smallest
// one, at every level: a foam of uniformly sized cells is a honeycomb, and
// what makes the reference read as drawn rather than generated is that a
// quarter-canvas lobe sits directly against a sliver.
func newDensity(level string, rng *rand.Rand) levels {
	// Few darts on purpose — see pack.
	l := levels{darts: 5}
	switch level {
	case "sparse":
		l.count = 22 + rng.IntN(7)
		l.rungs, l.base = 3, rnd.Uniform(rng, 0.075, 0.09)
		l.ratio, l.gap = rnd.Uniform(rng, 1.45, 1.70), rnd.Uniform(rng, 0.10, 0.22)
	case "open":
		l.count = 32 + rng.IntN(11)
		l.rungs, l.base = 4, rnd.Uniform(rng, 0.058, 0.070)
		l.ratio, l.gap = rnd.Uniform(rng, 1.42, 1.64), rnd.Uniform(rng, 0.08, 0.18)
	case "medium":
		l.count = 50 + rng.IntN(15)
		l.rungs, l.base = 4, rnd.Uniform(rng, 0.044, 0.054)
		l.ratio, l.gap = rnd.Uniform(rng, 1.40, 1.60), rnd.Uniform(rng, 0.06, 0.14)
	case "busy":
		l.count = 85 + rng.IntN(26)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.032, 0.040)
		l.ratio, l.gap = rnd.Uniform(rng, 1.36, 1.55), rnd.Uniform(rng, 0.04, 0.11)
	case "fine":
		// A step finer again. The ladder keeps its five rungs so the big
		// lobes survive — a sheet of uniformly tiny cells is a texture, and
		// what makes this read as a foam rather than as gravel is that a
		// quarter-canvas shape still sits among the chips.
		l.count = 260 + rng.IntN(70)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.016, 0.021)
		l.ratio, l.gap = rnd.Uniform(rng, 1.30, 1.46), rnd.Uniform(rng, 0.01, 0.06)
	default: // packed
		l.count = 140 + rng.IntN(41)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.024, 0.030)
		l.ratio, l.gap = rnd.Uniform(rng, 1.32, 1.50), rnd.Uniform(rng, 0.02, 0.08)
	}
	// Overscan scales with the cells: deep enough that a border cell has a
	// neighbour outside the frame to be cut against, and no deeper, because
	// every site placed out there is one the sheet does not get. The counts
	// above already carry the loss — roughly a third of them at the open
	// end, a tenth at the packed one.
	l.over = l.base * rnd.Uniform(rng, 1.6, 2.2)
	return l
}

// newLobes draws how much of the sheet bends — both ways it can.
//
// Merging is what separates this from stained glass: a pair of merged
// bubbles is a crescent that wraps around its neighbour, and a sheet with
// none of them reads as a shattered pane. The warp is the other half. The
// weighted metric only curves a wall where the two sites either side of it
// differ sharply in size, and most neighbours do not, so without a warp the
// typical wall comes out straight however many lobes there are.
//
// They belong on one axis because they are one decision. A sheet that bends
// its walls and never merges a cell is as inconsistent as the reverse.
func newLobes(level string, rng *rand.Rand, l *levels) {
	switch level {
	case "tidy":
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.02, 0.08), 2
		l.warp = rnd.Uniform(rng, 0.25, 0.55)
	case "few":
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.14, 0.26), 2
		l.warp = rnd.Uniform(rng, 0.55, 0.95)
	case "many":
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.32, 0.48), 3
		l.warp = rnd.Uniform(rng, 0.85, 1.35)
	default: // most
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.55, 0.75), 3
		l.warp = rnd.Uniform(rng, 1.1, 1.7)
	}
	l.swirl = rnd.Uniform(rng, 20, 32)
}

// newLine draws the weight of the ink.
//
// The width is a fraction of the smallest site radius, not a distance on
// the page. A line is heavy or fine *relative to the cells it divides*: the
// same 0.012 that reads as a confident stroke on an open sheet closes a
// packed one into a black net with pebbles in it, which is what the first
// version did.
//
// swell is the ratio between the junction and the mid-wall thickness, and
// it is the single number that decides whether the picture reads as a foam.
// At 0 the walls are a constant-width net; the reference's junctions are
// two to three times the thickness of the wall they join.
func newLine(level string, rng *rand.Rand, l *levels) {
	var f float64
	switch level {
	case "fine":
		f, l.swell = rnd.Uniform(rng, 0.05, 0.075), rnd.Uniform(rng, 1.2, 1.9)
	case "bold":
		f, l.swell = rnd.Uniform(rng, 0.15, 0.22), rnd.Uniform(rng, 1.0, 1.6)
	default: // drawn
		f, l.swell = rnd.Uniform(rng, 0.09, 0.14), rnd.Uniform(rng, 1.2, 1.9)
	}
	l.ink = l.base * f
	// How far from a junction the swelling reaches — a fraction of a cell,
	// not a multiple of the line. Sized off the ink instead, the reach on a
	// packed sheet is longer than the walls are, every point on every wall
	// counts as a junction, and the swelling that was meant to be a fillet
	// becomes a uniform dark halo with pebbles floating in it.
	l.node = l.base * rnd.Uniform(rng, 0.25, 0.45)
	// How hard the corners are rounded. A cell of a foam meets its
	// neighbours at 120°, so left alone it is a polygon with curved sides —
	// and at a glance a polygon is what it reads as, however well the sides
	// curve. Rounding is what turns it into a pebble.
	//
	// It has to stay well under half a cell. Past that the corners eat the
	// walls between them: the cells come apart into discs floating in ink
	// rather than sharing a boundary, and a foam whose cells do not touch is
	// not a foam. It also does the swelling's job at the junctions, which is
	// why the swell ranges above are half what they were before rounding
	// existed — run together they gave every node a blot.
	l.round = l.base * rnd.Uniform(rng, 0.16, 0.28)
}

// newFills sets the mix of fill styles.
//
// Every level keeps some empty cells. They are load-bearing: a sheet with
// every cell filled has nowhere to rest, and the reference leaves roughly
// one cell in six as bare paper.
func newFills(level string, l *levels) {
	switch level {
	case "net":
		// Bare paper everywhere: the structure with nothing in it. Weight 0
		// in the schema, so no seed lands on it — an unfilled sheet is a
		// drawing of the algorithm rather than a picture — but it is one
		// flag away, and it is the only way to actually look at what the
		// walls are doing.
		l.styles = [nstyles]float64{styleEmpty: 1}
	case "washed":
		l.styles = [nstyles]float64{styleWash: 10, stylePencil: 1, styleBands: 1, styleHatch: 0.5, styleEmpty: 2.5}
	case "drawn":
		l.styles = [nstyles]float64{styleWash: 2, stylePencil: 8, styleBands: 2, styleHatch: 2.5, styleEmpty: 3}
	case "airy":
		l.styles = [nstyles]float64{styleWash: 4, stylePencil: 4, styleBands: 1, styleHatch: 1, styleEmpty: 6}
	case "flat":
		// Every cell one flat colour. The plainest possible fill, and the
		// right one for judging a colour scheme: any texture on top of the
		// arrangement is a second thing to look at.
		l.styles = [nstyles]float64{styleFlat: 1}
	case "watercolour":
		// A painted sheet: every cell a wash, bar the few left as paper.
		// Those few are still load-bearing — a sheet with every cell filled
		// has nowhere to rest — but the pencil has no place on a sheet that
		// is about what the water did.
		l.styles = [nstyles]float64{styleWash: 14, styleEmpty: 2}
	default: // mixed
		l.styles = [nstyles]float64{styleWash: 6, stylePencil: 5, styleBands: 2, styleHatch: 1.5, styleEmpty: 3}
	}
}

// levelsFor resolves the whole sheet for one render: what the seed's traits
// drew, with any pinned flag laid over it. This is the point of the traits
// — a caller can move one number without restating the other twelve, and a
// caller who says nothing still gets a coherent sheet.
func (s *Sketch) levelsFor(tr trait.Set, rng *rand.Rand) levels {
	l := newDensity(tr.Get(dimDensity), rng)
	// The pack overrides land *before* the line is drawn, because the ink
	// width is a fraction of the smallest site: pinning --base has to move
	// the line with it, or a hand-set cell size comes out under a line
	// scaled for the one the seed drew.
	s.override([]knob{
		{"count", func() { l.count = s.pin.count }},
		{"rungs", func() { l.rungs = s.pin.rungs }},
		{"base", func() { l.base = s.pin.base }},
		{"ratio", func() { l.ratio = s.pin.ratio }},
		{"gap", func() { l.gap = s.pin.gap }},
		{"over", func() { l.over = s.pin.over }},
	})

	newLobes(tr.Get(dimLobes), rng, &l)
	newLine(tr.Get(dimLine), rng, &l)
	newFills(tr.Get(dimFills), &l)
	newMosaic(tr.Get(dimMosaic), rng, &l)
	newRelief(tr.Get(dimRelief), rng, &l)
	s.overrideLook(&l)
	newWater(tr.Get(dimWater), rng, &l)
	l.scheme = tr.Get(dimScheme)

	s.override([]knob{
		{"merge", func() { l.merge = s.pin.merge }},
		{"warp", func() { l.warp = s.pin.warp }},
		{"swirl", func() { l.swirl = s.pin.swirl }},
		{"max-lobe", func() { l.maxLobe = s.pin.maxLobe }},
		{"ink", func() { l.ink = s.pin.ink }},
		{"swell", func() { l.swell = s.pin.swell }},
		{"node", func() { l.node = s.pin.node }},
		{"round", func() { l.round = s.pin.round }},
		{"wash", func() { l.styles[styleWash] = s.pin.styles[styleWash] }},
		{"pencil", func() { l.styles[stylePencil] = s.pin.styles[stylePencil] }},
		{"bands", func() { l.styles[styleBands] = s.pin.styles[styleBands] }},
		{"hatch", func() { l.styles[styleHatch] = s.pin.styles[styleHatch] }},
		{"empty", func() { l.styles[styleEmpty] = s.pin.styles[styleEmpty] }},
		{"load", func() { l.water.load = s.pin.water.load }},
		{"tonal", func() { l.water.spread = s.pin.water.spread }},
		{"overshoot", func() { l.water.over = s.pin.water.over }},
		{"granulate", func() { l.water.grain = s.pin.water.grain }},
		{"bleed", func() { l.water.bleed = s.pin.water.bleed }},
		{"seep", func() { l.water.seep = s.pin.water.seep }},
	})
	// The warp displaces every lookup, so the pack has to reach far enough
	// that a displaced point still lands among sites. Drawn with the density
	// before the warp is known, the overscan does not know to allow for it.
	l.over = math.Max(l.over, l.base*l.warp*2.4)

	// --ink without --node would leave the swelling sized for the line the
	// seed drew rather than the one that was asked for.
	if s.knobs.WasSet("base") && !s.knobs.WasSet("node") {
		l.node = l.base * 0.35
	}
	if s.knobs.WasSet("base") && !s.knobs.WasSet("round") {
		l.round = l.base * 0.22
	}
	return l
}

// knob is one command-line override waiting to be applied.
type knob struct {
	name string
	set  func()
}

// override applies the knobs the caller actually gave. A knob left alone is
// the seed's, not the flag default's — which is why this asks WasSet rather
// than comparing values.
func (s *Sketch) override(knobs []knob) {
	for _, k := range knobs {
		if s.knobs.WasSet(k.name) {
			k.set()
		}
	}
}
