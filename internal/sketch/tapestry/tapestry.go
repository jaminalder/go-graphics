// Package tapestry implements sketch 002: striped, grained contour layers
// (spec: docs/sketches/002-tapestry.md).
//
// Four layers per pixel: a contour-noise base (as sketch 001, finer rings),
// a low-frequency region field tinting large areas toward the palette's
// darkest color, full-height vertical stripes that shift the color beneath
// them, and deterministic grain. Each seed draws its own composition
// parameters from bounded ranges, so seeds are distinct but stay presentable.
package tapestry

import (
	"fmt"
	"image"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/gradient"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamShuffleLow  = 1
	streamShuffleHigh = 2
	streamStripes     = 3
	streamParams      = 4
)

// regionSeedSalt derives the region-field noise seed from Context.Seed so
// the region field is independent of the contour field.
const regionSeedSalt = 0x7265676e // "regn"

// Sketch holds the structural knobs. The per-seed variation draws happen in
// Render (see the spec); these fields bound or fix that variation.
type Sketch struct {
	Octaves       int     // contour fBm max octave index
	RegionOctaves int     // region fBm max octave index
	GrainRes      float64 // grain lattice cells per canvas unit
	StreakRatio   float64 // streak-grain cell elongation (y cells = GrainRes/StreakRatio)
}

// New returns the sketch with its defaults.
func New() *Sketch {
	return &Sketch{
		Octaves:       2,
		RegionOctaves: 1,
		GrainRes:      1400,
		StreakRatio:   6,
	}
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "tapestry" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "contour noise layered with vertical stripes, region tints, and grain"
}

// Stripe blend modes. Colored stripes multiply (dye-like hue shift that
// keeps contrast); lighten/darken nudge toward white/black.
type stripeMode uint8

const (
	modeNone stripeMode = iota
	modeTint
	modeLighten
	modeDarken
)

// stripe is one full-height vertical band. end is its right edge in
// normalized u; mode/color/amount shift the underlying color; grainMul and
// streak control the grain inside the stripe.
type stripe struct {
	end      float64
	mode     stripeMode
	color    palette.Color // used by modeTint
	amount   float64
	grainMul float64
	streak   bool
}

// apply blends the stripe's effect into c.
func (st stripe) apply(c palette.Color) palette.Color {
	switch st.mode {
	case modeTint:
		return palette.Lerp(c, mulBlend(c, st.color), st.amount*1.3)
	case modeLighten:
		return palette.Lerp(c, palette.Color{R: 1, G: 1, B: 1}, st.amount*0.4)
	case modeDarken:
		return palette.Lerp(c, palette.Color{}, st.amount*0.6)
	default:
		return c
	}
}

// mulBlend is channelwise multiply — the dye/warp-thread blend.
func mulBlend(a, b palette.Color) palette.Color {
	return palette.Color{R: a.R * b.R, G: a.G * b.G, B: a.B * b.B}
}

// plan is everything Render derives from the seed before the pixel loop.
type plan struct {
	freq       float64
	lowThresh  float64
	highThresh float64
	noiseMin   float64
	noiseMax   float64

	gradLow, gradMid, gradHigh gradient.Discrete

	regionFreq     float64
	regionThresh   float64
	regionStrength float64
	regionTint     palette.Color

	stripes  []stripe
	grainAmt float64
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	if len(ctx.Palette.Colors) < 4 {
		return nil, fmt.Errorf("tapestry: palette %q needs at least 4 colors", ctx.Palette.Slug)
	}
	p := s.plan(ctx)
	field := noise.New(ctx.Seed)
	region := noise.New(ctx.Seed ^ regionSeedSalt)

	img := render.Raster(ctx.Width, ctx.Height, func(u, v float64) palette.Color {
		// Layer 1: contour base.
		n := field.FBM(u*p.freq, v*p.freq, s.Octaves)
		var c palette.Color
		switch {
		case n < p.lowThresh:
			c = p.gradLow.At(remap(n, p.noiseMin, p.lowThresh))
		case n < p.highThresh:
			c = p.gradMid.At(remap(n, p.lowThresh, p.highThresh))
		default:
			c = p.gradHigh.At(remap(n, p.highThresh, p.noiseMax))
		}

		// Layer 2: region tint — multiply blend, rings showing through.
		r := region.FBM(u*p.regionFreq, v*p.regionFreq, s.RegionOctaves)
		if w := smoothstep(p.regionThresh, p.regionThresh+0.08, r); w > 0 {
			c = palette.Lerp(c, mulBlend(c, p.regionTint), w*p.regionStrength)
		}

		// Layer 3: vertical stripe.
		st := p.stripeAt(u)
		c = st.apply(c)

		// Layer 4: grain.
		gx := int64(u * s.GrainRes)
		gy := int64(v * s.GrainRes)
		if st.streak {
			gy = int64(v * s.GrainRes / s.StreakRatio)
		}
		g := (noise.Hash01(ctx.Seed, gx, gy) - 0.5) * p.grainAmt * st.grainMul
		return palette.Color{R: c.R + g, G: c.G + g, B: c.B + g}.Clamp()
	})
	return img, nil
}

