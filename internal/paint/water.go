package paint

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// Watercolour is a second painting model beside the bristle brush.
//
// Where the brush deposits opaque paint that hides what is under it, a
// wash is transparent: pigment absorbs part of the light passing through
// it and returns the rest, so a stack of washes multiplies
// transmittances. That is what makes two colours crossing read as a
// believable third rather than as whichever was painted second, and it
// is why this file works in linear light — absorption is physical, and
// multiplying sRGB values instead turns every overlap to mud.
//
// A pool is built the way a painter's blob is: many near-transparent
// deposits of the same irregular shape, each deformed differently. The
// stack is the whole trick. Density variation, the soft broken edge and
// the concentration toward the middle all fall out of how many deposits
// happen to reach a given pixel, instead of having to be drawn.

// washAngles is the resolution of a deposit's radius table. A pool is a
// star-shaped blob, so one radius per angle describes it exactly and the
// per-pixel test becomes a lookup. Power of two so the wrap is a mask.
const washAngles = 256

// Wash describes a pool of watercolour.
type Wash struct {
	Layers     int     // near-transparent deposits stacked into one pool
	Ragged     float64 // edge deviation, as a fraction of radius
	Edge       float64 // extra pigment drawn back to the rim as it dried
	Grain      float64 // granulation strength; 0 is a perfectly flat wash
	GrainScale float64 // paper tooth size, canvas units
	Mottle     float64 // uneven pooling of pigment within one wash
	Scatter    float64 // light scattered back off the pigment, 0..1
	Body       float64 // opacity of the pigment itself, 0..1; 0 is a pure glaze
	Seed       uint64  // granulation lattice seed
}

// DefaultWash is a pool on cold-pressed paper: irregular but still
// recognisably round, with a clear rim and visible tooth.
func DefaultWash(seed uint64) Wash {
	return Wash{
		Layers:     40,
		Ragged:     0.22,
		Edge:       1.2,
		Grain:      0.18,
		GrainScale: 0.0038,
		Mottle:     0.45,
		Scatter:    0.14,
		Seed:       seed,
	}
}

// washRing is a radius table in pixels.
type washRing [washAngles]float64

// washBand is one deposit: the region between two radius tables. inner is
// unused — and left zero — on a filled pool.
type washBand struct{ outer, inner washRing }

// infRing is the identity for taking a per-angle minimum.
func infRing() washRing {
	var r washRing
	for i := range r {
		r[i] = math.Inf(1)
	}
	return r
}

// Pool lays a pool of col centred at (u, v) with nominal radius r.
// alpha is the strength the pool reaches where every deposit covers it.
func (w Wash) Pool(c *Canvas, rng *rand.Rand, u, v, r float64, col palette.Color, alpha float64) {
	w.lay(c, rng, u, v, r, 0, col, alpha)
}

// Ring lays an annular pool: the band of the given thickness straddling
// radius r, so it spans r ± thickness/2, with bare paper inside it.
//
// A ring is not a pool with a pool of ground painted over it. Both of its
// boundaries are wet edges, so both dry with a rim, and the pigment banks
// toward the middle of the band from each side — which is what a brushed
// circle of water actually does, and what makes the hole read as paper
// that was never painted rather than as something masked out afterwards.
// A thickness of 2·r or more closes the hole and the ring becomes a pool.
func (w Wash) Ring(c *Canvas, rng *rand.Rand, u, v, r, thickness float64, col palette.Color, alpha float64) {
	if thickness <= 0 {
		return
	}
	w.lay(c, rng, u, v, r+thickness/2, r-thickness/2, col, alpha)
}

