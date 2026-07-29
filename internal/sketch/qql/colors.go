package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/trait"
)

// newScheme resolves a palette into the colour plan for one piece: which
// ground it sits on, and — the part that does the work — two short
// sequences winnowed out of the palette's long one. Thinning rather than
// sampling keeps the survivors in their original neighbourly order, so the
// walk along them still moves between related colours.
func newScheme(tr trait.Set, set colorSet, f frame, rng *rand.Rand) scheme {
	bgOpts := make([]weighted[bgChoice], len(set.Backgrounds))
	for i, b := range set.Backgrounds {
		bgOpts[i] = weighted[bgChoice]{b, b.Weight}
	}
	bg := wc(rng, bgOpts)

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

	splatterOpts := make([]weighted[swatchKey], 0, len(set.Splatter))
	for _, s := range set.Splatter {
		if to, keep := bg.substitute(s.Color); keep {
			splatterOpts = append(splatterOpts, weighted[swatchKey]{to, s.Weight})
		}
	}
	var splatterChoices []swatchKey
	if len(splatterOpts) > 0 {
		n := max(1, int(math.Round(gauss(rng, 1.5, 2))))
		for range n {
			splatterChoices = append(splatterChoices, wc(rng, splatterOpts))
		}
	}

	variety := tr.Get(dimColorVariety)
	var oddsChoices []weighted[float64]
	var countChoices []weighted[int]
	switch variety {
	case "low":
		oddsChoices = []weighted[float64]{{0, 4}, {0.001, 2}, {0.002, 2}, {0.005, 2}}
		countChoices = []weighted[int]{{1, 1}, {2, 3}, {3, 4}, {4, 5}, {5, 3}}
	case "medium":
		oddsChoices = []weighted[float64]{{0, 3}, {0.002, 2}, {0.005, 2}, {0.01, 1}, {0.03, 1}}
		countChoices = []weighted[int]{{5, 1}, {6, 2}, {7, 3}, {8, 5}, {10, 3}, {15, 2}}
	default: // high — and the one-in-a-thousand piece that is nothing but splatter
		oddsChoices = []weighted[float64]{
			{0, 3}, {0.002, 2}, {0.005, 2}, {0.01, 1}, {0.03, 1}, {0.08, 1}, {0.5, 0.05},
		}
		countChoices = []weighted[int]{{10, 3}, {12, 4}, {15, 5}, {20, 3}, {25, 3}}
	}

	primary := winnow(rng, seq, wc(rng, countChoices))
	secondary := winnow(rng, seq, wc(rng, countChoices))

	return scheme{
		set:             set,
		Background:      set.swatch(bg.Color).Color(),
		Primary:         primary,
		Secondary:       secondary,
		SplatterCenterX: uniform(rng, f.w(-0.1), f.w(1.1)),
		SplatterCenterY: uniform(rng, f.h(-0.1), f.h(1.1)),
		SplatterOdds:    wc(rng, oddsChoices),
		SplatterChoices: splatterChoices,
	}
}
