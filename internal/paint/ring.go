package paint

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// Ring stamps an anti-aliased elliptical annulus centred on (u, v): the band
// of the given thickness straddling the ellipse with semi-axes rx and ry, so
// it spans radius r ± thickness/2. A thickness of 2·r or more closes the
// hole, leaving a filled ellipse of semi-axes r + thickness/2.
//
// Unlike Stroke it does not walk a dab train — each row's spans are solved
// directly, which is what makes tens of thousands of rings affordable at
// print size. Coverage uses the same ~1px soft edge as Dab, so rings and
// dabs sit together cleanly, and a sub-pixel ring still deposits
// proportionally rather than dropping out.
func (c *Canvas) Ring(u, v, rx, ry, thickness float64, col palette.Color, alpha float64) {
	px, py := u*c.scale, v*c.scale
	prx, pry := rx*c.scale, ry*c.scale
	half := thickness * c.scale / 2
	if prx <= 0 || pry <= 0 || half <= 0 || alpha <= 0 {
		return
	}

	rMin, rMax := math.Min(prx, pry), math.Max(prx, pry)
	// Reach of the anti-aliased band. Distance to the outline is measured
	// perpendicular to it, which for an eccentric ellipse is shorter than
	// the distance measured radially — by at most rMin/rMax — so the
	// circular envelope has to be widened by that factor to be sure of
	// covering the whole band.
	e := (half + 0.5) * rMax / rMin
	outer := rMax + e
	inner := math.Max(rMin-e, 0)

	y0 := max(int(math.Floor(py-outer)), 0)
	y1 := min(int(math.Ceil(py+outer)), c.h-1)
	band := ringBand{
		cx: px, invRx2: 1 / (prx * prx), invRy2: 1 / (pry * pry),
		rMin: rMin, half: half, col: col, alpha: alpha,
	}

	for y := y0; y <= y1; y++ {
		band.dy = float64(y) + 0.5 - py
		outSpan := outer*outer - band.dy*band.dy
		if outSpan <= 0 {
			continue
		}
		xOut := math.Sqrt(outSpan)
		xIn := 0.0
		if inSpan := inner*inner - band.dy*band.dy; inSpan > 0 {
			xIn = math.Sqrt(inSpan)
		}

		if xIn > 0 {
			c.ringSpan(y, px-xOut, px-xIn, band)
			c.ringSpan(y, px+xIn, px+xOut, band)
			continue
		}
		c.ringSpan(y, px-xOut, px+xOut, band)
	}
}

// ringBand is the per-row state Ring hoists out of the pixel loop.
type ringBand struct {
	cx, dy         float64
	invRx2, invRy2 float64
	rMin, half     float64
	col            palette.Color
	alpha          float64
}

// ringSpan blends one horizontal run of a ring.
func (c *Canvas) ringSpan(y int, from, to float64, b ringBand) {
	x0 := max(int(math.Floor(from)), 0)
	x1 := min(int(math.Ceil(to)), c.w-1)
	dy2 := b.dy * b.dy
	qy, gy := dy2*b.invRy2, dy2*b.invRy2*b.invRy2
	for x := x0; x <= x1; x++ {
		dx := float64(x) + 0.5 - b.cx
		dx2 := dx * dx
		// Signed distance to the ellipse outline, perpendicular to it.
		// k is 1 exactly on the outline and g is half the gradient of k²;
		// k(k-1)/g reduces to (radius − r) for a circle and stays finite
		// all the way to the centre, so fat rings fill instead of hollowing.
		k := math.Sqrt(dx2*b.invRx2 + qy)
		g := math.Sqrt(dx2*b.invRx2*b.invRx2 + gy)
		d := -b.rMin
		if g > 0 {
			d = k * (k - 1) / g
		}
		a := b.alpha * mathx.Clamp01(b.half-math.Abs(d)+0.5)
		if a <= 0 {
			continue
		}
		i := y*c.w + x
		p := c.pix[i]
		c.pix[i] = palette.Color{
			R: p.R*(1-a) + b.col.R*a,
			G: p.G*(1-a) + b.col.G*a,
			B: p.B*(1-a) + b.col.B*a,
		}
	}
}
