package riffle

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
)

// overlayPlan is the deliberately small vocabulary of the reusable water
// layer. It carries no bed, bank, caustic, foam, wake or exposed rock.
type overlayPlan struct {
	ink                     palette.Color
	alpha, ripples, shadows float64
	dots                    float64
	ground                  palette.Color
}

func newOverlayPlan(s *Sketch, pal palette.Palette) overlayPlan {
	return overlayPlan{
		ink:     overlayInk(pal),
		alpha:   s.overlayAlpha,
		ripples: s.overlayRipples,
		shadows: s.overlayShadows,
		dots:    s.overlayDots,
		ground:  overlayGround(s.ground),
	}
}

// overlayInk takes the coolest supplied swatch and washes it toward light.
// With the default Kandinsky Soft Pressure palette this starts at #8DCEE2.
func overlayInk(pal palette.Palette) palette.Color {
	ink := pal.Colors[0]
	best := warmth(ink)
	for _, c := range pal.Colors[1:] {
		if w := warmth(c); w < best {
			ink, best = c, w
		}
	}
	return ink.Lighten(0.24).Desaturate(0.16)
}

func overlayGround(name string) palette.Color {
	switch name {
	case "gray-light":
		return palette.Color{R: 0.82, G: 0.82, B: 0.82}
	case "gray-mid":
		return palette.Color{R: 0.59, G: 0.59, B: 0.59}
	case "gray-dark":
		return palette.Color{R: 0.36, G: 0.36, B: 0.36}
	default:
		return palette.Color{}
	}
}

// overlayPixel reduces the original material stack to three cues: current
// streaks, fine ripple facets and optional diffuse dots. Broad low-frequency
// shade is kept separate so a caller can judge or remove it independently.
func (p *plan) overlayPixel(u, v float64) render.LayerPixel {
	r := p.read(u, v)
	w := p.upstream(u, v, r)

	nx, ny := -w.slope*w.dirU, -w.slope*w.dirV
	lam := clampUnit((nx*p.lightX + ny*p.lightY) * 4.0)
	streak := clampUnit(w.streak * streakNorm)
	ripple := clampUnit(0.70*streak + 0.30*lam)

	shadeField := mathx.Smoothstep(-0.20, 0.50,
		p.nCaus.FBM(u/dappleWave+101, v/dappleWave-53, 2))
	shade := p.overlay.shadows * shadeField

	dark := p.overlay.ink.ContrastShade(0.10)
	light := p.overlay.ink.Lighten(0.20)
	tone := mathx.Clamp01(0.54 + 0.42*p.overlay.ripples*ripple - 0.52*shade)
	c := palette.Lerp(dark, light, tone)
	alpha := p.overlay.alpha * (0.76 + 0.20*p.overlay.ripples*math.Abs(ripple) + 0.28*shade)

	if dot := p.overlayDot(u, v); dot > 0 {
		c = palette.Lerp(c, light.Lighten(0.22), 0.55*dot)
		alpha += p.overlay.dots * 0.32 * dot
	}
	return render.LayerPixel{Color: c, Alpha: mathx.Clamp01(alpha)}
}

// overlayDot turns the old emergent boulders into pale, soft deposits. The
// core and loose halo echo the source sketch without reading as objects.
func (p *plan) overlayDot(u, v float64) float64 {
	if p.overlay.dots <= 0 {
		return 0
	}
	strongest := 0.0
	for _, d := range p.dots {
		rx := d.area() * 1.35
		ry := rx * 1.15
		q := math.Hypot((u-d.X)/rx, (v-d.Y)/ry)
		if q > 2.4 {
			continue
		}
		core := math.Exp(-1.7 * q * q)
		dq := (q - 1.25) / 0.42
		ring := math.Exp(-dq * dq)
		if v := 0.74*core + 0.26*ring; v > strongest {
			strongest = v
		}
	}
	return strongest
}

func (p *plan) overlayOnGround(u, v float64) palette.Color {
	px := p.overlayPixel(u, v)
	return compositeLinear(p.overlay.ground, px.Color, px.Alpha)
}

func compositeLinear(dst, src palette.Color, alpha float64) palette.Color {
	a := mathx.Clamp01(alpha)
	return palette.Color{
		R: palette.LinearToSRGB(palette.SRGBToLinear(src.R)*a + palette.SRGBToLinear(dst.R)*(1-a)),
		G: palette.LinearToSRGB(palette.SRGBToLinear(src.G)*a + palette.SRGBToLinear(dst.G)*(1-a)),
		B: palette.LinearToSRGB(palette.SRGBToLinear(src.B)*a + palette.SRGBToLinear(dst.B)*(1-a)),
	}
}
