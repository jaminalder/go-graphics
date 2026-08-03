package scree

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var (
	_ sketch.Configurable = (*Sketch)(nil)
	_ sketch.Traited      = (*Sketch)(nil)
)

// declare names every knob once. The composition knobs come first: each of
// them is an override on what a trait drew, and a knob left alone is the
// seed's, not this list's.
func (s *Sketch) declare() {
	o := opt.New()

	o.Int("count", "stones the pack aims for", "co", 2, 700, &s.pin.count)
	o.Int("rungs", "steps on the stone size ladder", "ru", 1, 9, &s.pin.rungs)
	o.Float("base", "smallest stone radius, canvas units", "ba", 0.005, 0.3, &s.pin.base)
	o.Float("ratio", "size ladder step ratio", "ra", 1.05, 3, &s.pin.ratio)
	o.Float("gap", "clearance between stones, x radius", "ga", -0.5, 2, &s.pin.gap)
	o.Float("over", "how far the pack reaches past the frame, canvas units", "ov", 0, 0.5, &s.pin.over)

	o.Float("merge", "share of stones merged into a neighbouring lobe", "mg", 0, 1, &s.pin.merge)
	o.Int("max-lobe", "most sites one lobe may absorb", "ml", 1, 6, &s.pin.maxLobe)
	o.Float("warp", "how far the whole bed is bent, x smallest stone", "wp", 0, 8, &s.pin.warp)
	o.Float("swirl", "wavelength of that bending, x smallest stone", "sl", 2, 60, &s.pin.swirl)
	o.Float("round", "radius a stone's corner is worn over, canvas units", "rd", 0, 0.3, &s.pin.round)

	o.Float("ink", "the joint's thickness, canvas units", "ik", 0, 0.05, &s.pin.ink)
	o.Float("swell", "extra thickness where three stones meet, x ink", "sw", 0, 8, &s.pin.swell)
	o.Float("node", "distance over which a third stone counts as near", "nd", 0.002, 0.3, &s.pin.node)
	o.Float("wobble", "hand wander of the joint, x its width", "wb", 0, 1, &s.Wobble)

	o.Float("weight", "how strongly a stone's size bends its walls; 0 is straight", "wt", 0, 2, &s.Weight)
	o.Float("grain", "paper tooth", "gn", 0, 0.4, &s.Grain)
	o.Float("load", "pigment in a stone at full tone", "ld", 0.05, 1, &s.Load)
	o.Float("pool", "wavelength of the pigment's pooling, canvas units", "pl", 0.02, 2, &s.Pool)
	o.Float("uneven", "how strongly the pigment pools", "un", 0, 1.5, &s.Uneven)

	o.Float("accent", "share of stones taking a colour from outside their passage", "ac", 0, 1, &s.Accent)
	o.Float("passage", "wavelength of the colour field, canvas units", "pa", 0.1, 4, &s.Passage)
	o.Float("shades", "how far a stone wanders from its palette swatch", "sh", 0, 2, &s.Shades)
	o.Float("saturate", "lift on every pigment's saturation", "sa", 0, 2, &s.Sat)
	o.Bool("gold", "reserve yellow for two or three rare gold nuggets", "gold", &s.Gold)

	o.Float("soak", "how much the water darkens a stone", "sk", 0, 1, &s.Soak)
	o.Float("sheen", "how much the water polishes it, x gloss", "sn", 0, 4, &s.Sheen)
	o.Float("depth", "how far a stone's colour goes toward the water's own", "dp", 0, 1, &s.Deep)

	s.declareCut(o)

	s.knobs = o
}

// Flags implements sketch.Configurable: the trait dimensions and the per-knob
// overrides share one flat namespace.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.traits.Flags(fs)
	s.knobs.Flags(fs)
}

// Configure implements sketch.Configurable. The trait part of the filename
// needs the resolved set, which only exists once the seed is known, so it is
// added by TraitSuffix at render time rather than here.
func (s *Sketch) Configure() (string, error) {
	if err := s.traits.Configure(); err != nil {
		return "", err
	}
	return s.knobs.Configure()
}
