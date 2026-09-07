package flame

import (
	"math"
	"math/rand/v2"

	fl "github.com/jaminalder/go-graphics/internal/flame"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

func compose(set settings, rng *rand.Rand) fl.System {
	var sys fl.System
	switch set.structure {
	case structureFilament:
		sys = filamentGenome(set, rng)
	case structureBloom:
		sys = bloomGenome(set, rng)
	case structureSpiral:
		sys = spiralGenome(set, rng)
	case structureFold:
		sys = foldGenome(set, rng)
	case structureJulia:
		sys = juliaGenome(set, rng)
	default:
		sys = spindleGenome(set, rng)
	}
	colorize(&sys, set, rng)
	return sys
}

func xf(vars []fl.Var, sx, sy, rot, tx, ty, color, weight float64) fl.Xform {
	a, b, c, d, e, f := fl.Affine(sx, sy, rot, tx, ty)
	return fl.Xform{
		A: a, B: b, C: c, D: d, E: e, F: f,
		Vars:       vars,
		Color:      color,
		ColorSpeed: 0.5,
		Weight:     weight,
	}
}

func post(x *fl.Xform, sx, sy, rot, tx, ty float64) {
	x.PA, x.PB, x.PC, x.PD, x.PE, x.PF = fl.Affine(sx, sy, rot, tx, ty)
	x.HasPost = true
}

func spindleGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	n := set.weave
	power := 4.0 + float64(rng.IntN(4)) // 4–7: branching nest, not a filled disc
	// Julian nest = cyan crystalline crossing (colour pinned in colorize).
	nest := xf(
		[]fl.Var{{Kind: fl.JulianN, Weight: 1, P: [4]float64{power, rnd.Uniform(rng, 0.78, 1.12)}}},
		rnd.Uniform(rng, 0.28, 0.46)*r, rnd.Uniform(rng, 0.28, 0.46)*r,
		rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.04, 0.04), rnd.Uniform(rng, -0.04, 0.04),
		0, rnd.Uniform(rng, 0.75, 1.15),
	)
	// Contractive linear copies reprint the nest at smaller scales — the
	// filigree engine. Two copies are the minimum for recursive hairlines.
	copy1 := xf(
		[]fl.Var{{Kind: fl.Linear, Weight: 1}},
		rnd.Uniform(rng, 0.2, 0.34), rnd.Uniform(rng, 0.28, 0.48),
		rnd.Uniform(rng, -0.22, 0.22), rnd.Uniform(rng, -0.1, 0.1), rnd.Uniform(rng, -0.05, 0.05),
		1, rnd.Uniform(rng, 0.8, 1.15),
	)
	copy2 := xf(
		[]fl.Var{{Kind: fl.Linear, Weight: 0.7}, {Kind: fl.Swirl, Weight: 0.3 * r}},
		rnd.Uniform(rng, 0.18, 0.3), rnd.Uniform(rng, 0.26, 0.42),
		rnd.Uniform(rng, -0.9, 0.9), rnd.Uniform(rng, -0.08, 0.08), rnd.Uniform(rng, -0.06, 0.06),
		1, rnd.Uniform(rng, 0.55, 0.9),
	)
	sys := fl.System{X: []fl.Xform{nest, copy1, copy2}}

	// Faint spherical arcs off the equator — Zander's secondary loops.
	if n >= 4 {
		arc := xf(
			[]fl.Var{{Kind: fl.Spherical, Weight: 0.55}, {Kind: fl.Linear, Weight: 0.45}},
			rnd.Uniform(rng, 0.42, 0.7), rnd.Uniform(rng, 0.42, 0.7),
			rnd.Uniform(rng, -0.5, 0.5), rnd.Uniform(rng, -0.12, 0.12), rnd.Uniform(rng, -0.12, 0.12),
			1, rnd.Uniform(rng, 0.22, 0.4),
		)
		post(&arc, rnd.Uniform(rng, 0.88, 1.05), rnd.Uniform(rng, 0.88, 1.05), rnd.Uniform(rng, -0.15, 0.15), 0, 0)
		sys.X = append(sys.X, arc)
	}
	if n >= 5 {
		// Second nest branch at a different power, low weight: denser
		// crossing without filling the plane.
		nest2 := xf(
			[]fl.Var{{Kind: fl.JulianN, Weight: 1, P: [4]float64{power - 1, rnd.Uniform(rng, 0.7, 1)}}},
			rnd.Uniform(rng, 0.35, 0.55)*r, rnd.Uniform(rng, 0.35, 0.55)*r,
			rnd.Uniform(rng, -0.6, 0.6), rnd.Uniform(rng, -0.06, 0.06), rnd.Uniform(rng, -0.06, 0.06),
			0, rnd.Uniform(rng, 0.2, 0.38),
		)
		sys.X = append(sys.X, nest2)
		needle := xf(
			[]fl.Var{{Kind: fl.Heart, Weight: 0.7}, {Kind: fl.Linear, Weight: 0.3}},
			rnd.Uniform(rng, 0.35, 0.5), rnd.Uniform(rng, 0.7, 1.0),
			rnd.Uniform(rng, -0.08, 0.08), 0, 0,
			1, rnd.Uniform(rng, 0.1, 0.2),
		)
		post(&needle, 0.78, 1.48, 0, 0, 0)
		sys.X = append(sys.X, needle)
	}

	// Final is a vertical post on a linear camera. A spherical final —
	// even mild — opened a black-hole eye on every seed and killed the
	// spindle. Nonlinear finals stay available for other structures.
	fin := xf([]fl.Var{{Kind: fl.Linear, Weight: 1}}, 0.78, 0.78, 0, 0, 0, 0.5, 1)
	post(&fin, 0.72, 1.52, 0, 0, 0)
	sys.Final = &fin
	sys.AddRotation(2)
	spindleXaos(&sys)
	return sys
}

