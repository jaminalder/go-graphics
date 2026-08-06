package iris

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

func (s *Sketch) declare() {
	o := opt.New()
	o.Float("scale", "field cycles around the stroma", "sc", 0.3, 6, &s.Scale)
	o.Int("octaves", "number of fBM components", "oc", 1, 8, &s.Octaves)
	o.Float("gain", "amplitude multiplier per octave", "gn", 0.2, 0.85, &s.Gain)
	o.Float("lacunarity", "frequency multiplier per octave", "la", 1.2, 3.5, &s.Lacunarity)
	o.Float("stretch", "how fast the field changes from pupil to limbus", "st", 0, 4, &s.Stretch)
	o.Float("twist", "radians the fibres rotate between pupil and limbus", "tw", -6, 6, &s.Twist)
	o.Float("fiber", "tangential warp: sideways meander of the fibres", "fb", 0, 3, &s.Fiber)
	o.Float("radial", "radial warp: interruption along a fibre", "rd", 0, 3, &s.Radial)
	o.Float("nested", "second warp displacement", "ns", 0, 3, &s.Nested)
	o.Float("depth", "gamma on the value axis: how far the midtones sink", "dp", 0, 1, &s.Depth)
	o.Float("gleam", "how far sharp crests brighten toward the light colour", "gl", 0, 1, &s.Gleam)
	o.Float("warmth", "how far the pupillary zone pulls toward the accent colour", "wm", 0, 1, &s.Warmth)
	o.Float("bands", "strata terraces across the stroma, or filament contour levels", "bd", 1, 40, &s.Bands)
	o.Float("cells", "crypt: cell density across the stroma", "cl", 2, 40, &s.Cells)
	o.Float("limbus", "iris radius as a fraction of canvas height", "lb", 0.15, 0.5, &s.Limbus)
	o.Float("pupil", "pupil radius as a fraction of the iris", "pu", 0.05, 0.6, &s.Pupil)
	s.knobs = o
	s.traits = trait.NewOptions(schema)
}

// Flags implements sketch.Configurable: the trait dimensions and the numeric
// overrides share one flat namespace.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.traits.Flags(fs)
	s.knobs.Flags(fs)
}

// Configure implements sketch.Configurable. The trait part of the filename
// needs the resolved set, which exists only once the seed is known, so it is
// added by TraitSuffix at render time.
func (s *Sketch) Configure() (string, error) {
	if err := s.traits.Configure(); err != nil {
		return "", err
	}
	return s.knobs.Configure()
}

// Schema implements sketch.Traited.
func (s *Sketch) Schema() trait.Schema { return schema }

// Traits implements sketch.Traited.
func (s *Sketch) Traits(ctx sketch.Context) trait.Set {
	return s.traits.Resolve(ctx.RNG(streamTraits))
}

// TraitSuffix implements sketch.Traited.
func (s *Sketch) TraitSuffix(set trait.Set) string { return s.traits.NameSuffix(set) }

// pin lays any explicitly set numeric flag over the drawn recipe. Pinning one
// number must not disturb the rest of the draw.
func (s *Sketch) pin(set settings) settings {
	for name, apply := range map[string]func(){
		"scale":      func() { set.scale = s.Scale },
		"octaves":    func() { set.octaves = s.Octaves },
		"gain":       func() { set.gain = s.Gain },
		"lacunarity": func() { set.lacunarity = s.Lacunarity },
		"stretch":    func() { set.stretch = s.Stretch },
		"twist":      func() { set.twist = s.Twist },
		"fiber":      func() { set.fiber = s.Fiber },
		"radial":     func() { set.radial = s.Radial },
		"nested":     func() { set.nested = s.Nested },
		"depth":      func() { set.depth = s.Depth },
		"gleam":      func() { set.gleam = s.Gleam },
		"warmth":     func() { set.warmth = s.Warmth },
		"bands":      func() { set.bands = s.Bands },
		"cells":      func() { set.cells = s.Cells },
		"limbus":     func() { set.limbus = s.Limbus },
		"pupil":      func() { set.pupil = s.Pupil },
	} {
		if s.knobs.WasSet(name) {
			apply()
		}
	}
	return set
}
