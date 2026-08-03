package scree

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/scheme"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The output space. Every dimension resolves to *ranges* rather than to
// numbers — QQL's idea, and the reason a trait is worth having over a flag:
// "shingle" means a shingle bed, not one particular shingle bed.
const (
	dimBed    = "bed"
	dimStones = "stones"
	dimFacets = "facets"
	dimLight  = "light"
	dimWet    = "wet"
	dimJoint  = "joint"
	dimScheme = "scheme"
)

var schema = trait.Schema{
	{
		Name: dimColourway, Key: "c", InName: true,
		Doc:    "which palette the bed is drawn from",
		Values: colourways,
	},
	{
		Name: dimBed, Key: "b", InName: true,
		Doc: "how coarse the bed is",
		Values: []trait.Value{
			{Name: "boulders", Weight: 1},
			{Name: "cobbles", Weight: 3},
			{Name: "shingle", Weight: 3},
			{Name: "gravel", Weight: 2},
			// Finer again, and weight 0 so no seed draws it: at that size a
			// stone is a chip and its facets are below what the page can
			// resolve, which is a texture to reach for on purpose.
			{Name: "grit", Weight: 0},
		},
	},
	{
		Name: dimStones, Key: "t", InName: true,
		Doc: "how worn the stones are",
		Values: []trait.Value{
			{Name: "worn", Weight: 3},
			{Name: "rolled", Weight: 3},
			{Name: "broken", Weight: 2},
			// Past what a river does. Weight 0: a deliberate departure rather
			// than an outcome to land on.
			{Name: "jumbled", Weight: 0},
		},
	},
	{
		Name: dimFacets, Key: "f", InName: true,
		Doc: "how finely each stone is cut into facets",
		Values: []trait.Value{
			{Name: "plates", Weight: 2},
			{Name: "cut", Weight: 3},
			{Name: "crazed", Weight: 3},
			{Name: "shattered", Weight: 1},
			// The control: the same surface and the same light with no facets
			// at all. Weight 0, because a smoothly shaded dome is the picture
			// the flat shading exists to replace — but it is one flag away,
			// and it is the only way to see what the facets are doing.
			{Name: "smooth", Weight: 0},
		},
	},
	{
		Name: dimLight, Key: "g", InName: true,
		Doc: "how the bed is lit",
		Values: []trait.Value{
			{Name: "raking", Weight: 2},
			{Name: "morning", Weight: 3},
			{Name: "noon", Weight: 2},
			{Name: "overcast", Weight: 1},
		},
	},
	{
		Name: dimWet, Key: "w", InName: true,
		Doc: "how much water is standing over the bed",
		Values: []trait.Value{
			{Name: "dry", Weight: 1},
			{Name: "damp", Weight: 2},
			{Name: "wet", Weight: 3},
			{Name: "sunk", Weight: 1},
		},
	},
	{
		Name: dimJoint, Key: "n",
		Doc: "the weight of the water between the stones",
		Values: []trait.Value{
			{Name: "fine", Weight: 2},
			{Name: "drawn", Weight: 3},
			{Name: "bold", Weight: 1},
		},
	},
	{
		Name: dimScheme, Key: "s", InName: true,
		Doc: "how colour is organised over the bed",
		Values: []trait.Value{
			// Weighted for a bed of a hundred small stones. The arrangements
			// that hold at that count are the ones with a *value* structure —
			// dominance, notan, anchor, quiet — and the ones that group colour
			// spatially, so the eye reads passages rather than a hundred
			// separate decisions. The strictly positional ones (gradient,
			// terrace) come out as a picture of the algorithm at this scale, so
			// they are available and rare.
			{Name: scheme.Dominance, Weight: 4},
			{Name: scheme.Passage, Weight: 3},
			{Name: scheme.Monochrome, Weight: 3},
			{Name: scheme.Analogous, Weight: 3},
			{Name: scheme.Notan, Weight: 2},
			{Name: scheme.Anchor, Weight: 2},
			{Name: scheme.Inherit, Weight: 2},
			{Name: scheme.BySize, Weight: 2},
			{Name: scheme.Temperature, Weight: 2},
			{Name: scheme.Complement, Weight: 2},
			{Name: scheme.ByDarkness, Weight: 1},
			{Name: scheme.Duet, Weight: 1},
			{Name: scheme.Triad, Weight: 1},
			{Name: scheme.Sequence, Weight: 1},
			{Name: scheme.Gradient, Weight: 1},
			{Name: scheme.Terrace, Weight: 1},
		},
	},
}

