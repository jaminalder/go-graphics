package flame

import (
	"math"
	"math/rand/v2"

	fl "github.com/jaminalder/go-graphics/internal/flame"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// Named structures other than spindle/chaos: each still has a recognisable
// character, but the primary variation and symmetry are drawn from a pool
// so seeds within one structure are not near-copies of one recipe.

func filamentGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	primaries := []uint8{fl.Spherical, fl.Swirl, fl.Handkerchief, fl.Hyperbolic, fl.Fisheye}
	primary := primaries[rng.IntN(len(primaries))]
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: primary, Weight: 1}},
				rnd.Uniform(rng, 0.4, 1.05)*r, rnd.Uniform(rng, 0.4, 1.05)*r,
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.5, 0.5), rnd.Uniform(rng, -0.5, 0.5),
				0, rnd.Uniform(rng, 0.7, 1.3)),
			xf([]fl.Var{{Kind: fl.Swirl, Weight: rnd.Uniform(rng, 0.4, 0.85)}, {Kind: fl.Sinusoidal, Weight: rnd.Uniform(rng, 0.15, 0.45) * r}},
				rnd.Uniform(rng, 0.3, 0.9)*r, rnd.Uniform(rng, 0.3, 0.9)*r,
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.55, 0.55), rnd.Uniform(rng, -0.55, 0.55),
				1, rnd.Uniform(rng, 0.5, 1.3)),
			xf([]fl.Var{{Kind: fl.Horseshoe, Weight: rnd.Uniform(rng, 0.3, 0.7)}, {Kind: fl.Linear, Weight: rnd.Uniform(rng, 0.3, 0.7)}},
				rnd.Uniform(rng, 0.35, 1.0), rnd.Uniform(rng, 0.35, 1.0),
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.4, 0.4),
				rng.Float64(), rnd.Uniform(rng, 0.4, 1.1)),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Eyefish, Weight: 0.4 * r}, {Kind: fl.Spherical, Weight: 0.6}},
			rnd.Uniform(rng, 0.35, 0.95), rnd.Uniform(rng, 0.35, 0.95),
			rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.4, 0.4),
			rng.Float64(), rnd.Uniform(rng, 0.35, 0.95),
		))
	}
	if set.weave >= 5 {
		extra := []uint8{fl.Disc, fl.Polar, fl.Power, fl.Curl}
		k := extra[rng.IntN(len(extra))]
		v := fl.Var{Kind: k, Weight: 1}
		if k == fl.Curl {
			v.P = [4]float64{rnd.Uniform(rng, -0.35, 0.35), rnd.Uniform(rng, -0.2, 0.2)}
		}
		sys.X = append(sys.X, xf(
			[]fl.Var{v},
			rnd.Uniform(rng, 0.3, 0.85), rnd.Uniform(rng, 0.3, 0.85),
			rnd.Uniform(rng, -0.8, 0.8), rnd.Uniform(rng, -0.25, 0.25), rnd.Uniform(rng, -0.25, 0.25),
			rng.Float64(), rnd.Uniform(rng, 0.2, 0.7),
		))
	}
	if rnd.Odds(rng, 0.35) {
		fin := xf([]fl.Var{{Kind: fl.Spherical, Weight: rnd.Uniform(rng, 0.4, 1)}, {Kind: fl.Linear, Weight: rnd.Uniform(rng, 0, 0.6)}},
			rnd.Uniform(rng, 0.6, 1.1), rnd.Uniform(rng, 0.6, 1.1), rnd.Uniform(rng, -0.4, 0.4), 0, 0, 0.5, 1)
		sys.Final = &fin
	}
	switch rng.IntN(5) {
	case 0:
		sys.AddDihedral()
	case 1:
		sys.AddRotation(2)
	case 2:
		sys.AddRotation(3)
	}
	return sys
}

func bloomGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	cores := []uint8{fl.Bubble, fl.Eyefish, fl.Fisheye, fl.Spherical}
	core := cores[rng.IntN(len(cores))]
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: core, Weight: 1}},
				rnd.Uniform(rng, 0.55, 1.2), rnd.Uniform(rng, 0.55, 1.2),
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.25, 0.25), rnd.Uniform(rng, -0.25, 0.25),
				0, rnd.Uniform(rng, 0.7, 1.3)),
			xf([]fl.Var{{Kind: fl.Eyefish, Weight: rnd.Uniform(rng, 0.35, 0.75)}, {Kind: fl.Spherical, Weight: rnd.Uniform(rng, 0.25, 0.65)}},
				rnd.Uniform(rng, 0.4, 1.0)*r, rnd.Uniform(rng, 0.4, 1.0)*r,
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.35, 0.35), rnd.Uniform(rng, -0.35, 0.35),
				1, rnd.Uniform(rng, 0.6, 1.2)),
			xf([]fl.Var{{Kind: fl.Sinusoidal, Weight: rnd.Uniform(rng, 0.5, 1)}, {Kind: fl.Swirl, Weight: rnd.Uniform(rng, 0, 0.5) * r}},
				rnd.Uniform(rng, 0.4, 1.0), rnd.Uniform(rng, 0.4, 1.0),
				rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.4, 0.4),
				rng.Float64(), rnd.Uniform(rng, 0.45, 1)),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Cylinder, Weight: rnd.Uniform(rng, 0.3, 0.7)}, {Kind: fl.Linear, Weight: rnd.Uniform(rng, 0.3, 0.7)}},
			rnd.Uniform(rng, 0.4, 0.95), rnd.Uniform(rng, 0.5, 1.15),
			rnd.Uniform(rng, -0.6, 0.6), rnd.Uniform(rng, -0.2, 0.2), 0,
			rng.Float64(), rnd.Uniform(rng, 0.35, 0.85),
		))
	}
	if set.weave >= 5 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Bubble, Weight: 0.5}, {Kind: fl.JulianN, Weight: 0.5, P: [4]float64{float64(2 + rng.IntN(4)), rnd.Uniform(rng, 0.5, 1)}}},
			rnd.Uniform(rng, 0.35, 0.8), rnd.Uniform(rng, 0.35, 0.8),
			rnd.Uniform(rng, -0.5, 0.5), 0, 0, rng.Float64(), rnd.Uniform(rng, 0.2, 0.55),
		))
	}
	switch rng.IntN(6) {
	case 0, 1:
		sys.AddRotation(2 + rng.IntN(3))
	case 2:
		sys.AddRotation(3)
		sys.AddDihedral()
	case 3:
		sys.AddDihedral()
	}
	return sys
}

func spiralGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	arms := []uint8{fl.Spiral, fl.Hyperbolic, fl.Handkerchief, fl.Polar, fl.Diamond}
	arm := arms[rng.IntN(len(arms))]
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: arm, Weight: 1}},
				rnd.Uniform(rng, 0.4, 1.05)*r, rnd.Uniform(rng, 0.4, 1.05)*r,
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.2, 0.2), rnd.Uniform(rng, -0.2, 0.2),
				0, rnd.Uniform(rng, 0.7, 1.3)),
			xf([]fl.Var{{Kind: fl.Polar, Weight: rnd.Uniform(rng, 0.35, 0.75)}, {Kind: fl.Heart, Weight: rnd.Uniform(rng, 0.25, 0.65)}},
				rnd.Uniform(rng, 0.4, 1.0), rnd.Uniform(rng, 0.4, 1.0),
				rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.3, 0.3), rnd.Uniform(rng, -0.2, 0.2),
				1, rnd.Uniform(rng, 0.55, 1.15)),
			xf([]fl.Var{{Kind: fl.Spherical, Weight: rnd.Uniform(rng, 0.4, 1)}, {Kind: fl.Swirl, Weight: rnd.Uniform(rng, 0, 0.5) * r}},
				rnd.Uniform(rng, 0.45, 1.05), rnd.Uniform(rng, 0.45, 1.05),
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.35, 0.35), rnd.Uniform(rng, -0.35, 0.35),
				rng.Float64(), rnd.Uniform(rng, 0.45, 1)),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Hyperbolic, Weight: 0.5 * r}, {Kind: fl.Linear, Weight: 0.5}},
			rnd.Uniform(rng, 0.35, 0.9), rnd.Uniform(rng, 0.35, 0.9),
			rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.2, 0.2), 0,
			rng.Float64(), rnd.Uniform(rng, 0.3, 0.75),
		))
	}
	if set.weave >= 5 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Ex, Weight: 1}},
			rnd.Uniform(rng, 0.3, 0.75), rnd.Uniform(rng, 0.3, 0.75),
			rnd.Uniform(rng, -0.6, 0.6), 0, 0, rng.Float64(), rnd.Uniform(rng, 0.2, 0.5),
		))
	}
	if rnd.Odds(rng, 0.4) {
		sys.AddRotation(2 + rng.IntN(3))
	}
	return sys
}

func foldGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	folds := []uint8{fl.Horseshoe, fl.Bent, fl.Disc, fl.Ex, fl.Popcorn}
	fold := folds[rng.IntN(len(folds))]
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: fold, Weight: 1}},
				rnd.Uniform(rng, 0.4, 1.05), rnd.Uniform(rng, 0.4, 1.05),
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.35, 0.35), rnd.Uniform(rng, -0.35, 0.35),
				0, rnd.Uniform(rng, 0.7, 1.3)),
			xf([]fl.Var{{Kind: fl.Disc, Weight: rnd.Uniform(rng, 0.4, 0.85)}, {Kind: fl.Spherical, Weight: rnd.Uniform(rng, 0.15, 0.55)}},
				rnd.Uniform(rng, 0.35, 0.95)*r, rnd.Uniform(rng, 0.35, 0.95)*r,
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.25, 0.25), rnd.Uniform(rng, -0.25, 0.25),
				1, rnd.Uniform(rng, 0.55, 1.15)),
			xf([]fl.Var{{Kind: fl.Diamond, Weight: rnd.Uniform(rng, 0.3, 0.7)}, {Kind: fl.Linear, Weight: rnd.Uniform(rng, 0.3, 0.7)}},
				rnd.Uniform(rng, 0.4, 1.0), rnd.Uniform(rng, 0.4, 1.0),
				rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.3, 0.3), rnd.Uniform(rng, -0.3, 0.3),
				rng.Float64(), rnd.Uniform(rng, 0.4, 1)),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Ex, Weight: 1}},
			rnd.Uniform(rng, 0.3, 0.85), rnd.Uniform(rng, 0.3, 0.85),
			rnd.Uniform(rng, -0.8, 0.8), rnd.Uniform(rng, -0.15, 0.15), 0,
			rng.Float64(), rnd.Uniform(rng, 0.25, 0.7),
		))
	}
	if set.weave >= 5 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Waves, Weight: 0.6}, {Kind: fl.Linear, Weight: 0.4}},
			rnd.Uniform(rng, 0.4, 0.9), rnd.Uniform(rng, 0.4, 0.9),
			rnd.Uniform(rng, -0.5, 0.5), rnd.Uniform(rng, -0.3, 0.3), rnd.Uniform(rng, -0.3, 0.3),
			rng.Float64(), rnd.Uniform(rng, 0.25, 0.6),
		))
	}
	if rnd.Odds(rng, 0.35) {
		sys.AddDihedral()
	} else if rnd.Odds(rng, 0.25) {
		sys.AddRotation(2)
	}
	return sys
}

func juliaGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	power := float64(2 + rng.IntN(6))
	var nest fl.Xform
	if rnd.Odds(rng, 0.55) {
		nest = xf(
			[]fl.Var{{Kind: fl.JulianN, Weight: 1, P: [4]float64{power, rnd.Uniform(rng, 0.5, 1.1) * r}}},
			rnd.Uniform(rng, 0.45, 1.15)*r, rnd.Uniform(rng, 0.45, 1.15)*r,
			rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.3, 0.3), rnd.Uniform(rng, -0.3, 0.3),
			0, rnd.Uniform(rng, 0.8, 1.4),
		)
	} else {
		nest = xf(
			[]fl.Var{{Kind: fl.Julia, Weight: 1}},
			rnd.Uniform(rng, 0.55, 1.2)*r, rnd.Uniform(rng, 0.55, 1.2)*r,
			rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.3, 0.3), rnd.Uniform(rng, -0.3, 0.3),
			0, rnd.Uniform(rng, 0.8, 1.4),
		)
	}
	sys := fl.System{
		X: []fl.Xform{
			nest,
			xf([]fl.Var{{Kind: fl.Linear, Weight: 1}},
				rnd.Uniform(rng, 0.3, 0.9), rnd.Uniform(rng, 0.3, 0.9),
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.55, 0.55), rnd.Uniform(rng, -0.55, 0.55),
				1, rnd.Uniform(rng, 0.55, 1.2)),
			xf([]fl.Var{{Kind: fl.Spherical, Weight: rnd.Uniform(rng, 0.3, 0.7)}, {Kind: fl.Sinusoidal, Weight: rnd.Uniform(rng, 0.3, 0.7)}},
				rnd.Uniform(rng, 0.4, 1.0), rnd.Uniform(rng, 0.4, 1.0),
				rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.25, 0.25), rnd.Uniform(rng, -0.25, 0.25),
				rng.Float64(), rnd.Uniform(rng, 0.4, 1)),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Exponential, Weight: 0.4 * r}, {Kind: fl.Linear, Weight: 0.6}},
			rnd.Uniform(rng, 0.3, 0.85), rnd.Uniform(rng, 0.3, 0.85),
			rnd.Uniform(rng, -0.6, 0.6), rnd.Uniform(rng, -0.2, 0.2), 0,
			rng.Float64(), rnd.Uniform(rng, 0.25, 0.7),
		))
	}
	if set.weave >= 5 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.JulianN, Weight: 1, P: [4]float64{power + 1, rnd.Uniform(rng, 0.45, 0.9)}}},
			rnd.Uniform(rng, 0.3, 0.7), rnd.Uniform(rng, 0.3, 0.7),
			rnd.Uniform(rng, -0.8, 0.8), 0, 0, rng.Float64(), rnd.Uniform(rng, 0.15, 0.45),
		))
	}
	switch rng.IntN(5) {
	case 0:
		sys.AddRotation(2)
	case 1:
		sys.AddRotation(3)
	case 2:
		sys.AddDihedral()
	}
	return sys
}
