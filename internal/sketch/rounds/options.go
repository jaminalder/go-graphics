package rounds

import (
	"flag"
	"fmt"

	"github.com/jaminalder/go-graphics/internal/sketch"
)

type cliOptions struct {
	human float64
}

var _ sketch.Configurable = (*Sketch)(nil)

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	fs.Float64Var(&s.opts.human, "human", -1, "humanization 0 (machine-perfect) to 1 (full dry-brush); default 0.8")
}

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) {
	if s.opts.human < 0 {
		return "", nil // default
	}
	if s.opts.human > 1 {
		return "", fmt.Errorf("--human must be within [0, 1]")
	}
	s.Human = s.opts.human
	return fmt.Sprintf("-h%g", s.opts.human), nil
}
