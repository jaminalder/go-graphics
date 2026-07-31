package rounds

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var _ sketch.Configurable = (*Sketch)(nil)

func (s *Sketch) declare() {
	o := opt.New()
	o.Float("human", "humanization, 0 machine-perfect to 1 full dry-brush", "h", 0, 1, &s.Human)
	s.knobs = o
}

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) { s.knobs.Flags(fs) }

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) { return s.knobs.Configure() }
