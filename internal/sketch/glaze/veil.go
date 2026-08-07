package glaze

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
)

// The veil is 013's nested domain warp asked a different question. There it
// builds a lit material and *is* the picture; here it only has to say how
// much water is over a point, how bright the finest thread on the surface is,
// and which way the plane under it was pulled.
//
// The vocabulary is re-stated rather than imported because `warp` exports an
// artwork, not a field, and because the role scales, the shaping curves and
// the anisotropy below are this sketch's aesthetic policy rather than 013's.

const (
	seedQX    = 0x676c617a652d7178
	seedQY    = 0x676c617a652d7179
	seedRX    = 0x676c617a652d7278
	seedRY    = 0x676c617a652d7279
	seedBody  = 0x676c617a652d6264
	seedPaper = 0x676c617a652d7061
	seedCover = 0x676c617a652d6376

	// rRole evaluates the second warp somewhat finer than the first, and
	// bodyRole the water field finer again, so the three roles stay
	// distinguishable instead of collapsing into one scale.
	rRole    = 1.19
	bodyRole = 1.33

	// paperTooth is the wavelength of the water's speckle, in canvas units.
	// It is a length, not a pixel count, so a print keeps a preview's grain.
	paperTooth = 0.0055

	// driftGain turns the displacement field's values, which are a normalized
	// fBM and so occupy well under half of [-1,1], into a direction of unit
	// order. Without it every drift would have to be quoted in units nobody
	// can read off the flag.
	driftGain = 2.6

	// bandCosine/bandSine turn the anisotropic frame 22 degrees off the axes.
	bandCosine = 0.9271838545667874
	bandSine   = 0.3746065934159120

	// coverShore is how wide the crossing from dry stone to open water is, in
	// units of the envelope field. Wide enough that the water thins toward a
	// shore rather than being cut out of the sheet with scissors.
	coverShore = 0.34
	// coverDrag is how far the veil's own first displacement pulls the
	// envelope about, so a shoreline follows the water's currents instead of
	// lying under them as an unrelated blob.
	coverDrag = 0.45
	// coverSpread widens the envelope's distribution. Perlin seldom reaches
	// its nominal extremes, and an envelope that never approaches 0 or 1
	// spends the ends of the coverage knob on nothing.
	coverSpread = 1.35

	// threadFold is the slope of the residual's first fold. It is the number
	// the single-ridge filament was tuned on, kept exactly so that density 1
	// is the veil the manner was designed around.
	threadFold = 1.45
	// threadWindow is how much of a fold counts as a thread. Tight on
	// purpose: most of the sheet is not in it at all, which is the only way a
	// highlight reads as a highlight.
	threadWindow = 0.72
	// gatherMid is the gather value that reproduces the original depth gate,
	// so the knob's default is the picture the manner already had.
	gatherMid = 0.35
)

// manner is the material the water is made of. It is one knob because its
// members are not settings of one veil, they are five different veils.
type manner uint8

const (
	mannerGlaze manner = iota
	mannerMarble
	mannerTerrace
	mannerFilament
	mannerSilk
)

// veilConfig is one manner's numbers after the caller's overrides. It is
// resolved before planning and never mutated afterwards.
type veilConfig struct {
	seed         uint64
	scale        float64
	warpStrength float64
	nestStrength float64
	stretch      float64
	driftScale   float64
	opacity      float64
	body         float64
	drift        float64
	glint        float64
	grain        float64
	coverage     float64
	coverScale   float64
	density      float64
	gather       float64
	terraces     int
	octaves      int
	gain         float64
	lacunarity   float64
	manner       manner
}

// preset is what a manner chooses on its own. Anything the user gave on the
// command line replaces the corresponding field afterwards.
func preset(m manner, seed uint64) veilConfig {
	c := veilConfig{
		seed:       seed,
		octaves:    5,
		gain:       0.5,
		lacunarity: 2,
		stretch:    1,
		driftScale: 3,
		grain:      0.14,
		terraces:   5,
		manner:     m,
		// A veil covers the sheet edge to edge until asked not to, so every
		// manner keeps the composition it was tuned on and the envelope is a
		// deliberate act rather than a default.
		coverage:   1,
		coverScale: 1.1,
		density:    1,
		gather:     gatherMid,
	}
	switch m {
	case mannerGlaze:
		c.scale, c.warpStrength, c.nestStrength = 0.85, 1.5, 2.0
		c.opacity, c.body, c.drift, c.glint = 0.95, 0.14, 0.014, 0.10
	case mannerMarble:
		c.scale, c.warpStrength, c.nestStrength = 0.75, 2.0, 2.6
		c.opacity, c.body, c.drift, c.glint = 0.70, 0.10, 0.070, 0.05
		c.driftScale = 8
	case mannerTerrace:
		c.scale, c.warpStrength, c.nestStrength = 0.80, 1.3, 1.6
		c.opacity, c.body, c.drift, c.glint = 0.95, 0.12, 0.008, 0.03
		// Three octaves, not five: plateaus need a field smooth enough to
		// have plateaus. At five the fine detail crosses every riser and the
		// sheets break up into the gradient they were quantised out of.
		c.octaves = 3
		c.terraces = 4
	case mannerFilament:
		c.scale, c.warpStrength, c.nestStrength = 1.20, 1.8, 2.4
		c.opacity, c.body, c.drift, c.glint = 0.55, 0.18, 0.010, 0.75
	case mannerSilk:
		c.scale, c.warpStrength, c.nestStrength = 1.60, 1.6, 2.2
		c.opacity, c.body, c.drift, c.glint = 0.80, 0.16, 0.018, 0.32
		c.stretch = 4.0
	}
	return c
}

