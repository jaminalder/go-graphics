package riffle

import (
	"flag"
	"fmt"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/sketch"
	"github.com/jaminalder/go-graphics/internal/trait"
)

var (
	_ sketch.Configurable = (*Sketch)(nil)
	_ sketch.Traited      = (*Sketch)(nil)
)

// declare names every knob once. All of them are overrides on what a trait
// drew: a knob left alone is the seed's, not this list's, which is why the
// defaults registered here are only ever seen in --help.
//
// None of these names may collide with the render command's own flags —
// --profile, --width, --height, --seed, --aa, --deep, --palette, --format,
// --out — because a sketch's options share that FlagSet.
func (s *Sketch) declare() {
	o := opt.New()

	o.Choice("medium", "rendering medium", "md", []string{"river", "overlay"}, &s.medium, nil)
	o.Choice("ground", "overlay preview ground", "g",
		[]string{"transparent", "gray-light", "gray-mid", "gray-dark"}, &s.ground, nil)
	o.Float("overlay-alpha", "overall opacity of the water layer", "oa", 0.02, 1, &s.overlayAlpha)
	o.Float("overlay-ripples", "strength of current streaks and wave facets", "or", 0, 2, &s.overlayRipples)
	o.Float("overlay-shadows", "strength of broad shadow patches", "os", 0, 2, &s.overlayShadows)
	o.Float("overlay-dots", "strength of washed-out boulder dots", "od", 0, 1, &s.overlayDots)

	o.Float("depth", "water on the thalweg in a pool, in extinction units", "dp", 0.1, 4, &s.pin.depth)
	o.Float("riffle", "amplitude of the pool-riffle sequence", "rf", 0, 0.9, &s.pin.riffle)
	o.Float("riffle-wave", "pool-riffle sequences down the frame", "rw", 0.2, 6, &s.pin.riffleWave)
	o.Float("dune", "irregularity of the bed", "du", 0, 1, &s.pin.dune)

	o.Float("channel-width", "half width of the channel, canvas units", "cw", 0.15, 2, &s.pin.channelWidth)
	o.Float("bend", "lateral swing of the centreline, canvas units", "bn", 0, 0.6, &s.pin.bend)
	o.Float("meander", "swings down the height of the frame", "mn", 0.1, 4, &s.pin.meander)
	o.Float("taper", "narrowing (negative) or opening (positive) downstream", "tp", -0.9, 0.9, &s.pin.taper)

	o.Float("speed", "mid-channel current", "sp", 0.05, 3, &s.pin.speed)
	o.Float("turbulence", "curl noise on the current", "tu", 0, 1.5, &s.pin.turbulence)
	o.Float("chop", "surface wave height, canvas units", "ch", 0, 0.03, &s.pin.chop)

	o.Int("rocks", "boulders in the channel", "ro", 0, 60, &s.pin.rocks)
	o.Float("rock-size", "typical boulder radius, canvas units", "rs", 0.005, 0.25, &s.pin.rockSize)
	o.Float("wake", "white water a boulder sheds", "wk", 0, 2, &s.pin.wake)
	o.Float("eddy", "circulation of the vortex pair behind a boulder", "ed", 0, 2, &s.pin.eddy)

	o.Int("steps", "steps of the upstream walk", "st", 2, 64, &s.pin.steps)
	o.Float("step", "time per step: a step is speed x this", "sl", 0.001, 0.06, &s.pin.step)

	o.Float("foam", "how easily water goes white", "fo", 0, 1, &s.pin.foam)
	o.Float("foam-life", "steps a bubble survives", "fl", 0.5, 40, &s.pin.foamLife)
	o.Float("bubbles", "bubble lattice scale, canvas units", "bu", 0.001, 0.05, &s.pin.bubbles)

	o.Float("extinction", "how fast light dies with depth", "ex", 0.2, 8, &s.pin.extinction)
	o.Float("milk", "how far the water's own colour is lifted toward light", "mk", 0, 1, &s.pin.milk)

	o.Float("sun", "sun azimuth, degrees", "su", 0, 360, &s.pin.sun)
	o.Float("sun-height", "sun altitude, degrees", "sh", 2, 89, &s.pin.sunHeight)
	o.Float("glint", "angular width of a sun glint, radians", "gl", 0.01, 1.2, &s.pin.glint)
	o.Float("sheen", "broad reflected sky on the surface", "sn", 0, 0.8, &s.pin.sheen)
	o.Float("caustic", "strength of the net of light on the bed", "ca", 0, 3, &s.pin.caustic)
	o.Float("caustic-scale", "caustic cell size in the shallows, canvas units", "cs", 0.004, 0.2, &s.pin.causticScale)
	o.Float("caustic-warp", "how hard the net is folded", "cp", 0, 2, &s.pin.causticWarp)
	o.Float("dapple", "patchy shade over the sun", "da", 0, 1, &s.pin.dapple)

	o.Float("pebble", "gravel scale, canvas units", "pb", 0.001, 0.06, &s.pin.pebble)

	s.knobs = o
}

