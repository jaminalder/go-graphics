package paint

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
)

// Brush is a bundle of bristles swept along a path.
//
// Every bristle traces its own laterally offset copy of the path, so the
// streaks a brush leaves always run *along* the stroke and never cross
// each other. That is the whole difference between a smear and a
// scribble: texture is a property of the brush geometry, not extra marks
// drawn on top of the shape. Gaps between bristles and bristles that run
// dry are the only source of texture, which means any path — arc, line,
// flow-field curve — picks up the same painted character for free.
type Brush struct {
	bristles []bristle
	width    float64
	n        int
	dry      float64
	grain    float64
}

// bristle is one hair: a fixed offset across the ferrule, the width of
// the track it lays down, how much paint it carries, and the phases of
// the slow wave that lifts it off the surface.
type bristle struct {
	off  float64 // lateral offset from the path center, canvas units
	w    float64 // track width, canvas units
	load float64 // alpha multiplier
	fr   [2]float64
	ph   [2]float64
}

// NewBrush builds a bundle of n bristles spanning width (canvas units).
// dry in [0,1] is the brush's single character knob: at 0 the bundle is
// even and fully charged and lays a flat, hard-edged coat, and as it
// rises the bristles splay, lose load toward the rim and start lifting
// off the surface.
func NewBrush(rng *rand.Rand, width float64, n int, dry float64) Brush {
	if n < 1 {
		n = 1
	}
	b := Brush{width: width, n: n, dry: mathx.Clamp01(dry), grain: width * 8}
	b.bristles = makeBristles(rng, width, n, b.dry, b.grain)
	return b
}

// Grain sets how far the brush travels, in canvas units, between lifting
// off the surface and settling back down. It is a property of the hand
// rather than of the ferrule, so a fine liner dragged across a whole
// shape wants a grain far longer than its own width — otherwise its
// marks break into dashes instead of thinning and fading like a smear.
// The default is eight ferrule widths.
func (b Brush) Grain(rng *rand.Rand, l float64) Brush {
	b.grain = math.Max(l, 1e-6)
	b.bristles = makeBristles(rng, b.width, b.n, b.dry, b.grain)
	return b
}

// Reload returns a brush of the same character with a freshly arranged
// bundle — the painter dipping and re-splaying the same brush. Sweeping a
// shape with a reloaded brush per pass keeps gaps from lining up between
// passes.
func (b Brush) Reload(rng *rand.Rand) Brush {
	b.bristles = makeBristles(rng, b.width, b.n, b.dry, b.grain)
	return b
}

// Width returns the ferrule width in canvas units.
func (b Brush) Width() float64 { return b.width }

// makeBristles arranges the bundle. Everything irregular about it scales
// with dry, because that is what dryness physically is: a loaded brush
// holds its bristles together in an even, fully charged block, and it is
// only as it empties that they splay apart, thin out and start skipping.
// Tying all three to one number means dry = 0 lays a machine-perfect
// coat with no extra "make it flat" special case.
func makeBristles(rng *rand.Rand, width float64, n int, dry, grain float64) []bristle {
	out := make([]bristle, n)
	spacing := width / float64(n)
	for i := range out {
		// Even lanes across the ferrule, jittered as the bundle splays.
		lane := (float64(i)+0.5)/float64(n) - 0.5
		off := lane*width + (rng.Float64()-0.5)*spacing*0.9*dry
		// Outer bristles carry less paint, which is what frays the edge
		// of a dry stroke and leaves a clean one crisp.
		edge := math.Abs(lane) * 2 // 0 at the core, ~1 at the rim
		out[i] = bristle{
			off: off,
			// Slightly wider than the lane so neighbours overlap and a wet
			// bundle leaves no hairlines between tracks.
			w:    spacing * (1.15 + (0.5*rng.Float64()-0.15)*dry),
			load: 1 - 0.7*dry*edge*rng.Float64(),
			fr:   dryWave(rng, grain),
			ph:   [2]float64{rng.Float64() * 2 * math.Pi, rng.Float64() * 2 * math.Pi},
		}
	}
	return out
}

