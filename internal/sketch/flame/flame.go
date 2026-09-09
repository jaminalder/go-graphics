// Package flame implements sketch 016: a fractal flame. A small set of
// nonlinear maps is iterated as a chaos game; the picture is the log-density
// of the orbit, coloured by the path that produced each point.
package flame

import (
	"fmt"
	"image"
	"sort"

	fl "github.com/jaminalder/go-graphics/internal/flame"
	"github.com/jaminalder/go-graphics/internal/gradient"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

const (
	streamTraits = 1
	streamRecipe = 2
	streamGenome = 3
	streamFrame  = 4
	streamOrbit  = 5
	frameSamples = 48000
)

// Sketch holds flame's public display controls. Use New for useful defaults.
type Sketch struct {
	Quality, Gamma, Vibrancy  float64
	Brightness, Gleam, Scale  float64
	Estimator, DeMin, DeCurve float64
	Oversample                int
	Filter                    float64
	WashSat                   float64
	knobs                     *opt.Set
	traits                    *trait.Options
}

// New returns a flame with Apophysis-like display defaults.
func New() *Sketch {
	tone := fl.DefaultTone()
	s := &Sketch{
		Quality:    40,
		Gamma:      tone.Gamma,
		Vibrancy:   tone.Vibrancy,
		Brightness: tone.Brightness,
		Gleam:      tone.Gleam,
		Scale:      1,
		Estimator:  9,
		DeMin:      0,
		DeCurve:    0.4,
		Oversample: 2,
		Filter:     fl.DefaultFilter,
		WashSat:    2.5,
	}
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "flame" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "a fractal flame: iterated nonlinear maps, log-density filaments"
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	if len(ctx.Palette.Colors) == 0 {
		return nil, fmt.Errorf("flame: palette %q has no colors", ctx.Palette.Slug)
	}
	set := s.Traits(ctx)
	pal, err := castPalette(set.Get(dimCast), ctx.Palette)
	if err != nil {
		return nil, err
	}
	if len(pal.Colors) == 0 {
		return nil, fmt.Errorf("flame: cast %q has no colors", set.Get(dimCast))
	}
	rec := s.pin(draw(set, ctx.RNG(streamRecipe)))
	sys := compose(rec, ctx.RNG(streamGenome))
	cam := fl.Frame(sys, ctx.RNG(streamFrame), frameSamples)
	cam.Scale *= rec.aperture * rec.scale
	sys.Cam = cam

	grad := colourMap(pal, rec.tint)
	lut := make([]palette.Color, 256)
	for i := range lut {
		lut[i] = grad.At(float64(i) / 255)
	}
	color := func(t float64) palette.Color {
		i := int(mathx.Clamp01(t)*255 + 0.5)
		if i > 255 {
			i = 255
		}
		return lut[i]
	}

	aa := ctx.AA
	if aa < 1 {
		aa = 1
	}
	ss := rec.oversample
	if ss < 1 {
		ss = 1
	}
	// Samples are per *output* pixel. Oversample only raises histogram
	// resolution; --aa still multiplies the sample budget.
	samples := int(rec.quality * float64(ctx.Width) * float64(ctx.Height) * float64(aa))
	if samples < 1024 {
		samples = 1024
	}

	hw, hh := ctx.Width*ss, ctx.Height*ss
	hist := fl.Accumulate(sys, color, hw, hh, samples, ctx.Seed^streamOrbit)
	// Estimator is specified in output pixels; hist bins are ss× finer,
	// matching flam3's estimator_radius * ss.
	ssF := float64(ss)
	est := fl.Estimate{
		Radius: rec.estimator * ssF,
		Min:    rec.deMin * ssF,
		Curve:  rec.deCurve,
	}
	var pix []palette.Color
	if rec.medium == mediumWashTone {
		pix = developWash(hist, rec.brightness, est, ctx.Seed, rec.washSat)
	} else {
		tone := fl.Tone{
			Gamma:      rec.gamma,
			Vibrancy:   rec.vibrancy,
			Brightness: rec.brightness,
			Gleam:      rec.gleam,
			Background: groundColour(pal, rec.ground),
			Estimate:   est,
		}
		pix = hist.Develop(tone)
	}
	if ss > 1 {
		pix, err = fl.Downsample(pix, hw, hh, ctx.Width, ctx.Height, ss, rec.filter)
		if err != nil {
			return nil, err
		}
	}
	if ctx.Deep {
		return render.ImageFromColorsDeep(ctx.Width, ctx.Height, pix), nil
	}
	return render.ImageFromColors(ctx.Width, ctx.Height, pix), nil
}

func colourMap(pal palette.Palette, t tint) gradient.Gradient {
	cols := append([]palette.Color(nil), pal.Colors...)
	if t == tintSplit {
		return flameRamp(cols)
	}
	return gradient.ThroughRGB(cols)
}

func flameRamp(cols []palette.Color) gradient.Gradient {
	if len(cols) == 0 {
		return gradient.ThroughRGB(nil)
	}
	sortWarm(cols)
	split := len(cols)
	for i, c := range cols {
		if warmth(c) >= 0 {
			split = i
			break
		}
	}
	cool := append([]palette.Color(nil), cols[:split]...)
	warm := append([]palette.Color(nil), cols[split:]...)
	sort.SliceStable(warm, func(i, j int) bool {
		return warm[i].Luminance() > warm[j].Luminance()
	})
	return gradient.ThroughRGB(append(cool, warm...))
}

func sortWarm(cols []palette.Color) {
	// Warmth is R−B; coolest first.
	sort.SliceStable(cols, func(i, j int) bool {
		return warmth(cols[i]) < warmth(cols[j])
	})
}

func warmth(c palette.Color) float64 { return c.R - c.B }

func groundColour(pal palette.Palette, g ground) palette.Color {
	switch g {
	case groundPaper:
		return paperGround
	case groundDusk:
		d := pal.Colors[0]
		for _, c := range pal.Colors {
			if c.Luminance() < d.Luminance() {
				d = c
			}
		}
		return palette.Color{R: d.R * 0.08, G: d.G * 0.08, B: d.B * 0.08}
	default:
		return palette.Color{}
	}
}