// override lays the flags the caller actually gave over what the traits
// drew. Only the ones visited on the command line are read — which is what
// opt.Set.WasSet is for, and the only way to tell a seed's value from a
// default.
func (s *Sketch) override(set *settings) {
	for _, o := range []struct {
		name string
		set  func()
	}{
		{"depth", func() { set.depth = s.pin.depth }},
		{"riffle", func() { set.riffle = s.pin.riffle }},
		{"riffle-wave", func() { set.riffleWave = s.pin.riffleWave }},
		{"dune", func() { set.dune = s.pin.dune }},
		{"channel-width", func() { set.channelWidth = s.pin.channelWidth }},
		{"bend", func() { set.bend = s.pin.bend }},
		{"meander", func() { set.meander = s.pin.meander }},
		{"taper", func() { set.taper = s.pin.taper }},
		{"speed", func() { set.speed = s.pin.speed }},
		{"turbulence", func() { set.turbulence = s.pin.turbulence }},
		{"chop", func() { set.chop = s.pin.chop }},
		{"rocks", func() { set.rocks = s.pin.rocks }},
		{"rock-size", func() { set.rockSize = s.pin.rockSize }},
		{"wake", func() { set.wake = s.pin.wake }},
		{"eddy", func() { set.eddy = s.pin.eddy }},
		{"steps", func() { set.steps = s.pin.steps }},
		{"step", func() { set.step = s.pin.step }},
		{"foam", func() { set.foam = s.pin.foam }},
		{"foam-life", func() { set.foamLife = s.pin.foamLife }},
		{"bubbles", func() { set.bubbles = s.pin.bubbles }},
		{"extinction", func() { set.extinction = s.pin.extinction }},
		{"milk", func() { set.milk = s.pin.milk }},
		{"sun", func() { set.sun = s.pin.sun }},
		{"sun-height", func() { set.sunHeight = s.pin.sunHeight }},
		{"glint", func() { set.glint = s.pin.glint }},
		{"sheen", func() { set.sheen = s.pin.sheen }},
		{"caustic", func() { set.caustic = s.pin.caustic }},
		{"caustic-scale", func() { set.causticScale = s.pin.causticScale }},
		{"caustic-warp", func() { set.causticWarp = s.pin.causticWarp }},
		{"dapple", func() { set.dapple = s.pin.dapple }},
		{"pebble", func() { set.pebble = s.pin.pebble }},
	} {
		if s.knobs.WasSet(o.name) {
			o.set()
		}
	}
}

// Flags implements sketch.Configurable: the trait dimensions and the
// per-knob overrides share one flat namespace.
func (s *Sketch) Flags(fs *flag.FlagSet) {
	s.traits.Flags(fs)
	s.knobs.Flags(fs)
}

// Configure implements sketch.Configurable. The trait part of the filename
// needs the resolved set, which only exists once the seed is known, so it is
// added by TraitSuffix at render time rather than here.
func (s *Sketch) Configure() (string, error) {
	if err := s.traits.Configure(); err != nil {
		return "", err
	}
	suffix, err := s.knobs.Configure()
	if err != nil {
		return "", err
	}
	if s.medium != "overlay" {
		for _, name := range []string{"ground", "overlay-alpha", "overlay-ripples", "overlay-shadows", "overlay-dots"} {
			if s.knobs.WasSet(name) {
				return "", fmt.Errorf("--%s requires --medium overlay", name)
			}
		}
	}
	return suffix, nil
}

// TraitSuffix implements sketch.Traited.
func (s *Sketch) TraitSuffix(set trait.Set) string { return s.traits.NameSuffix(set) }
