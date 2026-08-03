package riffle

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// The bed, and therefore the depth. Everything the eye reads as tone in this
// sketch comes from here: a pool is a dark shape and a run over gravel is a
// bright one, and the picture is composed of those shapes before it is
// composed of anything else.
//
// Land is water of negative depth. There is no second field for the banks
// and no mask anywhere: the channel's cross-section is a parabola, so it
// goes negative outside the banks on its own, and every fill downstream of
// here reads a negative depth as dry gravel.

// channel is the plan form: where the water is.
//
// The centreline is two sine harmonics rather than a noise field, and that
// is deliberate. Its *derivative* is needed at every step of every walk — it
// is the flow direction — and a noise derivative costs two more evaluations
// each time. Two harmonics with drawn amplitudes, wavelengths and phases are
// as irregular as a river needs and their slope is closed form.
type channel struct {
	centre  float64
	half    float64
	bend    float64
	meander float64
	phase   float64

	bend2, meander2, phase2 float64

	taper float64
}

// axis returns the u coordinate of the deepest line at v, and its slope
// du/dv — the direction the water is going.
func (c channel) axis(v float64) (x, slope float64) {
	w1 := 2 * math.Pi * c.meander
	w2 := 2 * math.Pi * c.meander2
	x = c.centre + c.bend*math.Sin(w1*v+c.phase) + c.bend2*math.Sin(w2*v+c.phase2)
	slope = c.bend*w1*math.Cos(w1*v+c.phase) + c.bend2*w2*math.Cos(w2*v+c.phase2)
	return x, slope
}

// halfWidth is the distance from the axis to the bank at v. A chute narrows
// downstream, which is what makes the water in it accelerate.
func (c channel) halfWidth(v float64) float64 {
	return math.Max(0.08, c.half*(1+c.taper*(v-0.5)))
}

// rock is one obstruction: a smooth dome raising the bed. A dome taller than
// the water over it breaks the surface and the rock is dry, so "submerged"
// and "emergent" are the same object at two heights rather than two kinds.
type rock struct {
	X, Y float64
	R    float64 // radius across the flow, canvas units
	Long float64 // stretch downstream; 1 is round, 4 is a gravel bar
	Rise float64 // how far it lifts the bed
	Wake float64 // white water it sheds, 0..1
	Spin float64 // circulation of the vortex pair behind it

	// DU, DV is the downstream direction at the rock, taken once when it is
	// placed. The plume behind it needs a direction and the channel's is the
	// right one; reading the axis again at every pixel of every walk would
	// be the same answer at four times the price.
	DU, DV float64
}

// bump is the rock's shape factor at a point: 1 at the centre, 0 at the rim,
// smooth at both. Squared falloff rather than a cosine because it is
// evaluated at every step of every walk.
func (r rock) bump(u, v float64) float64 {
	du := (u - r.X) / r.R
	dv := (v - r.Y) / (r.R * r.Long)
	t := du*du + dv*dv
	if t >= 1 {
		return 0
	}
	s := 1 - t
	return s * s
}

// area is the radius of a circle with the same footprint — what the flow
// field deflects around, since the dipole is a circular solution.
func (r rock) area() float64 { return r.R * math.Sqrt(r.Long) }

// reading is everything one lookup of the bed answers. The flow needs the
// across-channel position and the along-channel grade as well as the depth,
// and all three fall out of the same arithmetic, so they are returned
// together rather than computed twice.
type reading struct {
	depth float64 // negative on dry gravel
	x     float64 // across-channel: 0 on the thalweg, ±1 at the bank
	grade float64 // <1 on a riffle crest, >1 in a pool
	slope float64 // du/dv of the centreline here
	rock  float64 // strongest rock shape factor at this point, 0..1
}

// read evaluates the bed.
func (p *plan) read(u, v float64) reading {
	ax, slope := p.ch.axis(v)
	hw := p.ch.halfWidth(v)
	x := (u - ax) / hw

	// The deep line sits toward the *outside* of a bend, which is where a
	// real channel scours its pool. Skewing the parabola rather than moving
	// the axis keeps the banks where they are.
	xs := x - p.set.skew*slope
	cross := 1 - xs*xs

	// The pool–riffle sequence: the single most important structural term.
	// It is what puts a dark band across the picture and a bright one below
	// it, and it is what makes the water break somewhere in particular. The
	// phase is warped so the crossings are neither evenly spaced nor square
	// to the flow.
	ph := 2*math.Pi*v*p.set.riffleWave + p.phase +
		1.7*p.nBed.FBM(u*0.75, v*0.55, 1)
	grade := 1 - p.set.riffle*math.Cos(ph)

	d := p.set.depth * cross * grade
	d += p.set.depth * p.set.dune * p.nBed.FBM(u/duneWave, v/duneWave, 2)

	// Rocks take the *maximum*, not the sum. Two boulders that touch are one
	// lumpy dome either way, but summed they lift the bed by twice their own
	// height and a cluster tears a hole in the river — which is what a ledge
	// of five overlapping rocks did before this.
	rise, worst := 0.0, 0.0
	for i := range p.rocks {
		b := p.rocks[i].bump(u, v)
		if b <= 0 {
			continue
		}
		if h := p.rocks[i].Rise * b; h > rise {
			rise = h
		}
		if b > worst {
			worst = b
		}
	}
	return reading{depth: d - rise, x: xs, grade: grade, slope: slope, rock: worst}
}

// duneWave is the wavelength of the bed's own irregularity, in canvas units
// — a fraction of a channel width, so a riffle crest comes out as a ragged
// line rather than as a contour of a sine.
const duneWave = 0.115

