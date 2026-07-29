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
	mark     string
	ground   string
	detail   float64
	open     float64
	confetti float64
	overlap  float64
	minR     float64
	maxR     float64
	margin   float64
	mono     bool
}

var _ sketch.Configurable = (*Sketch)(nil)

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	fs.StringVar(&s.opts.field, "field", "", "flow field: flow (default), curl, ridge")
	fs.StringVar(&s.opts.grade, "grade", "", "size grading: vortex (default), patches")
	fs.StringVar(&s.opts.mark, "mark", "", "what a chain paints as: disc (default), ribbon, mixed, wash")
	fs.StringVar(&s.opts.ground, "ground", "", "canvas ground: light (default), dark")
	fs.Float64Var(&s.opts.detail, "detail", -1, "share of dots with concentric ring detail; default 0.09")
	fs.Float64Var(&s.opts.open, "open", -1, "share of dots painted as rings not discs; default 0.05")
	fs.Float64Var(&s.opts.confetti, "confetti", -1, "share of chains whose colour ignores the field; default 0.4")
	fs.Float64Var(&s.opts.overlap, "overlap", -1, "how far marks may crowd into each other, ×radius; default 0")
	fs.Float64Var(&s.opts.minR, "minr", -1, "smallest dot radius in canvas units; default 0.0035")
	fs.Float64Var(&s.opts.maxR, "maxr", -1, "largest dot radius in canvas units; default 0.0135")
	fs.Float64Var(&s.opts.margin, "margin", -1, "clear ground around the field; 0 bleeds off the edge; default 0.045")
	fs.BoolVar(&s.opts.mono, "mono", false, "build the ink set from one hue plus a rare accent")
}

var (
	fields  = map[string]Field{"curl": FieldCurl, "flow": FieldFlow, "ridge": FieldRidge}
	grades  = map[string]Grade{"vortex": GradeVortex, "patches": GradePatches}
	marks   = map[string]Mark{"disc": MarkDisc, "ribbon": MarkRibbon, "mixed": MarkMixed, "wash": MarkWash}
	grounds = map[string]Ground{"light": GroundLight, "dark": GroundDark}
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
	if v := s.opts.mark; v != "" {
		m, ok := marks[v]
		if !ok {
			return "", fmt.Errorf("--mark must be disc, ribbon, mixed or wash")
		}
		s.Mark, tag = m, append(tag, v)
	}
	if v := s.opts.ground; v != "" {
		g, ok := grounds[v]
		if !ok {
			return "", fmt.Errorf("--ground must be light or dark")
		}
		s.Ground, tag = g, append(tag, v)
	}
	if s.opts.mono {
		s.Mono, tag = true, append(tag, "mono")
	}
	for _, o := range []struct {
		name string
		val  float64
		dst  *float64
	}{
		{"detail", s.opts.detail, &s.Detail},
		{"open", s.opts.open, &s.Open},
		{"confetti", s.opts.confetti, &s.Confetti},
		{"overlap", s.opts.overlap, &s.Overlap},
		{"margin", s.opts.margin, &s.Margin},
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
	if v := s.opts.minR; v >= 0 {
		if v < 0.0005 || v > 0.1 {
			return "", fmt.Errorf("--minr must be within [0.0005, 0.1]")
		}
		s.MinR = v
		tag = append(tag, fmt.Sprintf("m%g", v))
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
