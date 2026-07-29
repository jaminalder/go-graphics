package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/trait"
)

// The traits say what kind of piece this is; the specs say what that means
// numerically for this particular seed. Everything here is probabilistic —
// "large rings" resolves to a distribution of large sizes, not one size —
// which is what lets a handful of discrete traits carry a very large space.

// ---------------------------------------------------------------- flow field

type flowKind int

const (
	flowLinear flowKind = iota
	flowRadial
)

type flowFieldSpec struct {
	kind flowKind
	// defaultTheta is the direction a linear field points, and the fallback
	// direction for the groups that ignore the field entirely.
	defaultTheta float64
	// circularity runs from 0 (rays straight out of the centre) to 1
	// (closed circles around it); the values between are spirals.
	circularity float64
	outward     bool
	clockwise   bool
}

func newFlowFieldSpec(tr trait.Set, rng *rand.Rand) flowFieldSpec {
	linear := func(theta float64) flowFieldSpec {
		if odds(rng, 0.5) {
			theta = pi(1) - theta // mirror left-right
		}
		if odds(rng, 0.5) {
			theta += pi(1) // flip up-down
		}
		return flowFieldSpec{kind: flowLinear, defaultTheta: modulo(theta, pi(2))}
	}
	radial := func(circularity float64) flowFieldSpec {
		return flowFieldSpec{
			kind:         flowRadial,
			circularity:  circularity,
			outward:      odds(rng, 0.5),
			clockwise:    odds(rng, 0.5),
			defaultTheta: wc(rng, []weighted[float64]{{pi(0), 1}, {pi(0.25), 1}, {pi(0.5), 1}}),
		}
	}

	switch tr.Get(dimFlowField) {
	case "horizontal":
		return linear(pi(0))
	case "diagonal":
		return linear(pi(0.25))
	case "vertical":
		return linear(pi(0.5))
	case "random-linear":
		return linear(uniform(rng, pi(0), pi(0.5)))
	case "explosive":
		return radial(uniform(rng, 0.2, 0.4))
	case "spiral":
		return radial(uniform(rng, 0.4, 0.75))
	case "circular":
		return radial(math.Min(uniform(rng, 0.75, 1.02), 1))
	default: // random-radial
		return radial(math.Min(math.Max(uniform(rng, -0.01, 1.01), 0), 1))
	}
}

// ------------------------------------------------------------------ spacing

// spacingSpec is the gap kept around each dot: partly proportional to its
// size, partly a fixed distance. The mix is what decides whether a piece
// reads as a texture of touching dots or as scattered marks on a ground.
type spacingSpec struct {
	multiplier float64
	constant   float64
}

func newSpacingSpec(tr trait.Set, f frame, rng *rand.Rand) spacingSpec {
	switch tr.Get(dimSpacing) {
	case "dense":
		return spacingSpec{
			multiplier: math.Max(gauss(rng, 1, 0.04), 0.98),
			constant:   f.w(math.Max(gauss(rng, 0, 0.002), 0)),
		}
	case "medium":
		switch {
		case odds(rng, 0.333): // mostly proportional
			return spacingSpec{
				multiplier: 1.15 + math.Max(gauss(rng, 0, 0.2), 0),
				constant:   f.w(math.Max(gauss(rng, 0, 0.001), 0)),
			}
		case odds(rng, 0.5): // mostly constant
			return spacingSpec{
				multiplier: uniform(rng, 1, 1.03),
				constant:   f.w(0.003) + f.w(math.Max(gauss(rng, 0, 0.005), 0)),
			}
		default: // some of both
			return spacingSpec{
				multiplier: 1.05 + math.Max(gauss(rng, 0, 0.1), 0),
				constant:   f.w(0.002) + f.w(math.Max(gauss(rng, 0, 0.0015), 0)),
			}
		}
	default: // sparse
		switch {
		case odds(rng, 0.333):
			return spacingSpec{
				multiplier: 1.25 + math.Max(gauss(rng, 0, 0.5), 0),
				constant:   f.w(math.Max(gauss(rng, 0, 0.002), 0)),
			}
		case odds(rng, 0.5):
			return spacingSpec{
				multiplier: uniform(rng, 1.01, 1.08),
				constant:   f.w(0.008) + f.w(math.Max(gauss(rng, 0, 0.02), 0)),
			}
		default:
			return spacingSpec{
				multiplier: 1.15 + math.Max(gauss(rng, 0, 0.3), 0),
				constant:   f.w(0.005) + f.w(math.Max(gauss(rng, 0, 0.006), 0)),
			}
		}
	}
}

// --------------------------------------------------------- colour turnover

// colorChangeOdds is how often the colour is stepped along the sequence: at
// the start of a group, and at the start of each flow line within one. Low
// odds give broad fields of one colour, high odds a confetti of them.
type colorChangeOdds struct {
	group float64
	line  float64
}