// lay is the shared body of Pool and Ring: a stack of deposits between an
// outer boundary and an optional inner one. rInner <= 0 means filled.
func (w Wash) lay(c *Canvas, rng *rand.Rand, u, v, rOuter, rInner float64, col palette.Color, alpha float64) {
	if w.Layers < 1 || rOuter <= 0 || alpha <= 0 {
		return
	}
	px, py, pr := u*c.scale, v*c.scale, rOuter*c.scale
	if pr < 0.4 {
		return
	}
	// A hole only survives if it is worth more than a pixel; below that
	// the ring is painted as the solid dot it would dry into anyway.
	pin := rInner * c.scale
	hollow := pin > 1

	// Raggedness is an edge deviation measured against the radius, which
	// is the right unit for a pool's silhouette and the wrong one for a
	// narrow band: the two boundaries wander independently, so once the
	// deviation approaches the width they cross, and the ring dries as a
	// string of beads instead of a ring. Capping it here rather than in
	// every caller is what makes a run of fine concentric rings possible
	// at all. A filled pool is as wide as it is round, so the cap never
	// binds and its silhouette is untouched.
	if hollow {
		w.Ragged = math.Min(w.Ragged, washRaggedCap*(pr-pin)/pr)
	}

	// Softness varies around the perimeter, as if parts of the paper
	// were wetter than others: where it is high the edge frays and the
	// deposits scatter, where it is low they land together and the
	// boundary stays crisp. A uniformly ragged outline is one of the
	// things that makes procedural watercolour read as procedural.
	soft := softnessRing(rng)
	base := w.outline(rng, &soft)
	// The hole gets its own outline and its own softness rather than a
	// scaled copy of the outer one: the two boundaries were wetted by
	// different strokes, and a ring whose inside echoes its outside reads
	// as an extruded shape instead of as a band of paint.
	var holeSoft, hole washRing
	if hollow {
		holeSoft = softnessRing(rng)
		hole = w.outline(rng, &holeSoft)
	}
	waves := mottleWaves(rng, pr)

	// How much a deposit may depart from its boundary — both the hold-back
	// that banks pigment inward and the deposit's own wobble — expressed
	// relative to that boundary's radius, because that is the unit the
	// radius tables are in.
	//
	// Both distances belong to the *band's width*, not to the radius. A
	// tenth of the radius is wider than a narrow ring altogether: the
	// hold-back lands every deposit but the last inside-out so the ring
	// dries at a fraction of the strength asked for, and the wobble
	// scatters the boundary over more than the band is wide, which
	// dissolves it into an airbrushed gradient with no edge and therefore
	// no rim. For a filled pool the band and the radius are the same
	// thing, so both are exactly what they have always been.
	width := pr
	if hollow {
		width = pr - pin
	}
	outerNarrow := width / pr
	innerNarrow := 0.0
	if hollow {
		innerNarrow = width / pin
	}

	layers := make([]washBand, w.Layers)
	var envelope washRing
	innerEnv := infRing()
	for l := range layers {
		// Deposits are refined in three groups: the early ones sit well
		// inside the nominal radius and only the last group reaches it.
		// Every deposit therefore covers the middle while only some reach
		// the rim, which is what banks the pigment toward the centre.
		stage := float64(l*3/w.Layers) / 2
		layers[l].outer = w.deposit(rng, pr, stage, -1, outerNarrow, &base, &soft)
		for i, v := range layers[l].outer {
			envelope[i] = math.Max(envelope[i], v)
		}
		if hollow {
			// The hole shrinks as the outer edge grows, so the band closes
			// on its own middle from both sides and banks pigment there.
			layers[l].inner = w.deposit(rng, pin, stage, +1, innerNarrow, &hole, &holeSoft)
			for i, v := range layers[l].inner {
				innerEnv[i] = math.Min(innerEnv[i], v)
			}
		}
	}
	outer := 0.0
	for _, v := range envelope {
		outer = math.Max(outer, v)
	}

	// As the water retreats it carries pigment to the boundary and leaves
	// it there, so the darkest part of a dried pool is a ring just inside
	// its edge. This is the most recognisable watercolour cue there is.
	// A real rim on a hand-sized wash is a millimetre or two: a few per
	// cent of the radius. At a fifth of the radius, which is where this
	// started, it stops reading as dried pigment and starts reading as a
	// stroke drawn round the shape.
	band := math.Max(outer*0.16, 2)
	if hollow {
		// On a narrow band the two rims would otherwise overlap into one
		// uniformly dark stripe, which is a drawn circle, not a dried one.
		inner := math.Inf(1)
		for _, v := range innerEnv {
			inner = math.Min(inner, v)
		}
		band = math.Max(math.Min(band, 0.35*(outer-inner)), 1.5)
	}

	// Per-deposit strength, set so a fully covered pool lands on alpha.
	perLayer := 1 - math.Pow(1-mathx.Clamp01(alpha), 1/float64(w.Layers))
	// Absorbance per deposit, so stacking n of them is one exponential
	// rather than a power — cheaper, and it keeps the arithmetic in the
	// units the physics is written in.
	ar := math.Log(transmit(col.R, perLayer))
	ag := math.Log(transmit(col.G, perLayer))
	ab := math.Log(transmit(col.B, perLayer))
	// Body: pigment loaded thickly enough to hide what is under it rather
	// than only tint it — gouache, or watercolour used as body colour.
	// Scatter alone cannot do this. A pale pigment barely absorbs, so a
	// stack of it transmits most of the ground however much it scatters,
	// and a white mark on a dark ground stays a ghost; hiding has to come
	// off the transmittance. At Body 1 a deposit passes only what alpha
	// says it should, and the mark dries as flat pigment colour.
	hide := math.Log(1 - w.Body*perLayer)
	ar += hide
	ag += hide
	ab += hide

	// Pigment does not only absorb: some light scatters back off the
	// particles before reaching what is underneath. Without this floor a
	// stack of washes marches to black and every crossing goes dead,
	// which is exactly the complaint painters have about mixing on the
	// palette instead of glazing. With it the stack asymptotes to the
	// pigment's own masstone, which is what a thick layer really does —
	// and as the pigment gains body, that masstone is all there is.
	floor := w.Scatter + (1-w.Scatter)*w.Body
	fr := floor * palette.SRGBToLinear(col.R)
	fg := floor * palette.SRGBToLinear(col.G)
	fb := floor * palette.SRGBToLinear(col.B)

	// The tooth is a property of the paper, so it keeps an absolute size
	// however large the wash is — but never so fine that it aliases into
	// noise at preview resolutions.
	cell := math.Max(w.GrainScale*c.scale, 2.5)

	x0 := max(int(px-outer-1), 0)
	x1 := min(int(px+outer+1), c.w-1)
	y0 := max(int(py-outer-1), 0)
	y1 := min(int(py+outer+1), c.h-1)

	for y := y0; y <= y1; y++ {
		dy := float64(y) + 0.5 - py
		for x := x0; x <= x1; x++ {
			dx := float64(x) + 0.5 - px
			d := math.Hypot(dx, dy)
			if d > outer+1 {
				continue
			}
			idx := (math.Atan2(dy, dx) + math.Pi) * (washAngles / (2 * math.Pi))

			// n is how many deposits reached this pixel, with a sub-pixel
			// ramp at each one's own edge so the pool anti-aliases itself.
			n := 0.0
			if hollow {
				for l := range layers {
					cov := math.Min(
						mathx.Clamp01(sampleRing(&layers[l].outer, idx)-d+0.5),
						mathx.Clamp01(d-sampleRing(&layers[l].inner, idx)+0.5))
					n += cov
				}
			} else {
				for l := range layers {
					n += mathx.Clamp01(sampleRing(&layers[l].outer, idx) - d + 0.5)
				}
			}
			if n <= 0 {
				continue
			}
			// How thickly this pixel is covered, before the rim and the
			// paper get their say.
			cov := n / float64(w.Layers)
			if w.Edge > 0 {
				// A ridge rather than a ramp. Coverage is already falling
				// away at the boundary, so a rim that simply climbs toward
				// the edge multiplies a number on its way to zero and
				// stays invisible; the deposit has to peak a little way
				// inside, where there is still pigment to darken.
				// Only the crisp arcs carry a rim. Where the paper was
				// wetter the pigment diffused instead of being carried to
				// a boundary, so a wash that rims evenly all the way round
				// reads as an outlined shape rather than a dried pool.
				env := sampleRing(&envelope, idx)
				t := (env - d) / band
				rim := w.Edge * (1 - 0.8*sampleRing(&soft, idx))
				lift := rim * math.Exp(-12*(t-0.3)*(t-0.3))
				if hollow {
					// The hole is a boundary the water retreated from too,
					// so it carries its own rim, measured outward from it.
					// The two are combined by taking the stronger, not by
					// adding: a point in the band belongs to whichever edge
					// the water left it at. Summed, a narrow ring — where
					// both tails cover the whole width — is lifted by twice
					// the rim everywhere and dries as a solid dark stripe.
					ti := (d - sampleRing(&innerEnv, idx)) / band
					rin := w.Edge * (1 - 0.8*sampleRing(&holeSoft, idx))
					lift = math.Max(lift, rin*math.Exp(-12*(ti-0.3)*(ti-0.3)))
				}
				n *= 1 + lift
			}
			// Both of these modulate how much pigment settled, not the
			// final colour, so they read as pigment sitting in the paper
			// rather than as a texture laid over the top.
			if w.Mottle > 0 {
				n *= 1 + w.Mottle*mottleAt(&waves, dx, dy)
			}
			if w.Grain > 0 {
				// Granulation is settled pigment, so it needs pigment to
				// be there: ungated it covers bare paper and dense wash
				// alike at the same strength, which reads as film grain
				// over the top of the picture rather than as grains in it.
				g := w.Grain * math.Pow(mathx.Clamp01(cov), 0.8)
				n *= 1 + g*(2*tooth(w.Seed, float64(x), float64(y), cell)-1)
			}
			if n <= 0.01 {
				continue
			}
			i := y*c.w + x
			p := c.pix[i]
			c.pix[i] = palette.Color{
				R: palette.LinearToSRGB(layer(p.R, math.Exp(n*ar), fr)),
				G: palette.LinearToSRGB(layer(p.G, math.Exp(n*ag), fg)),
				B: palette.LinearToSRGB(layer(p.B, math.Exp(n*ab), fb)),
			}
		}
	}
}

