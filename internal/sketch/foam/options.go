package foam

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var (
	_ sketch.Configurable = (*Sketch)(nil)
	_ sketch.Traited      = (*Sketch)(nil)
)

// declare names every knob once. The composition knobs come first: each of
// them is an override on what a trait drew, and a knob left alone is the
// seed's, not this list's.
func (s *Sketch) declare() {
	o := opt.New()

	o.Int("count", "sites the pack aims for", "co", 2, 600, &s.pin.count)
	o.Int("rungs", "steps on the site size ladder", "ru", 1, 9, &s.pin.rungs)
	o.Float("base", "smallest site radius, canvas units", "ba", 0.005, 0.3, &s.pin.base)
	o.Float("ratio", "size ladder step ratio", "ra", 1.05, 3, &s.pin.ratio)
	o.Float("gap", "clearance between sites, x radius", "ga", -0.5, 2, &s.pin.gap)
	o.Float("over", "how far the pack reaches past the frame, canvas units", "ov", 0, 0.5, &s.pin.over)
	o.Float("merge", "share of sites merged into a neighbouring lobe", "mg", 0, 1, &s.pin.merge)
	o.Int("max-lobe", "most sites one lobe may absorb", "ml", 1, 6, &s.pin.maxLobe)
	o.Float("warp", "how far the whole partition is bent, x smallest cell", "wp", 0, 8, &s.pin.warp)
	o.Float("swirl", "wavelength of that bending, x smallest cell", "sl", 2, 60, &s.pin.swirl)

	o.Float("ink", "wall thickness, canvas units", "ik", 0.0005, 0.05, &s.pin.ink)
	o.Float("swell", "extra wall thickness at a junction, x ink", "sw", 0, 8, &s.pin.swell)
	o.Float("node", "distance over which a third cell counts as near", "nd", 0.002, 0.3, &s.pin.node)
	o.Float("round", "radius a cell corner is rounded over, canvas units", "rd", 0, 0.3, &s.pin.round)

	o.Float("wash", "weight of the wash fill", "fw", 0, 20, &s.pin.styles[styleWash])
	o.Float("pencil", "weight of the pencil fill", "fp", 0, 20, &s.pin.styles[stylePencil])
	o.Float("bands", "weight of the concentric band fill", "fb", 0, 20, &s.pin.styles[styleBands])
	o.Float("hatch", "weight of the hatched fill", "fh", 0, 20, &s.pin.styles[styleHatch])
	o.Float("empty", "weight of leaving a cell as bare paper", "fe", 0, 20, &s.pin.styles[styleEmpty])

	// The paint. These are overrides on what the `water` trait drew — the
	// manner weights themselves are the trait and have no flags, because
	// they only mean anything set together.
	o.Float("load", "pigment laid in a typical cell", "ld", 0.05, 8, &s.pin.water.load)
	o.Float("tonal", "how far the load swings with a cell's tone", "tn", 0, 2, &s.pin.water.spread)
	o.Float("overshoot", "how badly the paint registers with the line, x ink", "os", 0, 12, &s.pin.water.over)
	o.Float("granulate", "heavy pigment settling into the paper's tooth", "gr", 0, 2.5, &s.pin.water.grain)
	o.Float("bleed", "share of walls whose two cells were painted wet", "bd", 0, 1, &s.pin.water.bleed)
	o.Float("seep", "how far a bleed carries, x the cell's own size", "sp", 0, 2, &s.pin.water.seep)
	o.Float("scatter", "light scattered back off the pigment", "sc", 0, 1, &s.Scatter)
	o.Float("tooth", "paper tooth the pigment granulates into, canvas units", "th", 0.0004, 0.05, &s.Tooth)

	o.Float("weight", "how strongly a site radius bends the walls; 0 is straight", "wt", 0, 2, &s.Weight)
	o.Float("wobble", "hand wander of the line and the strokes, x width", "wb", 0, 1, &s.Wobble)
	o.Float("rim", "strength of the wash's dried rim", "rm", 0, 1, &s.Rim)
	o.Float("rim-width", "how far in from the wall the rim reaches", "rw", 0.001, 0.2, &s.RimWide)
	o.Float("mottle", "wavelength of the wash's unevenness, canvas units", "mo", 0.02, 2, &s.Mottle)
	o.Float("blotch", "strength of that unevenness", "bl", 0, 1.5, &s.Blotch)
	o.Float("grain", "paper tooth", "gn", 0, 0.4, &s.Grain)
	o.Float("accent", "share of cells taking a colour from outside their passage", "ac", 0, 1, &s.Accent)
	o.Float("passage", "wavelength of the colour field, canvas units", "pa", 0.1, 4, &s.Passage)
	o.Float("shades", "how far a cell wanders from its palette swatch; 0 is the bare palette", "sh", 0, 2, &s.Shades)
	o.Float("flat", "how far a flat fill deepens at full tone; 1 is no deepening", "fl", 0.2, 1, &s.Flat)
	o.Float("stroke", "pencil stroke pitch, canvas units", "st", 0.001, 0.1, &s.Stroke)

	s.declareLook(o)

	s.knobs = o
}

// Flags implements sketch.Configurable: the trait dimensions and the
// per-knob overrides share one flat namespace.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.traits.Flags(fs)
	s.knobs.Flags(fs)
}

// Configure implements sketch.Configurable. The trait part of the filename
// needs the resolved set, which only exists once the seed is known, so it
// is added by TraitSuffix at render time rather than here.
func (s *Sketch) Configure() (string, error) {
	if err := s.traits.Configure(); err != nil {
		return "", err
	}
	return s.knobs.Configure()
}
