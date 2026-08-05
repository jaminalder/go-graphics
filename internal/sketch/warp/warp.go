// Package warp implements sketch 013: nested fBM domain warping organized by
// a broad activity field into quiet and strongly folded passages.
package warp

import (
	"fmt"
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/gradient"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

const (
	seedQX       = 0x776172702d7178
	seedQY       = 0x776172702d7179
	seedRX       = 0x776172702d7278
	seedRY       = 0x776172702d7279
	seedValue    = 0x776172702d7661
	seedActivity = 0x776172702d6163
)

type mode uint8

const (
	modePlain mode = iota
	modeSingle
	modeNested
)

type settings struct {
	scale, gain, lacunarity      float64
	warpStrength, nestedStrength float64
	octaves                      int
	mode                         mode
	appearance                   appearance
}

type appearance uint8

const (
	appearanceGradient appearance = iota
	appearanceStructure
)

// Sketch holds warp's public field controls. Use New for useful defaults.
type Sketch struct {
	Scale, Gain, Lacunarity      float64
	WarpStrength, NestedStrength float64
	Octaves                      int
	warpName, appearanceName     string
	fieldMode                    mode
	appearance                   appearance
	knobs                        *opt.Set
}

// New returns warp configured for a moderately folded nested field.
func New() *Sketch {
	s := &Sketch{
		Scale:          1.65,
		Gain:           0.5,
		Lacunarity:     2,
		WarpStrength:   3,
		NestedStrength: 4,
		Octaves:        5,
		warpName:       "nested",
		appearanceName: "gradient",
		fieldMode:      modeNested,
		appearance:     appearanceGradient,
	}
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "warp" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "flowing organic fields from nested fBM domain warps"
}

type plan struct {
	set       settings
	q         [2]*noise.Perlin
	r         [2]*noise.Perlin
	value     *noise.Perlin
	activity  *noise.Perlin
	low, high gradient.SmoothHSL
}

type fieldSample struct {
	value, qx, qy, rx, ry, activity float64
}

func (s *Sketch) plan(ctx sketch.Context) (plan, error) {
	if len(ctx.Palette.Colors) < 3 {
		return plan{}, fmt.Errorf("warp: palette %q needs at least 3 colors", ctx.Palette.Slug)
	}
	colors := palette.ByLuminance(ctx.Palette.Colors)
	middle := colors[len(colors)/2]
	return plan{
		set: settings{
			scale:          s.Scale,
			gain:           s.Gain,
			lacunarity:     s.Lacunarity,
			warpStrength:   s.WarpStrength,
			nestedStrength: s.NestedStrength,
			octaves:        s.Octaves,
			mode:           s.fieldMode,
			appearance:     s.appearance,
		},
		q:        [2]*noise.Perlin{noise.New(ctx.Seed ^ seedQX), noise.New(ctx.Seed ^ seedQY)},
		r:        [2]*noise.Perlin{noise.New(ctx.Seed ^ seedRX), noise.New(ctx.Seed ^ seedRY)},
		value:    noise.New(ctx.Seed ^ seedValue),
		activity: noise.New(ctx.Seed ^ seedActivity),
		low:      gradient.HSLBetween(colors[0], middle),
		high:     gradient.HSLBetween(middle, colors[len(colors)-1]),
	}, nil
}

func (p plan) fbm(field *noise.Perlin, x, y float64) float64 {
	sum, weight := 0.0, 0.0
	frequency, amplitude := 1.0, 1.0
	for range p.set.octaves {
		sum += field.At(x*frequency, y*frequency) * amplitude
		weight += amplitude
		frequency *= p.set.lacunarity
		amplitude *= p.set.gain
	}
	return sum / weight
}

// At maps one canvas coordinate through the field and selected appearance.
func (p plan) At(u, v float64) palette.Color {
	sample := p.sample(u, v)
	tone := mathx.Smoothstep(-0.48, 0.48, sample.value)
	if p.set.appearance == appearanceStructure {
		shift := 0.0
		if p.set.mode >= modeSingle {
			shift += 0.13 * math.Tanh(2.8*(sample.qx-sample.qy))
		}
		if p.set.mode == modeNested {
			shift += 0.11 * math.Tanh(2.8*(sample.rx+sample.ry))
		}
		tone = mathx.Clamp01(tone + shift)
	}
	if tone < 0.5 {
		return p.low.At(tone * 2).Clamp()
	}
	return p.high.At((tone - 0.5) * 2).Clamp()
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	p, err := s.plan(ctx)
	if err != nil {
		return nil, err
	}
	return sketch.Raster(ctx, p.At), nil
}

func (p plan) sample(u, v float64) fieldSample {
	x, y := u*p.set.scale, v*p.set.scale
	if p.set.mode == modePlain {
		return fieldSample{value: p.fbm(p.value, x, y)}
	}

	activityField := p.activity.At(u*0.65+17.3, v*0.65-9.7)
	activity := 0.12 + 1.28*mathx.Smoothstep(-0.4, 0.4, activityField)

	qx := p.fbm(p.q[0], x+3.1, y-7.9)
	qy := p.fbm(p.q[1], x-5.3, y+11.7)
	firstStrength := p.set.warpStrength * activity
	if p.set.mode == modeSingle {
		return fieldSample{
			value:    p.fbm(p.value, x+qx*firstStrength, y+qy*firstStrength),
			qx:       qx,
			qy:       qy,
			activity: activity,
		}
	}

	rx := p.fbm(p.r[0], x+qx*firstStrength+13.1, y+qy*firstStrength-4.7)
	ry := p.fbm(p.r[1], x+qx*firstStrength-8.3, y+qy*firstStrength+6.1)
	secondStrength := p.set.nestedStrength * activity

	return fieldSample{
		value:    p.fbm(p.value, x+rx*secondStrength, y+ry*secondStrength),
		qx:       qx,
		qy:       qy,
		rx:       rx,
		ry:       ry,
		activity: activity,
	}
}
