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
	seedQX        = 0x776172702d7178
	seedQY        = 0x776172702d7179
	seedRX        = 0x776172702d7278
	seedRY        = 0x776172702d7279
	seedValue     = 0x776172702d7661
	seedActivity  = 0x776172702d6163
	rRoleScale    = 1.17
	valueScale    = 1.31
	normalEpsilon = 0.0038
	materialDepth = 0.082
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
	appearanceFolded
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
		appearanceName: "folded",
		fieldMode:      modeNested,
		appearance:     appearanceFolded,
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
	set        settings
	q          [2]*noise.Perlin
	r          [2]*noise.Perlin
	value      *noise.Perlin
	activity   *noise.Perlin
	low, high  gradient.SmoothHSL
	ridgeColor palette.Color
}

type fieldSample struct {
	value, fine, height, ridge float64
	qx, qy, rx, ry, activity   float64
}

type materialSample struct {
	height, ridge float64
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
		q:          [2]*noise.Perlin{noise.New(ctx.Seed ^ seedQX), noise.New(ctx.Seed ^ seedQY)},
		r:          [2]*noise.Perlin{noise.New(ctx.Seed ^ seedRX), noise.New(ctx.Seed ^ seedRY)},
		value:      noise.New(ctx.Seed ^ seedValue),
		activity:   noise.New(ctx.Seed ^ seedActivity),
		low:        gradient.HSLBetween(colors[0], middle),
		high:       gradient.HSLBetween(middle, colors[len(colors)-1]),
		ridgeColor: colors[len(colors)-1],
	}, nil
}

func (p plan) fbm(field *noise.Perlin, x, y float64) float64 {
	value, _ := p.fbmWithFine(field, x, y)
	return value
}

func (p plan) fbmWithFine(field *noise.Perlin, x, y float64) (float64, float64) {
	sum, weight := 0.0, 0.0
	fine, fineWeight := 0.0, 0.0
	amplitude := 1.0
	for octave := range p.set.octaves {
		value := field.At(x, y)
		sum += value * amplitude
		weight += amplitude
		if octave >= p.set.octaves-2 {
			fine += value * amplitude
			fineWeight += amplitude
		}
		x, y = rotateScale(x, y, p.set.lacunarity)
		amplitude *= p.set.gain
	}
	if fineWeight == 0 {
		return sum / weight, 0
	}
	return sum / weight, fine / fineWeight
}

func rotateScale(x, y, frequency float64) (float64, float64) {
	const cosine = 0.8191520442889918
	const sine = 0.573576436351046
	return (cosine*x - sine*y) * frequency, (sine*x + cosine*y) * frequency
}

func material(raw, fine, activity float64) materialSample {
	body := math.Pow(mathx.Smoothstep(-0.46, 0.5, raw), 1.22)
	activityGate := mathx.Smoothstep(0.28, 1.08, activity)
	fold := 1 - math.Abs(fine)*2.35
	ridge := mathx.Smoothstep(0.68, 0.96, fold) * activityGate
	height := mathx.Clamp01(body*0.86 + ridge*0.34)
	return materialSample{height: height, ridge: ridge}
}

type vector3 struct {
	x, y, z float64
}

func normalize3(v vector3) vector3 {
	length := math.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z)
	return vector3{x: v.x / length, y: v.y / length, z: v.z / length}
}

func (p plan) materialHeight(u, v float64) float64 {
	return p.sample(u, v).height
}

func (p plan) materialNormal(u, v float64) vector3 {
	dx := p.materialHeight(u+normalEpsilon, v) - p.materialHeight(u-normalEpsilon, v)
	dy := p.materialHeight(u, v+normalEpsilon) - p.materialHeight(u, v-normalEpsilon)
	return normalize3(vector3{-materialDepth * dx, -materialDepth * dy, 2 * normalEpsilon})
}

func lightFactor(normal vector3) float64 {
	light := normalize3(vector3{0.48, -0.36, 0.8})
	diffuse := math.Max(0, normal.x*light.x+normal.y*light.y+normal.z*light.z)
	return math.Min(1.08, 0.28+0.9*diffuse)
}

func shadeLinear(color palette.Color, factor float64) palette.Color {
	return palette.Color{
		R: palette.LinearToSRGB(palette.SRGBToLinear(color.R) * factor),
		G: palette.LinearToSRGB(palette.SRGBToLinear(color.G) * factor),
		B: palette.LinearToSRGB(palette.SRGBToLinear(color.B) * factor),
	}.Clamp()
}

// At maps one canvas coordinate through the field and selected appearance.
func (p plan) At(u, v float64) palette.Color {
	sample := p.sample(u, v)
	tone := sample.height
	if p.set.appearance == appearanceGradient {
		tone = mathx.Smoothstep(-0.48, 0.48, sample.value)
	}
	var color palette.Color
	if tone < 0.5 {
		color = p.low.At(tone * 2)
	} else {
		color = p.high.At((tone - 0.5) * 2)
	}
	if p.set.appearance == appearanceGradient {
		return color.Clamp()
	}
	color = palette.Lerp(color, p.ridgeColor, sample.ridge*0.18)
	return shadeLinear(color, lightFactor(p.materialNormal(u, v)))
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
		value, fine := p.fbmWithFine(p.value, x, y)
		material := material(value, fine, 0)
		return fieldSample{value: value, fine: fine, height: material.height, ridge: material.ridge}
	}

	activityField := p.activity.At(u*0.65+17.3, v*0.65-9.7)
	activity := 0.12 + 1.28*mathx.Smoothstep(-0.4, 0.4, activityField)

	qx := p.fbm(p.q[0], x+3.1, y-7.9)
	qy := p.fbm(p.q[1], x-5.3, y+11.7)
	firstStrength := p.set.warpStrength * activity
	if p.set.mode == modeSingle {
		value, fine := p.fbmWithFine(p.value, x+qx*firstStrength, y+qy*firstStrength)
		material := material(value, fine, activity)
		return fieldSample{
			value:    value,
			fine:     fine,
			height:   material.height,
			ridge:    material.ridge,
			qx:       qx,
			qy:       qy,
			activity: activity,
		}
	}

	rx := p.fbm(p.r[0], (x+qx*firstStrength+13.1)*rRoleScale, (y+qy*firstStrength-4.7)*rRoleScale)
	ry := p.fbm(p.r[1], (x+qx*firstStrength-8.3)*rRoleScale, (y+qy*firstStrength+6.1)*rRoleScale)
	secondStrength := p.set.nestedStrength * activity
	value, fine := p.fbmWithFine(p.value,
		(x+rx*secondStrength)*valueScale,
		(y+ry*secondStrength)*valueScale,
	)
	material := material(value, fine, activity)

	return fieldSample{
		value:    value,
		fine:     fine,
		height:   material.height,
		ridge:    material.ridge,
		qx:       qx,
		qy:       qy,
		rx:       rx,
		ry:       ry,
		activity: activity,
	}
}
