package noise

import "math"

// Curl returns the curl of the fBm field treated as a stream function:
// the vector (∂ψ/∂y, −∂ψ/∂x) at (x, y).
//
// The result is divergence-free, which is the reason to prefer it over
// reading an angle straight out of the noise. An angle field has sources
// and sinks, so anything walking it drains into the sinks and piles up
// there; a divergence-free field can only circulate, so walkers form
// closed eddies and spread evenly. The returned vector is not normalised
// — its length is the local flow speed.
func (p *Perlin) Curl(x, y float64, octaves int) (vx, vy float64) {
	dpdy := p.FBM(x, y+curlEps, octaves) - p.FBM(x, y-curlEps, octaves)
	dpdx := p.FBM(x+curlEps, y, octaves) - p.FBM(x-curlEps, y, octaves)
	return dpdy / (2 * curlEps), -dpdx / (2 * curlEps)
}

// curlEps is the central-difference step, in noise-space units — a
// hundredth of a lattice cell. Sampling both components with the same
// step is what makes the discrete field exactly divergence-free rather
// than merely approximately so: the four corner samples cancel in pairs.
const curlEps = 0.01

// CurlAngle is Curl reduced to a direction in radians.
func (p *Perlin) CurlAngle(x, y float64, octaves int) float64 {
	vx, vy := p.Curl(x, y, octaves)
	return math.Atan2(vy, vx)
}

// Ridged returns ridged fractal noise in [0, 1]: 1 − |fBm|, which folds
// the field at its zero crossings so the maxima form sharp filaments
// instead of smooth hills.
func (p *Perlin) Ridged(x, y float64, octaves int) float64 {
	v := math.Abs(p.FBM(x, y, octaves)) / 1.1
	if v > 1 {
		v = 1
	}
	return 1 - v
}
