package qql

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

var (
	_ sketch.Configurable = (*Sketch)(nil)
	_ sketch.Traited      = (*Sketch)(nil)
)

// Flags implements sketch.Configurable: one flag per trait dimension, so
// any single axis of the output space can be pinned while the rest of the
// seed's draw is left alone. Flag names are stable — they are embedded in
// the recipe metadata of every rendered file.
func (s *Sketch) Flags(fs *flag.FlagSet) { s.opts.Flags(fs) }

// Configure implements sketch.Configurable. The trait part of the filename
// needs the resolved set, which only exists once the seed is known, so it
// is added by the trait schema at render time rather than here.
func (s *Sketch) Configure() (string, error) {
	return "", s.opts.Configure()
}

// TraitSuffix implements sketch.Traited.
func (s *Sketch) TraitSuffix(set trait.Set) string { return s.opts.NameSuffix(set) }
