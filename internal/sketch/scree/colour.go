package scree

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
	"github.com/jaminalder/go-graphics/internal/scheme"
	"github.com/jaminalder/go-graphics/internal/trait"
)

// The colours are part of the output space, not a setting beside it — the
// argument is set out in sketch 008's colourway.go. Sweeping seeds has to
// give different pictures in different colours; with the palette outside the
// output space every seed of a sweep comes out in whatever --palette said.
//
// What this sketch asks of a palette is hue count and a spread of value. Every
// stone is an opaque body of colour walled off from its neighbours by water,
// and a river bed is frankly polychrome — the whole pleasure of looking at one
// is that no two stones next to each other are quite the same rock. A
// four-colour palette divides a hundred-stone bed into four zones. It also
// needs a member pale enough to serve as wet paper and one dark enough to pass
// for deep water.

const dimColourway = "colourway"

// fromFlag hands colour duty back to whatever --palette names. Weight 0, so a
// seed never lands on it: using this sketch with a palette from outside the
// curated list is always a deliberate act.
const fromFlag = "from-flag"

// colourways are the palettes a seed may draw: broad, many-hued sets with a
// light member and a dark one.
var colourways = []trait.Value{
	{Name: "tchelitchew-hide-and-seek", Weight: 3},
	{Name: "kandinsky-apple-tree", Weight: 3},
	{Name: "cezanne-bathers", Weight: 3},
	{Name: "seurat-grande-jatte", Weight: 2},
	{Name: "gauguin-siesta", Weight: 2},
	{Name: "monet-water-lilies", Weight: 2},
	{Name: "sargent-carnation-lily", Weight: 2},
	{Name: "diebenkorn-seawall", Weight: 2},
	{Name: "redon-green-vase", Weight: 2},
	{Name: "matisse-collioure", Weight: 2},
	{Name: "hopper-night-windows", Weight: 2},
	{Name: "bruegel-icarus", Weight: 2},
	{Name: "klee-fire-evening", Weight: 1},
	{Name: "vangogh-arles", Weight: 1},
	{Name: "avery-bicycle-rider", Weight: 1},
	{Name: "varo-harmony", Weight: 1},
	{Name: "delaunay-bleriot", Weight: 1},
	{Name: "chagall-mariee", Weight: 1},
	{Name: fromFlag, Weight: 0},
}

// colours resolves the colourway trait to the palette a render works from.
func colours(name string, fromCLI palette.Palette) (palette.Palette, error) {
	if name == fromFlag {
		return fromCLI, nil
	}
	p, ok := palette.ByName(name)
	if !ok {
		return palette.Palette{}, fmt.Errorf("scree: no palette %q", name)
	}
	return p, nil
}

// yellowLike identifies pigments that would compete with the reserved gold.
func yellowLike(c palette.Color) bool {
	h, sat, _ := c.HSL()
	return yellowHue(h) && sat >= 0.20
}

func yellowHue(h float64) bool { return h >= 35 && h <= 75 }

// readsYellow applies the same hue/chroma test to a visible colour. HSL
// saturation is unstable near black: #120F0A is technically saturated amber
// but reads as a dark neutral, not as a colour competing with gold.
func readsYellow(c palette.Color) bool {
	h, sat, light := c.HSL()
	return yellowHue(h) && sat >= 0.20 && light >= 0.12
}

// withoutYellow removes yellow-like pigments while preserving provenance and
// the order of every remaining colour.
func withoutYellow(p palette.Palette) (palette.Palette, error) {
	colors := make([]palette.Color, 0, len(p.Colors))
	for _, c := range p.Colors {
		if !yellowLike(c) {
			colors = append(colors, c)
		}
	}
	if len(colors) == 0 {
		return palette.Palette{}, fmt.Errorf("scree: palette %q has no non-yellow colours for --gold", p.Slug)
	}
	// Duet expects two inputs. Repeating the sole surviving pigment keeps the
	// bed monochrome without restoring a colour that gold mode reserved.
	if len(colors) == 1 {
		colors = append(colors, colors[0])
	}
	p.Colors = colors
	return p, nil
}

