package scheme

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// Which ramp a strategy reads matters as much as where it reads it.
//
// A luminance ramp's neighbours agree about lightness and can disagree
// wildly about hue, so it is right for value structure and wrong for
// "something like this colour". A chroma ramp answers "which of these is
// actually a pigment", which most ColorLisa palettes need because they carry
// a near-neutral at each end. A warmth ramp is the only one a temperature
// run can follow.

// pick reads a ramp at t ∈ [0,1].
func pick(ramp []palette.Color, t float64) palette.Color {
	if len(ramp) == 0 {
		return palette.Color{}
	}
	return ramp[min(int(mathx.Clamp01(t)*float64(len(ramp))), len(ramp)-1)]
}

// away picks a colour somewhere else on the ramp entirely, wrapped round
// rather than nudged.
//
// A neighbouring swatch is the obvious choice and it is the wrong one: two
// colours that differ slightly read as one colour laid unevenly, which is
// what a plain fill already looks like.
func away(ramp []palette.Color, t float64, rng *rand.Rand) palette.Color {
	return pick(ramp, math.Mod(t+rnd.Uniform(rng, 0.3, 0.7), 1))
}

// byChroma orders a palette by colourfulness, most colourful first.
func byChroma(colors []palette.Color) []palette.Color {
	out := append([]palette.Color(nil), colors...)
	sat := func(c palette.Color) float64 { _, s, l := c.HSL(); return s * (1 - math.Abs(2*l-1)) }
	// Insertion sort: a palette is a handful of colours, and this keeps the
	// order stable without allocating a comparator per render.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && sat(out[j]) > sat(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// byWarmth orders a palette from coolest to warmest. Warmth is the hue's
// projection onto the orange axis; a colour with no saturation has no
// temperature at all and belongs in the middle rather than at an end.
func byWarmth(colors []palette.Color) []palette.Color {
	out := append([]palette.Color(nil), colors...)
	score := func(c palette.Color) float64 {
		h, s, _ := c.HSL()
		if s < 0.05 {
			return 0
		}
		return s * math.Cos((h-35)*math.Pi/180)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && score(out[j]) < score(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// farthestHue is the colour furthest round the wheel from c, weighted by
// saturation — an accent has to actually be a colour, so a distant grey does
// not win over a nearly-opposite pigment.
func farthestHue(colors []palette.Color, c palette.Color) palette.Color {
	if len(colors) == 0 {
		return c
	}
	h0, _, _ := c.HSL()
	best, bestScore := colors[0], -1.0
	for _, o := range colors {
		h, s, _ := o.HSL()
		d := math.Abs(math.Mod(math.Abs(h-h0), 360) - 180)
		if score := (180 - d) * (0.3 + 0.7*s); score > bestScore {
			best, bestScore = o, score
		}
	}
	return best
}

// hueArc is the run of palette colours whose hue lies within span degrees of
// c, ordered by hue so it can be read as a ramp. Never fewer than two: a
// palette with one strong hue in it still has to yield an analogous scheme,
// so the nearest neighbours are taken whether or not they are inside the arc.
func hueArc(colors []palette.Color, c palette.Color, span float64) []palette.Color {
	h0, _, _ := c.HSL()
	type scored struct {
		c palette.Color
		d float64
	}
	all := make([]scored, 0, len(colors))
	for _, o := range colors {
		h, _, _ := o.HSL()
		all = append(all, scored{o, hueGap(h, h0)})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].d < all[j].d })

	out := make([]palette.Color, 0, len(all))
	for i, a := range all {
		if a.d <= span || i < 3 {
			out = append(out, a.c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		hi, _, _ := out[i].HSL()
		hj, _, _ := out[j].HSL()
		return hi < hj
	})
	return out
}

// spread picks n palette colours as far apart round the hue wheel as the
// palette allows, greedily: the most colourful first, then whatever is
// furthest in hue from everything already chosen.
//
// Picked from the palette rather than computed as h, h+120, h+240 because
// the palettes here are paintings' colours and have provenance; a scheme
// that synthesises its own hues has stopped using them.
func spread(colors []palette.Color, n int) []palette.Color {
	if len(colors) == 0 {
		return nil
	}
	out := []palette.Color{colors[0]}
	for len(out) < n && len(out) < len(colors) {
		best, bestD := colors[0], -1.0
		for _, o := range colors {
			nearest := 360.0
			for _, p := range out {
				h1, _, _ := o.HSL()
				h2, _, _ := p.HSL()
				nearest = math.Min(nearest, hueGap(h1, h2))
			}
			_, sat, _ := o.HSL()
			if score := nearest * (0.4 + 0.6*sat); score > bestD {
				best, bestD = o, score
			}
		}
		out = append(out, best)
	}
	return out
}

// hueGap is the distance between two hues round the wheel, in degrees.
func hueGap(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	return math.Min(d, 360-d)
}
