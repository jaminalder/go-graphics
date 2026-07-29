package palette

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
)

// HSB is a color in hue/saturation/brightness (a.k.a. HSV): hue in degrees,
// saturation and brightness in percent. It is the space colors drift in —
// hue, vividness and lightness move independently there, which they do not
// in RGB.
type HSB struct {
	H, S, B float64
}

// Color converts to sRGB.
func (c HSB) Color() Color {
	h := math.Mod(math.Mod(c.H, 360)+360, 360) / 60
	s := mathx.Clamp01(c.S / 100)
	v := mathx.Clamp01(c.B / 100)
	chroma := s * v
	second := chroma * (1 - math.Abs(math.Mod(h, 2)-1))
	var r, g, b float64
	switch {
	case h < 1:
		r, g, b = chroma, second, 0
	case h < 2:
		r, g, b = second, chroma, 0
	case h < 3:
		r, g, b = 0, chroma, second
	case h < 4:
		r, g, b = 0, second, chroma
	case h < 5:
		r, g, b = second, 0, chroma
	default:
		r, g, b = chroma, 0, second
	}
	m := v - chroma
	return Color{R: r + m, G: g + m, B: b + m}
}

// HSB converts an sRGB color to hue/saturation/brightness.
func (c Color) HSB() HSB {
	r, g, b := mathx.Clamp01(c.R), mathx.Clamp01(c.G), mathx.Clamp01(c.B)
	maxV := math.Max(r, math.Max(g, b))
	minV := math.Min(r, math.Min(g, b))
	chroma := maxV - minV

	var h float64
	switch {
	case chroma == 0:
		h = 0
	case maxV == r:
		h = math.Mod((g-b)/chroma, 6)
	case maxV == g:
		h = (b-r)/chroma + 2
	default:
		h = (r-g)/chroma + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	var s float64
	if maxV > 0 {
		s = chroma / maxV
	}
	return HSB{H: h, S: s * 100, B: maxV * 100}
}

// Swatch is a color with room to move: a base in HSB plus, per channel, a
// standard deviation and a clamp box it may never leave. Drawing repeatedly
// from one swatch yields a family of closely related colors rather than one
// flat value, and stepping from the previous draw lets a run of marks drift
// across that family instead of flickering.
//
// This is the color model QQL uses; it is what keeps a palette alive over
// tens of thousands of marks without ever going off-key.
type Swatch struct {
	Name string
	Base HSB

	HMin, HMax, HStd float64
	SMin, SMax, SStd float64
	BMin, BMax, BStd float64
}

// Color is the swatch's unperturbed base color.
func (s Swatch) Color() Color { return s.Base.Color() }

// Draw perturbs the base once — the colour a new run of marks starts from.
func (s Swatch) Draw(rng *rand.Rand) HSB { return s.Step(rng, s.Base) }

// Step takes one random walk step away from the current value and back into
// the clamp box. Because it perturbs `from` rather than the base, a sequence
// of Steps wanders the box instead of hovering around the centre.
func (s Swatch) Step(rng *rand.Rand, from HSB) HSB {
	h := clampRange(from.H+rng.NormFloat64()*s.HStd, s.HMin, s.HMax)
	h = math.Mod(math.Mod(h, 360)+360, 360)
	return HSB{
		H: h,
		S: clampRange(from.S+rng.NormFloat64()*s.SStd, s.SMin, s.SMax),
		B: clampRange(from.B+rng.NormFloat64()*s.BStd, s.BMin, s.BMax),
	}
}

func clampRange(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// SwatchAround builds a swatch centred on an existing color, with clamp box
// half-widths and standard deviations given per channel. It is how a flat
// palette color joins the swatch world.
func SwatchAround(name string, c Color, hSpan, sSpan, bSpan float64) Swatch {
	base := c.HSB()
	return Swatch{
		Name: name, Base: base,
		HMin: base.H - hSpan, HMax: base.H + hSpan, HStd: hSpan / 2,
		SMin: math.Max(0, base.S-sSpan), SMax: math.Min(100, base.S+sSpan), SStd: sSpan / 2,
		BMin: math.Max(0, base.B-bSpan), BMax: math.Min(100, base.B+bSpan), BStd: bSpan / 2,
	}
}
