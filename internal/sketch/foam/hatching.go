package foam

// Hatching laid over the paint, one family per cell.
//
// The marks come from internal/hatch, which arranges them; what this file
// decides is the two things a hatch cannot know on its own — which family
// each cell gets, and what *tone* it should be encoding at each point.
//
// The tone is where the third dimension comes from. Hatching has been a
// shading technique for five hundred years for one reason: an engraver
// cannot make ink darker, so they make it *denser*, and density reads as
// light. Give every cell on a sheet one consistent light direction, let the
// hatch thin toward it and crowd away from it, and flat tiles come up off
// the page as rounded stones. Nothing here computes a normal or traces a
// ray; the shading is entirely in how close together the lines are.

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/hatch"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// The looks. Each is a structure plus a rule for what the marks encode.
const (
	hatchNone     = "none"
	hatchParallel = "parallel" // one straight family, flat
	hatchCross    = "cross"    // two families, flat
	hatchContour  = "contour"  // marks follow the cell's own outline
	hatchShade    = "shade"    // straight, thinning toward the light
	hatchDome     = "dome"     // contour, thinning toward the light
	hatchSphere   = "sphere"   // rings about the centre, thinning toward the light
	hatchEngrave  = "engrave"  // cross-hatch that only crosses in the shadow
	hatchStipple  = "stipple"  // dots, denser in the shadow
	hatchHollow   = "hollow"   // contour, densest at the rim — a cup, not a dome
	hatchFlow     = "flow"     // one wandering field over the whole sheet
	hatchWeave    = "weave"    // over-under, flat
	hatchFan      = "fan"      // arcs between two poles
	hatchWave     = "wave"     // wavy parallels
	hatchSpike    = "spike"    // tapered marks pointing into the light
)

// hatchNames are the looks, in the order they read best on a contact sheet.
var hatchNames = []string{
	hatchNone, hatchParallel, hatchCross, hatchContour, hatchWave,
	hatchShade, hatchDome, hatchSphere, hatchEngrave, hatchStipple,
	hatchHollow, hatchFlow, hatchWeave, hatchFan, hatchSpike,
}

// How a cell's hatching decides what it is encoding at a point.
const (
	toneFlat   = iota // nothing; the marks are even
	toneLit           // a light across the cell: thin toward it, dense away
	toneHollow        // dense at the rim, open in the middle
)

// hatching is one cell's family of marks.
type hatching struct {
	f    hatch.Func
	tone int
}

