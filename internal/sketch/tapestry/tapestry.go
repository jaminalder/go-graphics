// Package tapestry implements sketch 002: contour terrain with
// terrain-owned colorways (spec: docs/sketches/002-tapestry.md).
//
// One fBm field split into five value bands (deep-basin / basin / cloud /
// peak / high-peak), each colored by its own HSL-interpolated palette
// gradient — every hill is uniformly one coloring and color boundaries are
// contour lines. Optional layers: full-height vertical stripes, 3D relief
// shading, and deterministic grain. Each seed draws its composition
// parameters from bounded ranges, so seeds are distinct but stay
// presentable.
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
	streamShuffleB0 = 1
	streamShuffleB1 = 2
	streamStripes   = 3
	streamParams    = 4
	streamShuffleB3 = 5
	streamShuffleB4 = 6
	streamGrain     = 7 // height-grain draws; separate stream so the
	// base composition is identical with and without HeightGrain
)

// Sketch holds the structural knobs. The per-seed variation draws happen in
// Render (see the spec); these fields bound or fix that variation.
type Sketch struct {
	Octaves     int     // contour fBm max octave index
	GrainRes    float64 // grain lattice cells per canvas unit
	StreakRatio float64 // streak-grain cell elongation (y cells = GrainRes/StreakRatio)

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

	// HeightGrain boosts the grain strongly within one height range of the
	// terrain (a per-seed window of 15–30% of the noise value range), so
	// the same elevation band is textured across the whole image — like
	// scree at a certain altitude. Uses its own RNG stream: the
	// composition is identical with and without it, only grain differs.
	HeightGrain bool
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
		Octaves:      2,
		GrainRes:     1400,
		StreakRatio:  6,
		ReliefParams: DefaultReliefParams(),
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
//
// The noise range is split into five value bands, each with its own
// colorway gradient. Because band membership is a property of the terrain
// itself, every "hill" (or basin) is uniformly one coloring and the
// boundaries between colorings are contour lines — nothing reads as an
// overlay. grads[2] is the smooth cloud band; the others are shuffled.
type plan struct {
	freq  float64
	span  float64    // noise mapped over ±span
	cuts  [4]float64 // band boundaries: -t2, -t1, +t1, +t2
	bands int
	grads [5]gradient.Discrete

	stripes  []stripe
	grainAmt float64

	// Height grain (only set when Sketch.HeightGrain): folded noise values
	// inside [hgLo, hgHi] get the grain amplified by up to 1+hgBoost,
	// ramping over hgBlend at both window edges.
	hgLo    float64
	hgHi    float64
	hgBlend float64
	hgBoost float64
}

// fold reflects n back into [-span, span] (triangle wave). Without it,
// noise values beyond the mapped range clamp to the outer gradient's last
// color, leaving large flat single-color caps on the tallest hills; with
// it, the rings continue inward across the very tops.
func fold(n, span float64) float64 {
	r := math.Mod(n+span, 4*span)
	if r < 0 {
		r += 4 * span
	}
	if r > 2*span {
		r = 4*span - r
	}
	return r - span
}

// bandOf returns the band index for noise value n and the value range of
// that band.
func (p *plan) bandOf(n float64) (band int, lo, hi float64) {
	edges := [6]float64{-p.span, p.cuts[0], p.cuts[1], p.cuts[2], p.cuts[3], p.span}
	for b := 0; b < 4; b++ {
		if n < edges[b+1] {
			return b, edges[b], edges[b+1]
		}
	}
	return 4, edges[4], edges[5]
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	if len(ctx.Palette.Colors) < 4 {
		return nil, fmt.Errorf("tapestry: palette %q needs at least 4 colors", ctx.Palette.Slug)
	}
	p := s.plan(ctx)
	field := noise.New(ctx.Seed)

	img := render.Raster(ctx.Width, ctx.Height, func(u, v float64) palette.Color {
		// Layers 1+2: contour banding with terrain-owned colorways —
		// which value band this pixel's noise falls in decides both the
		// ring and its color register. bandFrac feeds relief shading.
		n := fold(field.FBM(u*p.freq, v*p.freq, s.Octaves), p.span)
		band, lo, hi := p.bandOf(n)
		idx, bandFrac := gradient.Locate(remap(n, lo, hi), p.bands)
		c := p.grads[band][idx]

		// Layer 3: vertical stripe.
		st := p.stripeAt(u)
		c = st.apply(c)

		// Layer 3b: relief shading over the assembled surface.
		if s.Relief {
			c = s.shadeRelief(field, p, u, v, bandFrac, c)
		}

		// Layer 4: grain, boosted within the height window when enabled.
		gx := int64(u * s.GrainRes)
		gy := int64(v * s.GrainRes)
		if st.streak {
			gy = int64(v * s.GrainRes / s.StreakRatio)
		}
		amt := p.grainAmt * st.grainMul
		if s.HeightGrain {
			w := smoothstep(p.hgLo-p.hgBlend, p.hgLo+p.hgBlend, n) *
				(1 - smoothstep(p.hgHi-p.hgBlend, p.hgHi+p.hgBlend, n))
			amt *= 1 + w*p.hgBoost
		}
		g := (noise.Hash01(ctx.Seed, gx, gy) - 0.5) * amt
		return palette.Color{R: c.R + g, G: c.G + g, B: c.B + g}.Clamp()
	})
	return img, nil
}

// plan makes all per-seed draws. Draw order is fixed — changing it changes
// every existing seed's image (breaks goldens deliberately, never silently).
func (s *Sketch) plan(ctx sketch.Context) plan {
	prm := ctx.RNG(streamParams)

	// Palette roles by luminance. The two darkest colors anchor two hill
	// families; each family's light partner is the remaining palette color
	// nearest in hue, so one hill reads as exactly two colors (plus their
	// shades) — no third hue appears along the HSL arc.
	byLum := append([]palette.Color(nil), ctx.Palette.Colors...)
	sort.SliceStable(byLum, func(i, j int) bool {
		return byLum[i].Luminance() < byLum[j].Luminance()
	})
	nL := len(byLum)
	light0 := byLum[nL-1]

	darkA, darkB := byLum[0], byLum[1]
	partners := append([]palette.Color(nil), byLum[2:]...)
	lightA := takeNearestHue(&partners, darkA)
	lightB := takeNearestHue(&partners, darkB)

	t1 := 0.08 + prm.Float64()*0.06 // cloud band half-width
	t2 := 0.28 + prm.Float64()*0.10 // deep-band cutoffs
	span := 0.55 + prm.Float64()*0.15
	bands := 20 + prm.IntN(21)

	p := plan{
		freq:  4 + prm.Float64()*2,
		span:  span,
		cuts:  [4]float64{-t2, -t1, t1, t2},
		bands: bands,

		grainAmt: 0.03 + prm.Float64()*0.03,
	}

	// Which family colors the basins vs the peaks varies per seed. The
	// outer band shows the family in full depth; the inner band is a
	// lighter register of the same two colors. The cloud stays smooth
	// between the lightest color and its nearest-hue companion.
	if prm.Float64() < 0.5 {
		darkA, lightA, darkB, lightB = darkB, lightB, darkA, lightA
	}
	cloudPartner := nearestHue(light0, byLum[nL-3:nL-1])
	sample := func(c1, c2 palette.Color) gradient.Discrete {
		return gradient.Sample(gradient.HSLBetween(c1, c2), bands)
	}
	lighter := func(d, l palette.Color) palette.Color { return palette.LerpHSL(d, l, 0.45) }
	p.grads = [5]gradient.Discrete{
		sample(darkA, lightA).Shuffled(ctx.RNG(streamShuffleB0)),                  // deep basin: family A, full depth
		sample(lighter(darkA, lightA), lightA).Shuffled(ctx.RNG(streamShuffleB1)), // basin: family A, light register
		sample(light0, cloudPartner),                                              // smooth cloud
		sample(lighter(darkB, lightB), lightB).Shuffled(ctx.RNG(streamShuffleB3)), // peak: family B, light register
		sample(darkB, lightB).Shuffled(ctx.RNG(streamShuffleB4)),                  // high peak: family B, full depth
	}

	p.stripes = s.planStripes(ctx)

	if s.HeightGrain {
		rg := ctx.RNG(streamGrain)
		width := (0.15 + rg.Float64()*0.15) * 2 * span // 15–30% of the value range
		center := (rg.Float64()*2 - 1) * (span - width/2)
		p.hgLo = center - width/2
		p.hgHi = center + width/2
		p.hgBlend = 0.04
		p.hgBoost = 2.5 + rg.Float64()*3.0
	}
	return p
}

// smoothstep is the standard cubic step from 0 (x≤lo) to 1 (x≥hi).
func smoothstep(lo, hi, x float64) float64 {
	t := math.Min(1, math.Max(0, (x-lo)/(hi-lo)))
	return t * t * (3 - 2*t)
}

// hueDist is the angular hue distance between two colors in degrees.
// Near-gray colors match any hue.
func hueDist(a, b palette.Color) float64 {
	ha, sa, _ := a.HSL()
	hb, sb, _ := b.HSL()
	if sa < 0.1 || sb < 0.1 {
		return 0
	}
	d := math.Abs(ha - hb)
	return math.Min(d, 360-d)
}

// nearestHue returns the candidate closest in hue to c.
func nearestHue(c palette.Color, candidates []palette.Color) palette.Color {
	best := candidates[0]
	for _, cand := range candidates[1:] {
		if hueDist(c, cand) < hueDist(c, best) {
			best = cand
		}
	}
	return best
}

// takeNearestHue removes and returns the candidate closest in hue to c.
func takeNearestHue(candidates *[]palette.Color, c palette.Color) palette.Color {
	bestI := 0
	for i, cand := range (*candidates)[1:] {
		if hueDist(c, cand) < hueDist(c, (*candidates)[bestI]) {
			bestI = i + 1
		}
	}
	best := (*candidates)[bestI]
	*candidates = append((*candidates)[:bestI], (*candidates)[bestI+1:]...)
	return best
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
