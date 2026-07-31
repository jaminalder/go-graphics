package qql

import "math"

// The geometry and framing vocabulary QQL's tables are written in. The
// sampling half of it — weighted choice, gaussians, thinning — moved to
// internal/rnd once a second sketch needed it; what is left here is what
// is specific to how the source states an angle or a length.

// pi returns v half-turns in radians, matching how the source expresses
// angles (pi(0.5) is a quarter turn).
func pi(v float64) float64 { return math.Pi * v }

// modulo is the always-positive remainder.
func modulo(n, m float64) float64 { return math.Mod(math.Mod(n, m)+m, m) }

// angle is the direction from p1 to p2, in [0, 2π).
func angle(x1, y1, x2, y2 float64) float64 {
	return modulo(math.Atan2(y2-y1, x2-x1), pi(2))
}

// dist is the distance between two points.
func dist(x1, y1, x2, y2 float64) float64 { return math.Hypot(x2-x1, y2-y1) }

// frame carries the canvas proportions. QQL states every length as a
// fraction of its virtual width or height; in canvas units the height is 1
// and the width is the aspect ratio, so the same fractions carry over
// unchanged and the composition is resolution independent.
type frame struct{ aspect float64 }

func (f frame) w(v float64) float64 { return v * f.aspect }

func (f frame) h(v float64) float64 { return v }
