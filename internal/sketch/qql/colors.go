package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// newScheme resolves a palette into the colour plan for one piece: which
// ground it sits on, and — the part that does the work — two short
// sequences winnowed out of the palette's long one. Thinning rather than
// sampling keeps the survivors in their original neighbourly order, so the
// walk along them still moves between related colours.
func newScheme(tr trait.Set, set colorSet, f frame, rng *rand.Rand) scheme {
	bgOpts := make([]rnd.Weighted[bgChoice], len(set.Backgrounds))
	for i, b := range set.Backgrounds {
		bgOpts[i] = rnd.Weighted[bgChoice]{V: b, W: b.Weight}
	}
	bg := rnd.Pick(rng, bgOpts)

	// The ground rewrites the palette: colours that would vanish against it
	// are swapped for a readable cousin, or dropped outright.
	seq := make([]swatchKey, 0, len(set.Seq))
	for _, k := range set.Seq {
		if to, keep := bg.substitute(k); keep {
			seq = append(seq, to)
		}
	}
	if len(seq) == 0 {
		seq = set.Seq // a ground that erased everything is no ground at all
	}

	splatterOpts := make([]rnd.Weighted[swatchKey], 0, len(set.Splatter))
	for _, s := range set.Splatter {
		if to, keep := bg.substitute(s.Color); keep {
			splatterOpts = append(splatterOpts, rnd.Weighted[swatchKey]{V: to, W: s.Weight})
		}
	}
	var splatterChoices []swatchKey
	if len(splatterOpts) > 0 {
		n := max(1, int(math.Round(rnd.Gauss(rng, 1.5, 2))))
		for range n {
			splatterChoices = append(splatterChoices, rnd.Pick(rng, splatterOpts))
		}
	}

	variety := tr.Get(dimColorVariety)
	var oddsChoices []rnd.Weighted[float64]
	var countChoices []rnd.Weighted[int]
	switch variety {
	case "low":
		oddsChoices = []rnd.Weighted[float64]{{V: 0, W: 4}, {V: 0.001, W: 2}, {V: 0.002, W: 2}, {V: 0.005, W: 2}}
		countChoices = []rnd.Weighted[int]{{V: 1, W: 1}, {V: 2, W: 3}, {V: 3, W: 4}, {V: 4, W: 5}, {V: 5, W: 3}}
	case "medium":
		oddsChoices = []rnd.Weighted[float64]{{V: 0, W: 3}, {V: 0.002, W: 2}, {V: 0.005, W: 2}, {V: 0.01, W: 1}, {V: 0.03, W: 1}}
		countChoices = []rnd.Weighted[int]{{V: 5, W: 1}, {V: 6, W: 2}, {V: 7, W: 3}, {V: 8, W: 5}, {V: 10, W: 3}, {V: 15, W: 2}}
	default: // high — and the one-in-a-thousand piece that is nothing but splatter
		oddsChoices = []rnd.Weighted[float64]{
			{V: 0, W: 3}, {V: 0.002, W: 2}, {V: 0.005, W: 2}, {V: 0.01, W: 1}, {V: 0.03, W: 1}, {V: 0.08, W: 1}, {V: 0.5, W: 0.05},
		}
		countChoices = []rnd.Weighted[int]{{V: 10, W: 3}, {V: 12, W: 4}, {V: 15, W: 5}, {V: 20, W: 3}, {V: 25, W: 3}}
	}

	primary := rnd.Winnow(rng, seq, rnd.Pick(rng, countChoices))
	secondary := rnd.Winnow(rng, seq, rnd.Pick(rng, countChoices))

	return scheme{
		set:             set,
		Background:      set.swatch(bg.Color).Color(),
		Primary:         primary,
		Secondary:       secondary,
		SplatterCenterX: rnd.Uniform(rng, f.w(-0.1), f.w(1.1)),
		SplatterCenterY: rnd.Uniform(rng, f.h(-0.1), f.h(1.1)),
		SplatterOdds:    rnd.Pick(rng, oddsChoices),
		SplatterChoices: splatterChoices,
	}
}
