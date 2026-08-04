package riffle

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// Surface is the riffle's all-water flow and wave field exposed as a pure
// point sampler. It contains no rendered colour, bed or shoreline.
type Surface struct {
	plan *plan
}

// SurfaceSample is what another material needs to put water over its own bed.
type SurfaceSample struct {
	DU, DV float64 // downstream unit direction
	Slope  float64 // signed surface slope along that direction
	Streak float64 // line-integral current texture, -1..1
	Ripple float64 // combined fine facet and current signal, -1..1
	Dapple float64 // broad tree-shadow field, 0..1
}

// NewSurface plans the pool/bend/field/clear/dappled water used by the seed-42
// riffle reference. The channel is widened beyond the frame: its direction
// and velocity remain, while banks and dry geometry cannot enter a sample.
func NewSurface(ctx sketch.Context, seed uint64) *Surface {
	ctx.Seed = seed
	rng := ctx.RNG(streamReach)
	set := defaults()
	reachLevel("pool", &set, rng)
	channelLevel("bend", &set, rng)
	bouldersLevel("field", &set, rng)
	waterLevel("clear", &set, rng)
	lightLevel("dappled", &set, rng)

	set.channelWidth = 2.4
	set.taper = 0
	set.bars = 0
	set.ledge = false
	set.caustic = 0
	set.foam = 0

	p := newPlan(ctx, set)
	// The submerged obstructions may bend the current, but they cannot lift
	// through the bed, make white water or appear as a second set of stones.
	for i := range p.rocks {
		p.rocks[i].Rise = 0
		p.rocks[i].Wake = 0
	}
	return &Surface{plan: p}
}

// At samples the flow-derived surface at one normalized coordinate.
func (s *Surface) At(u, v float64) SurfaceSample {
	p := s.plan
	r := p.read(u, v)
	w := p.upstream(u, v, r)

	// The original surface slope is fine chop. Over a detailed external bed it
	// disappears into the material, so the reusable surface also carries a
	// longer standing wave: crests cross the current, wander with broad flow
	// noise and break into shorter runs. This is still a field sampled at the
	// final pixel, not a drawn or composited ripple layer.
	warp := 0.055*p.nFlow.FBM(u/0.28+29, v/0.28-17, 2) +
		0.010*p.nRip.FBM(u/0.11-7, v/0.15+11, 1)
	downstream := v + warp
	phase := 2 * math.Pi * downstream / 0.085
	standing := 0.90*math.Sin(phase) + 0.10*math.Sin(0.53*phase+
		1.8*p.nFlow.FBM(u/0.28-31, v/0.28+23, 1))
	broken := 0.30 + 0.70*mathx.Smoothstep(-0.35, 0.45,
		p.nCaus.FBM(u/0.24-9, v/0.24+37, 2))
	wave := standing * broken

	// Fine facets keep the bands from becoming graphic stripes; LIC supplies
	// the subordinate along-current texture. The combined slope also drives
	// refraction, so the shadow and the shifted stones describe one surface.
	facet := math.Tanh(w.slope * 6)
	streak := clampUnit(w.streak * streakNorm)
	ripple := clampUnit(0.96*wave + 0.03*facet + 0.01*streak)
	slope := 0.025 * wave
	dapple := mathx.Smoothstep(-0.20, 0.50,
		p.nCaus.FBM(u/dappleWave+101, v/dappleWave-53, 2))

	return SurfaceSample{
		DU: w.dirU, DV: w.dirV,
		Slope: slope, Streak: streak,
		Ripple: ripple, Dapple: dapple,
	}
}
