// Package rnd is the sampling vocabulary the sketches' parameter tables
// are written in: weighted choice, gaussians, thinning, shuffling.
//
// It exists because nearly every interesting value in a generative sketch
// is a *weighted choice among a handful of hand-picked options, softened
// by a gaussian*. That combination is what makes a parameter like "large
// rings" mean usually-large rather than one fixed size, and it is what
// gives an output space genuine outliers instead of uniform noise. It was
// written for sketch 007 and then reinvented, more weakly, twice — which
// is what moved it here.
//
// Every function takes its generator explicitly: there is no package
// state, and determinism (invariant 1) stays the caller's to reason about.
//
// Stdlib-only leaf.
package rnd

import "math/rand/v2"

// Weighted pairs an option with its relative likelihood. Weights are
// relative within one list and need not sum to anything; a weight of 0 or
// less means the option is never drawn, which is how a value is kept
// reachable by an explicit override but out of every seed's reach.
type Weighted[T any] struct {
	V T
	W float64
}

// Pick draws one option with probability proportional to its weight. An
// empty or all-zero list yields the last option, so a table can never
// panic mid-render on a value nobody thought to weight.
func Pick[T any](rng *rand.Rand, opts []Weighted[T]) T {
	var total float64
	for _, o := range opts {
		if o.W > 0 {
			total += o.W
		}
	}
	if total <= 0 {
		var zero T
		if len(opts) == 0 {
			return zero
		}
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
	// Reachable only through float rounding at the very top of the range.
	return opts[len(opts)-1].V
}

// PickIndex is Pick over a bare weight list, for callers whose options are
// already an indexed slice.
func PickIndex(rng *rand.Rand, weights []float64) int {
	var total float64
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return len(weights) - 1
	}
	bisect := rng.Float64() * total
	var cum float64
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		cum += w
		if cum > bisect {
			return i
		}
	}
	return len(weights) - 1
}

// Choice picks one element uniformly.
func Choice[T any](rng *rand.Rand, xs []T) T { return xs[rng.IntN(len(xs))] }

// Uniform draws from [lo, hi).
func Uniform(rng *rand.Rand, lo, hi float64) float64 { return lo + rng.Float64()*(hi-lo) }

// Gauss draws from a normal distribution. This is the softener: a table
// gives the character, a gaussian around it gives the variety.
func Gauss(rng *rand.Rand, mean, stdev float64) float64 {
	return mean + stdev*rng.NormFloat64()
}

// Odds reports true with probability p.
func Odds(rng *rand.Rand, p float64) bool {
	if p <= 0 {
		return false
	}
	return rng.Float64() < p
}

// Winnow removes elements at random until at most n remain, keeping the
// order of the survivors.
//
// Thinning a long sequence rather than sampling a short one is the point:
// the survivors stay in their original neighbourly order, so a walk along
// them still moves in small steps. Sampling n freely gives a set with no
// order left in it.
func Winnow[T any](rng *rand.Rand, xs []T, n int) []T {
	out := make([]T, len(xs))
	copy(out, xs)
	for len(out) > n {
		i := rng.IntN(len(out))
		out = append(out[:i], out[i+1:]...)
	}
	return out
}

// Shuffled returns a permutation of xs, leaving the input alone.
func Shuffled[T any](rng *rand.Rand, xs []T) []T {
	out := make([]T, len(xs))
	copy(out, xs)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// Bag builds a draw pile with a long tail: after a shuffle, one element
// dominates, one supports it and the rest are progressively rarer.
//
// Drawing uniformly from a palette gives every colour equal presence,
// which reads as confetti; a dominant choice with rare accents is what
// reads as chosen. weights is the tail, longest first; elements past its
// end get weight 1.
func Bag[T any](rng *rand.Rand, xs []T, weights []int) []T {
	order := Shuffled(rng, xs)
	var bag []T
	for i, x := range order {
		w := 1
		if i < len(weights) {
			w = weights[i]
		}
		for range w {
			bag = append(bag, x)
		}
	}
	return bag
}
