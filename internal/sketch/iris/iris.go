// Package iris implements sketch 014: an abstract iris. The plane is remapped
// radially before any field is evaluated, so ordinary Perlin fBM sampled in
// that space runs as fibres from the pupil to the limbus. Nested anisotropic
// domain warps — tangential for meander, radial for interruption — then fold
// those fibres into the stroma. There is no lighting model: every value in the
// image is the field read through a radial colour ramp.
package iris

import (
	"fmt"
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

const (
	seedQX     = 0x697269732d7178
	seedQY     = 0x697269732d7179
	seedRX     = 0x697269732d7278
	seedRY     = 0x697269732d7279
	seedValue  = 0x697269732d7661
	seedComb   = 0x697269732d636f
	seedSector = 0x697269732d7365
	seedCrypt  = 0x697269732d6372

	// Named RNG streams. Traits are resolved before any number is drawn, so
	// adding a numeric range later never moves an existing seed's traits.
	streamTraits = 1
	streamRecipe = 2

	// ringBase is the noise-space radius the inner edge of the stroma is
	// mapped to. Its circumference (2*pi*ringBase) sets how many broad
	// fibre bundles fit around the pupil before octaves subdivide them.
	ringBase = 3.4

	// combBase is the noise-space radius of the comb field, which ignores
	// the radial coordinate entirely and therefore reads as straight spokes.
	combBase = 4.6

	rRoleScale   = 1.23
	valueScale   = 1.09
	edgeSoftness = 0.0032
)

type structure uint8

const (
	structureFiber structure = iota
	structureFilament
	structureStrata
	structureCrypt
)

type tint uint8

const (
	tintPlain tint = iota
	tintSector
)

type ground uint8

const (
	groundLight ground = iota
	groundDark
	groundPalette
)

type settings struct {
	scale, gain, lacunarity float64
	stretch, twist          float64
	fiber, radial, nested   float64
	depth, gleam, warmth    float64
	bands, cells            float64
	limbus, pupil           float64
	octaves                 int
	structure               structure
	tint                    tint
	ground                  ground
}

// Sketch holds iris's public controls. Use New for useful defaults.
type Sketch struct {
	Scale, Gain, Lacunarity float64
	Stretch, Twist          float64
	Fiber, Radial, Nested   float64
	Depth, Gleam, Warmth    float64
	Bands, Cells            float64
	Limbus, Pupil           float64
	Octaves                 int

	knobs  *opt.Set
	traits *trait.Options
}

// New returns iris with the numeric defaults a pinned flag falls back to.
// Left alone, every one of these is drawn from the seed's traits instead.
func New() *Sketch {
	s := &Sketch{
		Scale:      1.9,
		Gain:       0.5,
		Lacunarity: 2.05,
		Stretch:    0.5,
		Twist:      0,
		Fiber:      0.55,
		Radial:     0.2,
		Nested:     0.5,
		Depth:      0.45,
		Gleam:      0.16,
		Warmth:     0.35,
		Bands:      14,
		Cells:      12,
		Limbus:     0.42,
		Pupil:      0.28,
		Octaves:    5,
	}
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "iris" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "an abstract iris: nested fBM warped in a radially remapped plane"
}

type plan struct {
	set       settings
	q         [2]*noise.Perlin
	r         [2]*noise.Perlin
	value     *noise.Perlin
	combField *noise.Perlin
	sector    *noise.Perlin
	crypt     uint64
	centreU   float64
	centreV   float64

	body        valueRamp
	accent      palette.Color
	light, dark palette.Color
	limbal      palette.Color
	pupil       palette.Color
	groundColor palette.Color
}

// valueRamp is a palette laid out on a value axis: every colour sits at its
// own relative luminance rather than at an equal share of the ramp. A palette
// with four near-blacks and one cream therefore spends most of the ramp in the
// darks, which is where its colours actually are; without this the stroma
// blows out to the single light colour on any palette that has one.
type valueRamp struct {
	colors []palette.Color
	at     []float64
}

func newValueRamp(colors []palette.Color) valueRamp {
	at := make([]float64, len(colors))
	low, high := colors[0].Luminance(), colors[len(colors)-1].Luminance()
	span := high - low
	for i, c := range colors {
		if span <= 1e-6 {
			at[i] = float64(i) / float64(len(colors)-1)
			continue
		}
		// Half luminance, half equal share: pure luminance spacing starves
		// a palette whose colours cluster at one end, pure equal spacing
		// blows the sheet out to whichever single colour is lightest.
		at[i] = 0.5*(c.Luminance()-low)/span + 0.5*float64(i)/float64(len(colors)-1)
	}
	return valueRamp{colors: colors, at: at}
}

// At reads the ramp at t in [0,1].
func (r valueRamp) At(t float64) palette.Color {
	t = mathx.Clamp01(t)
	for i := 1; i < len(r.at); i++ {
		if t <= r.at[i] {
			width := r.at[i] - r.at[i-1]
			if width <= 1e-9 {
				return r.colors[i]
			}
			return palette.Lerp(r.colors[i-1], r.colors[i], (t-r.at[i-1])/width)
		}
	}
	return r.colors[len(r.colors)-1]
}

// accent picks the colour that will carry the pupillary zone: the most
// saturated warm colour available, falling back to the most saturated one.
// It is never the extreme light or dark, which the value ramp already uses.
func accent(colors []palette.Color) palette.Color {
	best, bestScore := colors[len(colors)/2], -math.MaxFloat64
	for _, c := range colors[1 : len(colors)-1] {
		_, saturation, lightness := c.HSL()
		warmth := c.R - c.B
		score := saturation*1.4 + warmth + (0.5 - math.Abs(lightness-0.55))
		if score > bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

func (s *Sketch) plan(ctx sketch.Context) (plan, error) {
	if len(ctx.Palette.Colors) < 4 {
		return plan{}, fmt.Errorf("iris: palette %q needs at least 4 colors", ctx.Palette.Slug)
	}
	if ctx.Height <= 0 || ctx.Width <= 0 {
		return plan{}, fmt.Errorf("iris: canvas %dx%d has no area", ctx.Width, ctx.Height)
	}
	set := s.pin(draw(s.Traits(ctx), ctx.RNG(streamRecipe)))
	colors := palette.ByLuminance(ctx.Palette.Colors)
	last := len(colors) - 1
	lightest := colors[last]
	darkest := colors[0]

	// Value, not radius, is the primary colour axis: the field reads from
	// the darkest colour in its cavities to the lightest on its crests, so
	// the structure stays legible everywhere on the sheet.
	body := newValueRamp(colors)

	var groundColor palette.Color
	switch set.ground {
	case groundDark:
		groundColor = darkest.ContrastShade(0.06).Desaturate(0.35)
	case groundPalette:
		groundColor = palette.Lerp(colors[1], darkest, 0.4).Desaturate(0.3)
	default:
		groundColor = lightest.Lighten(0.88).Desaturate(0.55)
	}

	aspect := float64(ctx.Width) / float64(ctx.Height)
	return plan{
		set:         set,
		q:           [2]*noise.Perlin{noise.New(ctx.Seed ^ seedQX), noise.New(ctx.Seed ^ seedQY)},
		r:           [2]*noise.Perlin{noise.New(ctx.Seed ^ seedRX), noise.New(ctx.Seed ^ seedRY)},
		value:       noise.New(ctx.Seed ^ seedValue),
		combField:   noise.New(ctx.Seed ^ seedComb),
		sector:      noise.New(ctx.Seed ^ seedSector),
		crypt:       ctx.Seed ^ seedCrypt,
		centreU:     aspect / 2,
		centreV:     0.5,
		body:        body,
		accent:      accent(colors),
		light:       lightest,
		dark:        darkest,
		limbal:      palette.Lerp(darkest, colors[1], 0.25),
		pupil:       darkest.ContrastShade(-0.22).Desaturate(0.2),
		groundColor: groundColor,
	}, nil
}

// fbm is the normalized rotated fBM the whole sketch samples with. The domain
// rotates between octaves so no scale accumulates along the sampling axes.
func (p plan) fbm(field *noise.Perlin, x, y float64) float64 {
	value, _ := p.fbmWithFine(field, x, y)
	return value
}

// fbmWithFine also returns the top two octaves on their own, the residual the
// sharp filament response is built from.
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

// stroma is the fibre reading at one point of the annulus.
type stroma struct {
	tone   float64 // body value, 0 = cavity, 1 = crest
	spark  float64 // narrow bright emphasis
	shadow float64 // narrow dark emphasis: crypts, risers, cell webs
}

// polar is one point of the annulus with its local frame.
type polar struct {
	s              float64 // 0 at the pupil edge, 1 at the limbus
	dirX, dirY     float64
	tanX, tanY     float64
	warpX, warpY   float64 // the fully warped point in remapped space
	firstX, firstY float64 // the once-warped point, for the comb
	broad          float64 // the first warp field itself, as a mottling term
}

// remap sends a direction and radius to the point the fields are read at. The
// stroma is a ring of noise-space radius rho, so a field evaluated there is
// continuous all the way around: there is no angular seam to hide.
func (p plan) remap(dirX, dirY, s float64) polar {
	// Twist rotates the sampling direction with radius, so the fibres leave
	// the pupil as a vortex instead of a starburst. It bends the field, not
	// the geometry: the iris stays a circle.
	if p.set.twist != 0 {
		angle := p.set.twist * s
		cosine, sine := math.Cos(angle), math.Sin(angle)
		dirX, dirY = cosine*dirX-sine*dirY, sine*dirX+cosine*dirY
	}
	rho := p.set.scale * (ringBase + p.set.stretch*s)
	px, py := dirX*rho, dirY*rho
	tanX, tanY := -dirY, dirX

	qx := p.fbm(p.q[0], px+3.1, py-7.9)
	qy := p.fbm(p.q[1], px-5.3, py+11.7)

	// Anisotropic displacement: tangential motion bends a fibre sideways,
	// radial motion breaks it along its length. Keeping them separate is
	// what stops the warp from dissolving the radial reading.
	wx := px + tanX*qx*p.set.fiber + dirX*qy*p.set.radial
	wy := py + tanY*qx*p.set.fiber + dirY*qy*p.set.radial
	firstX, firstY := wx, wy

	if p.set.nested > 0 {
		rx := p.fbm(p.r[0], (wx+13.1)*rRoleScale, (wy-4.7)*rRoleScale)
		ry := p.fbm(p.r[1], (wx-8.3)*rRoleScale, (wy+6.1)*rRoleScale)
		wx += tanX*rx*p.set.nested + dirX*ry*p.set.nested*0.45
		wy += tanY*rx*p.set.nested + dirY*ry*p.set.nested*0.45
	}
	return polar{
		s: s, dirX: dirX, dirY: dirY, tanX: tanX, tanY: tanY,
		warpX: wx * valueScale, warpY: wy * valueScale,
		firstX: firstX, firstY: firstY, broad: qx,
	}
}

// comb is a field whose coordinate ignores the radial position, so it is
// constant along every ray: straight spokes converging on the pupil, bent
// outward by the same tangential warp the stroma uses.
func (p plan) spokes(pt polar) float64 {
	rho := p.set.scale * combBase
	bend := (pt.firstX*pt.tanX + pt.firstY*pt.tanY) * 1.5 * pt.s
	return p.fbm(p.combField, pt.dirX*rho+pt.tanX*bend, pt.dirY*rho+pt.tanY*bend)
}

func (p plan) read(pt polar) stroma {
	switch p.set.structure {
	case structureFilament:
		return p.readFilament(pt)
	case structureStrata:
		return p.readStrata(pt)
	case structureCrypt:
		return p.readCrypt(pt)
	default:
		return p.readFiber(pt)
	}
}

// readFiber is the default: a broad warped body crossed by the comb, with the
// top octaves lifted into narrow crests.
func (p plan) readFiber(pt polar) stroma {
	value, fine := p.fbmWithFine(p.value, pt.warpX, pt.warpY)
	body := mathx.Smoothstep(-0.45, 0.45, value*0.82+p.spokes(pt)*0.34)

	// The strands are the zero set of the warped field, not its peaks: a
	// crease response over the whole field gives long meandering threads
	// where a peak response only gives blobs.
	crease := 1 - math.Abs(value*2.3+fine*0.55)
	strand := mathx.Smoothstep(0.46, 0.98, crease)

	// The broad warp field doubles as mottling: where it runs deep the
	// stroma keeps a soft dark passage, which is what stops an all-over
	// fibre texture from reading as fur.
	return stroma{
		tone:   mathx.Clamp01(body*0.76 + strand*0.34),
		spark:  strand,
		shadow: mathx.Smoothstep(0.06, 0.34, -pt.broad) * 0.55,
	}
}

// readFilament draws the level sets of the warped field itself. Lines crowd
// where the field is steep and disappear where it is calm, so the stroma comes
// out as intricate whorls separated by plain passages — a fingerprint rather
// than a fur. It is the one structure with no radial term at all: every
// contour is placed by the field, and only the warp makes them run outward.
func (p plan) readFilament(pt polar) stroma {
	value, fine := p.fbmWithFine(p.value, pt.warpX, pt.warpY)
	level := (value*2.6 + fine*0.7) * p.set.bands / 7
	distance := math.Abs(level - math.Round(level))
	thread := mathx.Smoothstep(0.19, 0.03, distance)
	body := mathx.Smoothstep(-0.42, 0.42, value)
	return stroma{
		tone:   mathx.Clamp01(0.16 + body*0.5 + thread*0.42),
		spark:  thread,
		shadow: mathx.Smoothstep(0.3, 0.46, distance) * 0.5,
	}
}

// readStrata terraces the radial coordinate displaced by the field, so the
// stroma reads as growth rings cut by the warp — a contour map of an eye. The
// steps alternate in value rather than climbing, so the rings stay legible
// however many of them there are.
func (p plan) readStrata(pt polar) stroma {
	value, fine := p.fbmWithFine(p.value, pt.warpX, pt.warpY)
	band := pt.s*p.set.bands + value*3.4 + fine*0.9
	step := math.Floor(band)
	frac := band - step

	// Every other ring sits low, so neighbours always contrast.
	high := math.Mod(math.Abs(step), 2) < 0.5
	level := 0.34
	if high {
		level = 0.78
	}
	// The step edge itself is a drawn line, not a gradient.
	riser := mathx.Smoothstep(0.09, 0.0, math.Min(frac, 1-frac))
	return stroma{
		tone:   mathx.Clamp01(level + value*0.28),
		spark:  0,
		shadow: riser,
	}
}

// readCrypt fills the warped space with cells. The space is already stretched
// radially, so the cells come out as elongated lacunae; each takes one flat
// tone from its own identity and the gaps between them draw the web.
func (p plan) readCrypt(pt polar) stroma {
	density := p.set.cells / 6
	cellX, cellY, f1, f2 := noise.WorleyCell(p.crypt, pt.warpX*density, pt.warpY*density)
	web := mathx.Smoothstep(0.015, 0.11, f2-f1)
	flat := noise.Hash01(p.crypt, cellX, cellY)
	value, _ := p.fbmWithFine(p.value, pt.warpX*0.7, pt.warpY*0.7)
	return stroma{
		tone:   mathx.Clamp01(0.22 + flat*0.62 + value*0.24),
		spark:  mathx.Smoothstep(0.42, 0.06, f1) * 0.5 * flat,
		shadow: 1 - web,
	}
}

// shade turns a stroma reading into colour. Value carries the structure and
// radius only bends it: the pupillary zone lifts and warms, the limbus sinks.
// No light is involved anywhere.
func (p plan) shade(pt polar, read stroma) palette.Color {
	tone := read.tone
	if p.set.tint == tintSector {
		// A slow field read on a small circle varies with angle only, so
		// whole wedges of the iris slide along the value ramp together.
		tone += p.fbm(p.sector, pt.dirX*0.85+4.2, pt.dirY*0.85-2.7) * 0.3
	}
	inner := 1 - mathx.Smoothstep(0.02, 0.62, pt.s)
	// Depth is a gamma on the value axis: it sinks the midtones toward the
	// dark end of the palette without touching the crests, which is what
	// keeps a bright strand reading as a strand and not as the whole sheet.
	tone = math.Pow(mathx.Clamp01(tone+inner*0.14), 1+p.set.depth*1.3)

	color := p.body.At(tone)
	color = palette.Lerp(color, p.accent, inner*p.set.warmth)
	color = palette.Lerp(color, p.light, read.spark*p.set.gleam)
	color = palette.Lerp(color, p.dark, read.shadow*(0.35+p.set.depth*0.55))

	// The two drawn edges: a soft outer zone sinking into a crisp limbal
	// ring, and a collar of shadow where the stroma meets the pupil. They
	// are the only places radius is allowed to overrule the field.
	color = palette.Lerp(color, p.limbal, mathx.Smoothstep(0.72, 0.97, pt.s)*0.45)
	color = palette.Lerp(color, p.limbal, mathx.Smoothstep(0.955, 0.995, pt.s)*0.85)
	return palette.Lerp(color, p.pupil, (1-mathx.Smoothstep(0, 0.05, pt.s))*0.5)
}

// At maps one canvas coordinate to colour.
func (p plan) At(u, v float64) palette.Color {
	du, dv := u-p.centreU, v-p.centreV
	radius := math.Hypot(du, dv) / p.set.limbus
	edge := edgeSoftness / p.set.limbus

	if radius >= 1+edge {
		return p.groundColor
	}
	if radius <= p.set.pupil-edge {
		return p.pupil
	}
	scale := math.Hypot(du, dv)
	dirX, dirY := du/scale, dv/scale
	s := mathx.Clamp01((radius - p.set.pupil) / (1 - p.set.pupil))

	pt := p.remap(dirX, dirY, s)
	color := p.shade(pt, p.read(pt))

	if radius > 1-edge {
		color = palette.Lerp(color, p.groundColor, mathx.Smoothstep(1-edge, 1+edge, radius))
	}
	if radius < p.set.pupil+edge {
		color = palette.Lerp(p.pupil, color, mathx.Smoothstep(p.set.pupil-edge, p.set.pupil+edge, radius))
	}
	return color.Clamp()
}

// Render implements sketch.Sketch.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	p, err := s.plan(ctx)
	if err != nil {
		return nil, err
	}
	return sketch.Raster(ctx, p.At), nil
}
