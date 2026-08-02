// Package scheme arranges colour over a composition made of many discrete
// regions — the cells of a foam, the tiles of a mosaic, the marks of a
// packing. It answers "which colour does this region get", given a palette
// and where the region sits.
//
// It is the colour counterpart of internal/hatch: hatch says how a region is
// filled, scheme says what colour it is filled with, and neither knows what
// kind of region it is looking at.
//
// The organising rule, and the reason this is a package rather than a
// function: a scheme answers *two* questions per region, hue and value. An
// arrangement of hue with no value structure has nothing to look at from
// across the room — squint at it and it goes flat grey — and that is the
// commonest way a correctly-harmonised palette still comes out looking like
// a swatch card. Every strategy below returns a Tone as well as a Fill, and
// the two are given different spatial rules on purpose: a sheet whose hue
// and weight both run along one axis is a gradient, and a gradient has no
// composition in it.
//
// Leaf: palette, mathx, noise, rnd.
package scheme

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// The strategies. Each is a different picture from the same structure and
// the same palette; see .claude/skills/colouring for what each is for and
// how each one fails.
const (
	Passage     = "passage"     // passages of related hue from a low-frequency field
	Gradient    = "gradient"    // hue runs along a direction across the frame
	Sequence    = "sequence"    // regions ordered, then walked along a sorted ramp
	Inherit     = "inherit"     // take a neighbour's colour, mutate rarely
	Dominance   = "dominance"   // three colours in roughly 70/20/10 proportion
	Complement  = "complement"  // a muted dominant with saturated opposite accents
	Analogous   = "analogous"   // hue confined to one arc of the wheel
	Triad       = "triad"       // three hues round the wheel, unequal proportion
	Monochrome  = "quiet"       // one pigment at every dilution, plus one spark
	Notan       = "notan"       // two or three values only, hue nearly constant
	Anchor      = "anchor"      // a cluster of dark regions anchoring the frame
	Temperature = "weather"     // warm-to-cool along a direction, value on its own field
	Duet        = "duet"        // two pigments, every colour a mix of them
	BySize      = "by-size"     // colour follows the region's area, not its position
	ByDarkness  = "by-darkness" // size decides the tone, a field decides the hue
)

// Names lists every strategy, in a stable order.
func Names() []string {
	return []string{
		Passage, Gradient, Sequence, Inherit, Dominance, Complement,
		Analogous, Triad, Monochrome, Notan, Anchor, Temperature, Duet,
		BySize, ByDarkness,
	}
}

// Has reports whether name is a known strategy.
func Has(name string) bool {
	for _, n := range Names() {
		if n == name {
			return true
		}
	}
	return false
}

// Region is what a scheme knows about one area it has to colour. Size is
// whatever the caller means by big — an area, an inradius squared — as long
// as it is consistent across the set; it is only ever read as a rank.
type Region struct {
	X, Y float64 // centroid, canvas units
	Size float64
}

// Colour is one region's answer: what to fill it with, a second colour for
// whatever the caller does with two, and how heavy it is.
//
// Tone in [0,1] is the value structure — 0 is barely touched, 1 is full
// strength. A caller filling flat should darken or saturate by it; a caller
// laying a wash should read it as a pigment load. Ignoring it throws away
// half of what a scheme decided.
type Colour struct {
	Fill   palette.Color
	Accent palette.Color
	Tone   float64
}

// Spec configures a mixer.
type Spec struct {
	// Name is the strategy; an unknown name resolves to Passage.
	Name string
	// Palette is the colours available. It is re-ordered internally as each
	// strategy needs; the caller's order is not read as meaningful.
	Palette []palette.Color
	// Seed makes the arrangement deterministic.
	Seed uint64
	// Aspect is the canvas width in canvas units (height is 1).
	Aspect float64
	// Passage is the wavelength of the spatial fields, in canvas units.
	// Roughly "how big should a patch of related colour be".
	Passage float64
	// Accent is the share of regions allowed off-scheme.
	Accent float64
	// Shades is how far a fill may wander from its palette colour in
	// lightness and saturation: 0 gives the bare swatches, 1 a wide family
	// around each one.
	//
	// A palette is a handful of colours, and a composition of a hundred
	// regions painted in exactly those reads as a chart of them. Real paint
	// has tints and shades of every pigment, and it is the *within-hue*
	// spread that makes a sheet look mixed rather than sampled. The drift is
	// mostly in value and saturation, with only a few degrees of hue: a
	// large hue jitter stops being a shade of the colour and becomes a
	// different colour, which is the arrangement's business, not this.
	Shades float64
	// Saturate lifts every fill's saturation by this share: 0 leaves the
	// palette as its painter mixed it, 1 doubles it.
	//
	// It is deliberately a separate lever from Shades. Shades spreads a
	// swatch into a family and keeps its character; this changes the
	// character, and a palette pushed far enough stops being the painting it
	// came from. Worth having anyway, because a wash lays pigment down
	// transparently and everything comes out a step quieter than the swatch.
	Saturate float64
}