// transmit is the fraction of light one deposit of this pigment passes,
// in linear light.
func transmit(channel, perLayer float64) float64 {
	return 1 - perLayer*(1-palette.SRGBToLinear(channel))
}

// layer puts one pigment layer of transmittance t and back-scatter floor
// f over a linear-light ground: what gets through the layer, plus what
// the layer itself scatters back.
func layer(base, t, f float64) float64 {
	return palette.SRGBToLinear(base)*t + f*(1-t)
}

// softnessRing gives each part of the perimeter its own edge character,
// from crisp (0) to fully frayed (1), varying slowly so neighbouring
// arcs agree.
func softnessRing(rng *rand.Rand) washRing {
	var amp, phase [2]float64
	for k := range amp {
		amp[k] = rng.Float64() / float64(k+1)
		phase[k] = rng.Float64() * 2 * math.Pi
	}
	bias := 0.35 + 0.4*rng.Float64()
	var out washRing
	for i := range out {
		t := float64(i) * (2 * math.Pi / washAngles)
		v := bias
		for k := range amp {
			v += 0.5 * amp[k] * math.Sin(float64(k+1)*t+phase[k])
		}
		out[i] = mathx.Clamp01(v)
	}
	return out
}

// outline is the pool's silhouette, shared by every deposit: the fine
// structure a wet edge has at several scales at once.
//
// It matters that this is computed once and not per deposit. Detail that
// differs between deposits averages out across the stack into a fuzzy
// halo — the fringe you get from randomising every layer at high
// frequency, and one of the surest ways to make procedural watercolour
// look wrong. Shared detail survives the stack and reads as a torn edge;
// the deposits then differ only in broad, low-frequency wobble, which is
// what spreads the boundary into a soft band without blurring it away.
func (w Wash) outline(rng *rand.Rand, soft *washRing) washRing {
	var amp, phase [washHarmonics]float64
	norm := 0.0
	for k := range amp {
		// Harmonic 1 is damped hard: on its own it only slides the whole
		// blob off centre, and at full strength it eats the deformation
		// budget and leaves every pool a displaced circle.
		// Steep falloff. A shallower one leaves the high harmonics loud
		// enough to serrate the outline into a starburst; a wet edge is
		// lobed at large scale with only fine crinkle on top of it.
		a := math.Pow(float64(k+1), -1.4)
		if k == 0 {
			a *= 0.3
		}
		amp[k] = a * (0.35 + 0.65*rng.Float64())
		phase[k] = rng.Float64() * 2 * math.Pi
		norm += a * a
	}
	// Normalise against the RMS, not the sum. The harmonics carry
	// independent phases, so they mostly cancel rather than pile up;
	// dividing by the sum assumes a worst case that never happens and
	// flattens the outline back to a circle.
	gain := w.Ragged / math.Sqrt(norm)

	var out washRing
	for i := range out {
		dev := 0.0
		for k := range amp {
			dev += amp[k] * harmonic(k+1, i, phase[k])
		}
		out[i] = 1 + gain*dev*(0.3+1.2*soft[i])
	}
	return out
}

