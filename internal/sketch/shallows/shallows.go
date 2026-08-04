// Package shallows implements sketch 012: a clear, rippled river surface
// over the faceted stones of scree, evaluated together as one material.
package shallows

import (
	"flag"
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/riffle"
	"github.com/jaminalder/go-graphics/internal/sketch/scree"
	"github.com/jaminalder/go-graphics/internal/trait"
)

var (
	_ sketch.Configurable = (*Sketch)(nil)
	_ sketch.Traited      = (*Sketch)(nil)
)

// Sketch combines a planned scree bed with a point-sampled riffle surface.
// The two fields meet before rasterisation; there is no intermediate image.
type Sketch struct {
	bed *scree.Sketch

	WaterSeed      uint64
	WaterDepth     float64
	WaterTint      float64
	Refraction     float64
	RippleStrength float64
	RippleShadow   float64
	RippleLight    float64
	DappleShadow   float64

	knobs *opt.Set
}

// New returns a clear shallow-water sketch. The bed retains scree's defaults
// and its complete trait and CLI vocabulary.
func New() *Sketch {
	s := &Sketch{
		bed:            scree.New(),
		WaterSeed:      42,
		WaterDepth:     0.68,
		WaterTint:      0.28,
		Refraction:     0.010,
		RippleStrength: 2.50,
		RippleShadow:   0.70,
		RippleLight:    0.45,
		DappleShadow:   0.03,
	}
	o := opt.New()
	o.Uint64("water-seed", "seed of the current and surface", "ws", &s.WaterSeed)
	o.Float("water-depth", "depth of clear water over the stones", "wd", 0, 1.5, &s.WaterDepth)
	o.Float("water-tint", "share of cool water colour in the bed", "wt", 0, 0.6, &s.WaterTint)
	o.Float("refraction", "surface displacement of the bed, canvas units", "re", 0, 0.15, &s.Refraction)
	o.Float("ripple-strength", "contrast and definition of surface ripples", "rs", 0, 2.5, &s.RippleStrength)
	o.Float("ripple-shadow", "dark side of each ripple facet", "rsh", 0, 0.7, &s.RippleShadow)
	o.Float("ripple-light", "lit side of each ripple facet", "rli", 0, 0.7, &s.RippleLight)
	o.Float("dapple-shadow", "broad tree shade over the water", "ds", 0, 0.5, &s.DappleShadow)
	s.knobs = o
	return s
}

func (s *Sketch) Name() string { return "shallows" }

func (s *Sketch) Describe() string {
	return "clear river ripples refracting and shading a bed of faceted stones"
}

// The output-space identity is the bed's. Water is deliberately continuous
// and controlled by explicit material knobs rather than a second trait draw.
func (s *Sketch) Schema() trait.Schema { return s.bed.Schema() }

func (s *Sketch) Traits(ctx sketch.Context) trait.Set { return s.bed.Traits(ctx) }

func (s *Sketch) TraitSuffix(set trait.Set) string { return s.bed.TraitSuffix(set) }

func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.bed.Flags(fs)
	s.knobs.Flags(fs)
}

func (s *Sketch) Configure() (string, error) {
	bedSuffix, err := s.bed.Configure()
	if err != nil {
		return "", err
	}
	waterSuffix, err := s.knobs.Configure()
	if err != nil {
		return "", err
	}
	return bedSuffix + waterSuffix, nil
}

func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	bed, err := s.bed.NewBed(ctx)
	if err != nil {
		return nil, err
	}
	surface := riffle.NewSurface(ctx, s.WaterSeed)
	water := coolest(ctx.Palette.Colors)
	light := water.Lighten(0.72)

	return sketch.Raster(ctx, func(u, v float64) palette.Color {
		w := surface.At(u, v)

		// A sloping facet shifts the point on the bed seen through it. Tanh
		// limits folds while retaining the narrow changes that define ripples.
		bend := s.Refraction * s.WaterDepth * math.Tanh(w.Slope*8)
		col := bed.At(u-bend*w.DU, v-bend*w.DV)

		// Clear water borrows a cool palette colour but leaves the stone's
		// local colour and facets dominant.
		tint := mathx.Clamp01(s.WaterTint * s.WaterDepth)
		col = palette.Lerp(col, valueMatched(water, col.Luminance()), tint)

		// Alternating dark and light facets are the surface cue. They act on
		// the visible bed itself, as water shadows and focused light do, rather
		// than sitting above it as a low-opacity blue veil.
		signal := math.Tanh(s.RippleStrength * w.Ripple)
		shadow := s.RippleShadow * mathx.Smoothstep(0.24, 0.88, -signal)
		shadow += s.DappleShadow * w.Dapple
		col = shadeLinear(col, 1-mathx.Clamp01(shadow))

		highlight := s.RippleLight * mathx.Smoothstep(0.30, 0.90, signal)
		return palette.Lerp(col, light, mathx.Clamp01(highlight)).Clamp()
	}), nil
}

func coolest(colors []palette.Color) palette.Color {
	best := colors[0]
	bestScore := math.Inf(-1)
	for _, c := range colors {
		// Prefer blue-green over purple, with enough value to remain clear.
		score := 1.25*c.B + 0.55*c.G - 1.15*c.R + 0.20*c.Luminance()
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

func valueMatched(c palette.Color, value float64) palette.Color {
	lum := c.Luminance()
	if lum < 1e-6 {
		return c
	}
	f := value / lum
	return palette.Color{R: c.R * f, G: c.G * f, B: c.B * f}.Clamp()
}

func shadeLinear(c palette.Color, factor float64) palette.Color {
	return palette.Color{
		R: palette.LinearToSRGB(palette.SRGBToLinear(c.R) * factor),
		G: palette.LinearToSRGB(palette.SRGBToLinear(c.G) * factor),
		B: palette.LinearToSRGB(palette.SRGBToLinear(c.B) * factor),
	}
}
