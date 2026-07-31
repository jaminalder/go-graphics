package pools

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

var (
	_ sketch.Configurable = (*Sketch)(nil)
	_ sketch.Traited      = (*Sketch)(nil)
)

// declare names every knob once. The composition knobs come first: each of
// them is an override on what the fill trait drew, and a knob left alone is
// the seed's, not this list's.
func (s *Sketch) declare() {
	o := opt.New()

	o.Int("count", "anchor circles before satellites", "co", 0, 400, &s.pin.count)
	o.Int("rungs", "steps on the size ladder", "ru", 1, 9, &s.pin.rungs)
	o.Float("base", "smallest circle radius, canvas units", "ba", 0.004, 0.2, &s.pin.base)
	o.Float("ratio", "size ladder step ratio", "ra", 1.05, 3, &s.pin.ratio)
	o.Float("satellites", "share of circles given an overlapping companion", "sat", 0, 1, &s.pin.satellites)
	o.Float("gap", "clearance between circles, x radius; negative lets them cross", "ga", -0.5, 2, &s.pin.gap)
	o.Float("margin", "clear paper at the edge", "ma", 0, 0.4, &s.pin.margin)

	o.Float("ragged", "wash edge deviation; 0 is a true circle, 0.22 a blob", "rg", 0, 0.6, &s.Ragged)
	o.Float("rings", "share of circles carrying inner rings", "ri", 0, 1, &s.Rings)
	o.Float("open", "share painted as annuli rather than discs", "op", 0, 1, &s.Open)
	o.Float("glaze", "share carrying a second pigment on top", "gl", 0, 1, &s.Glaze)
	o.Float("banded", "share filled with concentric rings", "bd", 0, 1, &s.Banded)
	o.Float("band-width", "ring pitch of a banded circle, canvas units", "bw", 0.0008, 0.1, &s.BandWidth)
	o.Float("band-overlap", "how far neighbouring rings cross, x pitch", "bo", 0, 2, &s.BandOverlap)
	o.Int("max-bands", "most rings a banded circle may have", "mb", 2, 40, &s.MaxBands)
	o.Float("alpha", "pool strength; below 1 keeps crossings readable", "al", 0.05, 1, &s.Alpha)
	o.Int("pigments", "palette colours in play", "pi", 1, 12, &s.Pigments)
	o.Float("ground", "strength of the painted ground wash; 0 is bare paper", "gr", 0, 1, &s.Ground)
	o.Float("ground-blotch", "wavelength of the ground's unevenness, canvas units", "gb", 0.02, 3, &s.GroundBlotch)

	s.knobs = o
}

// Flags implements sketch.Configurable: the trait dimensions and the
// per-knob overrides share one flat namespace.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.traits.Flags(fs)
	s.knobs.Flags(fs)
}

// Configure implements sketch.Configurable. The trait part of the filename
// needs the resolved set, which only exists once the seed is known, so it
// is added by TraitSuffix at render time rather than here.
func (s *Sketch) Configure() (string, error) {
	if err := s.traits.Configure(); err != nil {
		return "", err
	}
	return s.knobs.Configure()
}

// TraitSuffix implements sketch.Traited.
func (s *Sketch) TraitSuffix(set trait.Set) string { return s.traits.NameSuffix(set) }