// veil is the planned water: immutable seeded fields and nothing else.
// Sampling it allocates nothing and draws no randomness.
type veil struct {
	cfg   veilConfig
	q     [2]*noise.Perlin
	r     [2]*noise.Perlin
	body  *noise.Perlin
	paper *noise.Perlin
	cover *noise.Perlin
}

// veilSample is deliberately small: how much water is here, how bright the
// finest thread is, and which way the plane was pulled.
type veilSample struct {
	load     float64
	filament float64
	dx, dy   float64
}

func newVeil(cfg veilConfig) *veil {
	return &veil{
		cfg:   cfg,
		q:     [2]*noise.Perlin{noise.New(cfg.seed ^ seedQX), noise.New(cfg.seed ^ seedQY)},
		r:     [2]*noise.Perlin{noise.New(cfg.seed ^ seedRX), noise.New(cfg.seed ^ seedRY)},
		body:  noise.New(cfg.seed ^ seedBody),
		paper: noise.New(cfg.seed ^ seedPaper),
		cover: noise.New(cfg.seed ^ seedCover),
	}
}

// rotateScale turns the running domain between octaves so no two scales
// accumulate along the same axes. The angle is fixed, not random: it is a
// property of the field's construction, not of the seed.
func rotateScale(x, y, frequency float64) (float64, float64) {
	const cosine = 0.8191520442889918
	const sine = 0.573576436351046
	return (cosine*x - sine*y) * frequency, (sine*x + cosine*y) * frequency
}

func (v *veil) fbm(field *noise.Perlin, x, y float64) float64 {
	value, _ := v.fbmWithFine(field, x, y)
	return value
}

// fbmWithFine returns the normalized sum and, separately, the contribution of
// its top two octaves. The residual is what the filament manner draws: a
// ridge taken off the whole sum would follow the broad forms instead of
// running across them.
func (v *veil) fbmWithFine(field *noise.Perlin, x, y float64) (float64, float64) {
	sum, weight := 0.0, 0.0
	fine, fineWeight := 0.0, 0.0
	amplitude := 1.0
	for octave := range v.cfg.octaves {
		value := field.At(x, y)
		sum += value * amplitude
		weight += amplitude
		if octave >= v.cfg.octaves-2 {
			fine += value * amplitude
			fineWeight += amplitude
		}
		x, y = rotateScale(x, y, v.cfg.lacunarity)
		amplitude *= v.cfg.gain
	}
	if fineWeight == 0 {
		return sum / weight, 0
	}
	return sum / weight, fine / fineWeight
}

// At evaluates the veil at one normalized canvas coordinate.
func (v *veil) At(u, w float64) veilSample {
	c := v.cfg
	// Anisotropy is applied to the base coordinate, before any warping, so
	// that every later stage inherits it and the forms are genuinely drawn
	// out. Applied to the final lookup instead it was scrambled by the two
	// warps above it and did nothing but change the seed.
	//
	// The stretched frame is turned off the axes first. Bands running exactly
	// across the frame read as a machine's output however good the ripple on
	// them is; a few degrees is enough to stop that and costs two multiplies.
	aspect := math.Sqrt(math.Max(c.stretch, 1e-3))
	sx, sy := u*c.scale, w*c.scale
	if c.stretch != 1 {
		sx, sy = bandCosine*sx-bandSine*sy, bandSine*sx+bandCosine*sy
	}
	x, y := sx/aspect, sy*aspect

	qx := v.fbm(v.q[0], x+3.1, y-7.9)
	qy := v.fbm(v.q[1], x-5.3, y+11.7)
	ax, ay := x+qx*c.warpStrength, y+qy*c.warpStrength

	rx := v.fbm(v.r[0], (ax+13.1)*rRole, (ay-4.7)*rRole)
	ry := v.fbm(v.r[1], (ax-8.3)*rRole, (ay+6.1)*rRole)

	bx := (ax + rx*c.nestStrength) * bodyRole
	by := (ay + ry*c.nestStrength) * bodyRole
	raw, fine := v.fbmWithFine(v.body, bx, by)

	depth := mathx.Smoothstep(-0.28, 0.32, raw)
	load := c.opacity * v.shape(depth)

	// How much of the sheet is under water at all is a *place*, not a dial.
	// Scaling the load globally only makes a thin veil out of a thick one and
	// leaves the same composition; a broad envelope instead holds dry stone
	// and flooded passages in the same frame, which is where the drawing is.
	cover := v.envelope(u, w, qx, qy)
	load *= cover

	if c.grain > 0 {
		// The tooth is a multiplicative speckle on the load, so it disappears
		// where the water does instead of dusting the dry passages.
		load *= 1 + c.grain*v.paper.At(u/paperTooth, w/paperTooth)
	}

	// The drift is read at its own frequency. A displacement that varies only
	// over the whole canvas *translates* the bed rather than stretching it —
	// at four times the drift the picture was the same picture, moved. Smearing
	// a stone needs the offset to change appreciably across one.
	fx, fy := x*c.driftScale, y*c.driftScale
	dx := v.fbm(v.r[0], fx+29.3, fy+5.1)
	dy := v.fbm(v.r[1], fx-17.7, fy-2.3)

	return veilSample{
		load: math.Max(0, load),
		// Threads belong to the water: on dry stone they would be a net drawn
		// over nothing, so the envelope takes them away with the body.
		filament: v.thread(fine, depth) * cover,
		dx:       math.Tanh(driftGain * dx),
		dy:       math.Tanh(driftGain * dy),
	}
}

