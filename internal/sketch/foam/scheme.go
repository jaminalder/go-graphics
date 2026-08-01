package foam

// How colour is organised over the sheet.
//
// A packed foam is a hundred and fifty small cells, and at that count *how
// colour is distributed* matters more than what any one cell is painted
// with. Giving every cell a free draw from the palette gives confetti; a
// low-frequency field gives passages; but neither of those is the only way a
// painter organises a sheet, and each of the others below is a different
// picture from the same structure and the same wash.
//
// A scheme answers three questions per cell at once — the pigment, a second
// pigment for whatever the cell does wet-in-wet, and a *tone*: how heavily
// the cell was loaded. The tone is the important one. Hue arrangements alone
// give a sheet with no value structure, and a sheet with no value structure
// has nothing to look at from across the room.

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// The schemes.
const (
	schemePassage = "passage"     // passages of related hue with sparse accents
	schemeAnchor  = "anchor"      // a cluster of dark cells anchoring the sheet
	schemeQuiet   = "quiet"       // near-monochrome, with one or two saturated cells
	schemeWeather = "weather"     // a warm-to-cool gradient across the sheet
	schemeDuet    = "duet"        // two pigments, every colour a mix of them
	schemeByScale = "by-size"     // colour follows the cell's size, not its position
	schemeFromLum = "by-darkness" // small cells dark, laid on the luminance ramp
)

// mixer is the sheet's colour organisation, settled once per render.
type mixer struct {
	scheme  string
	ramp    []palette.Color // the palette, darkest first
	warm    []palette.Color // the palette, coolest first
	field   *noise.Perlin
	accent  float64
	passage float64

	// per-sheet choices the scheme needs
	house  palette.Color // quiet: the sheet's one pigment
	spark  palette.Color // quiet: the one cell that is allowed to shout
	duetA  palette.Color
	duetB  palette.Color
	cos    float64 // weather: the direction the temperature runs in
	sin    float64
	lo, hi float64 // weather: the extent of the sheet along it
	small  float64 // by-size: log of the smallest cell's area
	large  float64
}

// mixFor settles the sheet's colour organisation.
func (s *Sketch) mixFor(f *cells.Foam, l levels, rng *rand.Rand, field *noise.Perlin, ramp []palette.Color, aspect float64) *mixer {
	m := &mixer{
		scheme: l.scheme, ramp: ramp, warm: byWarmth(ramp), field: field,
		accent: s.Accent, passage: s.Passage,
	}

	// The quiet sheet's one pigment: something with enough body to carry a
	// whole sheet on dilution alone, so the middle of the ramp rather than
	// either end. Its accent is drawn as far from it in hue as the palette
	// allows — the point of a near-monochrome sheet is the one cell that is
	// not.
	m.house = ramp[len(ramp)/3+rng.IntN(max(len(ramp)/3, 1))]
	m.spark = farthestHue(ramp, m.house)

	// The duet: two pigments that are actually pigments. Picked by
	// saturation and then by hue distance rather than off the luminance
	// ramp, because most palettes here have a near-neutral member at each
	// end and a two-pigment sheet mixed from two greys is a sheet of grey —
	// which is exactly what the first version produced.
	chroma := byChroma(ramp)
	m.duetA = chroma[0]
	m.duetB = farthestHue(chroma[:max(len(chroma)*2/3, 2)], m.duetA)

	a := rng.Float64() * 2 * math.Pi
	m.cos, m.sin = math.Cos(a), math.Sin(a)
	// The extent of the frame along that direction, from its corners.
	m.lo, m.hi = math.Inf(1), math.Inf(-1)
	for _, c := range [][2]float64{{0, 0}, {aspect, 0}, {0, 1}, {aspect, 1}} {
		p := c[0]*m.cos + c[1]*m.sin
		m.lo, m.hi = math.Min(m.lo, p), math.Max(m.hi, p)
	}

	m.small, m.large = math.Inf(1), math.Inf(-1)
	for _, c := range f.Cells() {
		if c.Area <= 0 {
			continue
		}
		m.small = math.Min(m.small, math.Log(c.Area))
		m.large = math.Max(m.large, math.Log(c.Area))
	}
	if !(m.large > m.small) {
		m.small, m.large = 0, 1
	}
	return m
}

