// Package hatch fills a region with repeated marks.
//
// Hatching is not one effect, it is a family: parallel lines, cross-hatching,
// contours that follow a boundary, rays from a point, rings, a flow field,
// a fan, a zigzag, a scribble, stipple. What they share is that the *marks*
// are the same idea every time and only the *rule that arranges them*
// changes. So this package has one primitive — a coverage function — and the
// arrangement is a parameter of it.
//
//	cover := hatch.New(spec)
//	c := cover.Cover(hatch.Sample{U: u, V: v, Wall: h.Wall, Reach: cell.Inradius})
//	col := palette.Lerp(paper, ink, c)
//
// Colour is deliberately not here. The repo renders by evaluating a pure
// function per pixel, so a hatch that answers "how much ink is at this
// point" composes with anything: a caller lerps toward one ink, or two, or
// shifts hue along the marks, or uses the coverage as a mask for a wash.
// A hatch that returned colour would be usable by exactly one sketch.
//
// # What a hatch is allowed to know
//
// Several structures need more than a position. Contour hatching needs the
// distance to the boundary; radial and concentric need a centre;
// boundary-to-boundary needs the region's extent; a density gradient needs
// something to grade against. All of that arrives in Sample, which describes
// the point *and* the region containing it. It is deliberately a small
// bundle of numbers rather than a shape interface: a cell of internal/cells
// can fill it in (Hit.Wall, Cell.CX/CY, Cell.Inradius), and so can a circle,
// a polygon or a whole canvas, and none of them has to be known here.
//
// # Units
//
// Every length is in canvas units (v ∈ [0,1], u ∈ [0, aspect]) — never
// pixels — so a hatch is identical at preview and print size (invariant 2 in
// docs/ARCHITECTURE.md). Thickness is the one exception and is deliberately
// *dimensionless*: it is a fraction of the spacing, which is how an engraver
// thinks (the line-to-gap ratio is the tone) and which means a hatch fitted
// to a small region gets proportionally finer lines for free.
//
// Randomness comes from Spec.Seed through hash and Perlin lookups, never
// from a mutable generator, so a Hatch is pure and safe to call from the
// parallel pixel loop.
//
// Depends on mathx and noise only; otherwise a stdlib leaf.
package hatch

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
)

// Sample is one point together with what the caller knows about the region
// containing it. Only U and V are always required; the rest are what let a
// structure align itself to the shape it is filling, and each structure
// documents which it needs.
type Sample struct {
	// U, V is the point, in canvas units.
	U, V float64
	// CX, CY is the region's centre — its centroid will do. Radial,
	// concentric, fan and chord hatching are built around it, and every
	// structure uses it as the phase origin under AlignRegion.
	CX, CY float64
	// Axis is the region's own direction in radians, if it has one (the
	// principal axis of an elongated cell, say). Under AlignRegion the
	// hatch angle is measured from it. Zero is a fine answer for a region
	// with no preferred direction.
	Axis float64
	// Wall is the distance from the point to the nearest boundary of the
	// region, in canvas units — cells.Hit.Wall exactly. Contour hatching is
	// this number and nothing else. +Inf means "unbounded/unknown", and a
	// structure that needs it then draws nothing.
	Wall float64
	// Reach is the region's half-size in canvas units: its inscribed radius,
	// or half its width. It is the scale everything region-relative is
	// measured against — Fit, the ray count of a radial hatch, the disc a
	// chord hatch spans. Zero means unknown and is taken as 0.5, half the
	// canvas height.
	Reach float64
	// Tone is the value the caller wants the hatching to encode, in [0,1]:
	// 0 is the lightest, 1 the fullest. It does nothing until ToneDensity or
	// ToneWidth turns it on.
	Tone float64
}

// Structure is the rule that arranges the marks.
type Structure int

