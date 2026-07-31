package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// A ring-dot is never drawn as a clean circle. Each of its bands is laid
// down as a stack of thin concentric strokes, every one nudged off centre
// and given its own thickness, so the edge comes out worked rather than
// printed. That texture is the motif's signature: it survives at every
// size, and it is what keeps a hundred thousand circles from reading as
// clip art.

// minRadius is the smallest ring worth drawing, in canvas widths — below it
// a band has nothing left to say.
const minRadius = 0.0004

// eccentricity is how far a band may depart from a true circle, as a
// fraction of its radius.
const eccentricity = 0.007

func paintDots(cv *paint.Canvas, f frame, tr trait.Set, sch scheme, stack stackOffset, dots []dot, rng *rand.Rand, w *dotWash) {
	zebra := tr.Is(dimColorMode, "zebra")

	var splattered []dot
	for _, d := range dots {
		// Splatter clusters: the closer a dot is to the splatter centre,
		// the likelier it is to be flecked, falling off steeply with
		// distance so the flecks read as a spill rather than a dusting.
		if sch.SplatterOdds > 0 && len(sch.SplatterChoices) > 0 {
			fade := math.Pow(mathx.Rescale(dist(d.x, d.y, sch.SplatterCenterX, sch.SplatterCenterY),
				0, f.w(1.4), 1, 0), 2.5)
			if rnd.Odds(rng, sch.SplatterOdds*fade) {
				splattered = append(splattered, d)
			}
		}

		if stack.on {
			// The shadow copy: the same dot in the secondary colour, shifted
			// a hair, drawn underneath.
			shadow := d
			shadow.x += stack.x
			shadow.y += stack.y
			shadow.primary = d.secondary
			shadow.secondary = d.secondary
			shadow.density = d.density * rnd.Gauss(rng, 0.99, 0.03)
			drawRingDot(cv, f, shadow, rng, w)
		}

		if !zebra {
			// Outside zebra mode a dot is one colour throughout; the second
			// colour only shows in the stacked shadow.
			d.secondary = d.primary
		}
		drawRingDot(cv, f, d, rng, w)
	}

	for _, d := range splattered {
		c := sch.swatch(rnd.Choice(rng, sch.SplatterChoices))
		col := c.Draw(rng)
		d.primary, d.secondary = col, col
		d.density = math.Max(0.17, d.density*0.7)
		drawRingDot(cv, f, d, rng, w)
	}
}

// drawRingDot lays the bands of one dot from the outside in, alternating
// the two colours. Density decides how much of each band's share of the
// radius is ink: a dense dot is nearly solid, a sparse one is a target.
//
// w is the watercolour medium, or nil for ink. Where the bands go is
// settled identically either way — the medium only decides what they are
// laid with.
func drawRingDot(cv *paint.Canvas, f frame, d dot, rng *rand.Rand, w *dotWash) {
	rings := d.drawnRings(f)
	if rings < 1 {
		return
	}
	bandStep := d.scale / float64(rings)
	bandThickness := math.Max(f.w(minRadius), bandStep*(1-d.density))

	// A tightly packed dot has less room to let its bands wander.
	varianceAdjust := mathx.Rescale(d.density, 0.1, 1, 0.5, 1.2)
	var positionVariance float64
	if rings >= 7 {
		positionVariance = varianceAdjust * mathx.Rescale(float64(rings), 7, 9, 0.008, 0.005)
	} else {
		positionVariance = varianceAdjust * mathx.Rescale(float64(rings), 1, 7, 0.022, 0.008)
	}
	thicknessVariance := mathx.Rescale(d.density, 0.1, 1, 0.01, 0.13)

	primary := d.primary.Color()
	secondary := d.secondary.Color()

	band := 0
	for r := d.scale; r > f.w(minRadius); r -= bandStep {
		col := primary
		if band%2 == 1 {
			col = secondary
		}
		band++

		wobble := math.Min(f.w(0.0005), r*positionVariance)
		cx := rnd.Gauss(rng, d.x, wobble)
		cy := rnd.Gauss(rng, d.y, wobble)

		thickness := rnd.Gauss(rng, bandThickness, bandThickness*thicknessVariance)
		if r < f.w(0.002) && rings == 1 {
			// A dot too small to hold a ring becomes a disc, more or less
			// filled depending on its density.
			thickness = mathx.Rescale(d.density, 0, 1, r, r*0.25)
		}
		if rings == 1 && d.scale > f.w(0.02) {
			// Keep big single-ring dots from turning into fat doughnuts.
			thickness = mathx.Rescale(thickness, f.w(0.003), f.w(0.08), f.w(0.003), f.w(0.05))
			thickness = math.Min(thickness, mathx.Rescale(d.scale, 0, f.w(0.1), f.w(0.003), f.w(0.04)))
			thickness = math.Min(thickness, f.w(0.04))
		}

		if w != nil && w.band(cv, cx, cy, r, thickness, col, rng) {
			continue
		}
		drawMessyCircle(cv, f, cx, cy, r, thickness, varianceAdjust, col, rng)
	}
}

