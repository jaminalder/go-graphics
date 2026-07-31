// Package drift implements sketch 004: hand-painted dots following the
// streamlines of a Perlin flow field (spec: docs/sketches/004-drift.md).
// The first sketch built on the paint package's stamp-based brush model.
package drift

import (
	"image"
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamLayout = 1 // streamline walking and dot placement
	streamPaint  = 2 // per-dot style, colors, and brush jitter
	streamPaper  = 3 // background speckle
)

// Noise seed salts for the three fields.
const (
	saltAngle = 0x616e67 // "ang"
	saltSize  = 0x73697a // "siz"
	saltColor = 0x636f6c // "col"
)

// Style selects the disc mark; StyleMix draws per dot.
type Style uint8

const (
	StyleMix Style = iota
	StyleRings
	StyleScribble
	StyleGouache
)

// Sketch holds the structural knobs. Per-seed variation happens in plan.
type Sketch struct {
	MinR, MaxR float64 // dot radius range, canvas units
	Gap        float64 // minimum edge distance between dots
	AngleFreq  float64 // flow field frequency
	AngleSwing float64 // radians of direction swing across the field
	SizeFreq   float64 // size field frequency
	ColorFreq  float64 // color field frequency
	Starts     int     // streamline starts per axis
	MaxSteps   int     // dots attempted per streamline
	Style      Style

	style string
	knobs *opt.Set
}

// New returns the sketch with its defaults.
func New() *Sketch {
	s := &Sketch{
		MinR:       0.007,
		MaxR:       0.028,
		Gap:        0.0025,
		AngleFreq:  1.4,
		AngleSwing: 5.2,
		SizeFreq:   1.9,
		ColorFreq:  2.4,
		Starts:     26,
		MaxSteps:   42,
		Style:      StyleMix,
	}
	s.style = "mix"
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "drift" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "hand-painted dots (rings, scribble, gouache) along a flow field"
}

// dot is one placed, styled dot of the scene.
type dot struct {
	geom.Circle
	style Style
	main  palette.Color
	ink   palette.Color
}

// Render implements sketch.Sketch. Painting is stamp-based: Context.AA
// and Deep are not used (soft brush edges provide the anti-aliasing).
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	aspect := float64(ctx.Width) / float64(ctx.Height)
	dots := s.place(ctx, aspect)

	byLum := palette.ByLuminance(ctx.Palette.Colors)
	paper := byLum[len(byLum)-1].Lighten(0.55).Desaturate(0.4)
	darkest := byLum[0]

	cv := paint.NewCanvas(ctx.Width, ctx.Height, paper)

	// Paper speckle: sparse tiny ink dots.
	prng := ctx.RNG(streamPaper)
	for range 420 {
		cv.Dab(prng.Float64()*aspect, prng.Float64(), 0.0009+prng.Float64()*0.0009,
			darkest, 0.35+prng.Float64()*0.3)
	}

	// Paint every dot with its own jitter, in placement order.
	brng := ctx.RNG(streamPaint)
	for _, d := range dots {
		switch d.style {
		case StyleRings:
			paint.RingsDisc(cv, brng, d.X, d.Y, d.R, d.ink, d.main)
		case StyleScribble:
			paint.ScribbleDisc(cv, brng, d.X, d.Y, d.R, d.ink, d.main)
		default:
			paint.GouacheDisc(cv, brng, d.X, d.Y, d.R, d.main, d.ink)
		}
	}
	return cv.Image(), nil
}

// place walks streamlines of the flow field and drops dots wherever they
// fit. All budgets are fixed, so the loop is deterministic.
func (s *Sketch) place(ctx sketch.Context, aspect float64) []dot {
	angleN := noise.New(ctx.Seed ^ saltAngle)
	sizeN := noise.New(ctx.Seed ^ saltSize)
	colorN := noise.New(ctx.Seed ^ saltColor)
	lrng := ctx.RNG(streamLayout)

	byLum := palette.ByLuminance(ctx.Palette.Colors)
	nCol := len(byLum)
	darkest, lightest := byLum[0], byLum[nCol-1]

	radiusAt := func(x, y float64) float64 {
		t := mathx.Clamp01(sizeN.FBM(x*s.SizeFreq, y*s.SizeFreq, 1)/1.1 + 0.5)
		return s.MinR + (s.MaxR-s.MinR)*t*t // quadratic: many small, few large
	}
	styleFor := func(rng *rand.Rand) Style {
		if s.Style != StyleMix {
			return s.Style
		}
		switch r := rng.Float64(); {
		case r < 0.45:
			return StyleRings
		case r < 0.75:
			return StyleScribble
		default:
			return StyleGouache
		}
	}

	const margin = 0.03
	index := geom.NewIndex(aspect, 1, s.MaxR)
	var dots []dot

	for sy := 0; sy < s.Starts; sy++ {
		for sx := 0; sx < s.Starts; sx++ {
			x := (float64(sx) + 0.2 + 0.6*lrng.Float64()) / float64(s.Starts) * aspect
			y := (float64(sy) + 0.2 + 0.6*lrng.Float64()) / float64(s.Starts)
			misses := 0
			for step := 0; step < s.MaxSteps && misses < 5; step++ {
				r := radiusAt(x, y)
				inside := x > margin+r && x < aspect-margin-r && y > margin+r && y < 1-margin-r
				c := geom.Circle{X: x, Y: y, R: r}
				if inside && index.FitsWithGap(c, s.Gap) {
					index.Insert(c)

					// Coherent color patches with per-dot jitter.
					cval := mathx.Clamp01(colorN.FBM(x*s.ColorFreq, y*s.ColorFreq, 1)/1.1 + 0.5)
					ci := int(cval * float64(nCol))
					if lrng.Float64() < 0.25 {
						ci += 1 - lrng.IntN(3)
					}
					main := byLum[((ci%nCol)+nCol)%nCol]
					ink := darkest
					if main == darkest {
						ink = lightest
					}
					dots = append(dots, dot{Circle: c, style: styleFor(lrng), main: main, ink: ink})
				} else {
					misses++
				}
				angle := s.AngleSwing * angleN.FBM(x*s.AngleFreq, y*s.AngleFreq, 1)
				adv := 2*r + s.Gap + 0.004*lrng.Float64()
				x += adv * math.Cos(angle)
				y += adv * math.Sin(angle)
				if x < 0 || x > aspect || y < 0 || y > 1 {
					break
				}
			}
		}
	}
	return dots
}