// hatchFor builds the family for one cell.
//
// Spacing is set with Fit — so many marks across the region, whatever its
// size — rather than as a distance. A packed sheet runs from a quarter-canvas
// lobe to a chip, and one spacing in canvas units gives the lobe forty lines
// and the chip none.
func (s *Sketch) hatchFor(look string, rng *rand.Rand) hatching {
	if look == hatchNone {
		return hatching{}
	}
	base := hatch.Defaults()
	base.Seed = rng.Uint64()
	base.Align = hatch.AlignRegion
	base.Thickness = s.HatchWeight
	base.Softness = 0.5
	base.Jitter = 0.12
	// One angle per cell, quantised: free angles read as noise, and a hand
	// keeps to a handful of directions per sheet.
	base.Angle = float64(rng.IntN(8)) * math.Pi / 4
	base.Fit = s.HatchFit + rng.IntN(3)

	switch look {
	case hatchParallel:
		return hatching{f: hatch.Of(base), tone: toneFlat}

	case hatchCross:
		return hatching{f: hatch.Cross(base, 0, math.Pi/2), tone: toneFlat}

	case hatchContour:
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Structure = hatch.Contour
		})), tone: toneFlat}

	case hatchWave:
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Waveform = hatch.Sine
			p.Amplitude = 0.35
			p.Wavelength = 0.06
		})), tone: toneFlat}

	case hatchShade:
		// The plainest shading there is: one straight family whose spacing
		// opens toward the light. It is what a pen does.
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.ToneDensity = 3
			p.ToneWidth = 0.7
		})), tone: toneLit}

	case hatchDome:
		// The same lighting, but the marks follow the cell's own outline, so
		// they wrap the form instead of cutting across it. This is the one
		// that reads most convincingly as a stone: a contour line bending
		// round a lobe is exactly what an engraver draws on a curved surface.
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Structure = hatch.Contour
			p.ToneDensity = 3
			p.ToneWidth = 0.75
		})), tone: toneLit}

	case hatchSphere:
		// Rings about the centre rather than about the boundary: they ignore
		// the cell's shape, which makes them read as drawn *on* the cell.
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Structure = hatch.Concentric
			p.ToneDensity = 2.6
			p.ToneWidth = 0.7
		})), tone: toneLit}

	case hatchEngrave:
		// Two families, but the second only arrives where it is dark. That
		// is the engraver's actual method — one pass everywhere, a second
		// crossing it in the shadows — and it gives a much longer tonal range
		// than either family alone.
		one := base.With(func(p *hatch.Spec) { p.ToneDensity = 1.8; p.ToneWidth = 0.6 })
		two := base.Rotated(math.Pi / 2).With(func(p *hatch.Spec) {
			p.ToneDensity = 4 // thins away much faster: gone by mid-tone
			p.ToneWidth = 0.6
		})
		return hatching{f: hatch.Over(hatch.Of(one), hatch.Of(two)), tone: toneLit}

	case hatchSpike:
		// The marks run *along* the light rather than across it, and that
		// single choice is what makes them spikes: the width follows the
		// tone, the tone changes along the light, so each mark is fat at its
		// shadow end and tapers to nothing at its lit end. Every spike on
		// the sheet therefore points the same way — into the light — and the
		// cells read as lit from that side without anything being shaded.
		//
		// One angle for the whole sheet, not one per cell. A per-cell angle
		// is right for hatching that only has to sit *in* a shape; here the
		// marks are describing where the light comes from, and a light that
		// changes direction from cell to cell is not a light.
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Angle = s.Light * math.Pi / 180
			p.Thickness = s.HatchWeight * 1.5
			// Width alone does the taper. Thinning as well would drop a mark
			// out from under its own point — the density rule works on whole
			// marks, and a mark whose tone varies along its length would
			// vanish in the middle of itself.
			p.ToneWidth = 1.35
			p.ToneDensity = 0
			p.Softness = 0.35

			// One fineness over the whole sheet: an absolute spacing, not a
			// count fitted to each cell. Fit gives every region the same
			// *number* of marks, so a quarter-canvas lobe is combed with
			// half a dozen fat spikes while a chip beside it gets six fine
			// ones — the mark size then reads as a property of the cell
			// rather than of the hand holding the pen.
			p.Fit = 0
			p.Spacing = s.HatchPitch

			// And an irregular hand. Jitter moves each spike off the lattice,
			// and breaking the marks gives them uneven lengths — each one
			// carries its own dash phase, so they do not line up into rows.
			// Without both, a constant spacing reads as a printed screen,
			// which is exactly what the fitted version never did.
			p.Jitter = 0.38 * s.HatchVary
			p.Continuity = 1 - 0.32*s.HatchVary
			// The break is far longer than a cell is wide, on purpose. A
			// dash comparable to the spacing chops every spike into a row of
			// equal blocks and the whole cell reads as dither; a dash of
			// several cell-widths means a given spike is simply present,
			// absent, or cut short — which is what varies the lengths. Each
			// mark carries its own phase, so they never line up.
			p.Dash = s.HatchPitch * 40
		})), tone: toneLit}

	case hatchStipple:
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Structure = hatch.Stipple
			p.Thickness = s.HatchWeight * 2.2
			p.ToneWidth = 1
			p.ToneDensity = 0.4
		})), tone: toneLit}

	case hatchHollow:
		// The light inverted: densest at the rim, open in the middle. The
		// cell reads as a dish rather than as a stone, which is the same
		// trick and the opposite object.
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Structure = hatch.Contour
			p.ToneDensity = 2.8
			p.ToneWidth = 0.75
		})), tone: toneHollow}

	case hatchFlow:
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Structure = hatch.Flow
			p.Amplitude = 0.5
			p.Wavelength = 0.25
		})), tone: toneFlat}

	case hatchWeave:
		a := base
		b := base.Rotated(math.Pi / 2)
		over := hatch.Weave(a, b)
		return hatching{f: func(sm hatch.Sample) float64 {
			ca, cb := over(sm)
			return math.Max(ca, cb)
		}, tone: toneFlat}

	case hatchFan:
		return hatching{f: hatch.Of(base.With(func(p *hatch.Spec) {
			p.Structure = hatch.Fan
		})), tone: toneFlat}

	default:
		return hatching{}
	}
}

