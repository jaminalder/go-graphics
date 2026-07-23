package drift

import (
	"flag"
	"fmt"

	"github.com/jaminalder/go-graphics/internal/sketch"
)

type cliOptions struct {
	style string
}

var _ sketch.Configurable = (*Sketch)(nil)

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	fs.StringVar(&s.opts.style, "style", "mix", "painting style: mix|rings|scribble|gouache")
}

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) {
	switch s.opts.style {
	case "", "mix":
		s.Style = StyleMix
		return "", nil
	case "rings":
		s.Style = StyleRings
	case "scribble":
		s.Style = StyleScribble
	case "gouache":
		s.Style = StyleGouache
	default:
		return "", fmt.Errorf("unknown style %q (mix|rings|scribble|gouache)", s.opts.style)
	}
	return "-" + s.opts.style, nil
}