// draw gives one cell its pigment, its second pigment and its tone — how
// heavily it was loaded, in [0,1].
func (m *mixer) draw(c cells.Cell, rng *rand.Rand) (pigment, second palette.Color, tone float64) {
	switch m.scheme {
	case schemeAnchor:
		// A value structure: one broad field decides where the sheet is
		// heavy, and the dark cells arrive in a *cluster* rather than
		// scattered, which is what gives the composition somewhere to sit.
		g := mathx.Clamp01(0.5 + 2.2*m.field.FBM((c.CX+31.7)/(m.passage*1.4), (c.CY-19.3)/(m.passage*1.4), 2))
		if g > 0.62 {
			pigment = pick(m.ramp, rnd.Uniform(rng, 0, 0.4))
			tone = 0.72 + 0.28*g
		} else {
			pigment = pick(m.ramp, rnd.Uniform(rng, 0.35, 1))
			tone = 0.06 + 0.5*g
		}
		second = away(m.ramp, rng.Float64(), rng)

	case schemeQuiet:
		// Near-monochrome. Everything is one pigment at a different
		// dilution, and a handful of cells are allowed a colour.
		pigment, second = m.house, m.house
		tone = rnd.Uniform(rng, 0.05, 0.85)
		if rng.Float64() < m.accent*0.3 {
			pigment, second = m.spark, m.spark
			tone = rnd.Uniform(rng, 0.75, 1)
		}

	case schemeWeather:
		// A temperature gradient: hue follows a direction across the sheet
		// rather than a field, so the sheet turns over once instead of
		// blotching. Jittered per cell, or the boundary between the warm end
		// and the cool one is a visible straight line.
		p := (c.CX*m.cos + c.CY*m.sin - m.lo) / (m.hi - m.lo)
		t := mathx.Clamp01(p + rnd.Gauss(rng, 0, 0.1))
		pigment = pick(m.warm, t)
		second = pick(m.warm, mathx.Clamp01(t+rnd.Gauss(rng, 0, 0.18)))
		// Value runs on its own field rather than along the same axis: a
		// sheet that gets both its hue and its weight from one direction is
		// a gradient, and a gradient has no composition in it.
		tone = 0.1 + 0.85*mathx.Clamp01(0.5+1.8*m.field.FBM((c.CX-51.2)/(m.passage*1.2), (c.CY+38.6)/(m.passage*1.2), 2))

	case schemeDuet:
		// A limited sheet: two pigments, and every colour on it is a mix of
		// the same two. The discipline every watercolour teacher starts
		// with, and the reason a limited palette looks coherent.
		t := mathx.Clamp01(0.5+1.6*m.field.FBM(c.CX/m.passage, c.CY/m.passage, 3)) + rnd.Gauss(rng, 0, 0.14)
		t = mathx.Clamp01(t)
		pigment = palette.LerpHSL(m.duetA, m.duetB, t)
		// The second pigment is the other end, so a wet-in-wet cell shows
		// the two ingredients meeting rather than two mixes.
		second = m.duetB
		if t > 0.5 {
			second = m.duetA
		}
		tone = 0.15 + 0.8*mathx.Clamp01(0.5+1.8*m.field.FBM((c.CX+9.1)/(m.passage*0.8), (c.CY+4.4)/(m.passage*0.8), 2))

	case schemeByScale:
		// Colour follows the cell's *size*. The structure already sorts the
		// sheet into big lobes and slivers, and colouring by that makes the
		// packing itself legible — the sheet reads as a gradation without
		// having any spatial gradient in it at all.
		t := mathx.Clamp01((math.Log(math.Max(c.Area, 1e-9)) - m.small) / (m.large - m.small))
		pigment = pick(m.ramp, mathx.Clamp01(1-t+rnd.Gauss(rng, 0, 0.08)))
		second = away(m.ramp, 1-t, rng)
		// The slivers carry the weight: a hundred small dark chips between
		// pale lobes is a net drawn in colour.
		tone = 0.15 + 0.8*(1-t)

	case schemeFromLum:
		// The same idea run the other way: size decides the *tone* and a
		// field decides the hue, so the sheet keeps its passages and gains a
		// value structure that follows the packing.
		t := mathx.Clamp01((math.Log(math.Max(c.Area, 1e-9)) - m.small) / (m.large - m.small))
		hue := mathx.Clamp01(0.5 + 1.5*m.field.FBM(c.CX/m.passage, c.CY/m.passage, 3))
		pigment = pick(m.ramp, hue)
		second = away(m.ramp, hue, rng)
		tone = 0.1 + 0.85*(1-t)

	default: // passage
		// The sheet has passages of related colour — a green corner, a brown
		// corner — rather than confetti, with a share of cells taking an
		// accent from elsewhere on the ramp. The field's swing is nowhere
		// near ±1, so it is stretched before it is read: sampled raw, every
		// centroid lands mid-ramp and the sheet comes out in two colours.
		t := mathx.Clamp01(0.5 + 1.5*m.field.FBM(c.CX/m.passage, c.CY/m.passage, 3))
		pigment = pick(m.ramp, t)
		if rng.Float64() < m.accent {
			pigment = pick(m.ramp, rng.Float64())
		}
		second = away(m.ramp, t, rng)
		tone = 0.15 + 0.75*mathx.Clamp01(0.5+1.8*m.field.FBM((c.CX+17.3)/(m.passage*0.7), (c.CY-23.9)/(m.passage*0.7), 2))
	}
	return pigment, second, mathx.Clamp01(tone)
}

