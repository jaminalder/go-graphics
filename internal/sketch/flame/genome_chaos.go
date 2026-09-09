package flame

import (
	"math"
	"math/rand/v2"

	fl "github.com/jaminalder/go-graphics/internal/flame"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// chaosCatalog is the variation pool for structure=chaos. Electric Sheep
// variety comes from picking among many nonlinear maps per xform, not from
// jittering one fixed recipe — the six named structures alone cannot do that.
var chaosCatalog = []uint8{
	fl.Linear, fl.Sinusoidal, fl.Spherical, fl.Swirl, fl.Horseshoe,
	fl.Polar, fl.Handkerchief, fl.Heart, fl.Disc, fl.Spiral,
	fl.Hyperbolic, fl.Diamond, fl.Ex, fl.Julia, fl.Bent, fl.Waves,
	fl.Fisheye, fl.Popcorn, fl.Exponential, fl.Power, fl.Cosine,
	fl.Eyefish, fl.Bubble, fl.Cylinder, fl.Curl, fl.JulianN,
}

func chaosGenome(set settings, rng *rand.Rand) fl.System {
	r := math.Max(set.reach, 0.35)
	n := set.weave
	if rnd.Odds(rng, 0.4) {
		n++
	}
	if rnd.Odds(rng, 0.2) {
		n++
	}
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}

	sys := fl.System{X: make([]fl.Xform, 0, n+4)}
	for i := 0; i < n; i++ {
		sx := rnd.Uniform(rng, 0.2, 0.45+0.85*r)
		sy := rnd.Uniform(rng, 0.2, 0.45+0.85*r)
		if rnd.Odds(rng, 0.35) {
			sx, sy = sx*rnd.Uniform(rng, 0.35, 0.7), sy*rnd.Uniform(rng, 0.9, 1.4)
		}
		x := xf(
			chaosVars(rng, r),
			sx, sy,
			rnd.Uniform(rng, -math.Pi, math.Pi),
			rnd.Uniform(rng, -0.75*r, 0.75*r),
			rnd.Uniform(rng, -0.75*r, 0.75*r),
			rng.Float64(),
			rnd.Uniform(rng, 0.25, 1.5),
		)
		x.Opacity = rnd.Uniform(rng, 0.35, 1)
		x.ColorSpeed = rnd.Uniform(rng, 0.35, 1)
		if rnd.Odds(rng, 0.3) {
			post(&x,
				rnd.Uniform(rng, 0.65, 1.25), rnd.Uniform(rng, 0.65, 1.25),
				rnd.Uniform(rng, -0.5, 0.5),
				rnd.Uniform(rng, -0.2, 0.2), rnd.Uniform(rng, -0.2, 0.2),
			)
		}
		sys.X = append(sys.X, x)
	}

	if rnd.Odds(rng, 0.6) {
		finVars := chaosVars(rng, r)
		if rnd.Odds(rng, 0.45) {
			finVars = []fl.Var{chaosOne(rng, r)}
		}
		fin := xf(finVars,
			rnd.Uniform(rng, 0.55, 1.15), rnd.Uniform(rng, 0.55, 1.15),
			rnd.Uniform(rng, -0.6, 0.6), 0, 0, 0.5, 1,
		)
		if rnd.Odds(rng, 0.55) {
			post(&fin,
				rnd.Uniform(rng, 0.55, 1.45), rnd.Uniform(rng, 0.55, 1.45),
				rnd.Uniform(rng, -0.4, 0.4), 0, 0,
			)
		}
		sys.Final = &fin
	}

	chaosSymmetry(&sys, rng)
	if rnd.Odds(rng, 0.45) {
		chaosXaos(&sys, rng)
	}
	return sys
}

func chaosVars(rng *rand.Rand, reach float64) []fl.Var {
	n := 1 + rng.IntN(3) // 1–3
	out := make([]fl.Var, 0, n)
	var sum float64
	for i := 0; i < n; i++ {
		v := chaosOne(rng, reach)
		out = append(out, v)
		sum += v.Weight
	}
	if sum <= 0 {
		return []fl.Var{{Kind: fl.Spherical, Weight: 1}}
	}
	inv := 1 / sum
	for i := range out {
		out[i].Weight *= inv
	}
	return out
}

func chaosOne(rng *rand.Rand, reach float64) fl.Var {
	kind := chaosCatalog[rng.IntN(len(chaosCatalog))]
	v := fl.Var{Kind: kind, Weight: rnd.Uniform(rng, 0.25, 1.2)}
	switch kind {
	case fl.JulianN:
		v.P = [4]float64{
			float64(2 + rng.IntN(7)), // power 2–8
			rnd.Uniform(rng, 0.45, 0.55+0.7*reach),
		}
	case fl.Curl:
		v.P = [4]float64{rnd.Uniform(rng, -0.4, 0.4) * reach, rnd.Uniform(rng, -0.25, 0.25) * reach}
	}
	return v
}

func chaosSymmetry(sys *fl.System, rng *rand.Rand) {
	switch rng.IntN(10) {
	case 0, 1, 2:
		// none — asymmetry is most of Electric Sheep's look
	case 3, 4:
		sys.AddRotation(2)
	case 5:
		sys.AddRotation(3)
	case 6:
		sys.AddRotation(4)
	case 7:
		sys.AddDihedral()
	case 8:
		sys.AddRotation(2)
		sys.AddDihedral()
	default:
		sys.AddRotation(3)
		sys.AddDihedral()
	}
}

func chaosXaos(sys *fl.System, rng *rand.Rand) {
	sys.InitXaos()
	n := 0
	for _, x := range sys.X {
		if !x.Symmetry {
			n++
		}
	}
	if n < 2 {
		return
	}
	for i, x := range sys.X {
		if x.Symmetry {
			continue
		}
		for j, y := range sys.X {
			if y.Symmetry {
				continue
			}
			switch {
			case rnd.Odds(rng, 0.12):
				sys.SetXaos(i, j, 0)
			case rnd.Odds(rng, 0.2):
				sys.SetXaos(i, j, rnd.Uniform(rng, 0.05, 0.35))
			case rnd.Odds(rng, 0.25):
				sys.SetXaos(i, j, rnd.Uniform(rng, 1.2, 2.2))
			}
		}
	}
}
