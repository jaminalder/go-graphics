package flame

import (
	"math"
	"math/rand/v2"
	"sort"
)

// Xform is one weighted map F_i: affine, then a blend of variations, then
// an optional post affine. Color is the structural colour coordinate in [0,1].
// ColorSpeed is the blend toward Color (0.5 is the paper's (c+c_i)/2).
// A symmetry xform must not touch the colour coordinate.
type Xform struct {
	A, B, C, D, E, F       float64
	PA, PB, PC, PD, PE, PF float64
	HasPost                bool
	Vars                   []Var
	Color                  float64
	ColorSpeed             float64
	Weight                 float64
	Opacity                float64 // 0 means 1: existing xforms stay fully visible
	Symmetry               bool
}

// System is a complete IFS ready to iterate.
type System struct {
	X     []Xform
	Final *Xform
	Cam   Camera
	// Xaos is the relative-weight matrix: Xaos[from][to] multiplies
	// Weight[to] when picking the next map. Nil means all ones
	// (independent weighted picks).
	Xaos [][]float64
	cdf  []float64
	xcdf [][]float64
}

// Camera maps the mathematical plane onto the histogram. Scale is the
// width of the view, in math units, along the shorter image axis.
type Camera struct {
	CX, CY, Scale, Rotate float64
}

// Affine returns the six coefficients of a scale-then-rotate-then-translate
// transform. sx, sy are axis scales, rot is radians, tx, ty translation.
func Affine(sx, sy, rot, tx, ty float64) (a, b, c, d, e, f float64) {
	cs, sn := math.Cos(rot), math.Sin(rot)
	return cs * sx, -sn * sy, tx, sn * sx, cs * sy, ty
}

func (s *System) prepare() {
	n := len(s.X)
	s.cdf = cumulative(s.X, nil)
	s.xcdf = nil
	if n == 0 || !s.xaosActive() {
		return
	}
	s.xcdf = make([][]float64, n)
	for i := 0; i < n; i++ {
		row := s.Xaos[i]
		if len(row) < n {
			row = nil
		}
		s.xcdf[i] = cumulative(s.X, row)
		if s.xcdf[i][n-1] <= 0 {
			s.xcdf[i] = s.cdf
		}
	}
}

func (s *System) xaosActive() bool {
	n := len(s.X)
	if len(s.Xaos) != n {
		return false
	}
	for i := 0; i < n; i++ {
		if len(s.Xaos[i]) != n {
			return false
		}
	}
	return true
}

func cumulative(xs []Xform, xaosRow []float64) []float64 {
	cdf := make([]float64, len(xs))
	var acc float64
	for i, x := range xs {
		w := x.Weight
		if w < 0 {
			w = 0
		}
		if xaosRow != nil {
			xw := xaosRow[i]
			if xw < 0 {
				xw = 0
			}
			w *= xw
		}
		acc += w
		cdf[i] = acc
	}
	if acc <= 0 && len(xs) > 0 {
		cdf[len(cdf)-1] = 1
	}
	return cdf
}

func (s *System) pick(rng *rand.Rand, last int) int {
	if len(s.X) == 0 {
		return 0
	}
	cdf := s.cdf
	if last >= 0 && last < len(s.xcdf) {
		cdf = s.xcdf[last]
	}
	total := cdf[len(cdf)-1]
	if total <= 0 {
		return rng.IntN(len(s.X))
	}
	u := rng.Float64() * total
	for i, c := range cdf {
		if u <= c {
			return i
		}
	}
	return len(s.X) - 1
}

// InitXaos fills a ones matrix so SetXaos can punch holes. A ones
// matrix is the independent chaos game.
func (s *System) InitXaos() {
	n := len(s.X)
	s.Xaos = make([][]float64, n)
	for i := range s.Xaos {
		s.Xaos[i] = make([]float64, n)
		for j := range s.Xaos[i] {
			s.Xaos[i][j] = 1
		}
	}
}

// SetXaos sets the relative weight from → to. InitXaos if needed.
func (s *System) SetXaos(from, to int, w float64) {
	n := len(s.X)
	if from < 0 || to < 0 || from >= n || to >= n {
		return
	}
	if !s.xaosActive() {
		s.InitXaos()
	}
	if w < 0 {
		w = 0
	}
	s.Xaos[from][to] = w
}

