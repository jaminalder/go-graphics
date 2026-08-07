package iris

import (
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The output space. Each dimension is one thing a viewer would name if they
// were describing the piece out loud — what the stroma is made of, which way
// it runs, how finely it is divided — and each resolves to ranges rather than
// to numbers, so two seeds sharing a level still draw different irises.
const (
	dimStructure = "structure"
	dimWeave     = "weave"
	dimGrain     = "grain"
	dimReach     = "reach"
	dimAperture  = "aperture"
	dimTint      = "tint"
	dimGround    = "ground"
)

var schema = trait.Schema{
	{
		Name: dimStructure, Key: "s", InName: true,
		Doc: "what the warped field is read as",
		Values: []trait.Value{
			{Name: "fiber", Weight: 4},
			{Name: "filament", Weight: 2.5},
			{Name: "strata", Weight: 1.5},
			{Name: "crypt", Weight: 2},
		},
	},
	{
		Name: dimWeave, Key: "w", InName: true,
		Doc: "which way the stroma runs out of the pupil",
		// Weighted hard toward radial: a wound stroma is a strong effect and
		// a rare one, and a sheet of them stops reading as an iris at all.
		Values: []trait.Value{
			{Name: "radial", Weight: 7},
			{Name: "swirl", Weight: 2},
			{Name: "vortex", Weight: 0.8},
		},
	},
	{
		Name: dimGrain, Key: "g",
		Doc: "how finely the stroma is divided",
		Values: []trait.Value{
			{Name: "broad", Weight: 2},
			{Name: "fine", Weight: 3},
			{Name: "dense", Weight: 2},
		},
	},
	{
		Name: dimReach, Key: "r",
		Doc: "how far one thread holds together from pupil to limbus",
		Values: []trait.Value{
			{Name: "long", Weight: 3},
			{Name: "broken", Weight: 2},
		},
	},
	{
		Name: dimAperture, Key: "a",
		Doc: "how open the pupil is",
		Values: []trait.Value{
			{Name: "narrow", Weight: 2},
			{Name: "even", Weight: 3},
			{Name: "wide", Weight: 1.5},
		},
	},
	{
		Name: dimTint, Key: "t",
		Doc: "how hue is spread over the stroma",
		Values: []trait.Value{
			{Name: "plain", Weight: 4},
			{Name: "sector", Weight: 2},
		},
	},
	{
		Name: dimGround, Key: "b",
		Doc: "what surrounds the iris",
		Values: []trait.Value{
			{Name: "light", Weight: 4},
			{Name: "dark", Weight: 1.5},
			{Name: "shade", Weight: 1},
		},
	},
}

// draw resolves one point of the output space into the numbers the field is
// evaluated with. Levels give ranges; the generator picks inside them.
func draw(set trait.Set, rng *rand.Rand) settings {
	s := settings{
		gain:       rnd.Uniform(rng, 0.46, 0.55),
		lacunarity: rnd.Uniform(rng, 1.94, 2.2),
		fiber:      rnd.Uniform(rng, 0.4, 0.78),
		nested:     rnd.Uniform(rng, 0.3, 0.8),
		depth:      rnd.Uniform(rng, 0.36, 0.58),
		gleam:      rnd.Uniform(rng, 0.1, 0.24),
		warmth:     rnd.Uniform(rng, 0.18, 0.52),
		bands:      rnd.Uniform(rng, 9, 20),
		cells:      rnd.Uniform(rng, 10, 26),
		limbus:     rnd.Uniform(rng, 0.4, 0.445),
		octaves:    5,
	}

	switch set.Get(dimStructure) {
	case "filament":
		s.structure = structureFilament
	case "strata":
		s.structure = structureStrata
	case "crypt":
		s.structure = structureCrypt
	default:
		s.structure = structureFiber
	}

	// A twist is worth having in either hand.
	turn := 1.0
	if rnd.Odds(rng, 0.5) {
		turn = -1
	}
	switch set.Get(dimWeave) {
	case "swirl":
		s.twist = turn * rnd.Uniform(rng, 0.45, 1.05)
	case "vortex":
		s.twist = turn * rnd.Uniform(rng, 1.9, 3.6)
	default:
		s.twist = turn * rnd.Uniform(rng, 0, 0.18)
	}

	switch set.Get(dimGrain) {
	case "broad":
		s.scale = rnd.Uniform(rng, 1.1, 1.6)
	case "dense":
		s.scale = rnd.Uniform(rng, 2.4, 3.1)
		s.octaves = 6
	default:
		s.scale = rnd.Uniform(rng, 1.6, 2.3)
	}

	switch set.Get(dimReach) {
	case "broken":
		s.stretch = rnd.Uniform(rng, 0.95, 1.9)
		s.radial = rnd.Uniform(rng, 0.34, 0.7)
	default:
		s.stretch = rnd.Uniform(rng, 0.22, 0.6)
		s.radial = rnd.Uniform(rng, 0.1, 0.26)
	}

	switch set.Get(dimAperture) {
	case "narrow":
		s.pupil = rnd.Uniform(rng, 0.15, 0.22)
	case "wide":
		s.pupil = rnd.Uniform(rng, 0.36, 0.47)
	default:
		s.pupil = rnd.Uniform(rng, 0.24, 0.33)
	}

	if set.Is(dimTint, "sector") {
		s.tint = tintSector
	}
	switch set.Get(dimGround) {
	case "dark":
		s.ground = groundDark
	case "shade":
		s.ground = groundPalette
	default:
		s.ground = groundLight
	}
	return s
}
