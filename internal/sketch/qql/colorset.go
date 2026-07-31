package qql

import (
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// swatchKey indexes qqlSwatches. Colours are referred to by key rather than
// by value so a background's substitution table can rewrite a whole
// sequence cheaply, the way the source data expresses it.
type swatchKey int

// dropSwatch is the substitution target meaning "remove this colour from the
// sequence entirely" — some backgrounds kill a colour rather than replace it.
const dropSwatch swatchKey = -1

// colorSet is one palette's colour material: an ordered sequence to walk,
// the backgrounds it can sit on, and the colours splatter may use.
type colorSet struct {
	Name        string
	Seq         []swatchKey
	Backgrounds []bgChoice
	Splatter    []weightedSwatch
	// Swatches resolves a key. Native QQL palettes share one table; a
	// palette adapted from ColorLisa brings its own.
	Swatches []palette.Swatch
}

type bgChoice struct {
	Color  swatchKey
	Weight float64
	// Subs rewrites sequence colours that would not read against this
	// background. A value of dropSwatch removes the colour instead.
	Subs map[swatchKey]swatchKey
}

type weightedSwatch struct {
	Color  swatchKey
	Weight float64
}

func (s colorSet) swatch(k swatchKey) palette.Swatch { return s.Swatches[k] }

// substitute applies a background's rewrite rules to one colour, reporting
// false when the colour is dropped.
func (b bgChoice) substitute(k swatchKey) (swatchKey, bool) {
	to, ok := b.Subs[k]
	if !ok {
		return k, true
	}
	if to == dropSwatch {
		return 0, false
	}
	return to, true
}

// nativeColorSet returns one of QQL's seven city palettes.
func nativeColorSet(name string) (colorSet, bool) {
	p, ok := qqlPalettes[name]
	if !ok {
		return colorSet{}, false
	}
	p.Swatches = qqlSwatches
	return p, true
}

// scheme is the resolved colour plan for one piece: which background it sits
// on, the two winnowed sequences the ring dots walk, and how splatter behaves.
type scheme struct {
	set        colorSet
	Background palette.Color
	Primary    []swatchKey
	Secondary  []swatchKey

	SplatterOdds    float64
	SplatterCenterX float64
	SplatterCenterY float64
	SplatterChoices []swatchKey
}

func (s scheme) swatch(k swatchKey) palette.Swatch { return s.set.swatch(k) }

// pickNextColor steps forward or backward along a colour sequence. Walking
// the sequence rather than sampling it freely is what keeps neighbouring
// groups related instead of merely sharing a palette.
func pickNextColor(seqLen, current int, rng *rand.Rand) int {
	step := rnd.Pick(rng, []rnd.Weighted[int]{{V: 1, W: 0.64}, {V: 2, W: 0.24}, {V: 3, W: 0.1}, {V: 4, W: 0.02}})
	if rnd.Odds(rng, 0.5) {
		step = -step
	}
	n := seqLen
	return ((current+step)%n + n) % n
}