// plan makes all per-seed draws. Draw order is fixed — changing it changes
// every existing seed's image (breaks goldens deliberately, never silently).
func (s *Sketch) plan(ctx sketch.Context) plan {
	prm := ctx.RNG(streamParams)

	// Palette roles by luminance: lightest anchors the smooth middle
	// gradient, darkest tints the regions, two random remaining colors
	// anchor the shuffled gradients.
	byLum := append([]palette.Color(nil), ctx.Palette.Colors...)
	sort.SliceStable(byLum, func(i, j int) bool {
		return byLum[i].Luminance() < byLum[j].Luminance()
	})
	darkest, lightest := byLum[0], byLum[len(byLum)-1]
	rest := byLum[1 : len(byLum)-1]
	i := prm.IntN(len(rest))
	j := prm.IntN(len(rest) - 1)
	if j >= i {
		j++
	}
	warm, cool := rest[i], rest[j]

	span := 0.55 + prm.Float64()*0.15 // noise mapped over ±span
	thresh := 0.10 + prm.Float64()*0.08
	bands := 35 + prm.IntN(36)

	p := plan{
		freq:       4.5 + prm.Float64()*4,
		lowThresh:  -thresh,
		highThresh: thresh,
		noiseMin:   -span,
		noiseMax:   span,

		gradLow: gradient.Sample(gradient.CosineBetween(warm, lightest), bands).
			Shuffled(ctx.RNG(streamShuffleLow)),
		gradMid: gradient.Sample(gradient.CosineBetween(lightest, cool), bands),
		gradHigh: gradient.Sample(gradient.CosineBetween(cool, warm), bands).
			Shuffled(ctx.RNG(streamShuffleHigh)),

		regionFreq:     1.2 + prm.Float64(),
		regionThresh:   0.10 + prm.Float64()*0.15,
		regionStrength: 0.50 + prm.Float64()*0.35,
		regionTint:     darkest.Saturate(0.7).Lighten(0.3),

		grainAmt: 0.03 + prm.Float64()*0.03,
	}
	p.stripes = s.planStripes(ctx)
	return p
}

// planStripes covers [0, aspect] with random-width stripes.
func (s *Sketch) planStripes(ctx sketch.Context) []stripe {
	rng := ctx.RNG(streamStripes)
	aspect := float64(ctx.Width) / float64(ctx.Height)
	var stripes []stripe
	pos := 0.0
	for pos < aspect {
		var width float64
		if rng.Float64() < 0.45 {
			width = 0.003 + rng.Float64()*0.007 // thin line
		} else {
			width = 0.02 + rng.Float64()*0.07 // wide band
		}
		pos += width
		mode, col := stripeEffect(rng, ctx.Palette)
		stripes = append(stripes, stripe{
			end:      pos,
			mode:     mode,
			color:    col,
			amount:   stripeAmount(rng),
			grainMul: 0.3 + rng.Float64()*1.1,
			streak:   rng.Float64() < 0.2,
		})
	}
	return stripes
}

// stripeEffect picks a stripe's blend mode and, for tints, its dye color.
// Tint colors are lightened so the multiply blend shifts hue without
// crushing the values.
func stripeEffect(rng *rand.Rand, pal palette.Palette) (stripeMode, palette.Color) {
	switch r := rng.Float64(); {
	case r < 0.60:
		dye := pal.Color(rng.IntN(len(pal.Colors))).Lighten(0.35)
		return modeTint, dye
	case r < 0.72:
		return modeLighten, palette.Color{}
	case r < 0.85:
		return modeDarken, palette.Color{}
	default:
		return modeNone, palette.Color{}
	}
}

// stripeAmount draws the blend strength.
func stripeAmount(rng *rand.Rand) float64 {
	return 0.05 + rng.Float64()*0.25
}

// stripeAt finds the stripe containing u by binary search.
func (p *plan) stripeAt(u float64) stripe {
	lo, hi := 0, len(p.stripes)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if p.stripes[mid].end <= u {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return p.stripes[lo]
}

// remap maps x from [lo, hi] to [0,1]; Discrete.At clamps the result.
func remap(x, lo, hi float64) float64 {
	return (x - lo) / (hi - lo)
}

// smoothstep is the standard cubic step from 0 (x≤lo) to 1 (x≥hi).
func smoothstep(lo, hi, x float64) float64 {
	t := math.Min(1, math.Max(0, (x-lo)/(hi-lo)))
	return t * t * (3 - 2*t)
}
