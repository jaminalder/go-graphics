package hatchbook

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var _ sketch.Configurable = (*Sketch)(nil)

func (s *Sketch) declare() {
	o := opt.New()
	o.Choice("page", "which specimen sheet to draw", "", PageNames(), &s.Page, func(int) {})
	o.Float("margin", "border around the grid, canvas units", "mg", 0, 0.3, &s.Margin)
	o.Float("gutter", "space between squares, canvas units", "gt", 0, 0.2, &s.Gutter)
	s.knobs = o
}

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) { s.knobs.Flags(fs) }

// Configure implements sketch.Configurable. The page is the identity of the
// sheet rather than a variation on it, so it is always in the filename —
// opt would leave a default out, and four files called "hatchbook" is not a
// catalogue.
func (s *Sketch) Configure() (string, error) {
	suffix, err := s.knobs.Configure()
	if err != nil {
		return "", err
	}
	if _, err := pageByName(s.Page); err != nil {
		return "", err
	}
	return "-" + s.Page + suffix, nil
}