func newColorChangeOdds(tr trait.Set, rng *rand.Rand) colorChangeOdds {
	var group, line float64
	switch tr.Get(dimStructure) + "/" + tr.Get(dimColorVariety) {
	case "shadows/low":
		group, line = gauss(rng, 0.15, 0.15), uniform(rng, -0.004, 0.002)
	case "shadows/medium":
		group, line = gauss(rng, 0.55, 0.2), 0.01
	case "shadows/high":
		group, line = gauss(rng, 0.9, 0.1), uniform(rng, -0.1, 0.2)
	case "formation/low":
		group, line = gauss(rng, 0.5, 0.2), uniform(rng, -0.002, 0.003)
	case "formation/medium":
		group, line = gauss(rng, 0.75, 0.2), uniform(rng, -0.005, 0.01)
	case "formation/high":
		group, line = gauss(rng, 0.9, 0.1), uniform(rng, -0.1, 0.2)
	case "orbital/low":
		group, line = gauss(rng, 0.11, 0.08), uniform(rng, -0.002, 0.0015)
	case "orbital/medium":
		group, line = gauss(rng, 0.25, 0.1), uniform(rng, -0.01, 0.01)
	default: // orbital/high
		group, line = gauss(rng, 0.7, 0.2), uniform(rng, -0.1, 0.2)
	}

	// The base rates are calibrated for medium rings packed densely; bigger
	// or sparser marks need proportionally more turnover to read as varied.
	var bySize float64
	switch tr.Get(dimRingSize) {
	case "small":
		bySize = 0.5
	case "large":
		bySize = 1.1
	default:
		bySize = 1
	}
	var bySpacing float64
	switch tr.Get(dimSpacing) {
	case "medium":
		bySpacing = 1.1
	case "sparse":
		bySpacing = 2
	default:
		bySpacing = 1
	}

	mult := bySize * bySpacing
	return colorChangeOdds{
		group: clamp01(group * mult),
		line:  clamp01(line * mult),
	}
}

func clamp01(v float64) float64 { return math.Min(math.Max(v, 0), 1) }

// ---------------------------------------------------------------- dot size

// scaleGenerator emits dot radii. It is stateful: change() re-rolls the
// working mean, next() draws around it. Sizes therefore arrive in runs
// rather than independently, which is what keeps a piece legible.
type scaleGenerator struct {
	variety string
	mean    float64
	choices []weighted[float64]
}

func newScaleGenerator(tr trait.Set, f frame, rng *rand.Rand) *scaleGenerator {
	// The size ladder, in fractions of the canvas width.
	xs := []float64{f.w(0.0018), f.w(0.0025)}
	s := []float64{f.w(0.003), f.w(0.004), f.w(0.006)}
	m := []float64{f.w(0.012), f.w(0.017), f.w(0.023), f.w(0.048)}
	l := []float64{f.w(0.1), f.w(0.15), f.w(0.2), f.w(0.3)}

	size := tr.Get(dimRingSize)
	switch tr.Get(dimSizeVariety) {
	case "constant":
		var choices []weighted[float64]
		switch size {
		case "small":
			choices = []weighted[float64]{{xs[1], 2}, {s[0], 3}, {s[1], 2}, {s[2], 1}}
		case "medium":
			choices = []weighted[float64]{{m[0], 2}, {m[1], 3}, {m[2], 2}, {m[3], 1}}
		default:
			choices = []weighted[float64]{{l[0], 3}, {l[1], 2}, {l[2], 1}}
		}
		return &scaleGenerator{variety: "constant", mean: wc(rng, choices), choices: choices}

	case "variable":
		var choices []weighted[float64]
		switch size {
		case "small":
			choices = []weighted[float64]{{xs[1], 1.3}, {s[0], 2}, {s[1], 5}, {s[2], 8}, {m[0], 3}}
		case "medium":
			choices = []weighted[float64]{{s[2], 2}, {m[0], 8}, {m[1], 8}, {m[2], 13}, {m[3], 8}, {l[0], 5}}
		default:
			choices = []weighted[float64]{{m[1], 0.5}, {m[2], 2}, {m[3], 2}, {l[0], 5}, {l[1], 8}, {l[2], 8}, {l[3], 4}}
		}
		return &scaleGenerator{variety: "variable", mean: wc(rng, choices), choices: choices}

	default: // wild — starts anywhere on the ladder and roams widely
		var starts []float64
		var choices []weighted[float64]
		switch size {
		case "small":
			starts = []float64{s[1], s[2], m[0], m[1], m[2]}
			choices = []weighted[float64]{
				{xs[0], 3},
				{xs[1], 3},
				{s[0], 3},
				{s[1], 4},
				{s[2], 4},
				{m[0], 3},
				{m[1], 3},
				{m[2], 3},
			}
		case "medium":
			starts = []float64{s[2], m[0], m[1], m[2], m[3], l[0], l[1]}
			choices = []weighted[float64]{
				{xs[0], 1},
				{xs[1], 1},
				{s[0], 1},
				{s[1], 1},
				{s[2], 2},
				{m[0], 3},
				{m[1], 3},
				{m[2], 3},
				{m[3], 3},
				{l[0], 2},
				{l[1], 2},
				{l[2], 1},
			}
		default:
			starts = []float64{l[0], l[1], l[2], l[3]}
			choices = []weighted[float64]{
				{xs[0], 1},
				{xs[1], 1},
				{s[0], 1},
				{s[1], 1},
				{s[2], 1},
				{m[0], 1},
				{m[1], 1},
				{m[2], 1},
				{l[0], 2},
				{l[1], 5},
				{l[2], 5},
				{l[3], 5},
			}
		}
		return &scaleGenerator{variety: "wild", mean: choice(rng, starts), choices: choices}
	}
}