// levels is everything the traits resolve to: the pack, the stones' shape,
// the joint, the facets, the lamp and the water. Drawn once per render and
// then overridden by whatever the caller pinned on the command line.
type levels struct {
	// the bed
	count int     // stones, before the pack gives up
	rungs int     // steps on the size ladder
	base  float64 // smallest stone radius, canvas units
	ratio float64 // ladder step ratio
	gap   float64 // clearance between stones, ×radius
	over  float64 // overscan beyond the frame, canvas units
	darts int     // candidates thrown per stone

	// the stones' shape
	merge   float64 // share of sites that reach for a neighbour
	maxLobe int     // most sites one lobe may absorb
	warp    float64 // how far the bed is bent, ×smallest stone
	swirl   float64 // wavelength of that bending, ×smallest stone
	round   float64 // radius a stone's corners are worn over, canvas units

	// the joint
	ink   float64 // its thickness, canvas units
	swell float64 // extra thickness where three stones meet, ×ink
	node  float64 // distance over which a third stone counts as near

	fac facetLevels
	lit lightLevels
	wat waterLevels

	scheme string
}

// defaults are the levels shown in --help. What a render actually uses is
// drawn from the seed's traits.
func defaults(rng *rand.Rand) levels {
	l := newBed("shingle", rng)
	newStones("rolled", rng, &l)
	newJoint("drawn", rng, &l)
	newFacets("cut", rng, &l)
	newLight("morning", rng, &l)
	newWet("wet", rng, &l)
	l.scheme = scheme.Dominance
	l.settle()
	return l
}

// newBed draws the pack for one bed level.
//
// The ladder always reaches a stone several times the radius of the smallest
// one, at every level. A bed of uniformly sized stones is a honeycomb; what
// makes a river bed read as a river bed is that a stone you could sit on
// sits directly against grit.
func newBed(level string, rng *rand.Rand) levels {
	// Few darts on purpose — see pack.
	l := levels{darts: 5}
	switch level {
	case "boulders":
		l.count = 14 + rng.IntN(7)
		l.rungs, l.base = 3, rnd.Uniform(rng, 0.098, 0.125)
		l.ratio, l.gap = rnd.Uniform(rng, 1.45, 1.70), rnd.Uniform(rng, 0.05, 0.15)
	case "cobbles":
		l.count = 34 + rng.IntN(13)
		l.rungs, l.base = 4, rnd.Uniform(rng, 0.056, 0.070)
		l.ratio, l.gap = rnd.Uniform(rng, 1.40, 1.62), rnd.Uniform(rng, 0.03, 0.11)
	case "gravel":
		l.count = 150 + rng.IntN(51)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.023, 0.029)
		l.ratio, l.gap = rnd.Uniform(rng, 1.32, 1.50), rnd.Uniform(rng, 0.01, 0.06)
	case "grit":
		l.count = 280 + rng.IntN(81)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.015, 0.020)
		l.ratio, l.gap = rnd.Uniform(rng, 1.28, 1.44), rnd.Uniform(rng, 0.005, 0.04)
	default: // shingle
		l.count = 80 + rng.IntN(31)
		l.rungs, l.base = 5, rnd.Uniform(rng, 0.033, 0.042)
		l.ratio, l.gap = rnd.Uniform(rng, 1.36, 1.55), rnd.Uniform(rng, 0.02, 0.08)
	}
	// Overscan scales with the stones: deep enough that a border stone has a
	// neighbour outside the frame to be cut against, and no deeper, because
	// every site placed out there is one the bed does not get.
	l.over = l.base * rnd.Uniform(rng, 1.6, 2.2)
	return l
}