// The structures. Cross-hatching, weave and nesting are not here: they are
// combinations of these, and live in the composition functions.
const (
	// Parallel lays straight marks at a constant angle and spacing. With
	// Curvature it bends them into arcs of constant radius, which is curved
	// hatching; with Waveform it turns them into waves or zigzags.
	Parallel Structure = iota
	// Contour follows the region's boundary: the marks are level sets of
	// Sample.Wall, so they bend around a waist and close around a lobe
	// whatever shape it is. Needs Wall.
	Contour
	// Concentric is nested rings about the region's centre. Unlike Contour
	// it ignores the boundary shape, which is what makes it read as rings
	// drawn *on* the region rather than as its outline repeated. Needs
	// CX, CY.
	Concentric
	// Radial is rays from the region's centre. The angular pitch is
	// quantised so the family closes on itself with no seam. Needs CX, CY
	// and Reach.
	Radial
	// Fan spreads arcs between two poles placed one Reach either side of
	// the centre along the hatch angle. The marks are circular arcs through
	// both poles: tight near the poles, broad between them. Needs CX, CY
	// and Reach.
	Fan
	// Flow follows a vector field. The marks are level sets of a Perlin
	// stream function added to the parallel coordinate, which makes them
	// exact streamlines of a divergence-free field — they meander and never
	// cross or end. Amplitude sets how far the field bends them, Wavelength
	// how long the bends are.
	Flow
	// Scribble is Flow with the mean direction removed: the marks are level
	// sets of noise alone, so they wander and close into loops.
	Scribble
	// Stipple replaces the marks with dots on the same lattice — spacing
	// across, spacing along. Thickness is the dot diameter and ToneWidth
	// makes it a tonal stipple.
	Stipple
	// Chord connects two points on the region's boundary: the family is a
	// ring of chords each turning Angle radians around the boundary, so
	// every mark runs edge to edge and their envelope is a circle inside
	// the region. The region is taken as the disc of radius Reach about
	// CX, CY — the one structure here that needs the region to be roughly
	// round. Needs CX, CY and Reach.
	Chord
)

var structureNames = [...]string{
	"parallel", "contour", "concentric", "radial",
	"fan", "flow", "scribble", "stipple", "chord",
}

// String names the structure as it appears on the CLI and in a manifest.
func (s Structure) String() string {
	if s < 0 || int(s) >= len(structureNames) {
		return "unknown"
	}
	return structureNames[s]
}

// StructureNames lists every structure, in declaration order.
func StructureNames() []string { return structureNames[:] }

// StructureByName looks a structure up by its CLI name.
func StructureByName(name string) (Structure, bool) {
	for i, n := range structureNames {
		if n == name {
			return Structure(i), true
		}
	}
	return 0, false
}

// Waveform is the shape of the wobble applied along each mark.
type Waveform int

// The waveforms.
const (
	// Straight leaves the marks as the structure drew them.
	Straight Waveform = iota
	// Sine bends them into smooth waves.
	Sine
	// Zigzag breaks them into a sawtooth.
	Zigzag
)

var waveformNames = [...]string{"straight", "sine", "zigzag"}

// String names the waveform.
func (w Waveform) String() string {
	if w < 0 || int(w) >= len(waveformNames) {
		return "unknown"
	}
	return waveformNames[w]
}

// WaveformNames lists every waveform, in declaration order.
func WaveformNames() []string { return waveformNames[:] }

// Align says whether the hatch belongs to the canvas or to the region.
type Align int

// The alignments.
const (
	// AlignCanvas measures the angle and the mark phase in canvas space, so
	// neighbouring regions filled at the same angle share one continuous
	// screen — the marks run straight across the boundary between them.
	AlignCanvas Align = iota
	// AlignRegion measures the angle from Sample.Axis and the phase from
	// Sample.CX/CY, so every region carries its own hatch and the marks
	// break at the boundary. This is the difference between a mechanical
	// screen laid over a drawing and a hand filling each shape in turn.
	AlignRegion
)

var alignNames = [...]string{"canvas", "region"}

// String names the alignment.
func (a Align) String() string {
	if a < 0 || int(a) >= len(alignNames) {
		return "unknown"
	}
	return alignNames[a]
}

// AlignNames lists every alignment, in declaration order.
func AlignNames() []string { return alignNames[:] }

