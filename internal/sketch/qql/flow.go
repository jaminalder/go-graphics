package qql

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The flow field is the piece's skeleton. Every dot sits on a line traced
// through it, so the field is what turns a pile of circles into a structure
// — and because the field is built from a few large gestures rather than
// from noise, neighbouring lines stay related and larger forms emerge that
// nobody wrote down.

// fieldStep is the spacing of the field's samples, as a fraction of the
// canvas height, and also the distance a flow line advances per step.
const fieldStep = 0.002

// fieldOverscan is how far past the canvas the field extends, so lines can
// enter and leave the frame instead of starting and stopping at its edge.
const fieldOverscan = 0.2

// flowField is a grid of directions. A constant field carries no grid at
// all — a linear piece with no turbulence is one angle everywhere.
type flowField struct {
	f          frame
	lx, ty     float64
	spc        float64
	cols, rows int
	theta      []float64 // nil for a constant field
	constant   float64
}

func newFlowField(tr trait.Set, spec flowFieldSpec, f frame, rng *rand.Rand) *flowField {
	spc := f.h(fieldStep)
	ff := &flowField{
		f:   f,
		lx:  f.w(-fieldOverscan),
		ty:  f.h(-fieldOverscan),
		spc: spc,
	}
	ff.cols = int(math.Ceil((f.w(1+fieldOverscan) - ff.lx) / spc))
	ff.rows = int(math.Ceil((f.h(1+fieldOverscan) - ff.ty) / spc))

	disturbances := newDisturbances(tr, f, rng)

	if spec.kind == flowLinear {
		ff.constant = spec.defaultTheta
		if len(disturbances) == 0 {
			return ff // no grid needed
		}
		ff.theta = make([]float64, ff.cols*ff.rows)
		for i := range ff.theta {
			ff.theta[i] = spec.defaultTheta
		}
	} else {
		ff.theta = ff.radial(spec, rng)
	}

	ff.disturb(disturbances)
	return ff
}

// radial builds a field of rays, circles, or the spirals in between: the
// angle to the centre, turned by a fixed rotation. Circularity 0 leaves the
// vectors pointing straight out of the centre; circularity 1 turns them a
// quarter turn so they close into orbits.
func (ff *flowField) radial(spec flowFieldSpec, rng *rand.Rand) []float64 {
	f := ff.f
	rot := spec.circularity / 2
	if spec.outward {
		rot = 1 - rot
	}
	if spec.clockwise {
		rot = 2 - rot
	}
	rot = pi(rot)

	// The centre is usually one of a few telling positions — dead centre,
	// a third in, off the edge — and sometimes anywhere at all.
	cx := rnd.Pick(rng, []rnd.Weighted[float64]{
		{V: rnd.Uniform(rng, f.w(0), f.w(1)), W: 2},
		{V: f.w(-2.0 / 3.0), W: 0.5},
		{V: f.w(-1.0 / 3.0), W: 1},
		{V: f.w(0), W: 1},
		{V: f.w(1.0 / 3.0), W: 1.5},
		{V: f.w(1.0 / 2.0), W: 1.5},
		{V: f.w(2.0 / 3.0), W: 1.5},
		{V: f.w(1), W: 1.5},
		{V: f.w(4.0 / 3.0), W: 1},
		{V: f.w(5.0 / 3.0), W: 0.5},
	})
	cy := rnd.Pick(rng, []rnd.Weighted[float64]{
		{V: rnd.Uniform(rng, f.h(0), f.h(1)), W: 2},
		{V: f.h(-2.0 / 3.0), W: 0.5},
		{V: f.h(-1.0 / 3.0), W: 1},
		{V: f.h(0), W: 1},
		{V: f.h(1.0 / 3.0), W: 1.5},
		{V: f.h(1.0 / 2.0), W: 1.5},
		{V: f.h(2.0 / 3.0), W: 1.5},
		{V: f.h(1), W: 1},
		{V: f.h(4.0 / 3.0), W: 1},
		{V: f.h(5.0 / 3.0), W: 0.5},
	})

	theta := make([]float64, ff.cols*ff.rows)
	for i := 0; i < ff.cols; i++ {
		x := ff.lx + ff.spc*float64(i)
		for j := 0; j < ff.rows; j++ {
			y := ff.ty + ff.spc*float64(j)
			theta[i*ff.rows+j] = angle(x, y, cx, cy) + rot
		}
	}
	return theta
}

