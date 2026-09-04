package glaze

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// Water is a coloured medium, not a sheet of tinted plastic. What makes the
// difference at a glance is that the bed's own colour comes *through* it: the
// stones under a thin passage keep their hue and their facets, and the ones
// under a thick passage lose the red end first and sink toward the water's
// colour. That is absorption, and it is computed in linear light for the
// same reason `paint.Wash` is — multiplying sRGB takes the light away at the
// wrong rate and turns every deep passage to mud.
//
// The one deliberate untruth is where the extinction comes from. Taken
// straight from the pigment, a pale swatch barely absorbs anything and the
// veil disappears — the water would be as strong as the palette happened to
// be dark, which is nothing to do with what colour the water is. So the
// pigment's strongest channel is normalised to pass freely and only the
// *ratios* between its channels set the per-channel extinction; how much
// light the water takes overall is the separate clear term, which every
// channel pays alike. A pale sky blue therefore still filters as blue rather
// than as very slightly blue, and the depth of the water is a property of the
// water rather than of the swatch.

// filter is one water colour turned into optical constants.
type filter struct {
	// tint is the per-channel extinction that carries hue, already scaled.
	tintR, tintG, tintB float64
	// clear is the neutral extinction every channel takes: depth itself.
	clear float64
	// scatterR/G/B is the medium's own colour in linear light, and body how
	// much of it is scattered back rather than transmitted.
	scatterR, scatterG, scatterB float64
	body                         float64
}

const (
	// How hard the hue filters at full load, and how much plain darkening
	// comes with it. Tuned so that a full-load passage is unmistakably deep
	// water while a half-load one still reads every facet under it.
	tintGain  = 2.35
	clearGain = 0.55
	// A channel never passes *nothing*: a hard zero drives one channel to
	// black long before the others and the deep water goes to a flat primary.
	minTransmit = 0.05
)

func newFilter(pigment palette.Color, body float64) filter {
	r := palette.SRGBToLinear(pigment.R)
	g := palette.SRGBToLinear(pigment.G)
	b := palette.SRGBToLinear(pigment.B)
	peak := math.Max(r, math.Max(g, b))
	if peak < 1e-6 {
		peak = 1e-6
	}
	extinct := func(channel float64) float64 {
		t := math.Max(channel/peak, minTransmit)
		return tintGain * -math.Log(t)
	}
	return filter{
		tintR: extinct(r), tintG: extinct(g), tintB: extinct(b),
		clear:    clearGain,
		scatterR: r, scatterG: g, scatterB: b,
		body: body,
	}
}

// over sends the colour below through load units of water and adds what the
// water itself scatters back toward the eye.
func (f filter) over(under palette.Color, load float64) palette.Color {
	if load <= 0 {
		return under
	}
	channel := func(base, tint, scatter float64) float64 {
		transmitted := math.Exp(-load * (tint + f.clear))
		lit := palette.SRGBToLinear(base)*transmitted +
			f.body*scatter*(1-transmitted)
		return palette.LinearToSRGB(lit)
	}
	return palette.Color{
		R: channel(under.R, f.tintR, f.scatterR),
		G: channel(under.G, f.tintG, f.scatterG),
		B: channel(under.B, f.tintB, f.scatterB),
	}
}
