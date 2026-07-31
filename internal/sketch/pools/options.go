package pools

import (
	"flag"
	"fmt"
	"strings"

	"github.com/jaminalder/go-graphics/internal/sketch"
)

type cliOptions struct {
	count       int
	pigments    int
	rungs       int
	base        float64
	ratio       float64
	satellites  float64
	ragged      float64
	rings       float64
	open        float64
	glaze       float64
	banded      float64
	bandWidth   float64
	bandOverlap float64
	alpha       float64
	margin      float64
}

var _ sketch.Configurable = (*Sketch)(nil)

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	fs.IntVar(&s.opts.count, "count", -1, "anchor circles before satellites; default 26")
	fs.IntVar(&s.opts.pigments, "pigments", -1, "palette colours in play; default 4")
	fs.IntVar(&s.opts.rungs, "rungs", -1, "steps on the size ladder; default 5")
	fs.Float64Var(&s.opts.base, "base", -1, "smallest circle radius in canvas units; default 0.03")
	fs.Float64Var(&s.opts.ratio, "ratio", -1, "size ladder step ratio; default 1.55")
	fs.Float64Var(&s.opts.satellites, "satellites", -1, "share of circles given an overlapping companion; default 0.45")
	fs.Float64Var(&s.opts.ragged, "ragged", -1, "wash edge deviation; 0 is a true circle, 0.22 a blob; default 0.055")
	fs.Float64Var(&s.opts.rings, "rings", -1, "share of circles carrying inner rings; default 0.34")
	fs.Float64Var(&s.opts.open, "open", -1, "share painted as annuli rather than discs; default 0.28")
	fs.Float64Var(&s.opts.glaze, "glaze", -1, "share carrying a second pigment on top; default 0.16")
	fs.Float64Var(&s.opts.banded, "banded", -1, "share of circles filled with fine concentric rings; default 0.3")
	fs.Float64Var(&s.opts.bandWidth, "band-width", -1, "ring pitch of a banded circle, canvas units; default 0.0065")
	fs.Float64Var(&s.opts.bandOverlap, "band-overlap", -1, "how far neighbouring rings cross, x pitch; default 0.55")
	fs.Float64Var(&s.opts.alpha, "alpha", -1, "pool strength; below 1 keeps crossings readable; default 0.62")
	fs.Float64Var(&s.opts.margin, "margin", -1, "clear paper at the edge; default 0.06")
}

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) {
	var tag []string

	for _, o := range []struct {
		name     string
		val      float64
		lo, hi   float64
		dst      *float64
		abbrevAt int
	}{
		{"satellites", s.opts.satellites, 0, 1, &s.Satellites, 3},
		{"ragged", s.opts.ragged, 0, 0.6, &s.Ragged, 2},
		{"rings", s.opts.rings, 0, 1, &s.Rings, 2},
		{"open", s.opts.open, 0, 1, &s.Open, 2},
		{"glaze", s.opts.glaze, 0, 1, &s.Glaze, 2},
		{"banded", s.opts.banded, 0, 1, &s.Banded, 2},
		{"band-width", s.opts.bandWidth, 0.0008, 0.1, &s.BandWidth, 4},
		{"band-overlap", s.opts.bandOverlap, 0, 2, &s.BandOverlap, 4},
		{"alpha", s.opts.alpha, 0.05, 1, &s.Alpha, 2},
		{"margin", s.opts.margin, 0, 0.4, &s.Margin, 2},
		{"base", s.opts.base, 0.004, 0.2, &s.Base, 2},
		{"ratio", s.opts.ratio, 1.05, 3, &s.Ratio, 2},
	} {
		if o.val < 0 {
			continue
		}
		if o.val < o.lo || o.val > o.hi {
			return "", fmt.Errorf("--%s must be within [%g, %g]", o.name, o.lo, o.hi)
		}
		*o.dst = o.val
		tag = append(tag, fmt.Sprintf("%s%g", o.name[:o.abbrevAt], o.val))
	}

	for _, o := range []struct {
		name   string
		val    int
		lo, hi int
		dst    *int
	}{
		{"count", s.opts.count, 1, 400, &s.Count},
		{"pigments", s.opts.pigments, 1, 12, &s.Pigments},
		{"rungs", s.opts.rungs, 1, 9, &s.Rungs},
	} {
		if o.val < 0 {
			continue
		}
		if o.val < o.lo || o.val > o.hi {
			return "", fmt.Errorf("--%s must be within [%d, %d]", o.name, o.lo, o.hi)
		}
		*o.dst = o.val
		tag = append(tag, fmt.Sprintf("%s%d", o.name[:2], o.val))
	}

	if len(tag) == 0 {
		return "", nil
	}
	return "-" + strings.Join(tag, "-"), nil
}
