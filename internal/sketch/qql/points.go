package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// dot is one ring-dot, fully specified: where it sits, how big it is, the
// two colours it alternates between, and how it is built out of rings.
type dot struct {
	x, y      float64
	scale     float64
	primary   palette.HSB
	secondary palette.HSB
	rings     int
	density   float64
}

// packSectors is how many collision cells span the packing area on its
// shorter axis — the source's tuning, and a good one: fine enough that a
// query touches few neighbours, coarse enough that a large dot does not
// have to be listed in hundreds of cells.
const packSectors = 50

// packOverscan is how far past the canvas dots may be placed, so a piece
// with no margin runs off the edge instead of stopping at it.
const packOverscan = 0.05

// packer walks the flow lines and lays dots along them, rejecting any that
// would crowd one already placed. Rejection is the whole mechanism: the
// field says where dots would like to go, and the spacing rule decides
// which of those wishes survive. That is why dense pieces come out as
// packed textures and sparse ones as constellations, from the same lines.
type packer struct {
	traits  trait.Set
	frame   frame
	spacing spacingSpec
	odds    colorChangeOdds
	scales  *scaleGenerator
	rings   bullseyeGenerator
	scheme  scheme
	margins marginChecker
	index   *geom.Index
	tracer  *tracer
	spec    flowFieldSpec
	rng     *rand.Rand
}

func (p *packer) run(groups [][]pt, ignore []bool) []dot {
	rng := p.rng

	primaryIdx := rng.IntN(len(p.scheme.Primary))
	secondaryIdx := rng.IntN(len(p.scheme.Secondary))
	groupRings := p.rings.next(rng)

	var dots []dot
	for gi, group := range groups {
		// A group is the unit that shares a colour: most of the time it
		// inherits the last one, and now and then the piece turns a corner.
		if odds(rng, p.odds.group) {
			primaryIdx = pickNextColor(len(p.scheme.Primary), primaryIdx, rng)
			secondaryIdx = pickNextColor(len(p.scheme.Secondary), secondaryIdx, rng)
			groupRings = p.rings.next(rng)
		}
		dots = p.buildGroup(dots, group, ignore[gi], primaryIdx, secondaryIdx, groupRings)
	}
	return dots
}

func (p *packer) buildGroup(dots []dot, group []pt, ignore bool, primaryIdx, secondaryIdx int, rings bullseye) []dot {
	rng := p.rng
	f := p.frame

	if odds(rng, p.odds.line) {
		p.scales.change(rng)
	}
	// One size for the whole group. The generator keeps being re-rolled per
	// line below, but this value is what the group is drawn at — sizes
	// change between passages, not within them.
	scale := p.scales.next(f, rng)

	primarySwatch := p.scheme.swatch(p.scheme.Primary[primaryIdx])
	primary := primarySwatch.Draw(rng)
	secondarySwatch := p.scheme.swatch(p.scheme.Secondary[secondaryIdx])
	secondary := secondarySwatch.Draw(rng)

	// Big dots always keep a little air; below that the trait decides.
	multiplier := p.spacing.multiplier
	if scale > f.w(0.015) {
		multiplier = math.Max(multiplier, 1.02)
	}
	spacingRadius := math.Max(scale*multiplier+p.spacing.constant, scale*0.75)

	for _, start := range group {
		for _, q := range p.tracer.trace(start.X, start.Y, ignore, p.spec.defaultTheta) {
			if !p.margins.inBounds(q.X, q.Y, spacingRadius) {
				continue
			}
			c := geom.Circle{X: q.X, Y: q.Y, R: spacingRadius}
			if !p.index.FitsWithGap(c, 0) {
				continue
			}
			p.index.Insert(c)
			// The colours drift as the line is laid down, so a run of dots
			// is a family rather than a repeat of one value.
			primary = primarySwatch.Step(rng, primary)
			secondary = secondarySwatch.Step(rng, secondary)
			dots = append(dots, dot{
				x: q.X, y: q.Y, scale: scale,
				primary: primary, secondary: secondary,
				rings: rings.rings, density: rings.density,
			})
		}

		p.scales.change(rng)
		if odds(rng, p.odds.line) {
			primaryIdx = pickNextColor(len(p.scheme.Primary), primaryIdx, rng)
			// Swapping the swatch but not the current colour lets the walk
			// cross into the new colour instead of jumping to it.
			primarySwatch = p.scheme.swatch(p.scheme.Primary[primaryIdx])
			rings = p.rings.next(rng)
		}
		if odds(rng, p.odds.line) {
			secondaryIdx = pickNextColor(len(p.scheme.Secondary), secondaryIdx, rng)
			secondarySwatch = p.scheme.swatch(p.scheme.Secondary[secondaryIdx])
		}
	}
	return dots
}

// newPackIndex builds the collision index over the packing area.
func newPackIndex(f frame) *geom.Index {
	cell := f.h(1+2*packOverscan) / packSectors
	return geom.NewIndexIn(
		f.w(-packOverscan), f.h(-packOverscan),
		f.w(1+packOverscan), f.h(1+packOverscan), cell)
}

// drawnRings is how many rings a dot actually gets. A dot's natural ring
// count is capped by how much room it has: seven rings inside a three-pixel
// dot would be a grey smudge, so small dots quietly lose rings until the
// ring-dot motif is still visible. Preserving that motif at every size is
// what holds the whole output space together.
func (d dot) drawnRings(f frame) int {
	switch {
	case d.scale < rescale(d.density, 0.15, 1, f.w(0.0039), f.w(0.001)):
		return min(d.rings, 1)
	case d.scale < rescale(d.density, 0.15, 1, f.w(0.0072), f.w(0.0029)):
		return min(d.rings, 2)
	case d.scale < f.w(0.01):
		return min(d.rings, 3)
	case d.scale < f.w(0.012):
		return min(d.rings, 4)
	case d.scale < f.w(0.014):
		return min(d.rings, 5)
	case d.scale < f.w(0.017):
		return min(d.rings, 6)
	case d.scale < f.w(0.02):
		return min(d.rings, 7)
	case d.scale < f.w(0.023):
		return min(d.rings, 8)
	default:
		return d.rings
	}
}