// deposit is one near-transparent laying-down of pigment: the shared
// outline, pulled off the nominal radius by how early it went down and
// nudged by a little low-frequency wobble of its own. stage in [0,1] sets
// how close to that radius it reaches; dir is -1 for a boundary the paint
// grows outward to and +1 for one it retreats inward from, so a hole
// closes over the same three stages that an edge opens out. narrow is the
// band's width as a fraction of this boundary's radius, which is 1 for a
// filled pool and scales every departure down on a ring.
func (w Wash) deposit(rng *rand.Rand, pr, stage, dir, narrow float64, base, soft *washRing) washRing {
	// Deposits sit at very nearly the same size. Spreading their radii
	// widely dissolves the boundary into an airbrushed gradient, and a
	// wash with no boundary has no rim either — the two cues that
	// separate watercolour from a soft blur both live at the edge.
	scale := 1 + dir*washPull*narrow*(1-stage)*(0.5+0.5*rng.Float64())

	var amp, phase [4]float64
	for k := range amp {
		amp[k] = w.Ragged * 0.3 * narrow * (0.3 + 0.7*rng.Float64()) / float64(k+1)
		phase[k] = rng.Float64() * 2 * math.Pi
	}
	var out washRing
	for i := range out {
		wobble := 0.0
		for k := range amp {
			wobble += amp[k] * harmonic(k+1, i, phase[k])
		}
		v := base[i]*scale + wobble*(0.3+1.2*soft[i])
		out[i] = math.Max(v, 0.05) * pr
	}
	return out
}

