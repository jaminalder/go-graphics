// Package contour implements sketch 001: shuffled-gradient contour noise,
// a port of staticart's sketch_7 (spec: docs/sketches/001-contour-noise.md).
//
// A smooth fBm noise field is colored through three cosine gradients
// selected by noise value. The outer two gradients are shuffled — adjacent
// noise values jump between unrelated colors, producing sharp topographic
// contour rings — while the middle gradient stays smooth, giving cloudy
// transitions between the ringed regions.
package contour

import (
	"fmt"
	"image"

	"github.com/jaminalder/go-graphics/internal/gradient"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// RNG stream ids for the two shuffles (see Context.RNG).
const (
	streamShuffleLow  = 1
	streamShuffleHigh = 2
)

// Sketch holds the tunables. Zero value is not useful; use New.
type Sketch struct {
	Frequency float64 // noise cycles per canvas unit
	Octaves   int     // max fBm octave index (2 → three components)
	Bands     int     // discrete colors per gradient
	Gain      float64 // multiplier on the raw noise value before banding

	// Band thresholds and the outer mapping range of the (gained) noise.
	LowThreshold, HighThreshold float64
	NoiseMin, NoiseMax          float64
}

// New returns the sketch with the defaults from the spec. The original used
// pixel scale 0.006 on a 2000px canvas → 12 cycles per canvas unit.
func New() *Sketch {
	return &Sketch{
		Frequency:     6,
		Octaves:       2,
		Bands:         50,
		Gain:          1,
		LowThreshold:  -0.15,
		HighThreshold: 0.15,
		NoiseMin:      -0.6,
		NoiseMax:      0.6,
	}
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "contour" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "topographic contour rings from shuffled gradients over fBm noise"
}

// plan is the resolved contour field and its colour mapping. It is immutable
// after construction and safe to share across raster workers.
type plan struct {
	field                       *noise.Perlin
	low, mid, high              gradient.Discrete
	frequency, gain             float64
	octaves                     int
	lowThreshold, highThreshold float64
	noiseMin, noiseMax          float64
}

// plan resolves the palette and shuffled bands before the pixel loop.
// Palette colors 0, 1, 2 are the gradient endpoints (0→1 shuffled, 1→2
// smooth, 2→0 shuffled), as in the original.
func (s *Sketch) plan(ctx sketch.Context) (plan, error) {
	if len(ctx.Palette.Colors) < 3 {
		return plan{}, fmt.Errorf("contour: palette %q needs at least 3 colors", ctx.Palette.Slug)
	}
	c0, c1, c2 := ctx.Palette.Color(0), ctx.Palette.Color(1), ctx.Palette.Color(2)

	gradLow := gradient.Sample(gradient.CosineBetween(c0, c1), s.Bands).
		Shuffled(ctx.RNG(streamShuffleLow))
	gradMid := gradient.Sample(gradient.CosineBetween(c1, c2), s.Bands)
	gradHigh := gradient.Sample(gradient.CosineBetween(c2, c0), s.Bands).
		Shuffled(ctx.RNG(streamShuffleHigh))

	return plan{
		field:         noise.New(ctx.Seed),
		low:           gradLow,
		mid:           gradMid,
		high:          gradHigh,
		frequency:     s.Frequency,
		gain:          s.Gain,
		octaves:       s.Octaves,
		lowThreshold:  s.LowThreshold,
		highThreshold: s.HighThreshold,
		noiseMin:      s.NoiseMin,
		noiseMax:      s.NoiseMax,
	}, nil
}

// At evaluates one canvas coordinate.
func (p plan) At(u, v float64) palette.Color {
	n := p.field.FBM(u*p.frequency, v*p.frequency, p.octaves) * p.gain
	switch {
	case n < p.lowThreshold:
		return p.low.At(mathx.Remap(n, p.noiseMin, p.lowThreshold))
	case n < p.highThreshold:
		return p.mid.At(mathx.Remap(n, p.lowThreshold, p.highThreshold))
	default:
		return p.high.At(mathx.Remap(n, p.highThreshold, p.noiseMax))
	}
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	p, err := s.plan(ctx)
	if err != nil {
		return nil, err
	}
	return sketch.Raster(ctx, p.At), nil
}