// pick reads a ramp at t ∈ [0,1].
func pick(ramp []palette.Color, t float64) palette.Color {
	return ramp[min(int(mathx.Clamp01(t)*float64(len(ramp))), len(ramp)-1)]
}

// away picks the pigment a cell is *charged* with: somewhere else on the
// ramp entirely, wrapped round rather than nudged.
//
// A neighbouring swatch was the obvious choice and it is the wrong one.
// Wet-in-wet is only visible when the two pigments differ; charged with its
// own neighbour a cell dries as one colour with a slight unevenness, which
// is what the flat wash already looks like.
func away(ramp []palette.Color, t float64, rng *rand.Rand) palette.Color {
	return pick(ramp, math.Mod(t+rnd.Uniform(rng, 0.3, 0.7), 1))
}

// byChroma orders a palette by saturation, most colourful first.
func byChroma(colors []palette.Color) []palette.Color {
	out := append([]palette.Color(nil), colors...)
	sat := func(c palette.Color) float64 { _, s, l := c.HSL(); return s * (1 - math.Abs(2*l-1)) }
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && sat(out[j]) > sat(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// byWarmth orders a palette from coolest to warmest, so a temperature
// gradient has a ramp to run along. Warmth is the hue's projection onto the
// orange axis; a colour with no saturation has no temperature and sits in
// the middle.
func byWarmth(colors []palette.Color) []palette.Color {
	out := append([]palette.Color(nil), colors...)
	score := func(c palette.Color) float64 {
		h, s, _ := c.HSL()
		if s < 0.05 {
			return 0
		}
		return s * math.Cos((h-35)*math.Pi/180)
	}
	// Insertion sort: a palette is a handful of colours, and this keeps the
	// order stable without pulling in a comparator closure per render.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && score(out[j]) < score(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// farthestHue is the colour of the palette furthest round the wheel from c —
// the accent a near-monochrome sheet needs.
func farthestHue(colors []palette.Color, c palette.Color) palette.Color {
	h0, _, _ := c.HSL()
	best, bestD := c, -1.0
	for _, o := range colors {
		h, s, _ := o.HSL()
		d := math.Abs(math.Mod(math.Abs(h-h0), 360) - 180)
		// Distance round the wheel, weighted by saturation: an accent has to
		// actually be a colour.
		if score := (180 - d) * (0.3 + 0.7*s); score > bestD {
			best, bestD = o, score
		}
	}
	return best
}
