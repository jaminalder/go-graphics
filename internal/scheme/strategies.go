package scheme

import (
	"math"
	"sort"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// resolve colours every region. Strategies that depend on the *set* rather
// than on one region at a time — sequence, inherit — are written as whole
// passes; the rest are per-region and share the loop.
func (s *state) resolve() {
	switch s.spec.Name {
	case Sequence:
		s.sequence()
		s.shades()
		return
	case Inherit:
		s.inherit()
		s.shades()
		return
	}
	for i, r := range s.regions {
		s.out[i] = s.one(r)
	}
	s.shades()
}

// shades spreads each fill into a family around its palette colour.
//
// Applied as a pass over the finished arrangement rather than inside each
// strategy, so that it reaches sequence and inherit too, and so that a
// strategy never has to think about it. The tone is deliberately left alone:
// it is the value structure the arrangement decided, and letting a shade
// jitter move it would blur exactly the thing that carries the composition.
func (s *state) shades() {
	if s.spec.Shades <= 0 {
		return
	}
	k := s.spec.Shades
	for i := range s.out {
		h, sat, l := s.out[i].Fill.HSL()
		s.out[i].Fill = palette.FromHSL(
			h+rnd.Gauss(s.rng, 0, 4*k),
			mathx.Clamp01(sat*(1+rnd.Gauss(s.rng, 0, 0.28*k))),
			mathx.Clamp01(l+rnd.Gauss(s.rng, 0, 0.10*k)),
		)
	}
}

// one colours a single region.
func (s *state) one(r Region) Colour {
	switch s.spec.Name {
	case Gradient:
		// Hue runs along a direction across the frame. Jittered per region,
		// or the boundary between the two ends is a visible straight line
		// cutting across the composition.
		t := mathx.Clamp01(s.along(r) + rnd.Gauss(s.rng, 0, 0.09))
		return Colour{Fill: pick(s.lum, t), Accent: away(s.lum, t, s.rng), Tone: 0.15 + 0.8*s.value(r)}

	case Dominance:
		// Three colours in roughly 70/20/10. Proportion is as much of
		// harmony as hue is: a uniform draw over the palette gives every
		// colour equal presence, which reads as a sampler rather than as a
		// picture, and it is the commonest thing to get wrong.
		top := s.chroma[:min(3, len(s.chroma))]
		fill := top[rnd.PickIndex(s.rng, dominanceWeights[:len(top)])]
		return Colour{Fill: fill, Accent: farthestHue(s.chroma, fill), Tone: 0.15 + 0.8*s.value(r)}

	case Complement:
		// A muted dominant over most of the area with small saturated
		// accents of its opposite. Not 50/50 — a complementary scheme is
		// four fifths muted green and one fifth intense red, and split
		// evenly it is a flag.
		base, opp := s.chroma[0], farthestHue(s.chroma, s.chroma[0])
		if s.rng.Float64() < 0.18 {
			return Colour{Fill: opp, Accent: base, Tone: 0.85 + 0.15*s.rng.Float64()}
		}
		// Muted by desaturating the palette's own colour rather than by
		// inventing one: the palettes carry their provenance and a scheme
		// that synthesises hues is no longer painting with them.
		return Colour{Fill: base.Desaturate(rnd.Uniform(s.rng, 0.35, 0.6)), Accent: opp, Tone: 0.12 + 0.7*s.value(r)}

	case Analogous:
		// Hue confined to one arc of the wheel, chosen *from the palette*,
		// with the value doing the work. It needs a real value range or it
		// collapses into a single flat field.
		t := s.patch(r, 1, 0, 0)
		return Colour{
			Fill:   pick(s.arc, mathx.Clamp01(t+rnd.Gauss(s.rng, 0, 0.1))),
			Accent: farthestHue(s.chroma, s.arc[0]),
			Tone:   0.1 + 0.85*s.value(r),
		}

	case Triad:
		// Three palette colours spread as far round the wheel as the palette
		// allows, in unequal proportion — equal thirds is a flag rather than
		// a picture.
		k := rnd.PickIndex(s.rng, triadWeights[:len(s.triad)])
		return Colour{
			Fill:   s.triad[k],
			Accent: s.triad[(k+1)%len(s.triad)],
			Tone:   0.15 + 0.8*s.value(r),
		}

	case Monochrome:
		// One pigment at every dilution, and a handful of regions allowed to
		// shout. The spark is what makes the restraint read as a choice
		// rather than as a palette failure.
		//
		// The dilution is the Tone, not the Fill: baked into the colour it
		// would make a "near-monochrome" sheet report a hundred different
		// pigments, and a caller laying a wash would have no load to read.
		house := s.chroma[min(1, len(s.chroma)-1)]
		if s.rng.Float64() < s.spec.Accent*0.3 {
			spark := farthestHue(s.chroma, house)
			return Colour{Fill: spark, Accent: spark, Tone: rnd.Uniform(s.rng, 0.85, 1)}
		}
		return Colour{Fill: house, Accent: house, Tone: rnd.Uniform(s.rng, 0.05, 0.8)}

	case Notan:
		// Two or three values and almost no hue movement: the strongest
		// carrying power of any of them, and the one that survives being
		// looked at from across the room. The values are the palette's own
		// darkest, middle and lightest, so it stays a painting in these
		// colours rather than a greyscale study.
		g := s.patch(r, 1.3, 41.7, -13.9)
		switch {
		case g > 0.66:
			return Colour{Fill: s.lum[0], Accent: s.chroma[0], Tone: 1}
		case g < 0.36:
			return Colour{Fill: s.lum[len(s.lum)-1], Accent: s.chroma[0], Tone: 0.05}
		default:
			return Colour{Fill: s.lum[len(s.lum)/2], Accent: s.chroma[0], Tone: 0.5}
		}

	case Anchor:
		// A value structure with the dark regions arriving in a *cluster*
		// rather than scattered, which is what gives the composition
		// somewhere to sit.
		g := s.patch(r, 1.4, 31.7, -19.3)
		if g > 0.62 {
			return Colour{Fill: pick(s.lum, rnd.Uniform(s.rng, 0, 0.4)), Accent: away(s.lum, s.rng.Float64(), s.rng), Tone: 0.72 + 0.28*g}
		}
		return Colour{Fill: pick(s.lum, rnd.Uniform(s.rng, 0.35, 1)), Accent: away(s.lum, s.rng.Float64(), s.rng), Tone: 0.06 + 0.5*g}

	case Temperature:
		// Warm to cool along a direction, with the value on its own field.
		// Both on the same axis would be a gradient and nothing more.
		t := mathx.Clamp01(s.along(r) + rnd.Gauss(s.rng, 0, 0.1))
		return Colour{
			Fill:   pick(s.warm, t),
			Accent: pick(s.warm, mathx.Clamp01(t+rnd.Gauss(s.rng, 0, 0.18))),
			Tone:   0.1 + 0.85*s.patch(r, 1.2, -51.2, 38.6),
		}

	case Duet:
		// Two pigments, and every colour on the sheet a mix of the same two —
		// the discipline every watercolour teacher starts with. Both are
		// drawn by chroma rather than off the luminance ramp: most palettes
		// here have a near-neutral at each end, and two greys mix to grey.
		a := s.chroma[0]
		b := farthestHue(s.chroma[:max(len(s.chroma)*2/3, 2)], a)
		t := mathx.Clamp01(s.patch(r, 1, 0, 0) + rnd.Gauss(s.rng, 0, 0.14))
		other := b
		if t > 0.5 {
			other = a
		}
		return Colour{Fill: palette.LerpHSL(a, b, t), Accent: other, Tone: 0.15 + 0.8*s.patch(r, 0.8, 9.1, 4.4)}

	case BySize:
		// Colour follows the region's *size*. The structure has already
		// sorted itself into big shapes and slivers, and colouring by that
		// makes the structure legible — a gradation with no spatial gradient
		// in it at all. Reliable, and underused.
		t := s.rank(r)
		return Colour{
			Fill:   pick(s.lum, mathx.Clamp01(1-t+rnd.Gauss(s.rng, 0, 0.08))),
			Accent: away(s.lum, 1-t, s.rng),
			Tone:   0.15 + 0.8*(1-t),
		}

	case ByDarkness:
		// The same idea the other way round: size decides the tone and a
		// field decides the hue, so the composition keeps its passages and
		// gains a value structure that follows the packing.
		//
		// The hue field is its own — offset and at its own scale. Sharing
		// Passage's field made this scheme give 85% of cells the same colour
		// Passage did, differing only in weight, which is one idea shown
		// twice rather than two schemes.
		t := s.rank(r)
		hue := s.patch(r, 0.6, -72.4, 55.1)
		return Colour{Fill: pick(s.lum, hue), Accent: away(s.lum, hue, s.rng), Tone: 0.1 + 0.85*(1-t)}

	default: // Passage
		// Passages of related colour — a green corner, a brown corner —
		// rather than confetti, with a share of regions taking an accent
		// from elsewhere on the ramp.
		t := s.patch(r, 1, 0, 0)
		fill := pick(s.lum, t)
		if s.rng.Float64() < s.spec.Accent {
			fill = pick(s.lum, s.rng.Float64())
		}
		return Colour{Fill: fill, Accent: away(s.lum, t, s.rng), Tone: 0.15 + 0.75*s.value(r)}
	}
}

// dominanceWeights is the classical 70/20/10: a dominant, a secondary and an
// accent. The numbers matter less than the fact that they are unequal.
var dominanceWeights = [3]float64{7, 2, 1}

// triadWeights keeps a triad from reading as a flag.
var triadWeights = [3]float64{6, 3, 1}

// sequence walks a sorted ramp in region order, deviating as it goes.
//
// The order is spatial — regions projected onto one direction — so stepping
// along the ramp lays the composition down in bands. Shuffling the order
// instead gives the same colours with none of the structure, which is the
// whole difference between a sequence and a draw.
func (s *state) sequence() {
	idx := make([]int, len(s.regions))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return s.along(s.regions[idx[a]]) < s.along(s.regions[idx[b]])
	})

	t := s.rng.Float64()
	for _, i := range idx {
		// A small step forward with an occasional stumble. Too large a step
		// and the walk is a shuffle; too small and the whole sheet is one
		// colour.
		t += rnd.Gauss(s.rng, 1.0/float64(max(len(idx), 1))*float64(len(s.lum))*0.9, 0.02)
		t = math.Mod(math.Abs(t), 1)
		s.out[i] = Colour{
			Fill:   pick(s.lum, t),
			Accent: away(s.lum, t, s.rng),
			Tone:   0.15 + 0.8*s.value(s.regions[i]),
		}
	}
}