// drawMessyCircle fills one band with concentric strokes, each offset and
// re-thicknessed. The first few rounds wander further than the rest, which
// is what frays the outer edge while the interior stays solid.
func drawMessyCircle(cv *paint.Canvas, f frame, x, y, r, thickness, varianceAdjust float64, col palette.Color, rng *rand.Rand) {
	if r <= 0 || thickness <= 0 {
		return
	}

	// How finely the band is subdivided: thin bands get proportionally more
	// strokes, so they stay opaque instead of turning to lace.
	var divisor float64
	switch {
	case thickness > f.w(0.02):
		divisor = mathx.Rescale(thickness, f.w(0.02), f.w(0.04), f.w(0.00021), f.w(0.00022))
	case thickness > f.w(0.006):
		divisor = mathx.Rescale(thickness, f.w(0.006), f.w(0.02), f.w(0.00015), f.w(0.00021))
	case thickness > f.w(0.003):
		divisor = mathx.Rescale(thickness, f.w(0.003), f.w(0.006), f.w(0.00012), f.w(0.00015))
	default:
		divisor = mathx.Rescale(thickness, 0, f.w(0.006), f.w(0.00016), f.w(0.00012))
	}
	rounds := int(math.Ceil(math.Max(thickness/divisor, 1)))

	varianceRatio := varianceAdjust * mathx.Rescale(thickness, f.w(0.001), f.w(0.04), 0.08, 0.03)

	var meanThickness float64
	switch {
	case thickness > f.w(0.02):
		meanThickness = mathx.Rescale(thickness, f.w(0.02), f.w(0.04), f.w(0.0007), f.w(0.00073))
	case thickness > f.w(0.006):
		meanThickness = mathx.Rescale(thickness, f.w(0.006), f.w(0.02), f.w(0.0005), f.w(0.0007))
	default:
		meanThickness = mathx.Rescale(thickness, f.w(0.001), f.w(0.006), f.w(0.0001), f.w(0.0005))
	}

	for i := range rounds {
		rr := mathx.Rescale(float64(i), 0, float64(rounds), r, r-thickness)

		wobble := varianceAdjust * math.Min(f.w(0.0015), thickness*varianceRatio)
		spread := 1.0
		if i < 5 {
			wobble *= 1.5
			spread = 2
		}
		cx := rnd.Gauss(rng, x, wobble)
		cy := rnd.Gauss(rng, y, wobble)

		lineVariance := meanThickness * spread * mathx.Rescale(thickness, f.w(0.001), f.w(0.04), 0.25, 1.1)
		if rr < f.w(0.002) {
			lineVariance = meanThickness * 0.1
		}
		lineThickness := math.Max(rnd.Gauss(rng, meanThickness, lineVariance), f.w(0.0002))

		drawCleanCircle(cv, f, cx, cy, rr, lineThickness, col, rng)
	}
}

// drawCleanCircle stamps a single stroke: a very slightly elliptical ring,
// which is what stops a field of circles looking machined.
func drawCleanCircle(cv *paint.Canvas, f frame, x, y, r, thickness float64, col palette.Color, rng *rand.Rand) {
	r = math.Max(r-thickness*0.5, f.w(0.0002))
	weight := thickness * 0.95
	variance := math.Min(f.w(0.0015), r*eccentricity)
	rx := rnd.Gauss(rng, r, variance)
	ry := rnd.Gauss(rng, r, variance)
	if rx <= 0 || ry <= 0 {
		return
	}
	cv.Ring(x, y, rx, ry, weight, col, 1)
}
