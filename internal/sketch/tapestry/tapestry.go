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

	// DisableStripes turns layer 3 off (one pass-through stripe). Stripes
	// use their own RNG stream, so the rest of the composition is identical
	// to the striped render of the same seed.
	DisableStripes bool

	// Relief enables 3D shading: hillshade lighting from the noise
	// gradient, paper-cut shadows/rims at band edges, and a subtle
	// specular highlight. Purely a shading pass — the composition is
	// identical to the unshaded render of the same seed.
	Relief bool

	// ReliefParams tunes the relief pass; ignored unless Relief is set.
	ReliefParams ReliefParams
}

// ReliefParams are the lighting/shading knobs of the relief pass.
type ReliefParams struct {
	LightDir  [3]float64 // direction toward the light; +y points down the image
	Slope     float64    // height-gradient → surface-slope scale (terrain depth)
	Ambient   float64    // floor of the diffuse term, 0..1 (higher = flatter)
	EdgeWidth float64    // band-fraction distance affected by edge shading
	Shadow    float64    // darkening below a band edge (paper-cut shadow)
	Rim       float64    // brightening above a band edge (lit paper rim)
	Spec      float64    // specular strength
	Shininess float64    // specular exponent (higher = tighter highlight)
}

// DefaultReliefParams is the tuned prototype look: soft top-left light,
// moderate carve, engraved band edges, a whisper of gloss.
func DefaultReliefParams() ReliefParams {
	return ReliefParams{
		LightDir:  [3]float64{-0.6, -0.6, 0.75},
		Slope:     0.05,
		Ambient:   0.60,
		EdgeWidth: 0.45,
		Shadow:    0.30,
		Rim:       0.10,
		Spec:      0.10,
		Shininess: 16,
	}
}

// reliefEps is the finite-difference step in canvas units.
const reliefEps = 0.0005

// New returns the sketch with its defaults.
func New() *Sketch {
	return &Sketch{
		Octaves:       2,
		RegionOctaves: 2,
		GrainRes:      1400,
		StreakRatio:   6,
		ReliefParams:  DefaultReliefParams(),
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
		// Layer 1: contour base. bandFrac is the position within the
		// current color band (0 = lower edge), used by relief shading.
		n := field.FBM(u*p.freq, v*p.freq, s.Octaves)
		var (
			c        palette.Color
			bandFrac float64
		)
		switch {
		case n < p.lowThresh:
			c, bandFrac = bandAt(p.gradLow, remap(n, p.noiseMin, p.lowThresh))
		case n < p.highThresh:
			c, bandFrac = bandAt(p.gradMid, remap(n, p.lowThresh, p.highThresh))
		default:
			c, bandFrac = bandAt(p.gradHigh, remap(n, p.highThresh, p.noiseMax))
		}

		// Layer 2: region tint — multiply blend, rings showing through.
		r := region.FBM(u*p.regionFreq, v*p.regionFreq, s.RegionOctaves)
		if w := smoothstep(p.regionThresh, p.regionThresh+0.08, r); w > 0 {
			c = palette.Lerp(c, mulBlend(c, p.regionTint), w*p.regionStrength)
		}

		// Layer 3: vertical stripe.
		st := p.stripeAt(u)
		c = st.apply(c)

		// Layer 3b: relief shading over the assembled surface.
		if s.Relief {
			c = s.shadeRelief(field, p, u, v, bandFrac, c)
		}

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
	bands := 20 + prm.IntN(21)

	p := plan{
		freq:       4 + prm.Float64()*2,
		lowThresh:  -thresh,
		highThresh: thresh,
		noiseMin:   -span,
		noiseMax:   span,

		gradLow: gradient.Sample(gradient.CosineBetween(warm, lightest), bands).
			Shuffled(ctx.RNG(streamShuffleLow)),
		gradMid: gradient.Sample(gradient.CosineBetween(lightest, cool), bands),
		gradHigh: gradient.Sample(gradient.CosineBetween(cool, warm), bands).
			Shuffled(ctx.RNG(streamShuffleHigh)),

		regionFreq:     2.2 + prm.Float64(),
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
	if s.DisableStripes {
		return []stripe{{end: aspect + 1, mode: modeNone, grainMul: 1}}
	}
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

// bandAt returns the band color for t plus the fractional position within
// the band.
func bandAt(d gradient.Discrete, t float64) (palette.Color, float64) {
	idx, frac := gradient.Locate(t, len(d))
	return d[idx], frac
}

// shadeRelief treats the contour noise as a height field and shades the
// color: Lambertian hillshade from the field gradient, a paper-cut shadow
// just below each band edge with a lit rim just above it, and a subtle
// specular highlight. All in normalized coordinates — resolution
// independent like everything else.
func (s *Sketch) shadeRelief(field *noise.Perlin, p plan, u, v, bandFrac float64, c palette.Color) palette.Color {
	rp := s.ReliefParams
	h := func(x, y float64) float64 { return field.FBM(x*p.freq, y*p.freq, s.Octaves) }
	hx := (h(u+reliefEps, v) - h(u-reliefEps, v)) / (2 * reliefEps)
	hy := (h(u, v+reliefEps) - h(u, v-reliefEps)) / (2 * reliefEps)

	// Surface normal and normalized light/half vectors.
	nx, ny, nz := -rp.Slope*hx, -rp.Slope*hy, 1.0
	nl := math.Sqrt(nx*nx + ny*ny + nz*nz)
	nx, ny, nz = nx/nl, ny/nl, nz/nl
	lx, ly, lz := rp.LightDir[0], rp.LightDir[1], rp.LightDir[2]
	ll := math.Sqrt(lx*lx + ly*ly + lz*lz)
	lx, ly, lz = lx/ll, ly/ll, lz/ll

	diffuse := math.Max(0, nx*lx+ny*ly+nz*lz)
	shade := rp.Ambient + (1-rp.Ambient)*diffuse

	// Paper-cut edges: the band above (higher noise) casts a shadow on the
	// pixels just below its edge (bandFrac → 1); its own lower edge catches
	// light (bandFrac → 0).
	if rp.EdgeWidth > 0 {
		shade *= 1 - rp.Shadow*math.Max(0, 1-(1-bandFrac)/rp.EdgeWidth)
		shade *= 1 + rp.Rim*math.Max(0, 1-bandFrac/rp.EdgeWidth)
	}

	// Blinn-Phong specular, faded where the surface is unlit.
	hz := lz + 1 // half vector = light + view (0,0,1)
	hl := math.Sqrt(lx*lx + ly*ly + hz*hz)
	spec := rp.Spec * math.Pow(math.Max(0, nx*lx/hl+ny*ly/hl+nz*hz/hl), rp.Shininess) * diffuse

	return palette.Color{
		R: c.R*shade + spec,
		G: c.G*shade + spec,
		B: c.B*shade + spec,
	}.Clamp()
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
