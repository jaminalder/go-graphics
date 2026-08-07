// Package glaze implements sketch 015: a faceted stone bed seen through a
// painted translucent veil whose structure is a nested fBM domain warp.
package glaze

import (
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/sketch/scree"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// Sketch lays a warped translucent veil over a planned scree bed. The two
// meet before rasterisation; there is no intermediate image and no alpha.
type Sketch struct {
	bed *scree.Sketch

	WaterSeed  uint64
	WaterScale float64
	VeilWarp   float64
	VeilNest   float64
	Stretch    float64
	DriftScale float64
	Opacity    float64
	Body       float64
	Drift      float64
	Glint      float64
	GrainWater float64
	Coverage   float64
	CoverScale float64
	Density    float64
	Gather     float64
	Terraces   int

	veilName, castName string
	manner             manner
	cast               cast

	knobs *opt.Set
}

// New returns the default glaze: a smooth blue veil over scree's own bed.
func New() *Sketch {
	s := &Sketch{
		bed:       scree.New(),
		WaterSeed: 42,
		veilName:  "glaze",
		castName:  "cool",
		manner:    mannerGlaze,
		cast:      castCool,
	}
	// The public fields start on the default manner's numbers so that --help
	// shows real values and a caller constructing the sketch directly gets a
	// usable water without going through the flag set.
	s.adopt(preset(mannerGlaze, s.WaterSeed))
	s.declare()
	return s
}

func (s *Sketch) adopt(c veilConfig) {
	s.WaterScale, s.VeilWarp, s.VeilNest = c.scale, c.warpStrength, c.nestStrength
	s.Stretch, s.Opacity, s.Body = c.stretch, c.opacity, c.body
	s.DriftScale = c.driftScale
	s.Drift, s.Glint, s.GrainWater = c.drift, c.glint, c.grain
	s.Coverage, s.CoverScale = c.coverage, c.coverScale
	s.Density, s.Gather = c.density, c.gather
	s.Terraces = c.terraces
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "glaze" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "a stone bed under a painted veil of warped blue water"
}

// The output space is the bed's. Which water is laid over it is a choice of
// material, not a dimension a seed samples.
func (s *Sketch) Schema() trait.Schema { return s.bed.Schema() }

// Traits implements sketch.Traited.
func (s *Sketch) Traits(ctx sketch.Context) trait.Set { return s.bed.Traits(ctx) }

// TraitSuffix implements sketch.Traited.
func (s *Sketch) TraitSuffix(set trait.Set) string { return s.bed.TraitSuffix(set) }

// config resolves the manner's numbers and then applies whatever the caller
// actually set on the command line as overrides on them.
func (s *Sketch) config() veilConfig {
	c := preset(s.manner, s.WaterSeed)
	c.seed = s.WaterSeed
	set := func(name string, dst *float64, value float64) {
		if s.knobs.WasSet(name) {
			*dst = value
		}
	}
	set("water-scale", &c.scale, s.WaterScale)
	set("veil-warp", &c.warpStrength, s.VeilWarp)
	set("veil-nest", &c.nestStrength, s.VeilNest)
	set("stretch", &c.stretch, s.Stretch)
	set("opacity", &c.opacity, s.Opacity)
	set("body", &c.body, s.Body)
	set("drift", &c.drift, s.Drift)
	set("drift-scale", &c.driftScale, s.DriftScale)
	set("glint", &c.glint, s.Glint)
	set("grain-water", &c.grain, s.GrainWater)
	set("coverage", &c.coverage, s.Coverage)
	set("cover-scale", &c.coverScale, s.CoverScale)
	set("density", &c.density, s.Density)
	set("gather", &c.gather, s.Gather)
	if s.knobs.WasSet("terraces") {
		c.terraces = s.Terraces
	}
	return c
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	bed, err := s.bed.NewBed(ctx)
	if err != nil {
		return nil, err
	}
	cfg := s.config()
	water := newVeil(cfg)
	pigment := castColour(s.cast, ctx.Palette.Colors)
	medium := newFilter(pigment, cfg.body)
	glint := pigment.Lighten(0.84)

	return sketch.Raster(ctx, func(u, v float64) palette.Color {
		w := water.At(u, v)

		// The veil pulls the plane under it. At the glaze's drift this is a
		// gentle refraction; at the marble's it stretches whole stones into
		// currents, which is the drawing rather than an artefact of it.
		col := bed.At(u-cfg.drift*w.dx, v-cfg.drift*w.dy)

		// Absorption in linear light, not a lerp toward blue: the bed's own
		// colour comes *through* the water instead of being averaged away.
		col = medium.over(col, w.load)

		if cfg.glint > 0 {
			col = palette.Lerp(col, glint, mathx.Clamp01(w.filament*cfg.glint))
		}
		return col.Clamp()
	}), nil
}

// cast is which pigment the palette supplies for the water.
type cast uint8

const (
	castCool cast = iota
	castDeep
	castPale
	castSmoke
)

// castColour selects and manipulates a palette member rather than inventing a
// hue, so the artwork keeps the palette's provenance.
func castColour(c cast, colors []palette.Color) palette.Color {
	base := coolest(colors)
	ordered := palette.ByLuminance(colors)
	switch c {
	case castDeep:
		return palette.Lerp(base, ordered[0], 0.55)
	case castPale:
		return base.Lighten(0.42)
	case castSmoke:
		// Desaturating alone all but switches the water off: the extinction
		// is taken from the pigment's channel *ratios*, and a neutral has
		// none. Smoke therefore gives up in hue what it takes back in depth.
		return palette.Lerp(base.Desaturate(0.45), ordered[0], 0.35)
	default:
		return base
	}
}

// marineHue is the hue this sketch is looking for in a palette, in degrees:
// the blue-cyan a body of water is usually painted.
const marineHue = 205

// coolest picks the palette member that will make the most convincing water.
//
// 012 scores this on the raw channels, which is right for a *clear* film that
// only has to be cool and bright. It is wrong here: the extinction is taken
// from the pigment's hue, so a near-white with a faint blue bias scores well
// and then filters nothing at all — hokusai's cream beat its own navy, and
// the great wave came out bone dry. The score is therefore chroma around a
// marine hue, with lightness only as a mild tie-break.
func coolest(colors []palette.Color) palette.Color {
	best := colors[0]
	bestScore := math.Inf(-1)
	for _, c := range colors {
		h, s, l := c.HSL()
		aim := math.Max(0, 1-hueDistance(h, marineHue)/110)
		score := (s+0.30)*aim - 0.20*math.Abs(l-0.45)
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

// hueDistance is the shortest arc between two hues, in degrees.
func hueDistance(a, b float64) float64 {
	d := math.Abs(math.Mod(a-b+540, 360) - 180)
	return d
}