// newStones draws how worn the stones are: the corner rounding, the share of
// merged lobes, how hard the bed is bent, and how proud a stone stands.
//
// They are one decision, not four. A stone worn round enough to be a pebble
// and also a four-site lobe bent double is not a stone anyone has picked up;
// and a stone's plumpness follows its wear, because what rounds a corner in
// a river is the same tumbling that rounds the profile.
func newStones(level string, rng *rand.Rand, l *levels) {
	var round float64
	switch level {
	case "worn":
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.04, 0.12), 2
		l.warp, round = rnd.Uniform(rng, 0.35, 0.65), rnd.Uniform(rng, 0.24, 0.31)
		l.lit.rise = rnd.Uniform(rng, 0.62, 0.78)
	case "broken":
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.30, 0.45), 3
		l.warp, round = rnd.Uniform(rng, 0.90, 1.40), rnd.Uniform(rng, 0.09, 0.16)
		l.lit.rise = rnd.Uniform(rng, 0.40, 0.52)
	case "jumbled":
		// Lobes of four, bent hard, barely worn. A stone stops being a stone
		// and becomes a run that wanders across its neighbours.
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.70, 0.90), 4
		l.warp, round = rnd.Uniform(rng, 1.80, 2.60), rnd.Uniform(rng, 0.06, 0.12)
		l.lit.rise = rnd.Uniform(rng, 0.42, 0.66)
	default: // rolled
		l.merge, l.maxLobe = rnd.Uniform(rng, 0.13, 0.24), 2
		l.warp, round = rnd.Uniform(rng, 0.60, 1.00), rnd.Uniform(rng, 0.18, 0.26)
		l.lit.rise = rnd.Uniform(rng, 0.52, 0.68)
	}
	// The rounding is what turns a polygon into a pebble, and it has to stay
	// well under half a stone: past that the corners eat the walls between
	// them, the stones come apart into discs floating in water rather than
	// sharing a boundary, and a bed whose stones do not touch is a bag of
	// marbles.
	l.round = l.base * round
	l.swirl = rnd.Uniform(rng, 20, 32)
}

// newJoint draws the weight of the water between the stones.
//
// The width is a fraction of the smallest stone, not a distance on the page:
// a joint is heavy or fine *relative to what it separates*, and the same
// absolute width that reads as confident on a bed of boulders closes a bed
// of gravel into a dark net with chips in it.
//
// swell is the ratio between the junction and the mid-wall thickness. At 0
// the joints are a constant-width net, which reads as a drawn grid; water
// pools where three stones meet.
func newJoint(level string, rng *rand.Rand, l *levels) {
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
	// How far from a junction the swelling reaches — a fraction of a stone,
	// not a multiple of the joint. Sized off the ink instead, the reach on a
	// gravel bed is longer than the walls are, every point counts as a
	// junction, and the swelling becomes a uniform dark halo.
	l.node = l.base * rnd.Uniform(rng, 0.25, 0.45)
}

