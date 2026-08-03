package riffle

import (
	"fmt"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The output space. Six dimensions, and the reason each is one dimension
// rather than several flags is the same every time: the numbers under it
// only mean anything together. "riffle" is a depth, a speed, a wavelength, a
// turbulence, a chop and a foam threshold moving as one; set separately they
// give water that is deep and slow and covered in white, which is not a
// river, it is six knobs.
//
// Every level resolves to *ranges*, so two seeds at one level are two
// different riffles rather than the same one twice.

const (
	dimColourway = "colourway"
	dimReach     = "reach"
	dimChannel   = "channel"
	dimBoulders  = "boulders"
	dimWater     = "water"
	dimLight     = "light"
)

// fromFlag hands colour duty back to whatever --palette names. Weight 0: a
// seed never lands on it, so using this sketch with a palette from outside
// the curated list is always deliberate.
const fromFlag = "from-flag"

// colourways are palettes that hold up as river: a light warm pair that can
// be gravel, something cool and dark enough to be a metre of water, and a
// near-white for foam. Most of ColorLisa cannot do all three.
var colourways = []trait.Value{
	{Name: "hokusai-great-wave", Weight: 3},
	{Name: "monet-parasol", Weight: 3},
	{Name: "cezanne-bathers", Weight: 3},
	{Name: "bruegel-icarus", Weight: 2},
	{Name: "seurat-grande-jatte", Weight: 2},
	{Name: "manet-boating", Weight: 2},
	{Name: "durer-turf", Weight: 2},
	{Name: "diebenkorn-seawall", Weight: 2},
	{Name: "turner-val-daosta", Weight: 2},
	{Name: "sargent-villa-marlia", Weight: 1},
	{Name: "delaunay-air-iron-water", Weight: 1},
	{Name: "kandinsky-apple-tree", Weight: 1},
	{Name: "homer-clams", Weight: 1},
	{Name: "quidor-leatherstocking", Weight: 1},
	{Name: "hopper-night-windows", Weight: 1},
	{Name: "masaccio-tax-collector", Weight: 1},
	{Name: "vangogh-starry-night", Weight: 1},
	{Name: fromFlag, Weight: 0},
}

// schema is the sketch's output space.
var schema = trait.Schema{
	{
		Name: dimColourway, Key: "c", InName: true,
		Doc:    "which palette the water and its bed are drawn from",
		Values: colourways,
	},
	{
		Name: dimReach, Key: "r", InName: true,
		Doc: "the energy of the stretch of river",
		Values: []trait.Value{
			{Name: "pool", Weight: 2},
			{Name: "glide", Weight: 3},
			{Name: "run", Weight: 3},
			{Name: "riffle", Weight: 3},
			{Name: "rapid", Weight: 2},
			{Name: "cascade", Weight: 0},
		},
	},
	{
		Name: dimChannel, Key: "n", InName: true,
		Doc: "the plan form: where the water is and where it is not",
		Values: []trait.Value{
			{Name: "straight", Weight: 3},
			{Name: "bend", Weight: 4},
			{Name: "chute", Weight: 2},
			{Name: "bar", Weight: 2},
			{Name: "braid", Weight: 1},
		},
	},
	{
		Name: dimBoulders, Key: "b", InName: true,
		Doc: "how much rock breaks the surface",
		Values: []trait.Value{
			{Name: "clear", Weight: 1},
			{Name: "few", Weight: 3},
			{Name: "scattered", Weight: 3},
			{Name: "field", Weight: 2},
			{Name: "ledge", Weight: 1},
		},
	},
	{
		Name: dimWater, Key: "w", InName: true,
		Doc: "how far light gets into the column, and what colour is left",
		Values: []trait.Value{
			{Name: "clear", Weight: 3},
			{Name: "green", Weight: 3},
			{Name: "peat", Weight: 2},
			{Name: "glacial", Weight: 1},
			{Name: "silt", Weight: 1},
		},
	},
	{
		Name: dimLight, Key: "l", InName: true,
		Doc: "the sun on the water",
		Values: []trait.Value{
			{Name: "high", Weight: 4},
			{Name: "low", Weight: 2},
			{Name: "overcast", Weight: 2},
			{Name: "dappled", Weight: 2},
		},
	},
}

// settings is every number one render works from: what the six dimensions
// drew, with the caller's overrides laid on top afterwards.
type settings struct {
	// the bed
	depth      float64 // water on the thalweg in a pool, in extinction units
	riffle     float64 // amplitude of the pool–riffle sequence, 0..1
	riffleWave float64 // pool–riffle sequences down the height of the frame
	dune       float64 // irregularity of the bed

	// the channel
	channelWidth float64 // half width at mid-frame, canvas units
	bend         float64 // lateral swing of the centreline, canvas units
	meander      float64 // swings down the height of the frame
	taper        float64 // narrowing (negative) or opening (positive)
	skew         float64 // how far the deep line moves to the outside of a bend

	// the current
	speed      float64 // mid-channel speed, canvas units per step-time
	turbulence float64 // curl noise on top of it, × speed
	chop       float64 // surface wave height, canvas units

	// what is in the way
	rocks    int
	rockSize float64 // typical boulder radius, canvas units
	wake     float64 // white water a rock sheds
	eddy     float64 // circulation of the vortex pair behind it
	ledge    bool    // a line of rock across the channel
	bars     int     // mid-channel gravel bars

	// the upstream walk
	steps int
	step  float64 // seconds per step: a step is speed × this

	// foam
	foam     float64 // how easily water goes white, 0..1
	foamLife float64 // steps a bubble survives
	bubbles  float64 // bubble lattice scale, canvas units

	// the water column
	extinction float64 // how fast light dies with depth
	milk       float64 // how much the body colour is lifted toward light
	bodyWarm   bool    // the water's own colour comes off the warm end
	bodyDark   float64 // and is taken to this fraction of its own lightness

	// light
	sun          float64 // azimuth, degrees
	sunHeight    float64 // altitude, degrees
	glint        float64 // angular width of a sun glint, radians
	sheen        float64 // broad reflected sky
	caustic      float64 // strength of the net on the bed
	causticScale float64 // its cell size in the shallows, canvas units
	causticWarp  float64 // how hard the net is folded
	dapple       float64 // patchy shade over the sun

	pebble float64 // gravel scale, canvas units
}

// defaults are the numbers that do not belong to a trait — the ones that are
// facts about the medium rather than about this stretch of river.
func defaults() settings {
	return settings{
		dune:         0.30,
		skew:         0.30,
		steps:        20,
		step:         0.006,
		foamLife:     7,
		bubbles:      0.0140,
		glint:        0.11,
		sheen:        0.08,
		causticScale: 0.030,
		causticWarp:  0.55,
		pebble:       0.0060,
		// Shown in --help only; a knob left alone comes from the seed.
		depth: 1.1, riffle: 0.5, riffleWave: 1.4,
		channelWidth: 0.72, bend: 0.10, meander: 0.9,
		speed: 0.85, turbulence: 0.3, chop: 0.0016,
		rocks: 7, rockSize: 0.035, wake: 0.7, eddy: 0.5,
		foam: 0.5, extinction: 2.2, milk: 0, bodyDark: 0.5,
		sun: 130, sunHeight: 60, caustic: 1, dapple: 0,
	}
}

// reachLevel is the energy of the stretch. Depth and speed move opposite
// ways down the list, and the foam threshold follows the Froude number they
// imply rather than being set against it.
func reachLevel(level string, s *settings, rng *rand.Rand) {
	switch level {
	case "pool":
		s.depth = rnd.Uniform(rng, 1.7, 2.3)
		s.riffle = rnd.Uniform(rng, 0.16, 0.30)
		s.riffleWave = rnd.Uniform(rng, 0.6, 1.0)
		s.speed = rnd.Uniform(rng, 0.42, 0.58)
		s.turbulence = rnd.Uniform(rng, 0.06, 0.12)
		s.chop = rnd.Uniform(rng, 0.0018, 0.0033)
		s.foam = rnd.Uniform(rng, 0.20, 0.35)
	case "glide":
		s.depth = rnd.Uniform(rng, 1.15, 1.6)
		s.riffle = rnd.Uniform(rng, 0.26, 0.42)
		s.riffleWave = rnd.Uniform(rng, 0.8, 1.3)
		s.speed = rnd.Uniform(rng, 0.72, 0.92)
		s.turbulence = rnd.Uniform(rng, 0.08, 0.14)
		s.chop = rnd.Uniform(rng, 0.0027, 0.0045)
		s.foam = rnd.Uniform(rng, 0.30, 0.44)
	case "run":
		s.depth = rnd.Uniform(rng, 0.80, 1.15)
		s.riffle = rnd.Uniform(rng, 0.38, 0.55)
		s.riffleWave = rnd.Uniform(rng, 1.1, 1.7)
		s.speed = rnd.Uniform(rng, 0.86, 1.06)
		s.turbulence = rnd.Uniform(rng, 0.13, 0.20)
		s.chop = rnd.Uniform(rng, 0.0039, 0.0060)
		s.foam = rnd.Uniform(rng, 0.42, 0.56)
	case "riffle":
		s.depth = rnd.Uniform(rng, 0.52, 0.78)
		s.riffle = rnd.Uniform(rng, 0.50, 0.68)
		s.riffleWave = rnd.Uniform(rng, 1.5, 2.4)
		s.speed = rnd.Uniform(rng, 1.00, 1.22)
		s.turbulence = rnd.Uniform(rng, 0.19, 0.28)
		s.chop = rnd.Uniform(rng, 0.0057, 0.0084)
		s.foam = rnd.Uniform(rng, 0.55, 0.70)
	case "rapid":
		s.depth = rnd.Uniform(rng, 0.38, 0.58)
		s.riffle = rnd.Uniform(rng, 0.55, 0.75)
		s.riffleWave = rnd.Uniform(rng, 1.8, 2.8)
		s.speed = rnd.Uniform(rng, 1.18, 1.45)
		s.turbulence = rnd.Uniform(rng, 0.26, 0.37)
		s.chop = rnd.Uniform(rng, 0.0075, 0.0108)
		s.foam = rnd.Uniform(rng, 0.68, 0.82)
	default: // cascade — past what this vocabulary is for, weight 0
		s.depth = rnd.Uniform(rng, 0.24, 0.38)
		s.riffle = rnd.Uniform(rng, 0.62, 0.82)
		s.riffleWave = rnd.Uniform(rng, 2.4, 3.6)
		s.speed = rnd.Uniform(rng, 1.5, 1.9)
		s.turbulence = rnd.Uniform(rng, 0.36, 0.51)
		s.chop = rnd.Uniform(rng, 0.0105, 0.0150)
		s.foam = rnd.Uniform(rng, 0.86, 0.96)
	}
}

// channelLevel is the plan form. Width is a half width, so anything past
// ~0.75 on a square frame keeps both banks off the sheet and the picture is
// all water; below that the frame contains a bank.
func channelLevel(level string, s *settings, rng *rand.Rand) {
	switch level {
	case "straight":
		s.channelWidth = rnd.Uniform(rng, 0.80, 1.05)
		s.bend = rnd.Uniform(rng, 0.02, 0.06)
		s.meander = rnd.Uniform(rng, 0.5, 0.9)
		s.taper = rnd.Uniform(rng, -0.10, 0.10)
	case "bend":
		s.channelWidth = rnd.Uniform(rng, 0.62, 0.82)
		s.bend = rnd.Uniform(rng, 0.16, 0.30)
		s.meander = rnd.Uniform(rng, 0.55, 0.95)
		s.taper = rnd.Uniform(rng, -0.12, 0.12)
	case "chute":
		s.channelWidth = rnd.Uniform(rng, 0.72, 0.95)
		s.bend = rnd.Uniform(rng, 0.05, 0.14)
		s.meander = rnd.Uniform(rng, 0.6, 1.1)
		s.taper = rnd.Uniform(rng, -0.55, -0.34)
	case "bar":
		s.channelWidth = rnd.Uniform(rng, 0.70, 0.92)
		s.bend = rnd.Uniform(rng, 0.08, 0.20)
		s.meander = rnd.Uniform(rng, 0.5, 0.9)
		s.taper = rnd.Uniform(rng, -0.15, 0.15)
		s.bars = 1
	default: // braid
		s.channelWidth = rnd.Uniform(rng, 0.95, 1.25)
		s.bend = rnd.Uniform(rng, 0.06, 0.16)
		s.meander = rnd.Uniform(rng, 0.6, 1.0)
		s.taper = rnd.Uniform(rng, -0.15, 0.15)
		s.bars = 2
	}
}

// bouldersLevel is count and size together, because a field of small rocks
// and a couple of large ones are different rivers, not the same river with a
// different number in it.
func bouldersLevel(level string, s *settings, rng *rand.Rand) {
	s.wake = rnd.Uniform(rng, 0.35, 0.60)
	s.eddy = rnd.Uniform(rng, 0.10, 0.22)
	switch level {
	case "clear":
		s.rocks = 0
		s.rockSize = 0.03
	case "few":
		s.rocks = 2 + rng.IntN(3)
		s.rockSize = rnd.Uniform(rng, 0.070, 0.105)
	case "scattered":
		s.rocks = 6 + rng.IntN(5)
		s.rockSize = rnd.Uniform(rng, 0.045, 0.070)
	case "field":
		s.rocks = 14 + rng.IntN(9)
		s.rockSize = rnd.Uniform(rng, 0.030, 0.046)
	default: // ledge
		s.rocks = 3 + rng.IntN(4)
		s.rockSize = rnd.Uniform(rng, 0.040, 0.058)
		s.ledge = true
		s.eddy = rnd.Uniform(rng, 0.16, 0.28)
	}
}

// waterLevel is turbidity: how fast the column swallows the bed, and what
// colour is left when it has.
func waterLevel(level string, s *settings, rng *rand.Rand) {
	switch level {
	case "clear":
		s.extinction = rnd.Uniform(rng, 1.65, 2.15)
		s.milk, s.bodyDark = 0, rnd.Uniform(rng, 0.36, 0.48)
	case "green":
		s.extinction = rnd.Uniform(rng, 2.3, 2.9)
		s.milk, s.bodyDark = rnd.Uniform(rng, 0.05, 0.15), rnd.Uniform(rng, 0.46, 0.60)
	case "peat":
		s.extinction = rnd.Uniform(rng, 3.4, 4.2)
		s.milk, s.bodyDark, s.bodyWarm = 0, rnd.Uniform(rng, 0.34, 0.48), true
	case "glacial":
		s.extinction = rnd.Uniform(rng, 3.6, 4.6)
		s.milk, s.bodyDark = rnd.Uniform(rng, 0.28, 0.42), rnd.Uniform(rng, 0.68, 0.82)
	default: // silt
		s.extinction = rnd.Uniform(rng, 3.0, 3.8)
		s.milk, s.bodyDark, s.bodyWarm = rnd.Uniform(rng, 0.22, 0.34), rnd.Uniform(rng, 0.60, 0.78), true
	}
}

// lightLevel decides whether there are caustics at all. Overcast has none,
// and the sheet then has to be carried by tone and foam — the honest test of
// whether the depth field is doing its job.
func lightLevel(level string, s *settings, rng *rand.Rand) {
	s.sun = rnd.Uniform(rng, 0, 360)
	switch level {
	case "high":
		s.sunHeight = rnd.Uniform(rng, 52, 74)
		s.caustic = rnd.Uniform(rng, 0.50, 0.75)
		s.glint = rnd.Uniform(rng, 0.085, 0.130)
		s.sheen = rnd.Uniform(rng, 0.05, 0.09)
	case "low":
		s.sunHeight = rnd.Uniform(rng, 14, 26)
		s.caustic = rnd.Uniform(rng, 0.22, 0.38)
		s.glint = rnd.Uniform(rng, 0.130, 0.190)
		s.sheen = rnd.Uniform(rng, 0.09, 0.15)
	case "overcast":
		s.sunHeight = rnd.Uniform(rng, 78, 88)
		s.caustic = 0
		s.glint = rnd.Uniform(rng, 0.24, 0.34)
		s.sheen = rnd.Uniform(rng, 0.15, 0.24)
	default: // dappled
		s.sunHeight = rnd.Uniform(rng, 44, 66)
		s.caustic = rnd.Uniform(rng, 0.45, 0.70)
		s.glint = rnd.Uniform(rng, 0.090, 0.140)
		s.sheen = rnd.Uniform(rng, 0.05, 0.09)
		s.dapple = rnd.Uniform(rng, 0.65, 0.95)
	}
}

// colours resolves the colourway trait to the palette a render works from.
func colours(name string, fromCLI palette.Palette) (palette.Palette, error) {
	if name == fromFlag {
		return fromCLI, nil
	}
	p, ok := palette.ByName(name)
	if !ok {
		return palette.Palette{}, fmt.Errorf("riffle: no palette %q", name)
	}
	return p, nil
}
