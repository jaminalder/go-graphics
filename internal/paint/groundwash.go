package paint

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// Ground floods the whole canvas with a wash of col, at a density that
// varies the way a flat wash does when it dries: blotchy at a broad scale
// where the pigment pooled unevenly, speckled at the paper's own scale
// where its tooth caught the pigment.
//
// It is deliberately not a pool, and not a set of them. A pool is a shape,
// and a shape has a boundary; laid in a grid to cover a sheet, those
// boundaries are visible as fine arcs across the ground however soft they
// are made, and on a picture full of circles they read as more circles.
// Here there is nothing to have an edge — the density is a field evaluated
// per pixel, so the only structure in the ground is the structure that was
// asked for.
//
// The pigment behaves exactly as it does in a pool: absorption in linear
// light, with the same back-scatter floor and the same Body, so a ground
// and a mark laid in one colour agree.
//
// strength is the wash's nominal density, blotch the wavelength of its
// unevenness in canvas units. Layers, Ragged and Edge play no part — a
// sheet-wide wash has no edge to be ragged and no rim to dry into.
func (w Wash) Ground(c *Canvas, col palette.Color, strength, blotch float64) {
	if strength <= 0 {
		return
	}
	blotch = math.Max(blotch, 1e-3)
	n := noise.New(w.Seed)

	// One "layer" of pigment at the nominal strength; the field below
	// varies how much of it landed on each pixel.
	ar := math.Log(transmit(col.R, mathx.Clamp01(strength)))
	ag := math.Log(transmit(col.G, mathx.Clamp01(strength)))
	ab := math.Log(transmit(col.B, mathx.Clamp01(strength)))

	floor := w.Scatter + (1-w.Scatter)*w.Body
	fr := floor * palette.SRGBToLinear(col.R)
	fg := floor * palette.SRGBToLinear(col.G)
	fb := floor * palette.SRGBToLinear(col.B)

	cell := math.Max(w.GrainScale*c.scale, 2.5)

	for y := range c.h {
		v := (float64(y) + 0.5) / c.scale
		for x := range c.w {
			u := (float64(x) + 0.5) / c.scale

			d := 1.0
			if w.Mottle > 0 {
				// Three octaves: the broad unevenness of a wash that dried
				// heavier in some places, with a little structure inside it.
				// More octaves only add detail the tooth already supplies.
				d += w.Mottle * n.FBM(u/blotch, v/blotch, 3) / 1.1
			}
			if w.Grain > 0 {
				d *= 1 + w.Grain*(2*tooth(w.Seed, float64(x), float64(y), cell)-1)
			}
			if d <= 0 {
				continue
			}

			i := y*c.w + x
			p := c.pix[i]
			c.pix[i] = palette.Color{
				R: palette.LinearToSRGB(layer(p.R, math.Exp(d*ar), fr)),
				G: palette.LinearToSRGB(layer(p.G, math.Exp(d*ag), fg)),
				B: palette.LinearToSRGB(layer(p.B, math.Exp(d*ab), fb)),
			}
		}
	}
}