// Spec is a hatch as a set of parameters. The vocabulary is deliberately
// the one a person uses about hatching — angle, spacing, thickness,
// curvature, density, continuity, alignment — rather than one knob per
// structure.
//
// The zero Spec is a usable continuous parallel hatch, but start from
// Defaults: it fills in the spacings and softness that make a mark visible.
type Spec struct {
	// Structure is the arranging rule.
	Structure Structure

	// Angle is the direction of the marks, in radians, measured from the
	// +u axis. Under AlignRegion it is measured from Sample.Axis instead.
	// The Chord structure reads it as the turn each chord makes around the
	// boundary, because a chord family has no direction.
	Angle float64
	// Spacing is the pitch between marks, in canvas units. Ignored when
	// Fit is set.
	Spacing float64
	// Fit lays exactly this many marks across the region instead of using
	// Spacing, so every region gets the same number however large it is.
	// This is the alignment-to-the-shape knob that makes hatching read as
	// belonging to what it fills. 0 leaves Spacing in charge.
	Fit int
	// Thickness is the width of a mark as a fraction of the spacing: 0.25
	// is a light hatch, 0.5 is line-and-gap equal, above 1 the marks merge.
	// Relative rather than absolute so that a fitted hatch stays in
	// proportion to the region it fitted to.
	Thickness float64
	// Softness is how far a mark's edge fades, as a fraction of its
	// half-width. 0 is a hard edge (leave it to the renderer's
	// supersampling); around 0.4 reads as pencil.
	Softness float64

	// Curvature bends straight marks into arcs, in reciprocal canvas units:
	// the arc radius is 1/Curvature, so 2 curves noticeably over a canvas
	// and 0 is straight. Parallel and Flow only.
	Curvature float64
	// Waveform applies a wave along each mark.
	Waveform Waveform
	// Amplitude is how far the marks are displaced across themselves, as a
	// multiple of the spacing. It drives the Waveform and, for Flow and
	// Scribble, the strength of the noise field.
	Amplitude float64
	// Wavelength is the period of that displacement along the mark, in
	// canvas units — and, for Flow and Scribble, the scale of the field.
	Wavelength float64

	// Continuity is the share of a mark that is actually drawn, in [0,1]:
	// 1 is unbroken, 0.5 is a dash as long as its gap. Values ≤ 0 mean
	// unbroken, so the zero Spec is a continuous hatch.
	Continuity float64
	// Dash is the length of one dash-plus-gap along the mark, in canvas
	// units. Unset it is four spacings.
	Dash float64

	// Jitter displaces each mark from its lattice position by up to this
	// fraction of the spacing, and varies its width by the same share. It
	// is what stops a hatch reading as a printed screen; keep it under 0.4.
	Jitter float64

	// ToneDensity is how many times the hatch may halve its own density
	// where Sample.Tone is low. It thins by dropping every other mark, then
	// every other survivor — the way an engraver grades a tone — so the
	// marks that remain never split or bend. 0 turns tone off.
	ToneDensity float64
	// ToneWidth is how strongly Sample.Tone drives the mark width: at 1 a
	// mark ranges from nothing at tone 0 to twice Thickness at tone 1.
	ToneWidth float64

	// Align says whether the marks belong to the canvas or to the region.
	Align Align

	// Seed drives the jitter, the dash phases and the noise fields. Every
	// draw from it is a pure function of position, never a generator, so a
	// Hatch is safe to evaluate concurrently.
	Seed uint64
}

// Defaults is a plain continuous parallel hatch at 45°, fine enough to read
// at preview size. Build every spec from it so a field left alone is a
// sensible value rather than a zero.
func Defaults() Spec {
	return Spec{
		Structure:  Parallel,
		Angle:      math.Pi / 4,
		Spacing:    0.02,
		Thickness:  0.28,
		Softness:   0.4,
		Waveform:   Straight,
		Amplitude:  0.4,
		Wavelength: 0.12,
		Continuity: 1,
		Dash:       0.06,
	}
}

// Rotated returns the spec turned to a new angle — the usual way to build
// the second family of a cross-hatch.
func (s Spec) Rotated(angle float64) Spec {
	s.Angle = angle
	return s
}

// With returns the spec with f applied to a copy, for building a family of
// variants without mutating a shared value.
func (s Spec) With(f func(*Spec)) Spec {
	f(&s)
	return s
}

// Hatch is a resolved Spec: everything a coverage lookup needs, built once.
// It is immutable and its methods are pure, so one Hatch serves the whole
// parallel pixel loop.
type Hatch struct {
	spec  Spec
	field *noise.Perlin
	cos   float64
	sin   float64
}

// New resolves a Spec into a coverage function. Out-of-range and unset
// values are repaired here rather than at every lookup.
func New(s Spec) *Hatch {
	if s.Spacing <= 0 {
		s.Spacing = 0.02
	}
	if s.Thickness <= 0 {
		s.Thickness = 0.28
	}
	if s.Wavelength <= 0 {
		s.Wavelength = 0.12
	}
	if s.Dash <= 0 {
		s.Dash = 4 * s.Spacing
	}
	if s.Continuity <= 0 || s.Continuity > 1 {
		s.Continuity = 1
	}
	if s.Fit < 0 {
		s.Fit = 0
	}
	s.Softness = mathx.Clamp01(s.Softness)
	s.Jitter = math.Min(math.Max(s.Jitter, 0), 0.4)
	return &Hatch{
		spec:  s,
		field: noise.New(s.Seed ^ saltField),
		cos:   math.Cos(s.Angle),
		sin:   math.Sin(s.Angle),
	}
}

// Spec returns the resolved spec — the values actually in force, after the
// repairs New made. A manifest wants these, not what was asked for.
func (h *Hatch) Spec() Spec { return h.spec }

