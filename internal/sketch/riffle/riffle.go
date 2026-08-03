// Package riffle implements sketch 011: a small river seen from directly
// above (spec: docs/sketches/011-riffle.md).
//
// Every sketch before this one draws marks on a sheet, or divides a sheet
// into regions. This one draws a *material*. There is no plan of objects and
// no partition: the whole picture is one pure function of position, and
// every cue in it — the foam line, the eddy, the glassy tongue, the caustic
// net on the gravel — is one of three scalar fields showing through another.
//
// The fields are the bed (and therefore the depth), the velocity, and the
// surface. The third is not stored anywhere: each pixel walks *upstream*
// along the velocity field and asks what arrived with its water. That walk
// is line integral convolution, and it is the single most legible "this is
// moving water" cue there is.
//
// It is an approximation everywhere, deliberately. Nothing here is solved;
// everything is a closed form chosen because it reads as the thing it stands
// for. The places where the accurate answer was available and the legible
// one was taken instead are set out in the spec.
package riffle

import (
	"image"
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// RNG stream ids (see sketch.Context.RNG).
const (
	streamTraits  = 1 // which point of the output space this seed is
	streamReach   = 2 // the trait levels made numeric
	streamChannel = 3 // the plan form of the river
	streamRocks   = 4 // what is in the way
)

// Noise seed salts, so the fields are independent of one another: moving a
// rock must not redraw the pebbles.
const (
	saltBed     = 0x626564   // "bed"
	saltFlow    = 0x666c6f77 // "flow"
	saltTex     = 0x746578   // "tex"
	saltFoam    = 0x666f616d // "foam"
	saltCaustic = 0x63617573 // "caus"
	saltPebble  = 0x70656231 // "peb1"
	saltBubble  = 0x62756231 // "bub1"
	saltRipple  = 0x72697031 // "rip1"
)

// Sketch holds the tunables that are facts about the medium rather than
// about this stretch of river; everything else comes from a trait.
type Sketch struct {
	pin settings

	traits *trait.Options
	knobs  *opt.Set
}

// New returns the sketch with its defaults.
func New() *Sketch {
	s := &Sketch{
		pin:    defaults(),
		traits: trait.NewOptions(schema),
	}
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "riffle" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "a small river from above: riffles, foam lines, eddies and caustics on the bed"
}

// Schema implements sketch.Traited.
func (s *Sketch) Schema() trait.Schema { return schema }

// Traits implements sketch.Traited.
func (s *Sketch) Traits(ctx sketch.Context) trait.Set {
	return s.traits.Resolve(ctx.RNG(streamTraits))
}

// inks are the colours one render works from, all selected from the palette
// it was given: a warm pair for the gravel, something cool and dark enough
// to be a metre of water, and a near-white for foam.
type inks struct {
	gravelA, gravelB palette.Color
	dry              palette.Color
	sky, glint, foam palette.Color
	lit              palette.Color // gravel with the sun on it
	gravelMean       palette.Color // what the bed becomes when depth blurs it

	bodyLin [3]float64 // the water's own colour, in linear light
	k       [3]float64 // per-channel extinction
}

// plan is one river, drawn from the seed before a pixel is touched.
type plan struct {
	aspect float64
	set    settings
	ch     channel
	rocks  []rock
	ink    inks

	phase float64 // where the pool–riffle sequence starts

	nBed, nFlow, nTex, nFoam, nCaus, nRip *noise.Perlin
	causticSeed, pebbleSeed, bubbleSeed   uint64
	foamLo, foamHi, foamOn, foamFull      float64
	halfX, halfY, halfZ, sunPower         float64
	lightX, lightY                        float64
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	p, err := s.build(ctx)
	if err != nil {
		return nil, err
	}
	return sketch.Raster(ctx, p.pixel), nil
}

// build resolves the traits, lays out the river and derives its colours.
func (s *Sketch) build(ctx sketch.Context) (*plan, error) {
	tr := s.Traits(ctx)
	pal, err := colours(tr.Get(dimColourway), ctx.Palette)
	if err != nil {
		return nil, err
	}

	set := s.settingsFor(tr, ctx.RNG(streamReach))

	p := &plan{
		aspect: float64(ctx.Width) / float64(ctx.Height),
		set:    set,
		nBed:   noise.New(ctx.Seed ^ saltBed),
		nFlow:  noise.New(ctx.Seed ^ saltFlow),
		nTex:   noise.New(ctx.Seed ^ saltTex),
		nFoam:  noise.New(ctx.Seed ^ saltFoam),
		nCaus:  noise.New(ctx.Seed ^ saltCaustic),
		nRip:   noise.New(ctx.Seed ^ saltRipple),

		causticSeed: ctx.Seed ^ saltCaustic,
		pebbleSeed:  ctx.Seed ^ saltPebble,
		bubbleSeed:  ctx.Seed ^ saltBubble,
	}

	rng := ctx.RNG(streamChannel)
	p.phase = rng.Float64() * 2 * math.Pi
	p.ch = channel{
		centre:   p.aspect * rnd.Uniform(rng, 0.42, 0.58),
		half:     set.channelWidth * p.aspect,
		bend:     set.bend * p.aspect,
		meander:  set.meander,
		phase:    rng.Float64() * 2 * math.Pi,
		bend2:    set.bend * p.aspect * rnd.Uniform(rng, 0.22, 0.45),
		meander2: set.meander * rnd.Uniform(rng, 2.1, 3.4),
		phase2:   rng.Float64() * 2 * math.Pi,
		taper:    set.taper,
	}
	p.placeRocks(ctx.RNG(streamRocks))

	// Foam thresholds are relative to the reach's *own* nominal Froude
	// number, not absolute. Absolute thresholds are what flooded the first
	// version: depth and speed are in arbitrary units, so a number that left
	// a pool clean turned a riffle entirely white and one that suited the
	// riffle left the rapid untouched. Measured against the reach's own
	// average, "breaking" means what it should — locally much faster or much
	// shallower than the water around it — and how much of that a level
	// tolerates is what the foam knob says.
	nominal := set.speed / math.Sqrt(math.Max(set.depth*0.65, 0.05))
	p.foamLo = nominal * (3.0 - 1.7*set.foam)
	p.foamHi = p.foamLo * 1.6
	p.foamOn, p.foamFull = 0.10, 0.42

	// The sun, as a Blinn–Phong half vector with the viewer straight down.
	az := set.sun * math.Pi / 180
	alt := set.sunHeight * math.Pi / 180
	lx, ly, lz := math.Cos(az)*math.Cos(alt), math.Sin(az)*math.Cos(alt), math.Sin(alt)
	p.lightX, p.lightY = lx, ly
	hx, hy, hz := lx, ly, lz+1
	hn := 1 / math.Sqrt(hx*hx+hy*hy+hz*hz)
	p.halfX, p.halfY, p.halfZ = hx*hn, hy*hn, hz*hn
	p.sunPower = mathx.Rescale(set.sunHeight, 10, 80, 0.55, 1.0)

	p.ink = mixInks(pal, set)
	return p, nil
}

// warmth is R−B: positive for the ochres and browns a river bed is made of,
// negative for the blues and greens a metre of water turns into. Crude, and
// it does the job on every palette in the curated list — a hue-wheel
// distance buys nothing here and costs a conversion per colour.
func warmth(c palette.Color) float64 { return c.R - c.B }

// mixInks selects the render's colours from the palette. Nothing is
// invented: the pigments are the artist's, and the only liberties taken are
// lightening, darkening, desaturating and mixing (decision 39).
func mixInks(pal palette.Palette, set settings) inks {
	byLum := palette.ByLuminance(pal.Colors)
	lightest := byLum[len(byLum)-1]

	// The gravel comes off the warm end, but never off the lightest colour:
	// that one is the sky and the foam, and a bed painted in it leaves the
	// picture with no white left.
	pool := byLum[:len(byLum)-1]
	if len(pool) < 2 {
		pool = byLum
	}
	a, b := pool[0], pool[0]
	bestA, bestB := math.Inf(-1), math.Inf(-1)
	for _, c := range pool {
		w := warmth(c)
		switch {
		case w > bestA:
			a, b, bestA, bestB = c, a, w, bestA
		case w > bestB:
			b, bestB = c, w
		}
	}
	if a.Luminance() < b.Luminance() {
		a, b = b, a
	}

	// The body is the coolest colour on the sheet, or — for a peat-stained
	// or silty river — the darkest warm one.
	body := byLum[0]
	if set.bodyWarm {
		best := math.Inf(-1)
		for _, c := range byLum[:max(1, len(byLum)/2+1)] {
			if w := warmth(c); w > best {
				body, best = c, w
			}
		}
	} else {
		best := math.Inf(1)
		for _, c := range byLum {
			if w := warmth(c); w < best {
				body, best = c, w
			}
		}
	}
	h, sat, l := body.HSL()
	body = palette.FromHSL(h, sat*1.08, l*set.bodyDark)
	if set.milk > 0 {
		body = body.Lighten(set.milk).Desaturate(set.milk * 0.45)
	}

	ink := inks{
		gravelA:    a.Desaturate(0.12),
		gravelB:    b.Desaturate(0.12),
		dry:        a.Lighten(0.34).Desaturate(0.26),
		sky:        lightest.Lighten(0.18),
		glint:      lightest.Lighten(0.85),
		foam:       lightest.Lighten(0.7),
		lit:        palette.Lerp(a.Lighten(0.42), lightest, 0.35),
		gravelMean: palette.Lerp(b, a, 0.55),
	}
	ink.bodyLin = [3]float64{
		palette.SRGBToLinear(body.R),
		palette.SRGBToLinear(body.G),
		palette.SRGBToLinear(body.B),
	}

	// Per-channel extinction derived from the body colour itself, so the two
	// agree: the channel the water keeps is the channel light survives in.
	peak := math.Max(ink.bodyLin[0], math.Max(ink.bodyLin[1], ink.bodyLin[2]))
	if peak <= 0 {
		peak = 1
	}
	for i, v := range ink.bodyLin {
		ink.k[i] = set.extinction * (0.45 + 1.15*(1-v/peak))
	}
	return ink
}

// settingsFor resolves one render's numbers: what the six dimensions drew,
// with any pinned flag laid over the top. A caller can move one number
// without restating the other thirty, and a caller who says nothing still
// gets a coherent river.
func (s *Sketch) settingsFor(tr trait.Set, rng *rand.Rand) settings {
	set := defaults()
	reachLevel(tr.Get(dimReach), &set, rng)
	channelLevel(tr.Get(dimChannel), &set, rng)
	bouldersLevel(tr.Get(dimBoulders), &set, rng)
	waterLevel(tr.Get(dimWater), &set, rng)
	lightLevel(tr.Get(dimLight), &set, rng)
	s.override(&set)
	return set
}
