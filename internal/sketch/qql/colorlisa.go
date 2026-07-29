package qql

import (
	"fmt"
	"math"
	"sort"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// How far a ColorLisa colour is allowed to drift once it becomes a swatch.
// QQL's own boxes are tighter still (±2°, ±2%); a little more room here
// makes up for a five-colour palette having less material to work with.
const (
	lisaHueSpan    = 4
	lisaSatSpan    = 6
	lisaBrightSpan = 6

	// Steps interpolated between each adjacent pair of palette colours. Four
	// source colours plus three in-between steps each gives a sequence long
	// enough for the high colour-variety traits to have something to pick.
	lisaBlendSteps = 3

	// Sequence colours closer than this in luminance to the background are
	// dropped: they would read as holes rather than marks.
	lisaMinContrast = 0.1
)

// colorLisaSet adapts one of the project's flat five-colour palettes into
// QQL's colour model. The sequence is the palette walked cyclically with
// HSL blends between neighbours, so stepping along it moves in small
// related hops the way a native QQL sequence does; every entry gains a
// clamp box to drift inside; and each colour can serve as the ground, with
// the substitution rules dropping whatever would vanish against it.
func colorLisaSet(p palette.Palette) (colorSet, error) {
	if len(p.Colors) < 3 {
		return colorSet{}, fmt.Errorf("palette %q has only %d colors; qql needs at least 3", p.Slug, len(p.Colors))
	}

	var swatches []palette.Swatch
	var seq []swatchKey
	for i, c := range p.Colors {
		next := p.Colors[(i+1)%len(p.Colors)]
		for step := 0; step <= lisaBlendSteps; step++ {
			t := float64(step) / float64(lisaBlendSteps+1)
			blend := palette.LerpHSL(c, next, t)
			name := fmt.Sprintf("%s-%d", p.Slug, len(swatches))
			seq = append(seq, swatchKey(len(swatches)))
			swatches = append(swatches, palette.SwatchAround(name, blend, lisaHueSpan, lisaSatSpan, lisaBrightSpan))
		}
	}

	// Every palette colour can be the ground. Weight by how far its
	// luminance sits from the palette's middle: paintings are grounded on
	// their extremes, not their mid-tones.
	var mean float64
	for _, c := range p.Colors {
		mean += c.Luminance()
	}
	mean /= float64(len(p.Colors))

	backgrounds := make([]bgChoice, 0, len(p.Colors))
	for i, c := range p.Colors {
		key := swatchKey(len(swatches))
		swatches = append(swatches, palette.SwatchAround(
			fmt.Sprintf("%s-bg-%d", p.Slug, i), c, lisaHueSpan, lisaSatSpan, lisaBrightSpan))
		backgrounds = append(backgrounds, bgChoice{
			Color:  key,
			Weight: 0.25 + math.Abs(c.Luminance()-mean),
			Subs:   lisaSubstitutions(c, seq, swatches),
		})
	}

	return colorSet{
		Name:        p.Slug,
		Seq:         seq,
		Backgrounds: backgrounds,
		Splatter:    lisaSplatter(seq, swatches),
		Swatches:    swatches,
	}, nil
}

// lisaSubstitutions drops the sequence colours that would disappear against
// a given ground — but never so many that the sequence stops being usable.
func lisaSubstitutions(bg palette.Color, seq []swatchKey, swatches []palette.Swatch) map[swatchKey]swatchKey {
	type candidate struct {
		key      swatchKey
		contrast float64
	}
	var low []candidate
	bgLum := bg.Luminance()
	for _, k := range seq {
		c := math.Abs(swatches[k].Color().Luminance() - bgLum)
		if c < lisaMinContrast {
			low = append(low, candidate{k, c})
		}
	}
	if len(low) == 0 {
		return nil
	}
	// Drop the least visible first, and keep at least half the sequence.
	sort.Slice(low, func(i, j int) bool { return low[i].contrast < low[j].contrast })
	if maxDrop := len(seq) / 2; len(low) > maxDrop {
		low = low[:maxDrop]
	}
	subs := make(map[swatchKey]swatchKey, len(low))
	for _, c := range low {
		subs[c.key] = dropSwatch
	}
	return subs
}

// lisaSplatter picks the most vivid colours to fleck the piece with, which
// is the role splatter plays in the native palettes.
func lisaSplatter(seq []swatchKey, swatches []palette.Swatch) []weightedSwatch {
	keys := make([]swatchKey, len(seq))
	copy(keys, seq)
	sort.Slice(keys, func(i, j int) bool {
		return swatches[keys[i]].Base.S > swatches[keys[j]].Base.S
	})
	n := min(len(keys), 4)
	out := make([]weightedSwatch, n)
	for i := range n {
		out[i] = weightedSwatch{Color: keys[i], Weight: 1}
	}
	return out
}
