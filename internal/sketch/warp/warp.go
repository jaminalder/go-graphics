// Package warp implements sketch 013: nested fBM domain warping organized by
// a broad activity field into quiet and strongly folded passages.
package warp

import (
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
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
}

// Sketch holds warp's public field controls. Use New for useful defaults.
type Sketch struct {
	Scale, Gain, Lacunarity      float64
	WarpStrength, NestedStrength float64
	Octaves                      int
	fieldMode                    mode
}

// New returns warp configured for a moderately folded nested field.
func New() *Sketch {
	return &Sketch{
		Scale:          2.2,
		Gain:           0.5,
		Lacunarity:     2,
		WarpStrength:   2.2,
		NestedStrength: 2.8,
		Octaves:        5,
		fieldMode:      modeNested,
	}
}

type plan struct {
	set      settings
	q        [2]*noise.Perlin
	r        [2]*noise.Perlin
	value    *noise.Perlin
	activity *noise.Perlin
}

type fieldSample struct {
	value, qx, qy, rx, ry, activity float64
}

func (s *Sketch) plan(ctx sketch.Context) plan {
	return plan{
		set: settings{
			scale:          s.Scale,
			gain:           s.Gain,
			lacunarity:     s.Lacunarity,
			warpStrength:   s.WarpStrength,
			nestedStrength: s.NestedStrength,
			octaves:        s.Octaves,
			mode:           s.fieldMode,
		},
		q:        [2]*noise.Perlin{noise.New(ctx.Seed ^ seedQX), noise.New(ctx.Seed ^ seedQY)},
		r:        [2]*noise.Perlin{noise.New(ctx.Seed ^ seedRX), noise.New(ctx.Seed ^ seedRY)},
		value:    noise.New(ctx.Seed ^ seedValue),
		activity: noise.New(ctx.Seed ^ seedActivity),
	}
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

func (p plan) sample(u, v float64) fieldSample {
	x, y := u*p.set.scale, v*p.set.scale
	activityField := p.activity.At(u*0.65+17.3, v*0.65-9.7)
	activity := 0.25 + 0.95*mathx.Smoothstep(-0.4, 0.4, activityField)

	qx := p.fbm(p.q[0], x+3.1, y-7.9)
	qy := p.fbm(p.q[1], x-5.3, y+11.7)
	firstStrength := p.set.warpStrength * activity
	rx := p.fbm(p.r[0], x+qx*firstStrength+13.1, y+qy*firstStrength-4.7)
	ry := p.fbm(p.r[1], x+qx*firstStrength-8.3, y+qy*firstStrength+6.1)

	valueX, valueY := x, y
	switch p.set.mode {
	case modeSingle:
		valueX += qx * firstStrength
		valueY += qy * firstStrength
	case modeNested:
		secondStrength := p.set.nestedStrength * activity
		valueX += rx * secondStrength
		valueY += ry * secondStrength
	}

	return fieldSample{
		value:    p.fbm(p.value, valueX, valueY),
		qx:       qx,
		qy:       qy,
		rx:       rx,
		ry:       ry,
		activity: activity,
	}
}
