package paint

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// Glaze is the same pigment model as Wash, evaluated per pixel instead of
// per deposit.
//
// Wash builds a mark out of ~40 near-transparent deposits and asks how many
// of them reached a pixel; the answer is a *number of layers*, and the
// colour follows from stacking that many transmittances. A sketch that
// knows analytically how much pigment reached a point — because it has a
// real distance field to its own edge rather than a stack of stamps — wants
// the same physics with the layer count handed to it directly. This is that
// function, and `load` is the layer count taken continuous.
//
// Taking the deposit thickness to zero at fixed total load turns Wash's
// per-deposit transmittance 1 − α(1 − L) into Beer–Lambert:
//
//	T = exp(−load · (1 − L))
//
// so a load of 2 is exactly two loads of 1 laid one over the other. The
// absorption coefficient is 1 − L in *linear* light for the same reason
// water.go works there (decision 13): absorption is radiometric, and
// multiplying sRGB values turns every overlap to mud.
//
// scatter is the fraction of the pigment's own masstone that comes back off
// the particles rather than passing through. Without it a heavy load
// marches to black and every overlap goes dead; with it the stack
// asymptotes to the pigment's own colour.
func Glaze(ground, pigment palette.Color, load, scatter float64) palette.Color {
	if load <= 0 {
		return ground
	}
	return palette.Color{
		R: glazeChannel(ground.R, pigment.R, load, scatter),
		G: glazeChannel(ground.G, pigment.G, load, scatter),
		B: glazeChannel(ground.B, pigment.B, load, scatter),
	}
}

// glazeChannel puts one channel of pigment over one channel of ground.
func glazeChannel(ground, pigment, load, scatter float64) float64 {
	p := palette.SRGBToLinear(pigment)
	t := math.Exp(-load * (1 - p))
	return palette.LinearToSRGB(palette.SRGBToLinear(ground)*t + scatter*p*(1-t))
}
