package riffle

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
)

// The velocity field: a vector, not an angle, because everything downstream
// of it needs the *speed* as well as the direction — the step of the walk,
// the Froude number that decides whether water breaks, and the length of a
// streak all follow from how fast the water is moving.
//
// Four terms, each about one visible thing: the current, the deflection
// round a rock, the eddy behind it, and the shredding of the surface where
// the water is broken.

// velocity evaluates the flow at a point that has already been read.
func (p *plan) velocity(u, v float64, r reading) (fu, fv float64) {
	// The current. Direction is the channel tangent; speed is fastest mid
	// channel, dies at the bank, and rises where the pool–riffle term
	// shallows.
	inv := 1 / math.Hypot(r.slope, 1)
	du, dv := r.slope*inv, inv

	// The exponent on grade is well under 1, and that is a calibration, not
	// a physical claim: taken as a straight reciprocal a riffle crest came
	// out three times the speed of the pool above it, which drives the walk
	// most of the way across the frame in twenty steps and averages the
	// streak texture into grey. Held to a power of about a half, a riffle
	// runs half again as fast as the run it interrupts, which is what a
	// river does and what leaves a streak long enough to read.
	prof := math.Pow(mathx.Clamp01(1-r.x*r.x), 0.35)
	sp := p.set.speed * prof / math.Pow(math.Max(r.grade, 0.3), 0.55) * wetness(r.depth)
	fu, fv = du*sp, dv*sp

	// Rocks deflect and shed. The dipole is exact potential flow round a
	// cylinder; the wake behind it is not, because potential flow does not
	// have one (d'Alembert), and the wake is the whole point of a boulder.
	for i := range p.rocks {
		fu, fv = p.rocks[i].deflect(u, v, fu, fv)
	}

	// Turbulence, scaled by how broken the water already is: glassy water
	// stays glassy and a riffle is shredded. Curl noise rather than an angle
	// field because it is divergence-free — an angle field has sinks, and
	// streamlines that all arrive at the same place read as hair.
	if p.set.turbulence > 0 {
		t := p.set.turbulence * sp * turbGate(sp, r.depth)
		if t > 0 {
			c1u, c1v := p.nFlow.Curl(u/swirlWave, v/swirlWave, 0)
			c2u, c2v := p.nFlow.Curl(u/chopWave+13.7, v/chopWave-4.1, 0)
			fu += t * (0.86*c1u + 0.14*c2u) * curlNorm
			fv += t * (0.86*c1v + 0.14*c2v) * curlNorm
		}
	}
	return fu, fv
}

// Wavelengths of the two turbulence scales, canvas units: one that swings a
// whole strand of current, one that chops it up.
//
// The fine one is deliberately a small share of the total. Curl noise returns
// a gradient in *noise-space* units, so both scales come back at much the
// same amplitude while the fine one has four times the spatial frequency and
// therefore four times the shear. Given equal weight it curls every
// streamline into a closed eddy a thirtieth of the frame across, and a
// convolution along closed eddies is a field of cells — the picture came out
// as tooled leather rather than as a river. The broad scale swings a whole
// strand of current, which is what a streak wants to be.
const (
	swirlWave = 0.14
	chopWave  = 0.034
	// curlNorm brings the curl of a unit-amplitude fBm — a gradient of
	// roughly 2 in noise-space units — back to order 1.
	curlNorm = 0.5
)

// turbGate is the Froude number rolled into a 0..1 gate: water breaks up
// when it is moving fast for its depth, and that single expression is what
// makes a riffle break on its crest instead of everywhere.
func turbGate(speed, depth float64) float64 {
	return mathx.Smoothstep(0.5, 1.9, froude(speed, depth))
}

// froude is speed/√depth — the textbook criterion for breaking, and the
// reason this sketch needs no separate threshold on depth or on speed.
func froude(speed, depth float64) float64 {
	return speed / math.Sqrt(math.Max(depth, 0.035))
}