// wetness is 0 on dry gravel and 1 in water, with a band between that every
// consumer shares: the flow dies there, the foam dies there, and the gravel
// dries out there. One constant so the three agree.
func wetness(depth float64) float64 { return mathx.Smoothstep(0, 0.045, depth) }

// placeRocks scatters the obstructions. Each one takes its height from the
// water actually over it, so the same draw gives a submerged boulder in a
// pool and an emergent one on a riffle — which is what a river looks like,
// and what a fixed height would not give at any value.
func (p *plan) placeRocks(rng *rand.Rand) {
	set := p.set

	// Bars first: a mid-channel bar is a rock the size of the composition,
	// and the boulders should not be planted on top of it.
	for i := 0; i < set.bars; i++ {
		side := 1.0
		if i%2 == 1 || rng.Float64() < 0.5 {
			side = -1
		}
		y := rnd.Uniform(rng, 0.25, 0.75)
		ax, _ := p.ch.axis(y)
		hw := p.ch.halfWidth(y)
		r := rock{
			X:    ax + side*rnd.Uniform(rng, 0.10, 0.42)*hw,
			Y:    y,
			R:    rnd.Uniform(rng, 0.10, 0.17) * hw / 0.75,
			Long: rnd.Uniform(rng, 2.6, 4.4),
			Wake: 0.35,
			Spin: 0.4 * set.eddy,
		}
		bare := p.bareDepth(r.X, r.Y)
		r.Rise = bare * rnd.Uniform(rng, 1.15, 1.5)
		p.rocks = append(p.rocks, p.settle(r, bare))
	}

	if set.ledge {
		// A line of rock across the channel: the strongest composition the
		// sketch makes, because it divides the frame with one white bar.
		y := rnd.Uniform(rng, 0.3, 0.7)
		n := 5 + rng.IntN(4)
		for i := range n {
			t := (float64(i)+0.5)/float64(n)*2 - 1
			ax, _ := p.ch.axis(y)
			hw := p.ch.halfWidth(y)
			r := rock{
				X:    ax + t*hw*rnd.Uniform(rng, 0.80, 1.0),
				Y:    y + rnd.Gauss(rng, 0, 0.012),
				R:    set.rockSize * rnd.Uniform(rng, 1.1, 1.9),
				Long: rnd.Uniform(rng, 0.40, 0.62),
				Wake: rnd.Uniform(rng, 0.8, 1.0) * set.wake,
				Spin: rnd.Uniform(rng, 0.7, 1.0) * set.eddy,
			}
			bare := math.Max(0.05, p.bareDepth(r.X, r.Y))
			r.Rise = bare * rnd.Uniform(rng, 1.0, 1.5)
			p.rocks = append(p.rocks, p.settle(r, bare))
		}
	}

	for range set.rocks * 12 {
		if len(p.rocks) >= set.rocks+set.bars+9 {
			break
		}
		x := rnd.Uniform(rng, 0, p.aspect)
		y := rnd.Uniform(rng, -0.03, 1.03)
		r := rock{
			X:    x,
			Y:    y,
			R:    set.rockSize * rnd.Uniform(rng, 0.6, 1.5),
			Long: rnd.Uniform(rng, 0.75, 1.35),
			Wake: rnd.Uniform(rng, 0.55, 1.0) * set.wake,
			Spin: rnd.Uniform(rng, 0.6, 1.0) * set.eddy,
		}
		bare := p.bareDepth(x, y)
		if bare < 0.05 {
			continue // already dry land; a rock there is invisible
		}
		// Roughly a third of them break the surface. Below 1 the rock is a
		// shadow under the water with a boil over it; above 1 it is dry.
		r.Rise = bare * rnd.Uniform(rng, 0.45, 1.55)
		if p.crowded(r) {
			continue
		}
		p.rocks = append(p.rocks, p.settle(r, bare))
	}
}

// settle finishes a rock: it takes the downstream direction of the channel
// it sits in, and its wake is scaled by how close it comes to the surface. A
// boulder a metre under does not make white water, and gating the plume on
// emergence is what stops a deep pool full of rocks from foaming.
func (p *plan) settle(r rock, bare float64) rock {
	_, slope := p.ch.axis(r.Y)
	inv := 1 / math.Hypot(slope, 1)
	r.DU, r.DV = slope*inv, inv
	if bare > 1e-6 {
		r.Wake *= mathx.Smoothstep(0.55, 1.05, r.Rise/bare)
	}
	return r
}

// bareDepth is the depth before any rock is placed — what a rock's height is
// measured against.
func (p *plan) bareDepth(u, v float64) float64 {
	ax, slope := p.ch.axis(v)
	hw := p.ch.halfWidth(v)
	xs := (u-ax)/hw - p.set.skew*slope
	ph := 2*math.Pi*v*p.set.riffleWave + p.phase + 1.7*p.nBed.FBM(u*0.75, v*0.55, 1)
	d := p.set.depth * (1 - xs*xs) * (1 - p.set.riffle*math.Cos(ph))
	return d + p.set.depth*p.set.dune*p.nBed.FBM(u/duneWave, v/duneWave, 2)
}

// crowded rejects a rock that would sit on top of one already placed. Rocks
// that touch merge into one lumpy dome and stop reading as separate objects.
func (p *plan) crowded(r rock) bool {
	for _, o := range p.rocks {
		if math.Hypot(r.X-o.X, r.Y-o.Y) < 1.35*(r.area()+o.area()) {
			return true
		}
	}
	return false
}
