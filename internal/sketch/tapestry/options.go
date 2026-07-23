package tapestry

import (
	"flag"
	"fmt"
	"strings"

	"github.com/jaminalder/go-graphics/internal/sketch"
)

// cliOptions holds the raw flag values; Configure translates them into
// sketch fields. Kept separate so flag parsing never half-configures the
// sketch.
type cliOptions struct {
	noStripes    bool
	relief       bool
	reliefPreset string
	grain        bool
	crackle      bool
	grainSeed    uint64
	terraceSeed  uint64
	smooth       float64
}

var _ sketch.Configurable = (*Sketch)(nil)

// Flags implements sketch.Configurable. Flag names are stable — they are
// embedded in the recipe metadata of every rendered file.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	fs.BoolVar(&s.opts.noStripes, "no-stripes", false, "render without the vertical stripe layer")
	fs.BoolVar(&s.opts.relief, "relief", false, "3D relief shading (hillshade + paper-cut edges)")
	fs.StringVar(&s.opts.reliefPreset, "relief-preset", "", "named relief look (implies --relief): "+strings.Join(ReliefPresetNames(), "|"))
	fs.BoolVar(&s.opts.grain, "grain", false, "boost grain strongly on some of the wide terraces")
	fs.BoolVar(&s.opts.crackle, "crackle", false, "crack network on some of the wide terraces")
	fs.Uint64Var(&s.opts.grainSeed, "grain-seed", 0, "seed for the grain/crackle assignment (0 = terrace seed, implies --grain); vary for different effect layouts on the same image")
	fs.Uint64Var(&s.opts.terraceSeed, "terrace-seed", 0, "seed for the terrace layout (0 = main seed); vary for different terracings of the same composition")
	fs.Float64Var(&s.opts.smooth, "smooth", 0, "fBm persistence override, e.g. 0.35 for smoother terrace lines (0 = default 0.5)")
}

// Configure implements sketch.Configurable. The suffix part order is
// stable — it determines output filenames.
func (s *Sketch) Configure() (string, error) {
	o := s.opts
	var suffix strings.Builder

	if o.relief || o.reliefPreset != "" {
		s.Relief = true
		suffix.WriteString("-relief")
		if o.reliefPreset != "" && o.reliefPreset != "baseline" {
			params, ok := ReliefPreset(o.reliefPreset)
			if !ok {
				return "", fmt.Errorf("unknown relief preset %q (have %v)", o.reliefPreset, ReliefPresetNames())
			}
			s.ReliefParams = params
			suffix.WriteString("-" + o.reliefPreset)
		}
	}
	if o.noStripes {
		s.DisableStripes = true
		suffix.WriteString("-nostripes")
	}
	if o.grain || o.crackle || o.grainSeed != 0 {
		s.GrainSeed = o.grainSeed
		if o.crackle {
			s.TerraceCrackle = true
			suffix.WriteString("-crackle")
		}
		if o.grain || (o.grainSeed != 0 && !o.crackle) {
			s.TerraceGrain = true
			suffix.WriteString("-grain")
		}
		if o.grainSeed != 0 {
			fmt.Fprintf(&suffix, "-g%d", o.grainSeed)
		}
	}
	if o.terraceSeed != 0 {
		s.TerraceSeed = o.terraceSeed
		fmt.Fprintf(&suffix, "-t%d", o.terraceSeed)
	}
	if o.smooth != 0 {
		s.Persistence = o.smooth
		fmt.Fprintf(&suffix, "-smooth%v", o.smooth)
	}
	return suffix.String(), nil
}