// Salts keep the several uses of one seed independent: moving a mark must
// not also move its dash phase.
const (
	saltField uint64 = 0x9E3779B97F4A7C15
	saltLine  uint64 = 0xC2B2AE3D27D4EB4F
	saltWidth uint64 = 0x165667B19E3779F9
	saltPhase uint64 = 0x27D4EB2F165667C5
	saltDot   uint64 = 0x85EBCA77C2B2AE63
)

// Cover is how much ink the hatch puts at a sample, in [0,1].
func (h *Hatch) Cover(s Sample) float64 {
	c, _ := h.CoverLine(s)
	return c
}

// CoverLine is Cover plus the index of the mark the point belongs to —
// which mark of the family it is, counting from the phase origin. It is
// what a caller needs to colour mark by mark (every third line in a second
// ink) and what the weave uses to decide which family is on top.
func (h *Hatch) CoverLine(s Sample) (cover float64, line int) {
	if h.spec.Structure == Chord {
		return h.chord(s), 0
	}
	across, along, ok := h.coords(s)
	if !ok {
		return 0, 0
	}
	pitch := h.pitch(s)
	if h.spec.Structure == Stipple {
		return h.stipple(s, across, along, pitch)
	}
	return h.lines(s, across, along, pitch, h.steepness(s))
}

// steepness is how fast the across coordinate changes with position, which
// is 1 for every structure whose across coordinate is already a distance
// and is wildly uneven for the two built on a noise field.
//
// It has to be corrected for, and correcting it is the difference between a
// flow field and a smear. The marks are level sets of a stream function, and
// the gap between consecutive level sets is the pitch divided by the
// gradient — so where the field is slack the marks stand far apart, and if
// their width is a fraction of *that* gap they come out as blobs, while
// where the field is steep they crowd and vanish. Dividing the distance by
// the gradient and leaving the width alone gives marks of one width that
// converge and diverge, which is what following a field looks like.
func (h *Hatch) steepness(s Sample) float64 {
	if h.spec.Structure != Flow && h.spec.Structure != Scribble {
		return 1
	}
	// In canvas units, so the estimate is the same at any output size.
	const eps = 1e-4
	a0, _, _ := h.coords(s)
	su, sv := s, s
	su.U += eps
	sv.V += eps
	au, _, _ := h.coords(su)
	av, _, _ := h.coords(sv)
	return math.Max(math.Hypot((au-a0)/eps, (av-a0)/eps), 1e-6)
}

// Coords returns the mark coordinates at a sample, both in canvas units:
// across runs perpendicular to the marks, along runs down one. They are the
// hatch's own frame, and exposing them is what lets a caller shift colour
// along the stroke direction or band it across the family without this
// package knowing anything about colour. ok is false where the structure
// has nothing to say (a contour hatch with no boundary, a chord hatch).
func (h *Hatch) Coords(s Sample) (across, along float64, ok bool) {
	if h.spec.Structure == Chord {
		return 0, 0, false
	}
	return h.coords(s)
}

// lines turns the mark coordinates into the coverage of a family of lines.
//
// Three candidate marks are tested rather than the nearest one, because
// jitter can displace a neighbour closer than the mark whose slot the point
// falls in — and because a mark thicker than its pitch overlaps its
// neighbours by construction.
func (h *Hatch) lines(s Sample, across, along, pitch, steep float64) (float64, int) {
	q := across / pitch
	k0 := int(math.Round(q))
	// How many slots either side can reach this point. One is enough for an
	// even family — jitter and an overlapping thickness never move a mark
	// more than a slot — but where a field is steep the marks crowd into a
	// fraction of a slot, and testing only the neighbours leaves a haze of
	// half-covered pixels between them.
	span := 1 + int(h.spec.Thickness*steep/2)
	if span > 6 {
		span = 6
	}
	best, bestK := 0.0, k0
	for k := k0 - span; k <= k0+span; k++ {
		c := h.mark(s, across, along, pitch, steep, k)
		if c > best {
			best, bestK = c, k
		}
	}
	return best, bestK
}

// mark is the coverage contributed by one mark of the family.
func (h *Hatch) mark(s Sample, across, along, pitch, steep float64, k int) float64 {
	sp := h.spec
	weight := h.thin(k, s.Tone)
	if weight <= 0 {
		return 0
	}

	centre := float64(k) * pitch
	if sp.Jitter > 0 {
		centre += sp.Jitter * pitch * (hash(sp.Seed^saltLine, k) - 0.5) * 2
	}
	centre += sp.Amplitude * pitch * h.wave(along)

	half := h.halfWidth(s.Tone, k) * pitch
	if half <= 0 {
		return 0
	}
	soft := math.Max(sp.Softness*half, 1e-12)
	cover := 1 - mathx.Smoothstep(half-soft, half+soft, math.Abs(across-centre)/steep)
	if cover <= 0 {
		return 0
	}
	return cover * weight * h.dash(along, k)
}