// inks is everything on the sheet that is not a stone's own pigment: the
// paper under the water, the water between the stones, and the two colours
// the light and the shadow lean toward.
type inks struct {
	paper palette.Color
	joint palette.Color
	warm  palette.Color // the lamp's own colour
	cool  palette.Color // the sky's, which is what a shadow is lit by
	deep  palette.Color // the water's, which a submerged stone drifts toward
	ramp  []palette.Color
}

// inks decides all of them, from the palette rather than from a constant.
//
// A lamp mixed from the same paints as the stones sits inside the picture; an
// invented one — a fixed warm white, a fixed slate blue — reads as a filter
// laid over it, and it takes the palette's provenance with it (invariant 3).
func (s *Sketch) inks(byLum []palette.Color) inks {
	darkest, lightest := byLum[0], byLum[len(byLum)-1]
	warm, cool := warmest(byLum), coolest(byLum)

	return inks{
		// Wet paper: the palette's lightest taken most of the way to white and
		// half the way to grey. Everything on the sheet has to read as colour
		// laid *on* it.
		paper: lightest.Lighten(0.76).Desaturate(0.45),
		// The joint is the water between the stones, not a pigment: the
		// palette's darkest pushed down until it stops being a colour. Drawn
		// in a palette colour it joins the composition, and the bed stops
		// reading as stones lying in water.
		joint: shade(darkest.Desaturate(0.40), 0.38),
		warm:  warm.Lighten(0.60),
		cool:  shade(cool.Desaturate(0.20), 0.55),
		deep:  shade(cool, 0.62),
		// Everything in the palette is available as a stone. A restricted ramp
		// turns a river bed into a diagram of one.
		ramp: append([]palette.Color(nil), byLum...),
	}
}

// warmest and coolest are the palette members furthest toward the red and the
// blue end.
//
// Read off the sRGB channels rather than off the hue angle. Hue wraps, so
// comparing on it needs care that a single signed difference does not — and
// all this has to decide is which of a handful of paints looks like sunlight
// and which looks like sky.
func warmest(cs []palette.Color) palette.Color {
	best, score := cs[0], math.Inf(-1)
	for _, c := range cs {
		if w := c.R - c.B; w > score {
			best, score = c, w
		}
	}
	return best
}

func coolest(cs []palette.Color) palette.Color {
	best, score := cs[0], math.Inf(-1)
	for _, c := range cs {
		if w := c.B - c.R; w > score {
			best, score = c, w
		}
	}
	return best
}

// stone is one stone's dressing, settled before a pixel is drawn.
type stone struct {
	pigment palette.Color
	tone    float64 // the scheme's value for this stone, 0 pale .. 1 full
	load    float64 // how much of that pigment actually went on the paper
	nugget  bool
}

