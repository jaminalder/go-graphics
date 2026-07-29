package shoal

import (
	"flag"
	"fmt"
	"strings"

	"github.com/jaminalder/go-graphics/internal/sketch"
)

type cliOptions struct {
	field    string
	grade    string
	detail   float64
	open     float64
	confetti float64
	maxR     float64
}

var _ sketch.Configurable = (*Sketch)(nil)

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	fs.StringVar(&s.opts.field, "field", "", "flow field: curl (default), flow, ridge")
	fs.StringVar(&s.opts.grade, "grade", "", "dot size grading: vortex (default), patches")
	fs.Float64Var(&s.opts.detail, "detail", -1, "share of dots with concentric ring detail; default 0.05")
	fs.Float64Var(&s.opts.open, "open", -1, "share of dots painted as rings not discs; default 0.05")
	fs.Float64Var(&s.opts.confetti, "confetti", -1, "share of dots whose colour ignores the field; default 0.35")
	fs.Float64Var(&s.opts.maxR, "maxr", -1, "largest dot radius in canvas units; default 0.016")
}

var (
	fields = map[string]Field{"curl": FieldCurl, "flow": FieldFlow, "ridge": FieldRidge}
	grades = map[string]Grade{"vortex": GradeVortex, "patches": GradePatches}
)

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) {
	var tag []string

	if v := s.opts.field; v != "" {
		f, ok := fields[v]
		if !ok {
			return "", fmt.Errorf("--field must be curl, flow or ridge")
		}
		s.Field, tag = f, append(tag, v)
	}
	if v := s.opts.grade; v != "" {
		g, ok := grades[v]
		if !ok {
			return "", fmt.Errorf("--grade must be vortex or patches")
		}
		s.Grade, tag = g, append(tag, v)
	}
	for _, o := range []struct {
		name string
		val  float64
		dst  *float64
	}{
		{"detail", s.opts.detail, &s.Detail},
		{"open", s.opts.open, &s.Open},
		{"confetti", s.opts.confetti, &s.Confetti},
	} {
		if o.val < 0 {
			continue
		}
		if o.val > 1 {
			return "", fmt.Errorf("--%s must be within [0, 1]", o.name)
		}
		*o.dst = o.val
		tag = append(tag, fmt.Sprintf("%s%g", o.name[:1], o.val))
	}
	if v := s.opts.maxR; v >= 0 {
		if v < s.MinR || v > 0.2 {
			return "", fmt.Errorf("--maxr must be within [%g, 0.2]", s.MinR)
		}
		s.MaxR = v
		tag = append(tag, fmt.Sprintf("r%g", v))
	}

	if len(tag) == 0 {
		return "", nil
	}
	return "-" + strings.Join(tag, "-"), nil
}