// inherit gives each region its nearest already-coloured neighbour's colour,
// mutating occasionally.
//
// It builds large unified chunks with organic edges, which is what a field
// cannot do — a field's patches are always field-shaped. The mutation rate
// is the whole knob: raise it and the thing degenerates into confetti.
func (s *state) inherit() {
	idx := make([]int, len(s.regions))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return s.along(s.regions[idx[a]]) < s.along(s.regions[idx[b]])
	})

	const mutate = 0.16
	done := make([]int, 0, len(idx))
	for _, i := range idx {
		r := s.regions[i]
		parent := -1
		if len(done) > 0 && s.rng.Float64() >= mutate {
			best := math.Inf(1)
			for _, j := range done {
				o := s.regions[j]
				if d := math.Hypot(o.X-r.X, o.Y-r.Y); d < best {
					best, parent = d, j
				}
			}
		}
		if parent < 0 {
			t := s.rng.Float64()
			s.out[i] = Colour{Fill: pick(s.chroma, t), Accent: away(s.lum, t, s.rng), Tone: 0.15 + 0.8*s.value(r)}
		} else {
			// Inherited, with a hair of drift so a chunk is a family rather
			// than a flat stencil.
			h, sat, l := s.out[parent].Fill.HSL()
			s.out[i] = Colour{
				Fill:   palette.FromHSL(h+rnd.Gauss(s.rng, 0, 3), sat, mathx.Clamp01(l+rnd.Gauss(s.rng, 0, 0.03))),
				Accent: s.out[parent].Accent,
				Tone:   mathx.Clamp01(s.out[parent].Tone + rnd.Gauss(s.rng, 0, 0.06)),
			}
		}
		done = append(done, i)
	}
}