// levelsFor resolves the whole bed for one render: what the seed's traits
// drew, with any pinned flag laid over it. This is the point of the traits —
// a caller can move one number without restating the other thirty, and a
// caller who says nothing still gets a coherent bed.
func (s *Sketch) levelsFor(tr trait.Set, rng *rand.Rand) levels {
	l := newBed(tr.Get(dimBed), rng)
	// The pack overrides land *before* the joint is drawn, because the joint's
	// width is a fraction of the smallest stone: pinning --base has to move
	// the joint with it, or a hand-set stone size comes out under a joint
	// scaled for the one the seed drew.
	s.override([]knob{
		{"count", func() { l.count = s.pin.count }},
		{"rungs", func() { l.rungs = s.pin.rungs }},
		{"base", func() { l.base = s.pin.base }},
		{"ratio", func() { l.ratio = s.pin.ratio }},
		{"gap", func() { l.gap = s.pin.gap }},
		{"over", func() { l.over = s.pin.over }},
	})

	newStones(tr.Get(dimStones), rng, &l)
	newJoint(tr.Get(dimJoint), rng, &l)
	newFacets(tr.Get(dimFacets), rng, &l)
	newLight(tr.Get(dimLight), rng, &l)
	newWet(tr.Get(dimWet), rng, &l)
	l.scheme = tr.Get(dimScheme)

	s.override([]knob{
		{"merge", func() { l.merge = s.pin.merge }},
		{"max-lobe", func() { l.maxLobe = s.pin.maxLobe }},
		{"warp", func() { l.warp = s.pin.warp }},
		{"swirl", func() { l.swirl = s.pin.swirl }},
		{"round", func() { l.round = s.pin.round }},
		{"ink", func() { l.ink = s.pin.ink }},
		{"swell", func() { l.swell = s.pin.swell }},
		{"node", func() { l.node = s.pin.node }},

		{"facet", func() { l.fac.size = s.pin.fac.size }},
		{"facet-scale", func() { l.fac.scale = s.pin.fac.scale }},
		{"faceted", func() { l.fac.share = s.pin.fac.share }},
		{"cut", func() { l.fac.cut = s.pin.fac.cut }},
		{"flake", func() { l.fac.flake = s.pin.fac.flake }},
		{"crease", func() { l.fac.crease = s.pin.fac.crease }},

		{"rise", func() { l.lit.rise = s.Rise }},
		{"bearing", func() { l.lit.bearing = s.Light * math.Pi / 180 }},
		{"elevation", func() { l.lit.elev = s.Elevation }},
		{"ambient", func() { l.lit.amb = s.Ambient }},
		{"gloss", func() { l.lit.gloss = s.Gloss }},
		{"sharp", func() { l.lit.sharp = s.Sharp }},
		{"warmth", func() { l.lit.warmth = s.Warmth }},
		{"coolness", func() { l.lit.cool = s.Coolness }},

		{"soak", func() { l.wat.soak = s.Soak }},
		{"sheen", func() { l.wat.sheen = s.Sheen }},
		{"depth", func() { l.wat.deep = s.Deep }},
	})

	l.settle()

	// The warp displaces every lookup, so the pack has to reach far enough
	// that a displaced point still lands among stones. Drawn with the bed
	// before the warp is known, the overscan does not know to allow for it.
	l.over = math.Max(l.over, l.base*l.warp*2.4)

	// Nothing here has to rescue --base. The pack overrides land *above*
	// newStones and newJoint, so the wear, the swelling reach and the joint
	// width are all derived from the base that was actually asked for. An
	// earlier version re-derived the first two here from fixed fractions,
	// which was not merely redundant: it threw away what `stones` drew, so
	// pinning a stone size silently un-rounded the stones and `--bed X --base
	// Y --stones worn` came out with the wear of a broken bed.
	return l
}

// settle finishes the light once every source of it has had its say: the
// water polishes the surface, the lamp is aimed, and the two numbers that
// are properties of the *sampling* rather than of the picture are sized off
// the bed.
func (l *levels) settle() {
	// One lever, applied where the light is resolved rather than in four
	// places: wet stone is glossier and its highlight is tighter, and those
	// are the same fact about the water.
	l.lit.gloss *= l.wat.sheen
	l.lit.sharp *= l.wat.tight
	l.lit.aim()

	// The step the surface is differenced over, in canvas units — not pixels,
	// or a print would light a different picture from its preview
	// (invariant 2). A fraction of the smallest stone: fine enough to resolve
	// the shoulder of a dome, coarse enough not to read the field's own hash.
	l.lit.step = math.Max(l.base*0.06, 2e-4)
	// The dome's shoulder is vertical at the wall and a difference taken
	// across a wall compares two different stones, so the raw gradient is
	// unbounded exactly at the boundary. Capped, a rim is steep and the
	// terminator stays a surface rather than going to a black edge.
	// Softened deliberately. At 3 the rim of every stone turns far enough from
	// the lamp that the Lambert clamps to nothing all the way round the shadow
	// side, and the stone loses its colour into the joint before the joint is
	// even drawn.
	l.lit.maxSlope = 2.2
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