// dress settles every stone's colour.
//
// How colour is organised over the whole bed is decided once, by
// internal/scheme, before any stone is dressed: an arrangement is a property
// of the sheet, not of a stone. Stones are handed over by *area* rather than
// by inradius, because a scheme that reads size is asking how much of the
// sheet a region occupies, and a long thin lobe with a small inradius can
// still be one of the biggest shapes on the paper.
func (s *Sketch) dress(st *cells.Foam, l levels, rng *rand.Rand, aspect float64, ink inks) []stone {
	regions := make([]scheme.Region, st.Len())
	for i, c := range st.Cells() {
		regions[i] = scheme.Region{X: c.CX, Y: c.CY, Size: c.Area}
	}
	// The seed comes off the caller's generator rather than from the context,
	// so the arrangement is fixed by the same stream as the dressing and
	// nothing upstream has to be threaded through.
	m := scheme.New(scheme.Spec{
		Name:    l.scheme,
		Palette: ink.ramp,
		Seed:    rng.Uint64(),
		Aspect:  aspect,
		Passage: s.Passage,
		Accent:  s.Accent,
		Shades:  s.Shades,
		// The water lifts the saturation as well as darkening the stone, and
		// it does it here rather than afterwards so that the scheme's own
		// spread is applied to the colour the stone actually ends up being.
		Saturate: s.Sat + l.wat.sat,
	}, regions)

	out := make([]stone, st.Len())
	for i := range out {
		c := m.At(i)
		// What water does to a stone, all at once. Any one of the three on its
		// own reads as a mistake in the palette: darker alone is underexposed,
		// bluer alone is a colour cast, and more saturated alone is a slider
		// pushed too far. Together they are unmistakably wet.
		p := wetPigment(c.Fill, l, ink)
		out[i] = stone{
			pigment: p,
			tone:    c.Tone,
			// The floor is high on purpose. A scheme's palest stones sit near
			// tone 0, and a wash that thin is indistinguishable from bare
			// paper — which reads as a hole in the bed rather than as a pale
			// stone that the light will do the rest of the work on.
			load: mathx.Clamp01(s.Load * l.wat.load * (0.5 + 0.5*c.Tone)),
		}
	}
	return out
}

var nuggetGold = palette.MustHex("#F3C937")

func wetPigment(c palette.Color, l levels, ink inks) palette.Color {
	p := shade(c, 1-l.wat.soak*0.30)
	return palette.Lerp(p, ink.deep, l.wat.deep)
}

func goldPigment(l levels, ink inks) palette.Color {
	p := wetPigment(nuggetGold, l, ink)
	h, _, light := p.HSL()
	return palette.FromHSL(h, 1, light)
}

// muteOrdinaryYellows keeps scheme shading from moving a neighbouring taupe
// back into the hue range reserved for gold. Lightness and tone stay intact.
func muteOrdinaryYellows(skin []stone) {
	for i := range skin {
		skin[i].pigment = muteYellow(skin[i].pigment)
	}
}

func muteYellow(c palette.Color) palette.Color {
	h, sat, light := c.HSL()
	// Leave room for 8-bit dither to move a warm neutral a few degrees into
	// the reserved arc after this float colour is returned.
	if sat == 0 || light < 0.12 || h < 32 || h > 78 {
		return c
	}
	return palette.FromHSL(0, 0, light)
}

// nuggetCandidates returns the smaller two-thirds of the stones visible after
// the bed's warp. Visibility is measured on a fixed normalized grid so a
// preview and a print choose the same nuggets.
func (s *Sketch) nuggetCandidates(st *cells.Foam, field *noise.Perlin, l levels, aspect float64) []int {
	const rows = 96
	cols := max(1, int(math.Round(rows*aspect)))
	area := make([]int, st.Len())
	for y := range rows {
		for x := range cols {
			u := (float64(x) + 0.5) * aspect / float64(cols)
			v := (float64(y) + 0.5) / rows
			wu, wv := s.warp(field, l, u, v)
			area[st.At(wu, wv).Cell]++
		}
	}
	ids := make([]int, 0, st.Len())
	for id, n := range area {
		if n > 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := area[ids[i]], area[ids[j]]
		if a == b {
			return ids[i] < ids[j]
		}
		return a < b
	})
	return ids[:min(len(ids), (2*len(ids)+2)/3)]
}

// addNuggets chooses two or three candidates without replacement.
func (s *Sketch) addNuggets(skin []stone, st *cells.Foam, field *noise.Perlin, l levels, ink inks, rng *rand.Rand, aspect float64) {
	ids := s.nuggetCandidates(st, field, l, aspect)
	want := min(len(ids), 2+rng.IntN(2))
	for i := range want {
		j := i + rng.IntN(len(ids)-i)
		ids[i], ids[j] = ids[j], ids[i]
		id := ids[i]
		skin[id].pigment = goldPigment(l, ink)
		skin[id].nugget = true
	}
}

