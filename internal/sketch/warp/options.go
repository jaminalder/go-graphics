package warp

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var _ sketch.Configurable = (*Sketch)(nil)

func (s *Sketch) declare() {
	o := opt.New()
	o.Float("scale", "base field cycles per canvas unit", "sc", 0.25, 12, &s.Scale)
	o.Int("octaves", "number of fBM components", "oc", 1, 8, &s.Octaves)
	o.Float("gain", "amplitude multiplier per octave", "gn", 0.2, 0.85, &s.Gain)
	o.Float("lacunarity", "frequency multiplier per octave", "la", 1.2, 3.5, &s.Lacunarity)
	o.Float("warp-strength", "first domain displacement", "w1", 0, 8, &s.WarpStrength)
	o.Float("nested-strength", "second domain displacement", "w2", 0, 8, &s.NestedStrength)
	o.Choice("warp", "field development mode", "w", []string{"plain", "single", "nested"}, &s.warpName,
		func(value int) { s.fieldMode = mode(value) })
	o.Choice("appearance", "colour mapping", "ap", []string{"gradient", "structure"}, &s.appearanceName,
		func(value int) { s.appearance = appearance(value) })
	s.knobs = o
}

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) { s.knobs.Flags(fs) }

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) { return s.knobs.Configure() }
