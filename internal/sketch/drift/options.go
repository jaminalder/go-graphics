package drift

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var _ sketch.Configurable = (*Sketch)(nil)

func (s *Sketch) declare() {
	o := opt.New()
	o.Choice("style", "painting style", "", []string{"mix", "rings", "scribble", "gouache"}, &s.style,
		func(i int) { s.Style = []Style{StyleMix, StyleRings, StyleScribble, StyleGouache}[i] })
	s.knobs = o
}

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) { s.knobs.Flags(fs) }

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) { return s.knobs.Configure() }
