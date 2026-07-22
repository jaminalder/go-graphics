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

// colorway is one full gradient trio — the contour pattern rendered in one
// color register. Zones of the region field select between colorways.
type colorway struct {
	low, mid, high gradient.Discrete
}

// plan is everything Render derives from the seed before the pixel loop.
type plan struct {
	freq       float64
	lowThresh  float64
	highThresh float64
	noiseMin   float64
	noiseMax   float64
	bands      int

	// Colorway zones selected by the region field value: deep register on
	// the low tail, bright register on the high tail, mid between, with a
	// smoothstep crossfade of half-width zoneBlend at each threshold.
	zones      [3]colorway // deep, mid, bright
	regionFreq float64
	zoneT1     float64
	zoneT2     float64
	zoneBlend  float64

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
		// Layer 1: contour banding — which band, and where within it
		// (bandFrac feeds relief shading).
		n := field.FBM(u*p.freq, v*p.freq, s.Octaves)
		var (
			slot int // 0 low, 1 mid, 2 high
			t    float64
		)
		switch {
		case n < p.lowThresh:
			slot, t = 0, remap(n, p.noiseMin, p.lowThresh)
		case n < p.highThresh:
			slot, t = 1, remap(n, p.lowThresh, p.highThresh)
		default:
			slot, t = 2, remap(n, p.highThresh, p.noiseMax)
		}
		idx, bandFrac := gradient.Locate(t, p.bands)

		// Layer 2: colorway zones — the region field picks which color
		// register renders this band. Same band index in every zone, so
		// the ring geometry flows through zone borders.
		r := region.FBM(u*p.regionFreq, v*p.regionFreq, s.RegionOctaves)
		wDeep := 1 - smoothstep(p.zoneT1-p.zoneBlend, p.zoneT1+p.zoneBlend, r)
		wBright := smoothstep(p.zoneT2-p.zoneBlend, p.zoneT2+p.zoneBlend, r)
		c := p.zoneColor(slot, idx, wDeep, 1-wDeep-wBright, wBright)

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

	// Palette roles by luminance: the bright colorway uses the lightest
	// color plus two random mid colors; the mid and deep colorways step
	// down the luminance ladder from there.
	byLum := append([]palette.Color(nil), ctx.Palette.Colors...)
	sort.SliceStable(byLum, func(i, j int) bool {
		return byLum[i].Luminance() < byLum[j].Luminance()
	})
	lightest := byLum[len(byLum)-1]
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
		bands:      bands,

		regionFreq: 2.2 + prm.Float64(),
		zoneT1:     -0.18 + prm.Float64()*0.12,
		zoneT2:     0.06 + prm.Float64()*0.12,
		zoneBlend:  0.03 + prm.Float64()*0.04,

		grainAmt: 0.03 + prm.Float64()*0.03,
	}

	// One band permutation per gradient slot, shared by all colorways, so
	// ring N is the same ring in every zone — only its color register
	// changes across zone borders.
	permLow := ctx.RNG(streamShuffleLow).Perm(bands)
	permHigh := ctx.RNG(streamShuffleHigh).Perm(bands)
	build := func(warm, pale, cool palette.Color) colorway {
		return colorway{
			low:  gradient.Sample(gradient.CosineBetween(warm, pale), bands).Permuted(permLow),
			mid:  gradient.Sample(gradient.CosineBetween(pale, cool), bands),
			high: gradient.Sample(gradient.CosineBetween(cool, warm), bands).Permuted(permHigh),
		}
	}
	// Every register spans dark→light — the reference keeps light ring
	// accents even inside its deepest zones; narrow-luminance registers
	// turn murky.
	n := len(byLum)
	if warm == byLum[1] && cool == byLum[2] {
		warm, cool = cool, warm // keep bright distinct from the mid register
	}
	p.zones = [3]colorway{
		build(byLum[0], byLum[n-2], byLum[1]), // deep register
		build(byLum[1], byLum[n-1], byLum[2]), // mid register
		build(warm, lightest, cool),           // bright register
	}

	p.stripes = s.planStripes(ctx)
	return p
}

// zoneColor blends the same band across the three zone colorways.
func (p *plan) zoneColor(slot, idx int, wDeep, wMid, wBright float64) palette.Color {
	var c palette.Color
	for z, w := range [3]float64{wDeep, wMid, wBright} {
		if w <= 0 {
			continue
		}
		cw := p.zones[z]
		var g gradient.Discrete
		switch slot {
		case 0:
			g = cw.low
		case 1:
			g = cw.mid
		default:
			g = cw.high
		}
		c.R += w * g[idx].R
		c.G += w * g[idx].G
		c.B += w * g[idx].B
	}
	return c
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