// darkestShadow is the least light the bed reflects anywhere: the darkest a
// stone gets is its own pigment at the ambient, since the lean toward the sky
// is value-preserving by construction (atValue) and the wash only ever lays
// that pigment over paler paper.
func darkestShadow(skin []stone, l levels) float64 {
	lo := math.Inf(1)
	for _, d := range skin {
		lo = math.Min(lo, d.pigment.Luminance()*l.lit.amb)
	}
	return lo
}

// sink takes the joint down until it is a clear step below lim.
//
// The water has to be the darkest thing on the sheet — it is what tells the
// eye where one stone ends and the next begins, and a stone that sinks below
// it stops being a stone. The palette on its own cannot promise that: a bed
// painted in the palette's own darks, seen at the ambient, goes below any
// joint mixed from the same handful of colours. So the joint is not a fixed
// fraction of the darkest swatch; it is whatever it has to be to clear the
// deepest shadow this particular bed can actually throw.
//
// A clear step and not a hair: two values a few percent apart read as one.
func sink(c palette.Color, lim float64) palette.Color {
	const clear = 0.6
	lum := c.Luminance()
	if lum <= lim*clear {
		return c
	}
	return shade(c, lim*clear/math.Max(lum, 1e-6))
}

// waterLevels is everything the wet trait resolves to.
type waterLevels struct {
	soak  float64 // how much the water darkens a stone
	sat   float64 // and lifts its saturation
	deep  float64 // how far its colour goes toward the water's own
	sheen float64 // how much it polishes it, ×gloss
	tight float64 // and how far it tightens the highlight, ×sharp
	load  float64 // how heavily the stone reads as painted
}

// newWet draws how much water is standing over the bed.
//
// It is one axis and not six flags because a wet stone changes in all of
// these ways at the same time, and set separately they drift apart: a stone
// that darkened and saturated without its highlight tightening reads as a
// stone painted the wrong colour, and one whose highlight tightened without
// the colour coming up reads as a stone made of plastic.
func newWet(level string, rng *rand.Rand, l *levels) {
	var w waterLevels
	switch level {
	case "dry":
		// A bar above the waterline. Matte, pale, and the only level with a
		// highlight broad enough to read as a soft sheen rather than a gleam.
		w = waterLevels{
			soak: 0, sat: 0, deep: 0,
			sheen: rnd.Uniform(rng, 0.30, 0.50), tight: rnd.Uniform(rng, 0.5, 0.7),
			load: rnd.Uniform(rng, 0.80, 0.90),
		}
	case "damp":
		w = waterLevels{
			soak: rnd.Uniform(rng, 0.24, 0.38), sat: rnd.Uniform(rng, 0.08, 0.18),
			deep:  rnd.Uniform(rng, 0.02, 0.06),
			sheen: rnd.Uniform(rng, 0.85, 1.10), tight: rnd.Uniform(rng, 0.9, 1.1),
			load: rnd.Uniform(rng, 0.90, 0.98),
		}
	case "sunk":
		// Deep and still. The colour goes a long way toward the water's own,
		// which is what flattens a deep pool: everything in it agrees.
		w = waterLevels{
			soak: rnd.Uniform(rng, 0.60, 0.78), sat: rnd.Uniform(rng, 0.12, 0.24),
			deep:  rnd.Uniform(rng, 0.26, 0.40),
			sheen: rnd.Uniform(rng, 1.00, 1.30), tight: rnd.Uniform(rng, 1.1, 1.4),
			load: 1,
		}
	default: // wet
		w = waterLevels{
			soak: rnd.Uniform(rng, 0.45, 0.62), sat: rnd.Uniform(rng, 0.20, 0.34),
			deep:  rnd.Uniform(rng, 0.10, 0.20),
			sheen: rnd.Uniform(rng, 1.35, 1.75), tight: rnd.Uniform(rng, 1.3, 1.7),
			load: 1,
		}
	}
	l.wat = w
}
