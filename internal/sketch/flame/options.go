package flame

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
	o.Float("quality", "samples per output pixel", "q", 1, 400, &s.Quality)
	o.Float("gamma", "lifts faint filaments; higher is noisier", "gm", 0.4, 8, &s.Gamma)
	o.Float("vibrancy", "1 keeps colour, 0 gammas each channel", "vb", 0, 1, &s.Vibrancy)
	o.Float("brightness", "mid-density lift before the log", "br", 0.2, 8, &s.Brightness)
	o.Float("gleam", "how far the densest cores move toward white", "gl", 0, 1, &s.Gleam)
	o.Float("scale", "multiplies the auto-framed camera", "sc", 0.3, 4, &s.Scale)
	o.Float("estimator", "density-estimation radius in pixels; 0 disables", "de", 0, 40, &s.Estimator)
	o.Float("de-min", "minimum DE radius (cores)", "", 0, 20, &s.DeMin)
	o.Float("de-curve", "DE radius ~ 1/n^curve", "", 0.05, 2, &s.DeCurve)
	s.knobs = o
	s.traits = trait.NewOptions(schema)
}

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.traits.Flags(fs)
	s.knobs.Flags(fs)
}

// Configure implements sketch.Configurable.
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

func (s *Sketch) pin(set settings) settings {
	for name, apply := range map[string]func(){
		"quality":    func() { set.quality = s.Quality },
		"gamma":      func() { set.gamma = s.Gamma },
		"vibrancy":   func() { set.vibrancy = s.Vibrancy },
		"brightness": func() { set.brightness = s.Brightness },
		"gleam":      func() { set.gleam = s.Gleam },
		"scale":      func() { set.scale = s.Scale },
		"estimator":  func() { set.estimator = s.Estimator },
		"de-min":     func() { set.deMin = s.DeMin },
		"de-curve":   func() { set.deCurve = s.DeCurve },
	} {
		if s.knobs.WasSet(name) {
			apply()
		}
	}
	return set
}