// dryWave picks the two frequencies of a bristle's lift-off wave from the
// grain length. The second harmonic is kept close to the first so the
// wave stays slow: a bristle that lifts and re-engages every few pixels
// reads as grit or stitching, not as a smear.
func dryWave(rng *rand.Rand, grain float64) [2]float64 {
	base := 2 * math.Pi / (grain * (0.7 + 0.6*rng.Float64()))
	return [2]float64{base, base * (1.6 + 0.7*rng.Float64())}
}

// coverage is how much paint the bristle leaves at arc length s: a slow
// two-harmonic wave thresholded so the bristle lifts for long runs and
// re-engages, rather than flickering dab to dab.
func (br bristle) coverage(s, dry float64) float64 {
	if dry <= 0 {
		return 1
	}
	k := math.Sin(s*br.fr[0]+br.ph[0]) + 0.6*math.Sin(s*br.fr[1]+br.ph[1])
	// Threshold sweeps up through the wave's range as dry goes 0→1. The
	// ramp is wide so a bristle thins out and returns gradually; a narrow
	// one would chop the track into hard-ended dashes.
	t := -1.75 + 3.3*dry
	return mathx.Smoothstep(t, t+1.1, k)
}

// Stroke sweeps the brush along pts, painting each bristle as its own
// laterally offset track. alpha scales the whole bundle.
func (b Brush) Stroke(c *Canvas, pts []Pt, col palette.Color, alpha float64) {
	if len(pts) < 2 {
		if len(pts) == 1 {
			c.Dab(pts[0].X, pts[0].Y, b.width/2, col, alpha)
		}
		return
	}
	nx, ny := normals(pts)
	track := make([]Pt, len(pts))
	for _, br := range b.bristles {
		for i, p := range pts {
			track[i] = Pt{X: p.X + nx[i]*br.off, Y: p.Y + ny[i]*br.off}
		}
		b.dryStroke(c, track, br, col, alpha*br.load)
	}
}

// dryStroke walks one bristle's track dabbing at adaptive spacing, with
// the dab alpha modulated by the bristle's coverage along arc length.
func (b Brush) dryStroke(c *Canvas, pts []Pt, br bristle, col palette.Color, alpha float64) {
	r := br.w / 2
	step := math.Max(r*0.4, 0.7/c.scale)
	s := 0.0
	carry := 0.0
	for i := 1; i < len(pts); i++ {
		ax, ay := pts[i-1].X, pts[i-1].Y
		bx, by := pts[i].X, pts[i].Y
		segLen := math.Hypot(bx-ax, by-ay)
		if segLen == 0 {
			continue
		}
		for d := step - carry; d < segLen; d += step {
			t := d / segLen
			if a := alpha * br.coverage(s+d, b.dry); a > 0.004 {
				c.Dab(ax+(bx-ax)*t, ay+(by-ay)*t, r, col, a)
			}
		}
		carry = math.Mod(carry+segLen, step)
		s += segLen
	}
}

// normals returns the unit left-normal at every point of the polyline,
// taken from the neighbouring segments so offset tracks stay parallel
// through curves.
func normals(pts []Pt) (nx, ny []float64) {
	nx = make([]float64, len(pts))
	ny = make([]float64, len(pts))
	for i := range pts {
		var dx, dy float64
		switch {
		case i == 0:
			dx, dy = pts[1].X-pts[0].X, pts[1].Y-pts[0].Y
		case i == len(pts)-1:
			dx, dy = pts[i].X-pts[i-1].X, pts[i].Y-pts[i-1].Y
		default:
			dx, dy = pts[i+1].X-pts[i-1].X, pts[i+1].Y-pts[i-1].Y
		}
		l := math.Hypot(dx, dy)
		if l == 0 {
			nx[i], ny[i] = 0, 0
			continue
		}
		nx[i], ny[i] = -dy/l, dx/l
	}
	return nx, ny
}
