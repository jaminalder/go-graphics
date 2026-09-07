// Package flame is the fractal-flame mechanism: a weighted IFS of affine
// transforms composed with named nonlinear variations, iterated as a chaos
// game into a density histogram, then shown with log-density tone mapping.
// Optional xaos (relative weights), density estimation, and oversample
// with a Gaussian spatial downsample follow flam3.
//
// The sketch owns genomes, palette mapping and camera taste. This package
// owns the published algorithm and its performance constraints. See
// docs/reference/fractal-flames.md and docs/reference/flame-xaos-de.md.
package flame

import (
	"math"
	"math/rand/v2"
)

// Variation identifiers match Draves & Reckase's catalog numbers.
const (
	Linear uint8 = iota
	Sinusoidal
	Spherical
	Swirl
	Horseshoe
	Polar
	Handkerchief
	Heart
	Disc
	Spiral
	Hyperbolic
	Diamond
	Ex
	Julia
	Bent
	Waves
	Fisheye
	Popcorn
	Exponential
	Power
	Cosine
	Eyefish  = 27
	Bubble   = 28
	Cylinder = 29
	Curl     = 39
	JulianN  = 42 // flam3 plugin: power/dist Julia, the filigree engine
)

// Var is one weighted nonlinear map inside an xform.
type Var struct {
	Kind   uint8
	Weight float64
	P      [4]float64 // parametric coefficients (blob, pdj, curl, …)
}

func applyVar(v Var, x, y float64, xf Xform, rng *rand.Rand) (float64, float64) {
	r := math.Hypot(x, y)
	if r < 1e-20 {
		r = 1e-20
	}
	r2 := x*x + y*y
	if r2 < 1e-40 {
		r2 = 1e-40
	}
	theta := math.Atan2(x, y) // paper: θ = arctan(x/y)

	switch v.Kind {
	case Linear:
		return x, y
	case Sinusoidal:
		return math.Sin(x), math.Sin(y)
	case Spherical:
		s := 1 / r2
		return s * x, s * y
	case Swirl:
		s, c := math.Sincos(r2)
		return x*s - y*c, x*c + y*s
	case Horseshoe:
		inv := 1 / r
		return inv * (x - y) * (x + y), inv * 2 * x * y
	case Polar:
		return theta / math.Pi, r - 1
	case Handkerchief:
		return r * math.Sin(theta+r), r * math.Cos(theta-r)
	case Heart:
		s, c := math.Sincos(theta * r)
		return r * s, -r * c
	case Disc:
		t := theta / math.Pi
		s, c := math.Sincos(math.Pi * r)
		return t * s, t * c
	case Spiral:
		inv := 1 / r
		s, c := math.Sincos(theta)
		sr, cr := math.Sincos(r)
		return inv * (c + sr), inv * (s - cr)
	case Hyperbolic:
		s, c := math.Sincos(theta)
		return s / r, r * c
	case Diamond:
		st, ct := math.Sincos(theta)
		sr, cr := math.Sincos(r)
		return st * cr, ct * sr
	case Ex:
		p0 := math.Sin(theta + r)
		p1 := math.Cos(theta - r)
		p0c, p1c := p0*p0*p0, p1*p1*p1
		return r * (p0c + p1c), r * (p0c - p1c)
	case Julia:
		omega := 0.0
		if rng.Float64() < 0.5 {
			omega = math.Pi
		}
		sr := math.Sqrt(r)
		s, c := math.Sincos(theta/2 + omega)
		return sr * c, sr * s
	case Bent:
		ox, oy := x, y
		if x < 0 {
			ox = 2 * x
		}
		if y < 0 {
			oy = y / 2
		}
		return ox, oy
	case Waves:
		c2 := xf.C * xf.C
		f2 := xf.F * xf.F
		if math.Abs(c2) < 1e-12 {
			c2 = 1e-12
		}
		if math.Abs(f2) < 1e-12 {
			f2 = 1e-12
		}
		return x + xf.B*math.Sin(y/c2), y + xf.E*math.Sin(x/f2)
	case Fisheye:
		s := 2 / (r + 1)
		return s * y, s * x
	case Popcorn:
		return x + xf.C*math.Sin(math.Tan(3*y)), y + xf.F*math.Sin(math.Tan(3*x))
	case Exponential:
		e := math.Exp(x - 1)
		s, c := math.Sincos(math.Pi * y)
		return e * c, e * s
	case Power:
		st, ct := math.Sincos(theta)
		rp := math.Pow(r, st)
		return rp * ct, rp * st
	case Cosine:
		return math.Cos(math.Pi*x) * math.Cosh(y), -math.Sin(math.Pi*x) * math.Sinh(y)
	case Eyefish:
		s := 2 / (r + 1)
		return s * x, s * y
	case Bubble:
		s := 4 / (r2 + 4)
		return s * x, s * y
	case Cylinder:
		return math.Sin(x), y
	case Curl:
		p1, p2 := v.P[0], v.P[1]
		t1 := 1 + p1*x + p2*(x*x-y*y)
		t2 := p1*y + 2*p2*x*y
		den := t1*t1 + t2*t2
		if den < 1e-20 {
			den = 1e-20
		}
		inv := 1 / den
		return inv * (x*t1 + y*t2), inv * (y*t1 - x*t2)
	case JulianN:
		power, dist := v.P[0], v.P[1]
		if power == 0 {
			power = 2
		}
		if dist == 0 {
			dist = 1
		}
		absP := math.Abs(power)
		k := 0
		if rng != nil && absP >= 1 {
			k = rng.IntN(int(math.Floor(absP)))
		}
		// Plugin convention: θ = atan2(y, x). Recursive branches are
		// (θ + 2πk) / power; radius is r^(dist/power).
		a := (math.Atan2(y, x) + 2*math.Pi*float64(k)) / power
		rr := math.Pow(r, dist/power)
		s, c := math.Sincos(a)
		return rr * c, rr * s
	default:
		return x, y
	}
}
