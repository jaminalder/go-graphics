package hatch

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
)

// This file is where the structures actually differ. Each one maps a point
// to the same pair of numbers — how far across the family it is and how far
// along its own mark — and everything downstream (width, waveform, dashes,
// thinning, dots) is shared. That is the whole design: a structure is a
// change of coordinates, not a different kind of hatching.

// defaultReach is what an unknown region size is taken to be: half the
// canvas height, so a hatch with no region at all still behaves.
const defaultReach = 0.5

func reach(s Sample) float64 {
	if s.Reach > 0 {
		return s.Reach
	}
	return defaultReach
}

// origin is the point the mark phase is measured from, and the frame the
// angle is measured in.
func (h *Hatch) frame(s Sample) (ox, oy, cos, sin float64) {
	if h.spec.Align == AlignRegion {
		a := h.spec.Angle + s.Axis
		return s.CX, s.CY, math.Cos(a), math.Sin(a)
	}
	return 0, 0, h.cos, h.sin
}

// pitch is the spacing actually used at a sample, in the units of that
// structure's across coordinate.
//
// Fit is where alignment to the containing shape happens: rather than a
// fixed pitch it asks for a fixed *count* across the region, so a small cell
// and a large one both come out with the same number of marks. What "across
// the region" spans differs per structure — a wall distance runs from 0 to
// one Reach, a diameter runs across two, an angle runs round 2π — which is
// why this is not one formula.
func (h *Hatch) pitch(s Sample) float64 {
	r := reach(s)
	if h.spec.Fit > 0 {
		n := float64(h.spec.Fit)
		switch h.spec.Structure {
		case Contour, Concentric:
			return r / n
		case Radial, Fan:
			return 2 * math.Pi * r / n
		default:
			return 2 * r / n
		}
	}
	switch h.spec.Structure {
	case Radial, Fan:
		// The across coordinate of a radial or fan family is an angle, and
		// it wraps. Unless a whole number of marks fits into the turn there
		// is a seam where the last one meets the first, which reads as a
		// crack running out of the centre. Quantising the pitch to the
		// nearest exact fit costs at most half a mark of spacing error and
		// removes the seam entirely.
		n := math.Max(math.Round(2*math.Pi*r/h.spec.Spacing), 3)
		return 2 * math.Pi * r / n
	default:
		return h.spec.Spacing
	}
}

// coords maps a sample into the structure's own frame. ok is false where
// the structure cannot answer — a contour hatch outside any region.
func (h *Hatch) coords(s Sample) (across, along float64, ok bool) {
	ox, oy, cos, sin := h.frame(s)
	dx, dy := s.U-ox, s.V-oy

	switch h.spec.Structure {
	case Contour:
		if math.IsInf(s.Wall, 0) || math.IsNaN(s.Wall) {
			return 0, 0, false
		}
		// Along a contour there is no natural arc length, so the angle round
		// the centre stands in for one. It only has to be monotone along the
		// mark for dashes to fall at even intervals, which it is.
		return s.Wall, math.Atan2(s.V-s.CY, s.U-s.CX) * reach(s), true

	case Concentric:
		r := math.Hypot(s.U-s.CX, s.V-s.CY)
		return r, math.Atan2(s.V-s.CY, s.U-s.CX) * r, true

	case Radial:
		rx, ry := s.U-s.CX, s.V-s.CY
		theta := math.Atan2(ry, rx) - h.spec.Angle - regionAxis(h.spec, s)
		return wrapPi(theta) * reach(s), math.Hypot(rx, ry), true

	case Fan:
		return h.fan(s)

	case Flow:
		a, w := dx*cos+dy*sin, -dx*sin+dy*cos
		w = h.bend(a, w)
		// The marks are the level sets of w + amplitude·ψ, and the level sets
		// of a stream function are exactly the streamlines of the
		// divergence-free field ∇⊥ψ. So these really do follow a vector
		// field, rather than being straight lines wobbled to look as if they
		// did: they never cross and never stop.
		return w + h.spec.Amplitude*h.pitch(s)*h.stream(s.U, s.V), a, true

	case Scribble:
		// The same construction with the mean direction taken out. What is
		// left is the level sets of noise alone: they wander, close into
		// loops, and — because they are still level sets — never cross.
		a := dx*cos + dy*sin
		amp := math.Max(h.spec.Amplitude, 1) * h.pitch(s)
		return amp * h.stream(s.U, s.V), a, true

	default: // Parallel, Stipple
		a, w := dx*cos+dy*sin, -dx*sin+dy*cos
		return h.bend(a, w), a, true
	}
}

