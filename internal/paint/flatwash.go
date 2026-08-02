package paint

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// A flat wash, evaluated at a point rather than stamped as a shape.
//
// Ground floods a whole canvas this way and says why: a pool is a shape,
// and a shape has a boundary, so pools laid to cover an area leave their
// boundaries legible as fine arcs however soft they are made. A field has
// nothing to have an edge.
//
// The same argument answers a harder question — how to paint a *region*
// that is not round. Covering it with pools inherits every one of their
// boundaries, and no amount of care with where they go removes them; the
// shapes are simply the wrong primitive for filling a shape. Evaluated per
// point instead, the caller decides where the paint stops, so the fill is
// exact to whatever region it is answering for and the wash contributes
// only its *character*: the broad unevenness of pigment that pooled as it
// dried, and the speckle of the paper's tooth.
//
// It is a simpler thing than a pool — no stack of deposits, no ragged
// outline, no rim of its own — and that is the point. What it keeps is the
// part that reads as watercolour at a glance: transparency, absorption in
// linear light so two washes over each other make a believable third
// colour, and a surface that is never quite even.

// FlatWash is a wash that fills whatever its caller says it fills.
type FlatWash struct {
	// Blotch is the wavelength of the pigment's pooling, in canvas units.
	Blotch float64
	// Mottle is how strongly it pools; 0 is a perfectly even wash.
	Mottle float64
	// Grain is how much the paper's tooth catches the pigment.
	Grain float64
	// Tooth is the size of that tooth, in canvas units.
	Tooth float64
	// Scatter is light scattered back off the pigment rather than through
	// it, and Body the pigment's own opacity: 0 is a pure glaze.
	Scatter, Body float64
	// Seed fixes the fields.
	Seed uint64

	n *noise.Perlin
}

// NewFlatWash returns a wash on cold-pressed paper, ready to use.
func NewFlatWash(seed uint64) FlatWash {
	w := FlatWash{
		Blotch:  0.26,
		Mottle:  0.5,
		Grain:   0.22,
		Tooth:   0.0032,
		Scatter: 0.14,
		Seed:    seed,
	}
	w.n = noise.New(seed)
	return w
}

// Over lays a wash of col at the given strength over what is already at
// (u, v), and returns the result.
//
// strength is the density of pigment: 0 leaves the ground alone, 1 is a
// full-strength wash. Laying two washes over each other multiplies their
// transmittances, which is why a crossing reads as a third colour rather
// than as whichever went on last.
func (w FlatWash) Over(under, col palette.Color, strength, u, v float64) palette.Color {
	if strength <= 0 {
		return under
	}
	d := w.density(u, v)
	if d <= 0 {
		return under
	}
	s := mathx.Clamp01(strength)
	ar := math.Log(transmit(col.R, s))
	ag := math.Log(transmit(col.G, s))
	ab := math.Log(transmit(col.B, s))

	floor := w.Scatter + (1-w.Scatter)*w.Body
	return palette.Color{
		R: palette.LinearToSRGB(layer(under.R, math.Exp(d*ar), floor*palette.SRGBToLinear(col.R))),
		G: palette.LinearToSRGB(layer(under.G, math.Exp(d*ag), floor*palette.SRGBToLinear(col.G))),
		B: palette.LinearToSRGB(layer(under.B, math.Exp(d*ab), floor*palette.SRGBToLinear(col.B))),
	}
}

// density is how much of the nominal pigment landed at a point: broad
// pooling times the paper's tooth.
//
// Both are measured in canvas units, not pixels, so the same seed gives the
// same picture at any output size — Ground works from the pixel grid because
// it is walking one, and copying that here would make a print disagree with
// its preview.
func (w FlatWash) density(u, v float64) float64 {
	d := 1.0
	if w.Mottle > 0 && w.n != nil {
		b := math.Max(w.Blotch, 1e-3)
		d += w.Mottle * w.n.FBM(u/b, v/b, 3) / 1.1
	}
	if w.Grain > 0 {
		t := math.Max(w.Tooth, 1e-5)
		d *= 1 + w.Grain*(2*tooth(w.Seed, u, v, t)-1)
	}
	return d
}