func spindleXaos(sys *fl.System) {
	sys.InitXaos()
	var nest, body []int
	for i, x := range sys.X {
		if x.Symmetry {
			continue
		}
		if hasJulianN(x) {
			nest = append(nest, i)
		} else {
			body = append(body, i)
		}
	}
	// Nest may barely follow nest: copies must reprint it. Body→nest is
	// the recursion; nest→body is allowed so filaments leave the crossing.
	for _, i := range nest {
		for _, j := range nest {
			sys.SetXaos(i, j, 0.04)
		}
		for _, j := range body {
			sys.SetXaos(i, j, 1.2)
		}
	}
	for _, i := range body {
		for _, j := range nest {
			sys.SetXaos(i, j, 1.4)
		}
		// Mild body→body so the gold mass does not clog into a solid.
		for _, j := range body {
			sys.SetXaos(i, j, 0.55)
		}
	}
}

func hasJulianN(x fl.Xform) bool {
	for _, v := range x.Vars {
		if v.Kind == fl.JulianN {
			return true
		}
	}
	return false
}

func filamentGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: fl.Spherical, Weight: 1}},
				rnd.Uniform(rng, 0.55, 0.9), rnd.Uniform(rng, 0.55, 0.9),
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.35, 0.35), rnd.Uniform(rng, -0.35, 0.35),
				0, 1),
			xf([]fl.Var{{Kind: fl.Swirl, Weight: 0.7}, {Kind: fl.Sinusoidal, Weight: 0.3 * r}},
				rnd.Uniform(rng, 0.4, 0.75)*r, rnd.Uniform(rng, 0.4, 0.75)*r,
				rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.4, 0.4),
				1, rnd.Uniform(rng, 0.6, 1.2)),
			xf([]fl.Var{{Kind: fl.Horseshoe, Weight: 0.5}, {Kind: fl.Linear, Weight: 0.5}},
				rnd.Uniform(rng, 0.5, 0.9), rnd.Uniform(rng, 0.5, 0.9),
				rnd.Uniform(rng, -0.8, 0.8), rnd.Uniform(rng, -0.25, 0.25), rnd.Uniform(rng, -0.25, 0.25),
				0.45, rnd.Uniform(rng, 0.5, 1)),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Eyefish, Weight: 0.4 * r}, {Kind: fl.Spherical, Weight: 0.6}},
			rnd.Uniform(rng, 0.45, 0.8), rnd.Uniform(rng, 0.45, 0.8),
			rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.3, 0.3), rnd.Uniform(rng, -0.3, 0.3),
			0.75, rnd.Uniform(rng, 0.4, 0.8),
		))
	}
	if set.weave >= 5 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Disc, Weight: 1}},
			rnd.Uniform(rng, 0.4, 0.7), rnd.Uniform(rng, 0.4, 0.7),
			rnd.Uniform(rng, -0.5, 0.5), 0, 0,
			0.2, rnd.Uniform(rng, 0.2, 0.5),
		))
	}
	if rnd.Odds(rng, 0.3) {
		sys.AddDihedral()
	}
	return sys
}

func bloomGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: fl.Bubble, Weight: 1}},
				rnd.Uniform(rng, 0.7, 1.05), rnd.Uniform(rng, 0.7, 1.05),
				rnd.Uniform(rng, -0.4, 0.4), 0, 0, 0, 1),
			xf([]fl.Var{{Kind: fl.Eyefish, Weight: 0.6}, {Kind: fl.Spherical, Weight: 0.4}},
				rnd.Uniform(rng, 0.5, 0.85)*r, rnd.Uniform(rng, 0.5, 0.85)*r,
				rnd.Uniform(rng, -0.8, 0.8), rnd.Uniform(rng, -0.2, 0.2), rnd.Uniform(rng, -0.2, 0.2),
				1, 0.9),
			xf([]fl.Var{{Kind: fl.Sinusoidal, Weight: 1}},
				rnd.Uniform(rng, 0.55, 0.9), rnd.Uniform(rng, 0.55, 0.9),
				rnd.Uniform(rng, -0.5, 0.5), rnd.Uniform(rng, -0.25, 0.25), rnd.Uniform(rng, -0.25, 0.25),
				0.4, 0.7),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Cylinder, Weight: 0.5}, {Kind: fl.Linear, Weight: 0.5}},
			rnd.Uniform(rng, 0.5, 0.8), rnd.Uniform(rng, 0.6, 1),
			rnd.Uniform(rng, -0.3, 0.3), 0, 0, 0.7, 0.5,
		))
	}
	if rnd.Odds(rng, 0.45) {
		sys.AddRotation(3)
	}
	return sys
}

func spiralGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: fl.Spiral, Weight: 1}},
				rnd.Uniform(rng, 0.55, 0.9)*r, rnd.Uniform(rng, 0.55, 0.9)*r,
				rnd.Uniform(rng, -1, 1), 0, 0, 0, 1),
			xf([]fl.Var{{Kind: fl.Polar, Weight: 0.6}, {Kind: fl.Heart, Weight: 0.4}},
				rnd.Uniform(rng, 0.5, 0.85), rnd.Uniform(rng, 0.5, 0.85),
				rnd.Uniform(rng, -0.6, 0.6), rnd.Uniform(rng, -0.15, 0.15), 0,
				1, 0.85),
			xf([]fl.Var{{Kind: fl.Spherical, Weight: 1}},
				rnd.Uniform(rng, 0.6, 0.95), rnd.Uniform(rng, 0.6, 0.95),
				rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.2, 0.2), rnd.Uniform(rng, -0.2, 0.2),
				0.4, 0.7),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Hyperbolic, Weight: 0.5 * r}, {Kind: fl.Linear, Weight: 0.5}},
			rnd.Uniform(rng, 0.45, 0.75), rnd.Uniform(rng, 0.45, 0.75),
			rnd.Uniform(rng, -0.5, 0.5), 0, 0, 0.65, 0.4,
		))
	}
	return sys
}

func foldGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: fl.Horseshoe, Weight: 1}},
				rnd.Uniform(rng, 0.55, 0.9), rnd.Uniform(rng, 0.55, 0.9),
				rnd.Uniform(rng, -1, 1), rnd.Uniform(rng, -0.2, 0.2), rnd.Uniform(rng, -0.2, 0.2),
				0, 1),
			xf([]fl.Var{{Kind: fl.Disc, Weight: 0.7}, {Kind: fl.Spherical, Weight: 0.3}},
				rnd.Uniform(rng, 0.45, 0.8)*r, rnd.Uniform(rng, 0.45, 0.8)*r,
				rnd.Uniform(rng, -0.7, 0.7), 0, 0, 1, 0.8),
			xf([]fl.Var{{Kind: fl.Diamond, Weight: 0.5}, {Kind: fl.Linear, Weight: 0.5}},
				rnd.Uniform(rng, 0.5, 0.85), rnd.Uniform(rng, 0.5, 0.85),
				rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.15, 0.15), rnd.Uniform(rng, -0.15, 0.15),
				0.4, 0.65),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Ex, Weight: 1}},
			rnd.Uniform(rng, 0.4, 0.7), rnd.Uniform(rng, 0.4, 0.7),
			rnd.Uniform(rng, -0.5, 0.5), 0, 0, 0.75, 0.35,
		))
	}
	return sys
}