// regionAxis is the region's own direction when the hatch is aligned to it,
// and zero otherwise. Radial hatching reads the angle directly rather than
// through frame, so it needs this separately.
func regionAxis(sp Spec, s Sample) float64 {
	if sp.Align == AlignRegion {
		return s.Axis
	}
	return 0
}

// bend turns straight marks into arcs of radius 1/Curvature.
//
// The centre sits one radius away along the across axis, so the across
// coordinate becomes R − |P − C|. Expanded for small a/R that is
// w − a²/2R: the marks keep their spacing and pick up a sagitta, which is
// what curving a hatch means. As Curvature goes to zero it returns the
// straight coordinate exactly.
func (h *Hatch) bend(along, across float64) float64 {
	k := h.spec.Curvature
	if k == 0 {
		return across
	}
	r := 1 / k
	return r - math.Copysign(math.Hypot(along, across-r), r)
}

// stream is the scalar field whose level sets the flow and scribble
// structures follow. Three octaves: one is too smooth to read as a field,
// and beyond three the marks fray faster than a line width.
func (h *Hatch) stream(u, v float64) float64 {
	w := math.Max(h.spec.Wavelength, 1e-6)
	return h.field.FBM(u/w, v/w, 3)
}

// fan places two poles one Reach either side of the centre along the hatch
// angle and returns the bipolar coordinates about them.
//
// The angle a point subtends between the two poles is constant along a
// circular arc through both, so the level sets of that angle are a family
// of arcs spreading from one pole to the other — a fan, and one that closes
// on itself with no seam because the angle wraps by exactly 2π.
func (h *Hatch) fan(s Sample) (across, along float64, ok bool) {
	_, _, cos, sin := h.frame(s)
	r := reach(s)
	ax, ay := s.U-(s.CX-cos*r), s.V-(s.CY-sin*r)
	bx, by := s.U-(s.CX+cos*r), s.V-(s.CY+sin*r)
	sigma := math.Atan2(ax*by-ay*bx, ax*bx+ay*by)
	na := ax*ax + ay*ay
	nb := bx*bx + by*by
	if na <= 0 || nb <= 0 {
		return 0, 0, false
	}
	// The companion coordinate: log of the distance ratio, which is
	// monotone from pole to pole and so serves as arc length along a mark.
	return sigma * r, 0.5 * math.Log(na/nb) * r, true
}

// chord fills the region with marks that each run from one point on its
// boundary to another.
//
// The boundary is taken as the circle of radius Reach about the centre —
// the one place in this package where the region has to be roughly round,
// and the honest cost of describing a region by a centre and a scale rather
// than by its outline. Chord i leaves the boundary at angle 2πi/n and
// arrives at 2πi/n + Angle, so every mark is edge to edge, the family
// closes, and its envelope is the circle of radius Reach·|cos(Angle/2)|.
func (h *Hatch) chord(s Sample) float64 {
	r := reach(s)
	n := h.spec.Fit
	if n <= 0 {
		n = int(math.Round(2 * math.Pi * r / h.spec.Spacing))
	}
	if n < 3 {
		n = 3
	}
	if n > 512 {
		n = 512
	}
	turn := h.spec.Angle
	if turn == 0 {
		turn = math.Pi / 2
	}
	half := h.spec.Thickness * h.spec.Spacing / 2
	if h.spec.Fit > 0 {
		half = h.spec.Thickness * (2 * r / float64(h.spec.Fit)) / 2
	}
	if half <= 0 {
		return 0
	}
	soft := math.Max(h.spec.Softness*half, 1e-12)

	px, py := s.U-s.CX, s.V-s.CY
	// Nothing outside the disc can be on a chord of it.
	if px*px+py*py > (r+half)*(r+half) {
		return 0
	}
	best := math.Inf(1)
	step := 2 * math.Pi / float64(n)
	for i := range n {
		t := float64(i)*step + regionAxis(h.spec, s)
		ax, ay := r*math.Cos(t), r*math.Sin(t)
		bx, by := r*math.Cos(t+turn), r*math.Sin(t+turn)
		if d := segDist(px, py, ax, ay, bx, by); d < best {
			best = d
		}
	}
	return 1 - mathx.Smoothstep(half-soft, half+soft, best)
}

// segDist is the distance from a point to a line segment.
func segDist(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	wx, wy := px-ax, py-ay
	den := vx*vx + vy*vy
	t := 0.0
	if den > 0 {
		t = (wx*vx + wy*vy) / den
		t = math.Min(math.Max(t, 0), 1)
	}
	return math.Hypot(wx-t*vx, wy-t*vy)
}

// wrapPi folds an angle into (−π, π].
func wrapPi(a float64) float64 {
	for a <= -math.Pi {
		a += 2 * math.Pi
	}
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	return a
}
