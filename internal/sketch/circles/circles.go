// Package circles implements sketch 003: a canvas packed with
// non-overlapping circles of varied sizes, each filled with shades of one
// palette color in a random structure — stripes, a dot raster, or Voronoi
// tiles (spec: docs/sketches/003-circles.md).
//
// The plan is a pure scene model (circle list + fill specs); the pixel
// function only evaluates it. A vector backend could consume the same
// scene later without touching the art logic.
package circles

import (
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/geom"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamLayout = 1 // packing: radii and positions
	streamFills  = 2 // per-circle appearance draws
)

// Sketch holds the structural knobs. Per-seed variation happens in plan.
type Sketch struct {
	MaxR     float64 // largest circle radius, canvas units
	MinR     float64 // smallest circle radius, canvas units
	Gap      float64 // minimum distance between circle edges (and to canvas edge)
	Attempts int     // packing attempts (fixed budget keeps the loop deterministic)
	PosTries int     // placement tries per attempt
}

// New returns the sketch with its defaults.
func New() *Sketch {
	return &Sketch{
		MaxR:     0.16,
		MinR:     0.008,
		Gap:      0.004,
		Attempts: 2400,
		PosTries: 30,
	}
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "circles" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "packed circles filled with striped, dotted, and voronoi shade patterns"
}

type fillKind uint8

const (
	fillStripes fillKind = iota
	fillDots
	fillVoronoi
)

// circleSpec is one circle of the scene: geometry plus fill recipe.
type circleSpec struct {
	geom.Circle
	kind    fillKind
	angle   float64 // pattern rotation (stripes: direction)
	scale   float64 // feature size, fraction of r
	phase   float64
	dotSize float64 // fillDots: dot radius as fraction of half a cell
	shades  []palette.Color
	salt    uint64 // per-circle hash/Worley seed
}

// plan is the full scene.
type plan struct {
	bg      palette.Color
	circles []circleSpec
	index   *geom.Index
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	p := s.plan(ctx)

	img := sketch.Raster(ctx, func(u, v float64) palette.Color {
		idx := p.index.At(u, v)
		if idx < 0 {
			return p.bg
		}
		return p.circles[idx].eval(u, v)
	})
	return img, nil
}

// eval computes the fill color at canvas point (u, v), which must lie
// inside the circle.
func (c *circleSpec) eval(u, v float64) palette.Color {
	// Circle-local frame: translate, rotate by -angle, normalize by r.
	dx, dy := u-c.X, v-c.Y
	sin, cos := math.Sin(-c.angle), math.Cos(-c.angle)
	px := (dx*cos - dy*sin) / c.R
	py := (dx*sin + dy*cos) / c.R

	n := len(c.shades)
	shadeAt := func(i, j int64) palette.Color {
		return c.shades[int(noise.Hash01(c.salt, i, j)*float64(n))%n]
	}

	switch c.kind {
	case fillStripes:
		band := int64(math.Floor(px/c.scale + c.phase))
		return shadeAt(band, 0)

	case fillDots:
		g := c.scale
		i := int64(math.Floor(px/g + c.phase))
		j := int64(math.Floor(py/g + c.phase))
		fx := px/g + c.phase - float64(i) - 0.5
		fy := py/g + c.phase - float64(j) - 0.5
		if math.Sqrt(fx*fx+fy*fy)*2 < c.dotSize {
			return shadeAt(i, j)
		}
		return c.shades[0] // ground shade

	default: // fillVoronoi
		ci, cj, f1, f2 := noise.WorleyCell(c.salt, px/c.scale, py/c.scale)
		col := shadeAt(ci, cj)
		// Subtle tone-on-tone border, as in tapestry's crackle.
		if m := 1 - mathx.Smoothstep(0.04, 0.14, f2-f1); m > 0 {
			col = palette.Lerp(col, col.ContrastShade(0.12), 0.7*m)
		}
		return col
	}
}

// plan makes all per-seed draws: packing on the layout stream, appearance
// on the fills stream. Draw order is fixed (changing it changes every
// seed's image — breaks goldens deliberately, never silently).
func (s *Sketch) plan(ctx sketch.Context) plan {
	aspect := float64(ctx.Width) / float64(ctx.Height)
	index := s.pack(ctx.RNG(streamLayout), aspect)
	circles := make([]circleSpec, len(index.Circles()))
	for i, c := range index.Circles() {
		circles[i].Circle = c
	}

	// Palette roles: the lightest color recedes into the background; all
	// colors (including the lightest) can anchor circles.
	byLum := palette.ByLuminance(ctx.Palette.Colors)
	bg := byLum[len(byLum)-1].Lighten(0.5).Desaturate(0.3)

	frng := ctx.RNG(streamFills)
	stripeAngles := []float64{0, math.Pi / 2, math.Pi / 4, -math.Pi / 4}
	for i := range circles {
		c := &circles[i]

		base := ctx.Palette.Colors[frng.IntN(len(ctx.Palette.Colors))]
		c.shades = shadeLadder(base, 3+frng.IntN(3))

		switch r := frng.Float64(); {
		case r < 0.45:
			c.kind = fillStripes
			c.angle = stripeAngles[frng.IntN(len(stripeAngles))] + (frng.Float64()-0.5)*0.14
			c.scale = 0.12 + frng.Float64()*0.23
		case r < 0.70:
			c.kind = fillDots
			c.angle = (frng.Float64() - 0.5) * 0.2
			c.scale = 0.18 + frng.Float64()*0.17
			c.dotSize = 0.55 + frng.Float64()*0.25
		default:
			c.kind = fillVoronoi
			c.angle = frng.Float64() * math.Pi
			c.scale = 0.15 + frng.Float64()*0.15
		}
		c.phase = frng.Float64()
		c.salt = frng.Uint64()

		// Floor the feature size in absolute canvas units: small circles
		// get a few bold elements instead of sub-pixel confetti.
		const minFeature = 0.008
		if c.scale*c.R < minFeature {
			c.scale = math.Min(minFeature/c.R, 1)
		}
	}

	return plan{bg: bg, circles: circles, index: index}
}

// shadeLadder builds count shades of base as an HSL lightness ladder,
// dark to light, all in the base color's family.
func shadeLadder(base palette.Color, count int) []palette.Color {
	// hi is capped below the background's lightness so even the palest
	// circle keeps a visible edge.
	h, s, l := base.HSL()
	lo := math.Max(0.15, l-0.25)
	hi := math.Min(0.85, l+0.30)
	shades := make([]palette.Color, count)
	for i := range shades {
		t := float64(i) / float64(count-1)
		shades[i] = palette.FromHSL(h, s, lo+t*(hi-lo))
	}
	return shades
}