// at samples the field, or reports false when the point has left it.
func (ff *flowField) at(x, y float64) (float64, bool) {
	if x < ff.lx || y < ff.ty {
		return 0, false
	}
	i := int((x - ff.lx) / ff.spc)
	j := int((y - ff.ty) / ff.spc)
	if i >= ff.cols || j >= ff.rows {
		return 0, false
	}
	if ff.theta == nil {
		return ff.constant, true
	}
	return ff.theta[i*ff.rows+j], true
}

// disturbance is one soft eddy in the field: a rotation at its centre
// falling linearly to nothing at its rim.
type disturbance struct {
	cx, cy float64
	theta  float64
	radius float64
}

func newDisturbances(tr trait.Set, f frame, rng *rand.Rand) []disturbance {
	var count int
	var thetaSpan float64
	switch tr.Get(dimTurbulence) {
	case "low":
		count = rnd.Pick(rng, []rnd.Weighted[int]{{V: 10, W: 2}, {V: 15, W: 3}, {V: 20, W: 2}, {V: 30, W: 1}})
		thetaSpan = rnd.Pick(rng, []rnd.Weighted[float64]{{V: pi(0.005), W: 1}, {V: pi(0.01), W: 1}})
	case "high":
		count = rnd.Pick(rng, []rnd.Weighted[int]{{V: 20, W: 1}, {V: 30, W: 2}, {V: 40, W: 3}, {V: 50, W: 2}, {V: 60, W: 1}})
		thetaSpan = rnd.Pick(rng, []rnd.Weighted[float64]{{V: pi(0.05), W: 1}, {V: pi(0.1), W: 1}, {V: pi(0.15), W: 1}})
	default: // none
		return nil
	}

	lx, rx := f.w(-fieldOverscan), f.w(1+fieldOverscan)
	ty, by := f.h(-fieldOverscan), f.h(1+fieldOverscan)
	out := make([]disturbance, count)
	for i := range out {
		out[i] = disturbance{
			cx:     rnd.Uniform(rng, lx, rx),
			cy:     rnd.Uniform(rng, ty, by),
			theta:  rnd.Gauss(rng, 0, thetaSpan),
			radius: math.Max(math.Abs(rnd.Gauss(rng, f.w(0.35), f.w(0.35))), f.w(0.1)),
		}
	}
	return out
}

func (ff *flowField) disturb(ds []disturbance) {
	for _, d := range ds {
		i0 := max(int(math.Floor((d.cx-d.radius-ff.lx)/ff.spc)), 0)
		i1 := min(int(math.Ceil((d.cx+d.radius-ff.lx)/ff.spc)), ff.cols-1)
		j0 := max(int(math.Floor((d.cy-d.radius-ff.ty)/ff.spc)), 0)
		j1 := min(int(math.Ceil((d.cy+d.radius-ff.ty)/ff.spc)), ff.rows-1)
		for i := i0; i <= i1; i++ {
			x := ff.lx + ff.spc*float64(i)
			for j := j0; j <= j1; j++ {
				y := ff.ty + ff.spc*float64(j)
				ff.theta[i*ff.rows+j] += mathx.Rescale(dist(d.cx, d.cy, x, y), 0, d.radius, d.theta, 0)
			}
		}
	}
}

// pt is a point in canvas units.
type pt struct{ X, Y float64 }

// tracer walks start points into flow lines, reusing one buffer: a dense
// piece traces millions of points and none of them outlive the packing test
// that consumes them.
type tracer struct {
	field  *flowField
	length int
	step   float64
	buf    []pt
}

func newTracer(ff *flowField, f frame, rng *rand.Rand) *tracer {
	length := rnd.Choice(rng, []int{500, 650, 850})
	return &tracer{
		field:  ff,
		length: length,
		step:   f.w(fieldStep),
		buf:    make([]pt, 0, length),
	}
}

// trace follows the field from a start point until the line leaves the
// field or runs out of length. When ignore is set the line runs straight in
// the field's default direction instead.
func (t *tracer) trace(x, y float64, ignore bool, defaultTheta float64) []pt {
	t.buf = t.buf[:0]
	for range t.length {
		theta, ok := t.field.at(x, y)
		if !ok {
			break
		}
		if ignore {
			theta = defaultTheta
		}
		t.buf = append(t.buf, pt{x, y})
		x += t.step * math.Cos(theta)
		y += t.step * math.Sin(theta)
	}
	return t.buf
}