func (x Xform) apply(px, py float64, rng *rand.Rand) (float64, float64) {
	tx := x.A*px + x.B*py + x.C
	ty := x.D*px + x.E*py + x.F
	if len(x.Vars) == 0 {
		return tx, ty
	}
	var ox, oy float64
	for _, v := range x.Vars {
		if v.Weight == 0 {
			continue
		}
		vx, vy := applyVar(v, tx, ty, x, rng)
		ox += v.Weight * vx
		oy += v.Weight * vy
	}
	if x.HasPost {
		ox, oy = x.PA*ox+x.PB*oy+x.PC, x.PD*ox+x.PE*oy+x.PF
	}
	return ox, oy
}

func (x Xform) blendColor(c float64) float64 {
	if x.Symmetry {
		return c
	}
	speed := x.ColorSpeed
	if speed == 0 {
		speed = 0.5
	}
	return c*(1-speed) + x.Color*speed
}

func (x Xform) opacity() float64 {
	if x.Opacity <= 0 {
		return 1
	}
	if x.Opacity > 1 {
		return 1
	}
	return x.Opacity
}

// AddRotation appends n-1 rotation xforms whose weights equal the current
// total, so an n-way rotational symmetry has balanced density. Colour is
// left untouched on those maps.
func (s *System) AddRotation(n int) {
	if n < 2 {
		return
	}
	var sum float64
	for _, x := range s.X {
		if x.Weight > 0 {
			sum += x.Weight
		}
	}
	if sum <= 0 {
		sum = 1
	}
	for k := 1; k < n; k++ {
		ang := 2 * math.Pi * float64(k) / float64(n)
		a, b, c, d, e, f := Affine(1, 1, ang, 0, 0)
		s.X = append(s.X, Xform{
			A: a, B: b, C: c, D: d, E: e, F: f,
			Vars:     []Var{{Kind: Linear, Weight: 1}},
			Weight:   sum,
			Symmetry: true,
		})
	}
}

// AddDihedral appends a reflection in x, weight-balanced against the
// current total. Combine with AddRotation for the dihedral groups.
func (s *System) AddDihedral() {
	var sum float64
	for _, x := range s.X {
		if x.Weight > 0 {
			sum += x.Weight
		}
	}
	if sum <= 0 {
		sum = 1
	}
	s.X = append(s.X, Xform{
		A: -1, E: 1,
		Vars:     []Var{{Kind: Linear, Weight: 1}},
		Weight:   sum,
		Symmetry: true,
	})
}

// Frame estimates a camera that fits the attractor with a modest margin.
// n is the number of plotted points used for the percentile box; it does
// not depend on output pixels.
func Frame(s System, rng *rand.Rand, n int) Camera {
	if n < 32 {
		n = 32
	}
	s.prepare()
	xs := make([]float64, 0, n)
	ys := make([]float64, 0, n)
	x, y := rng.Float64()*2-1, rng.Float64()*2-1
	last := -1
	for i := 0; i < n+warmup; i++ {
		xi := s.pick(rng, last)
		last = xi
		xf := s.X[xi]
		x, y = xf.apply(x, y, rng)
		if bad(x, y) {
			x, y = rng.Float64()*2-1, rng.Float64()*2-1
			last = -1
			continue
		}
		px, py := x, y
		if s.Final != nil {
			px, py = s.Final.apply(px, py, rng)
			if bad(px, py) {
				continue
			}
		}
		if i >= warmup {
			xs = append(xs, px)
			ys = append(ys, py)
		}
	}
	if len(xs) < 16 {
		return Camera{Scale: 4}
	}
	sort.Float64s(xs)
	sort.Float64s(ys)
	lo := int(float64(len(xs)) * 0.02)
	hi := int(float64(len(xs))*0.98) - 1
	if hi <= lo {
		hi = len(xs) - 1
		lo = 0
	}
	minX, maxX := xs[lo], xs[hi]
	minY, maxY := ys[lo], ys[hi]
	cx := (minX + maxX) / 2
	cy := (minY + maxY) / 2
	dx := maxX - minX
	dy := maxY - minY
	span := dx
	if dy > span {
		span = dy
	}
	if span < 1e-6 {
		span = 2
	}
	return Camera{CX: cx, CY: cy, Scale: span * 1.08}
}

func (c Camera) project(x, y float64, w, h int) (px, py float64) {
	dx, dy := x-c.CX, y-c.CY
	if c.Rotate != 0 {
		cs, sn := math.Cos(c.Rotate), math.Sin(c.Rotate)
		dx, dy = cs*dx-sn*dy, sn*dx+cs*dy
	}
	short := float64(h)
	if w < h {
		short = float64(w)
	}
	scale := c.Scale
	if scale <= 0 {
		scale = 4
	}
	s := short / scale
	return float64(w)*0.5 + dx*s, float64(h)*0.5 - dy*s
}

func bad(x, y float64) bool {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return true
	}
	return x*x+y*y > 1e12
}
