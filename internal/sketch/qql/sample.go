package qql

import (
	"math"
	"math/rand/v2"
)

// The sampling vocabulary QQL's parameter tables are written in. Nearly
// every value in the algorithm is a weighted choice among a handful of
// hand-picked options, then softened by a gaussian — that combination is
// what makes a trait like "large rings" mean *usually* large rather than
// one fixed size, and it is why the output space has outliers at all.
//
// These live here rather than in a shared package until a second sketch
// needs them; see docs/sketches/007-qql.md.

// weighted pairs an option with its relative likelihood.
type weighted[T any] struct {
	V T
	W float64
}

// wc picks an option with probability proportional to its weight.
func wc[T any](rng *rand.Rand, opts []weighted[T]) T {
	var total float64
	for _, o := range opts {
		if o.W > 0 {
			total += o.W
		}
	}
	if total <= 0 {
		return opts[len(opts)-1].V
	}
	bisect := rng.Float64() * total
	var cum float64
	for _, o := range opts {
		if o.W <= 0 {
			continue
		}
		cum += o.W
		if cum > bisect {
			return o.V
		}
	}
	return opts[len(opts)-1].V
}

// choice picks one element uniformly.
func choice[T any](rng *rand.Rand, xs []T) T { return xs[rng.IntN(len(xs))] }

// uniform draws from [lo, hi).
func uniform(rng *rand.Rand, lo, hi float64) float64 { return lo + rng.Float64()*(hi-lo) }

// gauss draws from a normal distribution.
func gauss(rng *rand.Rand, mean, stdev float64) float64 {
	return mean + stdev*rng.NormFloat64()
}

// odds reports true with probability p.
func odds(rng *rand.Rand, p float64) bool {
	if p <= 0 {
		return false
	}
	return rng.Float64() < p
}

// winnow removes elements at random until at most n remain, keeping the
// order of the survivors. Thinning a long sequence rather than sampling a
// short one is what leaves the remaining colours in their original
// neighbourly order, so a walk along them still moves in small steps.
func winnow[T any](rng *rand.Rand, xs []T, n int) []T {
	out := make([]T, len(xs))
	copy(out, xs)
	for len(out) > n {
		i := rng.IntN(len(out))
		out = append(out[:i], out[i+1:]...)
	}
	return out
}

// shuffled returns a permutation of xs.
func shuffled[T any](rng *rand.Rand, xs []T) []T {
	out := make([]T, len(xs))
	copy(out, xs)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// rescale maps value from one interval to another, clamping to the source
// interval first so callers can pass out-of-range values safely.
func rescale(value, oldMin, oldMax, newMin, newMax float64) float64 {
	v := math.Min(math.Max(value, math.Min(oldMin, oldMax)), math.Max(oldMin, oldMax))
	return newMin + (v-oldMin)*(newMax-newMin)/(oldMax-oldMin)
}

// pi returns v half-turns in radians, matching how the source expresses
// angles (pi(0.5) is a quarter turn).
func pi(v float64) float64 { return math.Pi * v }

// modulo is the always-positive remainder.
func modulo(n, m float64) float64 { return math.Mod(math.Mod(n, m)+m, m) }

// angle is the direction from p1 to p2, in [0, 2π).
func angle(x1, y1, x2, y2 float64) float64 {
	return modulo(math.Atan2(y2-y1, x2-x1), pi(2))
}

// dist is the distance between two points.
func dist(x1, y1, x2, y2 float64) float64 { return math.Hypot(x2-x1, y2-y1) }

// frame carries the canvas proportions. QQL states every length as a
// fraction of its virtual width or height; in canvas units the height is 1
// and the width is the aspect ratio, so the same fractions carry over
// unchanged and the composition is resolution independent.
type frame struct{ aspect float64 }

func (f frame) w(v float64) float64 { return v * f.aspect }

func (f frame) h(v float64) float64 { return v }
