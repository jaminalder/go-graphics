package glaze

import (
	"flag"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

var (
	_ sketch.Configurable = (*Sketch)(nil)
	_ sketch.Traited      = (*Sketch)(nil)
)

// declare names the water's knobs. --veil comes first because every knob
// below it is an override on what that one choice already decided.
func (s *Sketch) declare() {
	o := opt.New()

	o.Choice("veil", "the material the water is made of", "v",
		[]string{"glaze", "marble", "terrace", "filament", "silk"}, &s.veilName,
		func(value int) { s.manner = manner(value) })
	o.Choice("cast", "which pigment the palette supplies", "cs",
		[]string{"cool", "deep", "pale", "smoke"}, &s.castName,
		func(value int) { s.cast = cast(value) })

	o.Uint64("water-seed", "seed of the veil's fields", "ws", &s.WaterSeed)
	o.Float("water-scale", "veil cycles per canvas unit", "vs", 0.2, 10, &s.WaterScale)
	o.Float("veil-warp", "first domain displacement of the veil", "v1", 0, 8, &s.VeilWarp)
	o.Float("veil-nest", "second domain displacement of the veil", "v2", 0, 8, &s.VeilNest)
	o.Float("stretch", "anisotropy of the veil's forms; 1 is isotropic", "st", 0.25, 6, &s.Stretch)
	o.Float("opacity", "the densest the water gets", "op", 0, 1.2, &s.Opacity)
	o.Float("body", "pigment opacity; 0 is a pure glaze", "bd", 0, 1, &s.Body)
	o.Float("drift", "bed displacement by the veil, canvas units", "dr", 0, 0.25, &s.Drift)
	o.Float("drift-scale", "how fine that displacement varies, x the veil scale", "dz", 0.5, 24, &s.DriftScale)
	o.Float("glint", "strength of the filament highlight", "gt", 0, 1.2, &s.Glint)
	o.Float("grain-water", "paper tooth in the veil", "gw", 0, 0.5, &s.GrainWater)
	o.Int("terraces", "plateaus when the veil terraces", "tr", 2, 12, &s.Terraces)

	s.knobs = o
}

// Flags implements sketch.Configurable: the bed's flat namespace plus the
// water's.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.bed.Flags(fs)
	s.knobs.Flags(fs)
}

// Configure implements sketch.Configurable.
func (s *Sketch) Configure() (string, error) {
	bedSuffix, err := s.bed.Configure()
	if err != nil {
		return "", err
	}
	waterSuffix, err := s.knobs.Configure()
	if err != nil {
		return "", err
	}
	return bedSuffix + waterSuffix, nil
}
