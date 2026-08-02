package foam

// How colour is organised over the sheet.
//
// A packed foam is a hundred and fifty small cells, and at that count *how
// colour is distributed* matters more than what any one cell is painted
// with. Giving every cell a free draw from the palette gives confetti; a
// low-frequency field gives passages; but neither is the only way a painter
// organises a sheet, and each strategy is a different picture from the same
// structure and the same fill.
//
// The strategies themselves live in internal/scheme, because arranging
// colour over a set of regions is not a fact about foams — a packing, a
// mosaic or a set of brush marks all want the same vocabulary. This file is
// the adapter: it turns cells into regions, and hands the answers back in
// the terms the dressing wants.

import (
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/scheme"
)

// The schemes a seed or a flag may name. Kept as foam-local constants so the
// trait schema reads in one place; the values are internal/scheme's.
const (
	schemePassage    = scheme.Passage
	schemeGradient   = scheme.Gradient
	schemeSequence   = scheme.Sequence
	schemeInherit    = scheme.Inherit
	schemeDominance  = scheme.Dominance
	schemeComplement = scheme.Complement
	schemeAnalogous  = scheme.Analogous
	schemeTriad      = scheme.Triad
	schemeQuiet      = scheme.Monochrome
	schemeNotan      = scheme.Notan
	schemeAnchor     = scheme.Anchor
	schemeWeather    = scheme.Temperature
	schemeDuet       = scheme.Duet
	schemeByScale    = scheme.BySize
	schemeFromLum    = scheme.ByDarkness
)

// mixer is the sheet's colour arrangement, resolved once per render.
type mixer struct{ m *scheme.Mixer }

// mixFor settles the arrangement for every cell.
//
// Cells are handed over by *area*, not by inradius: a scheme that reads size
// is asking how much of the sheet a region occupies, and a long thin lobe
// with a small inradius can still be one of the biggest shapes on the paper.
func (s *Sketch) mixFor(f *cells.Foam, l levels, rng *rand.Rand, ramp []palette.Color, aspect float64) *mixer {
	regions := make([]scheme.Region, f.Len())
	for i, c := range f.Cells() {
		regions[i] = scheme.Region{X: c.CX, Y: c.CY, Size: c.Area}
	}
	// The seed comes off the caller's generator rather than from the context,
	// so the arrangement is determined by the same stream as the dressing and
	// nothing upstream has to be threaded through.
	return &mixer{m: scheme.New(scheme.Spec{
		Name:    l.scheme,
		Palette: ramp,
		Seed:    rng.Uint64(),
		Aspect:  aspect,
		Passage: s.Passage,
		Accent:  s.Accent,
	}, regions)}
}

// draw gives cell i its pigment, the pigment it is charged with, and its
// tone — how heavily it was loaded, in [0,1].
func (m *mixer) draw(i int) (pigment, second palette.Color, tone float64) {
	c := m.m.At(i)
	return c.Fill, c.Accent, c.Tone
}