// deflect adds this rock's contribution to a flow vector.
//
// The dipole term is the exact solution for a cylinder in a uniform stream,
// rotated into the incident direction rather than assumed to point down the
// page: water slows to a stagnation point on the upstream face and speeds up
// past the shoulders. The rotation is done with the incident vector's own
// components, so there is no trigonometry in the inner loop.
func (r rock) deflect(u, v, fu, fv float64) (float64, float64) {
	a := r.area()
	dx, dy := u-r.X, v-r.Y
	rr := dx*dx + dy*dy
	reach := 9 * a * a
	if rr > reach {
		return fu, fv
	}
	sp := math.Hypot(fu, fv)
	if sp < 1e-9 {
		return fu, fv
	}
	if rr < a*a {
		rr = a * a // inside the rock the field is held finite; it is dry anyway
	}

	// Rotate the offset into the frame where the flow runs along +x.
	cx, cy := fu/sp, fv/sp
	px := dx*cx + dy*cy
	py := -dx*cy + dy*cx

	k := -sp * a * a / (rr * rr)
	qx := k * (px*px - py*py)
	qy := k * (2 * px * py)

	fu += qx*cx - qy*cy
	fv += qx*cy + qy*cx

	// The wake: a counter-rotating pair planted a radius and a half
	// downstream, a radius apart. Between them the water runs back upstream,
	// which is exactly what it does behind a rock and why foam sits there
	// instead of leaving.
	if r.Spin > 0 {
		wx, wy := r.X+cx*1.5*a, r.Y+cy*1.5*a
		nx, ny := -cy, cx
		g := r.Spin * sp * a
		for s := -1.0; s <= 1.0; s += 2 {
			vx, vy := wx+s*nx*0.9*a, wy+s*ny*0.9*a
			ex, ey := u-vx, v-vy
			d2 := ex*ex + ey*ey
			fall := math.Exp(-d2 / (5 * a * a))
			c := -s * g * fall / (d2 + 0.35*a*a)
			fu += c * -ey
			fv += c * ex
		}
	}
	return fu, fv
}

// walk is what one pixel learns by looking upstream.
type walk struct {
	streak float64 // line integral convolution of the surface texture
	slope  float64 // the surface's derivative along the flow
	foam   float64 // foam born upstream and carried here
	speed  float64 // at the pixel itself
	dirU   float64
	dirV   float64
}

// upstream walks the streamline backwards from a pixel, accumulating
// everything that arrived with the water.
//
// The step is velocity × time, not a fixed arc length. That one choice is
// what makes streaks long on a fast tongue and short in slack water, and it
// is what makes the walk stall in an eddy so foam piles up there.
func (p *plan) upstream(u, v float64, r reading) walk {
	var w walk
	qu, qv, qr := u, v, r

	var tex, texN float64
	var foam, foamN float64

	for i := range p.set.steps {
		if i > 0 {
			qr = p.read(qu, qv)
		}
		fu, fv := p.velocity(qu, qv, qr)
		sp := math.Hypot(fu, fv)

		t := p.texture(qu, qv)
		if i == 0 {
			w.speed = sp
			if sp > 1e-9 {
				w.dirU, w.dirV = fu/sp, fv/sp
			} else {
				w.dirV = 1
			}
		}
		// The convolution runs the whole walk. Its step is a fifth of the
		// texture's own wavelength, which is what makes the average a smear
		// rather than a mean of independent samples — at a step near the
		// wavelength the sum is white noise and the surface goes flat.
		tex += t
		texN++
		fw := math.Exp(-float64(i) / p.set.foamLife)
		foam += fw * p.foamSource(qu, qv, qr, sp)
		foamN += fw

		qu -= fu * p.set.step
		qv -= fv * p.set.step
	}

	if texN > 0 {
		w.streak = tex / texN
	}
	if foamN > 0 {
		w.foam = foam / foamN
	}
	w.slope = p.rippleSlope(u, v, w.dirU, w.dirV, w.speed, r.depth)
	return w
}

// texture is the field the convolution smears: fine, isotropic, and with no
// structure of its own, so every streak in the picture is the flow's.
func (p *plan) texture(u, v float64) float64 {
	return p.nTex.FBM(u/texWave, v/texWave, 1)
}

// texWave is the streak width, canvas units. It has to be several times the
// walk's step: the convolution is a smear along the streamline, and a smear
// needs its samples correlated.
const texWave = 0.017

// rippleSlope is the surface's derivative along the flow, from its own fine
// field rather than from the convolution's.
//
// The first version took it as the difference between the walk's first two
// samples, which is free and wrong. Those samples are a *streak* wavelength
// apart, so the ripples came out at the streak's own scale — long dashes,
// arranged in rows by the convolution, which read as basketry rather than as
// water. Ripples and streaks are two different scales of the same surface,
// and they need two fields: this one is fine, and the shading of it is what
// makes chop; the convolution is coarse and long, and it is what makes
// current.
//
// One-sided along the flow, because a riffle's standing waves have their
// crests across it and nearly all the slope is in that one direction.
func (p *plan) rippleSlope(u, v, du, dv, speed, depth float64) float64 {
	if p.set.chop <= 0 {
		return 0
	}
	const h = ripWave * 0.3
	a := p.nRip.FBM(u/ripWave, v/ripWave, 1)
	b := p.nRip.FBM((u-du*h)/ripWave, (v-dv*h)/ripWave, 1)
	// Gated on the Froude number: a glide is glass and a riffle is broken,
	// and one chop for both is what makes a pool look like corrugated iron.
	gate := 0.22 + 0.78*turbGate(speed, depth)
	return p.set.chop * gate * (a - b) / h
}

// ripWave is the chop's wavelength, canvas units — a fifth of a streak.
const ripWave = 0.020