// halfWidth is half a mark's width as a fraction of the pitch, after tone
// and jitter have had their say.
func (h *Hatch) halfWidth(tone float64, k int) float64 {
	sp := h.spec
	t := sp.Thickness
	if sp.ToneWidth > 0 {
		t *= 1 + sp.ToneWidth*(2*mathx.Clamp01(tone)-1)
	}
	if sp.Jitter > 0 {
		t *= 1 + sp.Jitter*(hash(sp.Seed^saltWidth, k)-0.5)
	}
	return math.Max(t, 0) / 2
}

// wave is the displacement applied along a mark, in units of the pitch.
func (h *Hatch) wave(along float64) float64 {
	switch h.spec.Waveform {
	case Sine:
		return math.Sin(2 * math.Pi * along / h.spec.Wavelength)
	case Zigzag:
		// A triangle wave: |saw| rescaled to [-1,1].
		return 4*math.Abs(frac(along/h.spec.Wavelength+0.25)-0.5) - 1
	default:
		return 0
	}
}

// thin is how much of mark k survives the tone, in [0,1].
//
// A density gradient cannot simply stretch the pitch: a lattice whose pitch
// varies with position has to split or merge its marks somewhere, and both
// are visible. Halving instead — drop every other mark, then every other
// survivor — is what an engraver does, and the marks that remain are
// exactly where they were.
func (h *Hatch) thin(k int, tone float64) float64 {
	if h.spec.ToneDensity <= 0 {
		return 1
	}
	level := h.spec.ToneDensity * (1 - mathx.Clamp01(tone))
	if level <= 0 {
		return 1
	}
	oct := int(level)
	frac := level - float64(oct)
	if oct > 30 {
		oct, frac = 30, 0
	}
	step := 1 << oct
	switch {
	case mod(k, 2*step) == 0:
		return 1
	case mod(k, step) == 0:
		return 1 - frac
	default:
		return 0
	}
}

// dash cuts a mark into segments along its own length. Each mark gets its
// own phase, because dashes that line up across the family read as a grid
// of tiles rather than as broken strokes.
func (h *Hatch) dash(along float64, k int) float64 {
	if h.spec.Continuity >= 1 {
		return 1
	}
	phase := hash(h.spec.Seed^saltPhase, k)
	x := math.Abs(frac(along/h.spec.Dash+phase)-0.5) * 2
	const edge = 0.08
	return 1 - mathx.Smoothstep(h.spec.Continuity-edge, h.spec.Continuity+edge, x)
}

// stipple puts dots on the lattice instead of lines. The nine nearest
// lattice sites are tested because a jittered dot may reach past its own
// cell.
func (h *Hatch) stipple(s Sample, across, along, pitch float64) (float64, int) {
	sp := h.spec
	ki := int(math.Round(across / pitch))
	mi := int(math.Round(along / pitch))
	best, bestK := 0.0, ki
	for k := ki - 1; k <= ki+1; k++ {
		weight := h.thin(k, s.Tone)
		if weight <= 0 {
			continue
		}
		for m := mi - 1; m <= mi+1; m++ {
			cx := float64(k) * pitch
			cy := float64(m) * pitch
			if sp.Jitter > 0 {
				cx += sp.Jitter * pitch * (hash2(sp.Seed^saltDot, k, m) - 0.5) * 2
				cy += sp.Jitter * pitch * (hash2(sp.Seed^saltLine, k, m) - 0.5) * 2
			}
			r := h.halfWidth(s.Tone, k*7919+m) * pitch
			if r <= 0 {
				continue
			}
			soft := math.Max(sp.Softness*r, 1e-12)
			d := math.Hypot(across-cx, along-cy)
			c := (1 - mathx.Smoothstep(r-soft, r+soft, d)) * weight
			if c > best {
				best, bestK = c, k
			}
		}
	}
	return best, bestK
}

func frac(x float64) float64 { return x - math.Floor(x) }

// mod is a non-negative remainder; Go's % keeps the sign of the dividend,
// which would make the thinning lattice asymmetric about the origin.
func mod(a, m int) int {
	r := a % m
	if r < 0 {
		r += m
	}
	return r
}

func hash(seed uint64, k int) float64 { return noise.Hash01(seed, int64(k), 0) }

func hash2(seed uint64, k, m int) float64 { return noise.Hash01(seed, int64(k), int64(m)) }
