package hatch

// Composition is the other half of the design. Cross-hatching is two
// parallel families; weave is two families with a rule about which is on
// top; nesting is a hatch whose parameters change with the region. None of
// those needs a structure of its own — they need a way to put coverage
// functions together, which is what this file is.

// Func is a resolved hatch as a plain function, so that combinations of
// hatches are the same kind of thing as a single one.
type Func func(Sample) float64

// Func returns the hatch as a plain coverage function.
func (h *Hatch) Func() Func { return h.Cover }

// Of resolves a spec straight to a Func, for the common case of building
// one inline.
func Of(s Spec) Func { return New(s).Cover }

// Over layers coverage functions front to back: the result is what you see
// when each is drawn over the last in one ink. Coverage composes as
// transparency (1 − ∏(1 − cᵢ)) rather than by addition, so two families
// crossing give a crossing rather than a burnt-out patch.
func Over(fs ...Func) Func {
	return func(s Sample) float64 {
		clear := 1.0
		for _, f := range fs {
			clear *= 1 - f(s)
		}
		return 1 - clear
	}
}

// Cross is hatching crossed with itself at other angles: the canonical
// cross-hatch is Cross(spec, spec.Angle+π/2), and a third or fourth family
// darkens it the way an etcher would.
//
// Each family is given its own seed so their jitter and dash phases are
// independent; two families that wander in step read as one drawn twice.
func Cross(s Spec, angles ...float64) Func {
	fs := make([]Func, 0, len(angles)+1)
	fs = append(fs, Of(s))
	for i, a := range angles {
		fs = append(fs, Of(s.With(func(c *Spec) {
			c.Angle = a
			c.Seed ^= uint64(i+1) * saltLine
		})))
	}
	return Over(fs...)
}

// Weave lays two families over each other with an over-under rule and
// reports what is visible of each, so a caller can put them in different
// inks and see the threads pass over and under one another.
//
// Which family is on top alternates with the parity of the two mark indices
// — thread 3 of one crossing thread 4 of the other — which is exactly the
// rule a plain weave follows. Anything else (one family always on top, or a
// random choice per crossing) reads as two hatchings stacked, not as cloth.
func Weave(a, b Spec) func(Sample) (coverA, coverB float64) {
	ha, hb := New(a), New(b)
	return func(s Sample) (float64, float64) {
		ca, ka := ha.CoverLine(s)
		cb, kb := hb.CoverLine(s)
		if mod(ka+kb, 2) == 0 {
			// a is the thread passing over.
			return ca, cb * (1 - ca)
		}
		return ca * (1 - cb), cb
	}
}

// Nested is a hatch whose parameters change with the region: a sheet
// divided into panels, each hatched differently, or a cell whose interior
// is subdivided again.
//
// sub is the caller's subdivision. It re-describes the point in terms of
// the sub-region containing it — a new centre, reach, wall and axis — and
// says which of the inner hatches fills it. Re-describing rather than just
// choosing is the point: an inner hatch fitted to its region has to be told
// what that region is, and the caller is the only one who knows.
func Nested(sub func(Sample) (Sample, int), inner ...Func) Func {
	return func(s Sample) float64 {
		if len(inner) == 0 {
			return 0
		}
		in, i := sub(s)
		if i < 0 || i >= len(inner) {
			return 0
		}
		return inner[i](in)
	}
}

// Mask multiplies a hatch by a caller-supplied coverage — the region's own
// shape, a vignette, a wash's density. It is how a hatch is confined to
// something whose outline this package has no way to know.
func Mask(f Func, m func(Sample) float64) Func {
	return func(s Sample) float64 { return f(s) * m(s) }
}