func juliaGenome(set settings, rng *rand.Rand) fl.System {
	r := set.reach
	sys := fl.System{
		X: []fl.Xform{
			xf([]fl.Var{{Kind: fl.Julia, Weight: 1}},
				rnd.Uniform(rng, 0.7, 1.1)*r, rnd.Uniform(rng, 0.7, 1.1)*r,
				rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.15, 0.15), rnd.Uniform(rng, -0.15, 0.15),
				0, 1.2),
			xf([]fl.Var{{Kind: fl.Linear, Weight: 1}},
				rnd.Uniform(rng, 0.45, 0.75), rnd.Uniform(rng, 0.45, 0.75),
				rnd.Uniform(rng, -math.Pi, math.Pi), rnd.Uniform(rng, -0.4, 0.4), rnd.Uniform(rng, -0.4, 0.4),
				1, 0.8),
			xf([]fl.Var{{Kind: fl.Spherical, Weight: 0.5}, {Kind: fl.Sinusoidal, Weight: 0.5}},
				rnd.Uniform(rng, 0.5, 0.85), rnd.Uniform(rng, 0.5, 0.85),
				rnd.Uniform(rng, -0.6, 0.6), 0, 0, 0.4, 0.6),
		},
	}
	if set.weave >= 4 {
		sys.X = append(sys.X, xf(
			[]fl.Var{{Kind: fl.Exponential, Weight: 0.4 * r}, {Kind: fl.Linear, Weight: 0.6}},
			rnd.Uniform(rng, 0.4, 0.7), rnd.Uniform(rng, 0.4, 0.7),
			rnd.Uniform(rng, -0.3, 0.3), 0, 0, 0.7, 0.35,
		))
	}
	return sys
}

func colorize(sys *fl.System, set settings, rng *rand.Rand) {
	creative := 0
	for i := range sys.X {
		if !sys.X[i].Symmetry {
			creative++
		}
	}
	if creative == 0 {
		return
	}
	// Spindle + split: nest is cyan, body is gold. Order-based split put
	// cool on the first map and warm on a random later one, which made a
	// gold core in a cyan envelope — the opposite of the Zander roles.
	if set.structure == structureSpindle && set.tint == tintSplit {
		colorizeSpindleSplit(sys, rng)
		return
	}
	idx := 0
	switch set.tint {
	case tintWalk:
		for i := range sys.X {
			if sys.X[i].Symmetry {
				continue
			}
			if creative == 1 {
				sys.X[i].Color = 0.5
			} else {
				sys.X[i].Color = float64(idx) / float64(creative-1)
			}
			idx++
		}
	case tintStain:
		base := rng.Float64()
		spread := rnd.Uniform(rng, 0.04, 0.14)
		for i := range sys.X {
			if sys.X[i].Symmetry {
				continue
			}
			sys.X[i].Color = clamp01(base + rnd.Uniform(rng, -spread, spread))
		}
	default: // split: first creative map cool (0), a later one warm (1)
		warmAt := 1
		if creative > 2 {
			warmAt = 1 + rng.IntN(creative-1)
		}
		for i := range sys.X {
			if sys.X[i].Symmetry {
				continue
			}
			sys.X[i].ColorSpeed = 0.92
			switch idx {
			case 0:
				sys.X[i].Color = 0
			case warmAt:
				sys.X[i].Color = 1
			default:
				// Stay on a coloured end. The mid-ramp is the muddy
				// cyan↔gold join; parking maps there makes a silver flame.
				if rng.Float64() < 0.3 {
					sys.X[i].Color = rnd.Uniform(rng, 0, 0.18)
				} else {
					sys.X[i].Color = rnd.Uniform(rng, 0.62, 1)
				}
			}
			idx++
		}
	}
}

func colorizeSpindleSplit(sys *fl.System, rng *rand.Rand) {
	for i := range sys.X {
		if sys.X[i].Symmetry {
			continue
		}
		sys.X[i].ColorSpeed = 0.94
		if hasJulianN(sys.X[i]) {
			sys.X[i].Color = rnd.Uniform(rng, 0, 0.08) // cyan nest
			continue
		}
		sys.X[i].Color = rnd.Uniform(rng, 0.72, 1) // gold / amber mass
	}
}

func clamp01(t float64) float64 {
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}