// Mixer is a resolved arrangement: every region already has its colour.
//
// Resolving up front rather than on demand is what lets a strategy depend on
// the *set* — inherit needs its neighbours settled, sequence needs an order,
// by-size needs the extent — and it keeps the whole thing deterministic
// without threading a generator through the caller's pixel loop.
type Mixer struct {
	out []Colour
}

// New resolves the arrangement for every region.
func New(spec Spec, regions []Region) *Mixer {
	m := &Mixer{out: make([]Colour, len(regions))}
	if len(regions) == 0 || len(spec.Palette) == 0 {
		return m
	}
	s := newState(spec, regions)
	s.resolve()
	m.out = s.out
	return m
}

// At returns the colour of region i.
func (m *Mixer) At(i int) Colour {
	if i < 0 || i >= len(m.out) {
		return Colour{Tone: 1}
	}
	return m.out[i]
}

// Len is the number of regions resolved.
func (m *Mixer) Len() int { return len(m.out) }

// state is the working set for one resolve.
type state struct {
	spec    Spec
	regions []Region
	out     []Colour
	rng     *rand.Rand
	field   *noise.Perlin

	lum    []palette.Color // by luminance, darkest first
	chroma []palette.Color // most colourful first
	warm   []palette.Color // coolest first
	arc    []palette.Color // an analogous run: neighbours on the hue wheel
	triad  []palette.Color // three palette colours spread round the wheel

	cos, sin float64 // the direction a gradient or a temperature run follows
	lo, hi   float64 // the frame's extent along it
	small    float64 // log of the smallest region
	large    float64
}

func newState(spec Spec, regions []Region) *state {
	if !Has(spec.Name) {
		spec.Name = Passage
	}
	if spec.Passage <= 0 {
		spec.Passage = 0.75
	}
	if spec.Aspect <= 0 {
		spec.Aspect = 1
	}
	s := &state{
		spec:    spec,
		regions: regions,
		out:     make([]Colour, len(regions)),
		rng:     rand.New(rand.NewPCG(spec.Seed, 0x5c1e)),
		field:   noise.New(spec.Seed ^ 0x5c1e),
		lum:     palette.ByLuminance(spec.Palette),
	}
	s.chroma = byChroma(s.lum)
	s.warm = byWarmth(s.lum)
	s.arc = hueArc(s.chroma, s.chroma[0], 60)
	s.triad = spread(s.chroma, 3)

	a := s.rng.Float64() * 2 * math.Pi
	s.cos, s.sin = math.Cos(a), math.Sin(a)
	s.lo, s.hi = math.Inf(1), math.Inf(-1)
	for _, c := range [][2]float64{{0, 0}, {spec.Aspect, 0}, {0, 1}, {spec.Aspect, 1}} {
		p := c[0]*s.cos + c[1]*s.sin
		s.lo, s.hi = math.Min(s.lo, p), math.Max(s.hi, p)
	}

	s.small, s.large = math.Inf(1), math.Inf(-1)
	for _, r := range regions {
		if r.Size <= 0 {
			continue
		}
		s.small = math.Min(s.small, math.Log(r.Size))
		s.large = math.Max(s.large, math.Log(r.Size))
	}
	if !(s.large > s.small) {
		s.small, s.large = 0, 1
	}
	return s
}

// rank is where a region sits in the size order, 0 smallest, 1 largest.
func (s *state) rank(r Region) float64 {
	return mathx.Clamp01((math.Log(math.Max(r.Size, 1e-12)) - s.small) / (s.large - s.small))
}

// along is where a region sits across the frame in the chosen direction.
func (s *state) along(r Region) float64 {
	return mathx.Clamp01((r.X*s.cos + r.Y*s.sin - s.lo) / (s.hi - s.lo))
}

// patch samples the low-frequency field at a region's centroid, stretched.
//
// The field's own swing is nowhere near ±1, so sampled raw every centroid
// lands in the middle of the ramp and the composition comes out in two
// colours out of nine. The stretch is what makes it reach the ends.
func (s *state) patch(r Region, scale, offX, offY float64) float64 {
	w := s.spec.Passage * scale
	return mathx.Clamp01(0.5 + 1.5*s.field.FBM((r.X+offX)/w, (r.Y+offY)/w, 3))
}

// value is the tone field — deliberately a different field from the hue's,
// at a different scale and offset, so hue and weight do not run together.
func (s *state) value(r Region) float64 {
	return mathx.Clamp01(0.5 + 1.8*s.field.FBM((r.X+17.3)/(s.spec.Passage*0.7), (r.Y-23.9)/(s.spec.Passage*0.7), 2))
}
