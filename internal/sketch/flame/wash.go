package flame

import (
	"math"

	fl "github.com/jaminalder/go-graphics/internal/flame"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// saltWash keys FlatWash mottling so it does not share a lattice with
// anything else derived from the render seed.
const saltWash = 0x57415348 // 'WASH'

// paperGround is the wash medium's ground. Ember may sit on void or dusk;
// wash is pigment on paper — no gleam, no nebula defaults.
var paperGround = palette.Color{R: 0.93, G: 0.9, B: 0.84}

// developWash interprets the attractor measure as pigment load on paper.
// Same Measure as ember; FlatWash supplies pooling and tooth. No gleam.
// Only manner stain exists today; when rimmed/pooled land they branch here.
func developWash(hist *fl.Hist, bright float64, est fl.Estimate, seed uint64, washSat float64) []palette.Color {
	if washSat <= 0 {
		washSat = 1
	}
	d := hist.Measure(bright, est)
	wash := stainWash(seed, washSat)

	out := make([]palette.Color, d.W*d.H)
	scale := 1 / float64(d.H)
	// Filament recipes sit at low log-density; a steep pow leaves them as
	// ghosts on paper. Softer gamma + sat-linked gain puts pigment down.
	pow := 0.55
	loadGain := 1.05 + 0.28*math.Max(washSat-1, 0)
	for y := 0; y < d.H; y++ {
		v := (float64(y) + 0.5) * scale
		for x := 0; x < d.W; x++ {
			i := y*d.W + x
			load := d.Load[i]
			if load <= 0 {
				out[i] = paperGround
				continue
			}
			u := (float64(x) + 0.5) * scale
			pigment := enrichPigment(palette.Color{R: d.R[i], G: d.G[i], B: d.B[i]}, washSat)
			strength := math.Pow(mathx.Clamp01(load), pow) * loadGain
			out[i] = wash.Over(paperGround, pigment, mathx.Clamp01(strength), u, v)
		}
	}
	return out
}

func stainWash(seed uint64, washSat float64) paint.FlatWash {
	w := paint.NewFlatWash(seed ^ saltWash)
	w.Blotch = 0.2
	w.Mottle = 0.55
	w.Grain = 0.24
	w.Tooth = 0.003
	w.Scatter = 0.18
	// Body is what keeps enriched chroma from dying into cream paper.
	w.Body = mathx.Clamp01(0.22 + 0.45*math.Max(washSat-1, 0))
	return w
}

// enrichPigment pushes HSB saturation for wash-sat. Multiplying S alone
// barely moves muted mean colours; for sat>1 we also lerp toward full
// chroma so the cast's cool/warm roles survive glazing.
func enrichPigment(c palette.Color, sat float64) palette.Color {
	if sat == 1 {
		return c
	}
	h := c.HSB()
	if sat < 1 {
		h.S = math.Max(0, h.S*sat)
		return h.Color()
	}
	t := 1 - 1/sat
	h.S = math.Min(100, h.S*(1+0.5*(sat-1))+((100-h.S)*t))
	h.B = math.Min(100, h.B*(1+0.15*(sat-1)))
	return h.Color()
}
