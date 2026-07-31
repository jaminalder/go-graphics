package shoal

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var _ sketch.Configurable = (*Sketch)(nil)

// declare names every knob once; opt turns that into flag registration,
// range checking and the filename suffix.
func (s *Sketch) declare() {
	o := opt.New()

	o.Choice("field", "flow field", "", []string{"flow", "curl", "ridge"}, &s.field,
		func(i int) { s.Field = []Field{FieldFlow, FieldCurl, FieldRidge}[i] })
	o.Choice("grade", "size grading", "", []string{"vortex", "patches"}, &s.grade,
		func(i int) { s.Grade = []Grade{GradeVortex, GradePatches}[i] })
	o.Choice("mark", "what a chain paints as", "", []string{"disc", "ribbon", "mixed", "wash"}, &s.mark,
		func(i int) { s.Mark = []Mark{MarkDisc, MarkRibbon, MarkMixed, MarkWash}[i] })
	o.Choice("ground", "canvas ground", "", []string{"light", "dark"}, &s.ground,
		func(i int) { s.Ground = []Ground{GroundLight, GroundDark}[i] })
	o.Bool("mono", "build the ink set from one hue plus a rare accent", "mono", &s.Mono)

	o.Float("detail", "share of dots with concentric ring detail", "d", 0, 1, &s.Detail)
	o.Float("open", "share of dots painted as rings not discs", "o", 0, 1, &s.Open)
	o.Float("confetti", "share of chains whose colour ignores the field", "c", 0, 1, &s.Confetti)
	o.Float("overlap", "how far marks may crowd into each other, x radius", "v", 0, 1, &s.Overlap)
	o.Float("margin", "clear ground around the field; 0 bleeds off the edge", "m", 0, 1, &s.Margin)
	o.Float("minr", "smallest dot radius in canvas units", "n", 0.0005, 0.1, &s.MinR)
	o.Float("maxr", "largest dot radius in canvas units", "r", 0.0005, 0.2, &s.MaxR)

	s.knobs = o
}

// Flags implements sketch.Configurable.
func (s *Sketch) Flags(fs *flag.FlagSet) { s.knobs.Flags(fs) }

// Configure implements sketch.Configurable. The choice knobs carry no tag
// of their own: their value is already a word, so it goes into the name
// unprefixed the way it always has.
func (s *Sketch) Configure() (string, error) {
	suffix, err := s.knobs.Configure()
	if err != nil {
		return "", err
	}
	if s.MaxR < s.MinR {
		return "", errSmallMax
	}
	return suffix, nil
}
