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

	// the fills
	styles [nstyles]float64 // relative weight of each style
}

// defaults are the levels shown in --help. What a render actually uses is
// drawn from the seed's traits.
func defaults(rng *rand.Rand) levels {
	l := newDensity("medium", rng)
	newLobes("few", rng, &l)
	newLine("drawn", rng, &l)
	newFills("mixed", &l)
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
		f, l.swell = rnd.Uniform(rng, 0.05, 0.075), rnd.Uniform(rng, 2.0, 3.0)
	case "bold":
		f, l.swell = rnd.Uniform(rng, 0.15, 0.22), rnd.Uniform(rng, 1.8, 2.8)
	default: // drawn
		f, l.swell = rnd.Uniform(rng, 0.09, 0.14), rnd.Uniform(rng, 2.0, 3.0)
	}
	l.ink = l.base * f
	// How far from a junction the swelling reaches — a fraction of a cell,
	// not a multiple of the line. Sized off the ink instead, the reach on a
	// packed sheet is longer than the walls are, every point on every wall
	// counts as a junction, and the swelling that was meant to be a fillet
	// becomes a uniform dark halo with pebbles floating in it.
	l.node = l.base * rnd.Uniform(rng, 0.25, 0.45)
}

// newFills sets the mix of fill styles.
//
// Every level keeps some empty cells. They are load-bearing: a sheet with
// every cell filled has nowhere to rest, and the reference leaves roughly
// one cell in six as bare paper.
func newFills(level string, l *levels) {
	switch level {
	case "washed":
		l.styles = [nstyles]float64{styleWash: 10, stylePencil: 1, styleBands: 1, styleHatch: 0.5, styleEmpty: 2.5}
	case "drawn":
		l.styles = [nstyles]float64{styleWash: 2, stylePencil: 8, styleBands: 2, styleHatch: 2.5, styleEmpty: 3}
	case "airy":
		l.styles = [nstyles]float64{styleWash: 4, stylePencil: 4, styleBands: 1, styleHatch: 1, styleEmpty: 6}
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

	s.override([]knob{
		{"merge", func() { l.merge = s.pin.merge }},
		{"warp", func() { l.warp = s.pin.warp }},
		{"swirl", func() { l.swirl = s.pin.swirl }},
		{"max-lobe", func() { l.maxLobe = s.pin.maxLobe }},
		{"ink", func() { l.ink = s.pin.ink }},
		{"swell", func() { l.swell = s.pin.swell }},
		{"node", func() { l.node = s.pin.node }},
		{"wash", func() { l.styles[styleWash] = s.pin.styles[styleWash] }},
		{"pencil", func() { l.styles[stylePencil] = s.pin.styles[stylePencil] }},
		{"bands", func() { l.styles[styleBands] = s.pin.styles[styleBands] }},
		{"hatch", func() { l.styles[styleHatch] = s.pin.styles[styleHatch] }},
		{"empty", func() { l.styles[styleEmpty] = s.pin.styles[styleEmpty] }},
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
