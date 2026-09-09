package flame

import (
	"math/rand/v2"

	fl "github.com/jaminalder/go-graphics/internal/flame"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

const (
	dimStructure = "structure"
	dimWeave     = "weave"
	dimReach     = "reach"
	dimAperture  = "aperture"
	dimTint      = "tint"
	dimGround    = "ground"
	dimMedium    = "medium"
	dimManner    = "manner"
)

// mediumWash develops the histogram as pigment on paper instead of an
// emissive log-density on a void. Weight 0: opt-in only, so no existing
// seed moves off ember.
const mediumWash = "wash"

// mannerStain is FlatWash over density — the first wash character. Later
// manners (rimmed, pooled, charged) append here if stain already reads painted.
const mannerStain = "stain"

var schema = trait.Schema{
	{
		Name: dimStructure, Key: "s", InName: true,
		Doc: "the body of the flame",
		Values: []trait.Value{
			{Name: "chaos", Weight: 5},
			{Name: "spindle", Weight: 2.5},
			{Name: "filament", Weight: 3},
			{Name: "bloom", Weight: 2.5},
			{Name: "spiral", Weight: 2.5},
			{Name: "fold", Weight: 2},
			{Name: "julia", Weight: 2},
		},
	},
	{
		Name: dimWeave, Key: "w",
		Doc: "how many maps speak",
		Values: []trait.Value{
			{Name: "sparse", Weight: 3},
			{Name: "chorus", Weight: 4},
			{Name: "dense", Weight: 2},
		},
	},
	{
		Name: dimReach, Key: "r",
		Doc: "how wild the nonlinearities are",
		Values: []trait.Value{
			{Name: "gentle", Weight: 2},
			{Name: "vivid", Weight: 4},
			{Name: "wild", Weight: 2},
		},
	},
	{
		Name: dimAperture, Key: "a",
		Doc: "how tightly the camera sits",
		Values: []trait.Value{
			{Name: "tight", Weight: 2},
			{Name: "frame", Weight: 4},
			{Name: "wide", Weight: 2},
		},
	},
	{
		Name: dimTint, Key: "t", InName: true,
		Doc: "how colour is assigned to maps",
		Values: []trait.Value{
			{Name: "split", Weight: 4},
			{Name: "walk", Weight: 3},
			{Name: "stain", Weight: 1.5},
		},
	},
	{
		Name: dimGround, Key: "b",
		Doc: "what the flame sits on",
		Values: []trait.Value{
			{Name: "void", Weight: 6},
			{Name: "dusk", Weight: 1.5},
			{Name: "paper", Weight: 0.8},
		},
	},
	{
		Name: dimCast, Key: "c", InName: true,
		Doc:    "which palette casts the flame",
		Values: casts,
	},
	// medium and manner are appended so earlier dimensions keep their
	// draws; wash at weight 0 means ember stays the seed default.
	{
		Name: dimMedium, Key: "md",
		Doc: "how the attractor is developed",
		Values: []trait.Value{
			{Name: "ember", Weight: 1},
			{Name: mediumWash, Weight: 0},
		},
	},
	{
		Name: dimManner, Key: "mn",
		Doc: "wash character when medium is wash",
		Values: []trait.Value{
			{Name: mannerStain, Weight: 1},
		},
	},
}

type structure uint8

const (
	structureChaos structure = iota
	structureSpindle
	structureFilament
	structureBloom
	structureSpiral
	structureFold
	structureJulia
)

type tint uint8

const (
	tintSplit tint = iota
	tintWalk
	tintStain
)

type ground uint8

const (
	groundVoid ground = iota
	groundDusk
	groundPaper
)

type medium uint8

const (
	mediumEmber medium = iota
	mediumWashTone
)

type manner uint8

const (
	mannerStainLevel manner = iota
)

type settings struct {
	structure         structure
	weave             int
	reach             float64
	aperture          float64
	tint              tint
	ground            ground
	medium            medium
	manner            manner
	gamma, vibrancy   float64
	brightness, gleam float64
	quality           float64
	scale             float64
	estimator         float64
	deMin, deCurve    float64
	oversample        int
	filter            float64
	washSat           float64
	washBody          float64 // <0 → derive from washSat in stainWash
	washPow           float64 // ≤0 → default 0.55 in developWash
}

func draw(set trait.Set, rng *rand.Rand) settings {
	s := settings{
		gamma:      rnd.Uniform(rng, 2.6, 3.4),
		vibrancy:   rnd.Uniform(rng, 0.8, 0.94),
		brightness: rnd.Uniform(rng, 0.85, 1.25),
		gleam:      rnd.Uniform(rng, 0.28, 0.48),
		quality:    40,
		scale:      1,
		estimator:  9,
		deMin:      0,
		deCurve:    0.4,
		oversample: 2,
		filter:     fl.DefaultFilter,
		washSat:    2.5,
		washBody:   -1,
		washPow:    0,
	}

	switch set.Get(dimStructure) {
	case "chaos":
		s.structure = structureChaos
	case "filament":
		s.structure = structureFilament
	case "bloom":
		s.structure = structureBloom
	case "spiral":
		s.structure = structureSpiral
	case "fold":
		s.structure = structureFold
	case "julia":
		s.structure = structureJulia
	default:
		s.structure = structureSpindle
	}

	switch set.Get(dimWeave) {
	case "sparse":
		s.weave = 3
	case "dense":
		s.weave = 5
	default:
		s.weave = 4
	}

	switch set.Get(dimReach) {
	case "gentle":
		s.reach = rnd.Uniform(rng, 0.35, 0.55)
	case "wild":
		s.reach = rnd.Uniform(rng, 0.85, 1.15)
	default:
		s.reach = rnd.Uniform(rng, 0.6, 0.85)
	}

	switch set.Get(dimAperture) {
	case "tight":
		s.aperture = rnd.Uniform(rng, 0.72, 0.88)
	case "wide":
		s.aperture = rnd.Uniform(rng, 1.15, 1.45)
	default:
		s.aperture = rnd.Uniform(rng, 0.95, 1.08)
	}

	switch set.Get(dimTint) {
	case "walk":
		s.tint = tintWalk
	case "stain":
		s.tint = tintStain
	default:
		s.tint = tintSplit
	}

	switch set.Get(dimGround) {
	case "dusk":
		s.ground = groundDusk
	case "paper":
		s.ground = groundPaper
	default:
		s.ground = groundVoid
	}

	if set.Get(dimMedium) == mediumWash {
		s.medium = mediumWashTone
		s.ground = groundPaper
		s.gleam = 0
	}
	// Only stain exists today; draw still names it so --manner stays wired.
	s.manner = mannerStainLevel
	return s
}