// envelope says how much of the water reaches this point at all. It is read
// at its own low frequency, dragged about by the veil's first displacement so
// that a shoreline follows the currents, and shaped by one threshold: at
// coverage 1 it is 1 everywhere and the veil is edge to edge, at 0 it is 0
// everywhere and the bed is dry.
func (v *veil) envelope(u, w, qx, qy float64) float64 {
	c := v.cfg
	if c.coverage >= 1 {
		return 1
	}
	x := u*c.coverScale + qx*coverDrag
	y := w*c.coverScale + qy*coverDrag
	// Two octaves, no more: an envelope with fine detail in it stops being a
	// composition and becomes a second texture competing with the threads.
	fx, fy := rotateScale(x, y, 2)
	e := (v.cover.At(x+2.7, y-8.1) + 0.5*v.cover.At(fx+19.3, fy+4.7)) / 1.5
	t := mathx.Clamp01(0.5 + coverSpread*e)
	// The threshold sweeps past both ends of the field, so the knob really
	// does reach "all of it" and "none of it" rather than asymptotes.
	lo := 1.10 - 1.45*c.coverage
	return mathx.Smoothstep(lo, lo+coverShore, t)
}

// thread is the narrow bright response the filament manner draws. Two things
// keep it a highlight rather than a frost: the window on the residual is
// tight, so most of the sheet is not in it at all; and it is gated by the
// water's own depth, so threads gather in the currents and leave the shallows
// alone. Ungated it covered the whole frame evenly and read as etched glass.
//
// The residual is folded into a triangle wave rather than taken as one ridge
// off zero. Both give the same line where the field crosses zero, but the
// wave *repeats*, so asking for more threads draws more of them — the way a
// contour map gets busier at a finer interval — instead of merely widening
// the single line there is. Widening it was the first thing tried and it
// turns filaments into slugs.
func (v *veil) thread(fine, depth float64) float64 {
	t := fine*threadFold*v.cfg.density + 0.5
	fold := 1 - math.Abs(2*(t-math.Floor(t))-1)
	ridge := mathx.Smoothstep(threadWindow, 0.99, fold)
	lo, hi := v.gate()
	return ridge * mathx.Smoothstep(lo, hi, depth)
}

// gate is the window on the water's own depth in which threads are drawn.
// Low gather spreads them over everything the water touches; high gather
// packs them into the deepest passages and leaves long calm stretches of thin
// water between. gatherMid reproduces the window the manner was tuned on
// exactly, so the knob's default is not a new picture.
func (v *veil) gate() (float64, float64) {
	g := v.cfg.gather - gatherMid
	centre := 0.37 + 0.62*g
	width := 0.50 * (1 - 0.60*g)
	return centre - width/2, centre + width/2
}

// shape turns the field's depth into a quantity of water. It is where the
// manners stop sharing a picture.
func (v *veil) shape(depth float64) float64 {
	switch v.cfg.manner {
	case mannerTerrace:
		return terrace(depth, v.cfg.terraces)
	case mannerFilament:
		// A thin, nearly even body: the threads are the drawing, and a body
		// with its own strong structure competes with them.
		return 0.34 + 0.52*depth
	case mannerGlaze:
		// Steeper than linear, so the veil genuinely runs dry somewhere
		// rather than merely getting paler everywhere.
		return math.Pow(depth, 1.35)
	default:
		return depth
	}
}

// terrace quantises into flat plateaus with a fast but antialiased riser, so
// the water reads as overlapping sheets rather than as a gradient, and
// without stair-stepping into aliasing at print size.
func terrace(depth float64, steps int) float64 {
	n := float64(steps)
	t := depth * n
	floor := math.Floor(t)
	frac := t - floor
	return math.Min(1, (floor+mathx.Smoothstep(0.82, 1.0, frac))/n)
}