func (g *scaleGenerator) next(f frame, rng *rand.Rand) float64 {
	switch g.variety {
	case "constant":
		return gauss(rng, g.mean, math.Min(f.w(0.01), g.mean*0.05))
	case "variable":
		return gauss(rng, g.mean, math.Min(f.w(0.035), g.mean*0.15))
	default:
		return gauss(rng, g.mean, g.mean*0.3)
	}
}

func (g *scaleGenerator) change(rng *rand.Rand) {
	switch g.variety {
	case "constant":
		// A constant piece keeps one mean for its whole life.
	case "variable":
		g.mean = wc(rng, g.choices)
		g.mean = gauss(rng, g.mean, g.mean*0.1)
	default:
		g.mean = wc(rng, g.choices)
		g.mean = gauss(rng, g.mean, g.mean*0.3)
	}
}

// -------------------------------------------------------------- ring counts

// bullseye is how one dot is built: how many rings, and how much of the
// available radius is ink.
type bullseye struct {
	rings   int
	density float64
}

// bullseyeGenerator turns the bullseye and ring-thickness traits into a
// weighted menu of ring counts. The weights fall off geometrically, so one
// count dominates a piece and the others punctuate it.
type bullseyeGenerator struct {
	densityMean float64
	densitySpan float64
	ringOptions []weighted[int]
}

func newBullseyeGenerator(tr trait.Set, rng *rand.Rand) bullseyeGenerator {
	counts := bullseyeRings(tr.Get(dimBullseye))

	var mean, span float64
	switch tr.Get(dimRingThickness) {
	case "thin":
		mean, span = 0.85, 0.15
	case "thick":
		mean, span = 0.28, 0.1
	default: // mixed
		mean, span = 0.7, 1.0
	}
	dropoff := rescale(span, 0, 1, 1, 0.35)

	var options []weighted[int]
	weight := 1.0
	for n := len(counts) * 2; n > 0 && weight > 0.001; n-- {
		options = append(options, weighted[int]{choice(rng, counts), weight})
		weight *= dropoff
	}
	return bullseyeGenerator{densityMean: mean, densitySpan: span, ringOptions: options}
}

func (g bullseyeGenerator) next(rng *rand.Rand) bullseye {
	density := math.Min(math.Max(gauss(rng, g.densityMean, g.densitySpan/2), 0.17), 0.93)
	return bullseye{rings: wc(rng, g.ringOptions), density: density}
}

// ----------------------------------------------------------------- margins

// marginChecker keeps marks off the edge of the paper. The bottom margin is
// the widest of the three, which is how a mounted print wants it.
type marginChecker struct {
	frame  frame
	margin float64
	bottom float64
}

func newMarginChecker(tr trait.Set, f frame) marginChecker {
	switch tr.Get(dimMargin) {
	case "none":
		return marginChecker{frame: f, margin: f.w(-0.05), bottom: f.w(-0.05)}
	case "crisp":
		return marginChecker{frame: f, margin: f.w(0.003), bottom: f.w(0.003)}
	default: // wide
		return marginChecker{frame: f, margin: f.w(0.07), bottom: f.w(0.08)}
	}
}

func (m marginChecker) inBounds(x, y, spacing float64) bool {
	f := m.frame
	return !(x-spacing < m.margin ||
		x+spacing >= f.w(1)-m.margin ||
		y-spacing < m.margin ||
		y+spacing > f.h(1)-m.bottom)
}

// --------------------------------------------------------------- stacking

// stackOffset is the shift of the shadow copy drawn under every dot in a
// stacked piece — a small, consistent displacement that reads as depth.
type stackOffset struct {
	on   bool
	x, y float64
}

func newStackOffset(tr trait.Set, f frame, rng *rand.Rand) stackOffset {
	if !tr.Is(dimColorMode, "stacked") {
		return stackOffset{}
	}
	return stackOffset{
		on: true,
		x:  gauss(rng, 0, f.w(0.0013)),
		y:  math.Abs(gauss(rng, 0, f.w(0.0013))),
	}
}

// ignoreFlowField is the chance that a whole group ignores the field and
// runs straight instead. Most pieces never do it; when they do, the straight
// groups cut across the curved ones and give the composition a second voice.
func newIgnoreFlowFieldOdds(rng *rand.Rand) float64 {
	return wc(rng, []weighted[float64]{{0, 10}, {0.5, 2}, {0.8, 1}, {0.9, 1}})
}