// washPull is how far the earliest deposits are held back from the
// boundary, as a fraction of the band's width.
const washPull = 0.1

// washRaggedCap is the most of a band's width its edge deviation may take
// up before the two boundaries start crossing each other.
const washRaggedCap = 0.35

// washHarmonics is how many terms shape a pool's silhouette.
const washHarmonics = 24

// sinTable holds one period of a sine at the ring's angular resolution.
// Every harmonic of a ring lands exactly on these samples, so the whole
// outline is table lookups: without it, twenty-four harmonics across
// forty deposits for every pool in a field is millions of calls to Sin.
var sinTable = func() [washAngles]float64 {
	var t [washAngles]float64
	for i := range t {
		t[i] = math.Sin(float64(i) * 2 * math.Pi / washAngles)
	}
	return t
}()

// harmonic is sin(k·θ_i + phase) evaluated from the table, using the
// angle-sum identity so the phase need not be on a sample point.
func harmonic(k, i int, phase float64) float64 {
	idx := (k * i) & (washAngles - 1)
	sin := sinTable[idx]
	cos := sinTable[(idx+washAngles/4)&(washAngles-1)]
	return sin*math.Cos(phase) + cos*math.Sin(phase)
}

// sampleRing reads a radius table at a fractional index, wrapping and
// interpolating so the boundary stays smooth between entries.
func sampleRing(t *washRing, idx float64) float64 {
	i := int(idx)
	f := idx - float64(i)
	i &= washAngles - 1
	return t[i] + (t[(i+1)&(washAngles-1)]-t[i])*f
}

// tooth is the paper's grain: cell noise, whose feature points are
// scattered rather than sitting on a lattice.
//
// Square-lattice value noise was what this used first, and on a large
// wash its maxima line up into visible rows and columns — the interior
// reads as a grid of soft pixels rather than as paper. Any noise whose
// extrema are pinned to a lattice has this problem; it is invisible only
// while the cells stay near pixel size.
func tooth(seed uint64, px, py, cell float64) float64 {
	// Two octaves at incommensurate scales. One octave puts exactly one
	// feature in every cell, so the pits keep a quasi-regular spacing and
	// read as a mesh laid over the wash; overlaying a second breaks that
	// up into the irregular pitting real paper has.
	f1, _ := noise.Worley(seed, px/cell, py/cell)
	g1, _ := noise.Worley(seed^0x5bf0, px/(cell*2.37), py/(cell*2.37))
	return mathx.Clamp01(0.62*f1/0.9 + 0.38*g1/0.9)
}

// mottleTerms is how many plane waves build the pooling field.
const mottleTerms = 5

// mottleWave is one plane wave of the pigment-pooling field.
type mottleWave struct{ kx, ky, phase, amp float64 }

// mottleWaves builds the low-frequency unevenness inside a single wash
// from a few plane waves at random headings. Their wavelengths are set
// from the pool's own radius, so a large wash and a small one show the
// same amount of pooling instead of the large one coming out as a slab
// of flat colour. Being a sum of sines rather than lattice noise, it has
// no preferred direction and nothing to line up.
func mottleWaves(rng *rand.Rand, pr float64) [mottleTerms]mottleWave {
	var out [mottleTerms]mottleWave
	norm := 0.0
	for i := range out {
		// Wavelengths spread widely rather than clustering. A handful of
		// waves at similar periods interfere into visible parallel
		// banding; spreading them turns the same budget into blotches.
		lambda := pr * 1.45 * math.Pow(0.72, float64(i))
		th := rng.Float64() * 2 * math.Pi
		k := 2 * math.Pi / math.Max(lambda, 1e-6)
		amp := math.Pow(0.78, float64(i))
		out[i] = mottleWave{
			kx:    k * math.Cos(th),
			ky:    k * math.Sin(th),
			phase: rng.Float64() * 2 * math.Pi,
			amp:   amp,
		}
		norm += amp
	}
	for i := range out {
		out[i].amp /= norm
	}
	return out
}

// mottleAt evaluates the pooling field at an offset from the pool centre.
func mottleAt(w *[mottleTerms]mottleWave, dx, dy float64) float64 {
	v := 0.0
	for _, m := range w {
		v += m.amp * math.Sin(m.kx*dx+m.ky*dy+m.phase)
	}
	return v
}