// hatchOver lays a cell's marks on top of whatever it was painted with.
//
// The marks are the pigment itself, pressed harder — not a separate ink.
// Hatching in a neutral over a coloured fill reads as a screen laid on the
// picture; hatching in a deeper mix of the cell's own colour reads as the
// same paint gone over twice, which is what it is meant to be.
func (s *Sketch) hatchOver(sh *sheet, h cells.Hit, col palette.Color, u, v float64) palette.Color {
	if h.Cell < 0 || h.Cell >= len(sh.hatches) {
		return col
	}
	hz := sh.hatches[h.Cell]
	if hz.f == nil {
		return col
	}
	c := sh.foam.Cells()[h.Cell]
	reach := c.Inradius
	if reach <= 0 {
		reach = math.Max(sh.level.base, 1e-4)
	}

	sm := hatch.Sample{
		U: u, V: v, CX: c.CX, CY: c.CY,
		Wall: h.Wall, Reach: reach,
		Tone: s.hatchTone(hz.tone, c, h, reach, u, v),
	}
	cover := mathx.Clamp01(hz.f(sm))
	if hz.tone != toneFlat {
		// The light presses the marks as well as spacing them. Thinning
		// alone drops lines by whole octaves, so the lit side of a cell goes
		// from eight lines to four to two — a change in *texture* long
		// before it is a change in tone, and at cell size it reads as a
		// pattern varying rather than as a surface turning. Pressing them
		// lighter as well is what closes that gap.
		cover *= 0.12 + 0.88*sm.Tone
	}
	if cover <= 0 {
		return col
	}
	return palette.Lerp(col, shade(col, 0.52), cover*s.Hatching)
}

// hatchTone is what the marks encode at a point.
func (s *Sketch) hatchTone(mode int, c cells.Cell, h cells.Hit, reach, u, v float64) float64 {
	switch mode {
	case toneLit:
		// A single light bearing for the whole sheet, which is what makes a
		// field of separately shaded cells read as one lit surface rather
		// than as a hundred unrelated ones. Inconsistent lighting is the
		// thing that makes fake three dimensions look fake.
		a := s.Light * math.Pi / 180
		lx, ly := math.Cos(a), math.Sin(a)
		d := ((u-c.CX)*lx + (v-c.CY)*ly) / reach
		return mathx.Clamp01(0.5 + 0.5*d)
	case toneHollow:
		if math.IsInf(h.Wall, 1) {
			return 0
		}
		return mathx.Clamp01(1 - h.Wall/reach)
	default:
		return 0.5
	}
}

// hatchAll builds every cell's family.
func (s *Sketch) hatchAll(f *cells.Foam, look string, rng *rand.Rand) []hatching {
	out := make([]hatching, f.Len())
	if look == hatchNone {
		return out
	}
	for i := range out {
		out[i] = s.hatchFor(look, rng)
	}
	return out
}
