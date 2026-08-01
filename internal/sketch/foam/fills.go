package foam

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/cells"
	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/noise"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/rnd"
)

// The fill styles. This is the layer the whole sketch exists for: the
// structure says which cell a point is in and how far it is from that
// cell's own wall, and a style turns those two numbers into paint. Nothing
// here knows the shape of the region it is filling, which is exactly why a
// style works as well inside a crescent as inside a bubble.
const (
	styleWash   = iota // transparent pigment with a dried rim
	stylePencil        // directional strokes laid over the paper
	styleBands         // rings following the cell's own outline
	styleHatch         // thin parallel lines on bare paper
	styleEmpty         // nothing; bare paper
	nstyles
)

// tooth is the paper's grain, in canvas units. Finer than sketch 008's,
// because there it is a wash granulating and here it is a pencil catching.
const tooth = 0.0025

// dress is everything one cell is filled with, settled before a single
// pixel is drawn.
type dress struct {
	style    int
	pigment  palette.Color
	deep     palette.Color // the same pigment pressed harder: rims and bands
	light    palette.Color // the pigment let down toward the paper
	cos, sin float64       // stroke direction
	pitch    float64       // band spacing, canvas units
	stroke   float64       // stroke spacing, canvas units
	press    float64       // how hard the pencil was leaning
}

// dress settles every cell's pigment and style.
func (s *Sketch) dress(f *cells.Foam, l levels, rng *rand.Rand, field *noise.Perlin, aspect float64, paper palette.Color, ramp []palette.Color) []dress {
	out := make([]dress, f.Len())
	for i, c := range f.Cells() {
		out[i] = s.dressOne(c, l, rng, field, aspect, paper, ramp)
	}
	return out
}

func (s *Sketch) dressOne(c cells.Cell, l levels, rng *rand.Rand, field *noise.Perlin, aspect float64, paper palette.Color, ramp []palette.Color) dress {
	// Colour comes from a low-frequency field sampled at the cell's own
	// centroid, so the sheet has *passages* — a green corner, a brown
	// corner — the way the reference does. Drawing each cell's colour freely
	// gives every pigment equal presence everywhere, which reads as
	// confetti; the accents are what stop the passages becoming zones.
	// The field's swing is nowhere near ±1, so it is stretched before it is
	// read: sampled raw, every centroid lands in the middle of the ramp and
	// the sheet comes out in two colours out of nine.
	t := mathx.Clamp01(0.5 + 1.5*field.FBM(c.CX/s.Passage, c.CY/s.Passage, 3))
	idx := min(int(t*float64(len(ramp))), len(ramp)-1)
	if rng.Float64() < s.Accent {
		idx = rng.IntN(len(ramp))
	}
	base := ramp[idx]

	d := dress{
		pigment: base,
		deep:    shade(base, 0.74),
		light:   palette.Lerp(paper, base, 0.62),
		press:   rnd.Uniform(rng, 0.72, 1.0),
	}
	// Hatching runs at one angle per cell. Free angles read as noise; the
	// reference's hand keeps to a handful of directions per sheet, so the
	// angle is quantised to a twelfth of a turn.
	a := float64(rng.IntN(12)) * math.Pi / 6
	d.cos, d.sin = math.Cos(a), math.Sin(a)
	d.stroke = s.Stroke * rnd.Uniform(rng, 0.8, 1.4)
	// Bands are spaced off the cell's own inradius so that a small cell gets
	// two or three rings rather than a moiré, and a large one gets rings
	// wide enough to read as rings.
	d.pitch = math.Max(c.Inradius/float64(2+rng.IntN(3)), s.Stroke)

	d.style = rnd.PickIndex(rng, l.styles[:])
	// A pattern needs room. Below a few pitches of inradius, bands and
	// hatching both collapse into a flat dark smear, which is a worse mark
	// than a plain wash — and the slivers between big lobes are exactly
	// where the packing puts the smallest cells.
	if (d.style == styleBands || d.style == styleHatch) && c.Inradius < 3*s.Stroke {
		d.style = styleWash
	}
	// And bands need a cell that is roughly round. The wall-distance field
	// ridges along a cell's medial axis, and in a long or forked cell it is
	// those ridges the rings trace: instead of concentric rings the cell
	// fills with a tangle of pinched loops, which at print size reads as a
	// carved mask. Roundness is 1 for a disc and climbs with elongation, so
	// this is a cheap shape test the measured cell already pays for, and it
	// is the reason cells.Cell carries an inradius at all.
	if d.style == styleBands && roundness(c, aspect) > 1.8 {
		d.style = styleWash
	}
	return d
}

// roundness is a cell's area against the area of the disc of its own
// inscribed radius: 1 for a disc, larger the longer or more forked it is.
func roundness(c cells.Cell, aspect float64) float64 {
	if c.Inradius <= 0 {
		return math.Inf(1)
	}
	return c.Area * aspect / (math.Pi * c.Inradius * c.Inradius)
}

// fill paints one point inside its cell.
func (s *Sketch) fill(d dress, h cells.Hit, field *noise.Perlin, seed uint64, u, v float64, paper palette.Color) palette.Color {
	var col palette.Color
	switch d.style {
	case stylePencil:
		col = s.pencil(d, h, field, u, v, paper)
	case styleBands:
		col = s.bands(d, h, u, v, field)
	case styleHatch:
		col = s.hatch(d, h, field, u, v, paper)
	case styleEmpty:
		col = paper
	default:
		col = s.wash(d, h, field, u, v)
	}
	return shade(col, 1+s.grain(seed, u, v))
}

// wash is transparent pigment that dried in place: uneven at a broad scale
// where the water pooled, and denser in the last stretch before the wall,
// which is where a drying pool leaves its pigment.
func (s *Sketch) wash(d dress, h cells.Hit, field *noise.Perlin, u, v float64) palette.Color {
	col := shade(d.pigment, 1+s.Blotch*field.FBM(u/s.Mottle, v/s.Mottle, 3))
	rim := 1 - mathx.Smoothstep(0, s.RimWide, h.Wall)
	return palette.Lerp(col, d.deep, rim*s.Rim)
}

// pencil is directional strokes laid over the paper, wandering slightly so
// the hatching is not mechanical, and pressed harder as it approaches the
// wall — which is what a hand does when it is filling up to a line.
func (s *Sketch) pencil(d dress, h cells.Hit, field *noise.Perlin, u, v float64, paper palette.Color) palette.Color {
	// Nearly full coverage, with the paper showing only in the grooves
	// between strokes. The reference's pencil work is a *filled* cell that
	// happens to have been filled by hand — read as open striping it becomes
	// a swatch of pattern, which is what the hatch style is for.
	cover := s.strokes(d, field, u, v, d.stroke, 0.62, 1.0)
	press := d.press * (0.86 + 0.14*field.At(u/0.05, v/0.05))
	edge := (1 - mathx.Smoothstep(0, s.RimWide*1.6, h.Wall)) * 0.25
	return palette.Lerp(paper, d.pigment, mathx.Clamp01(cover*press+edge))
}

// hatch is the same gesture with the pencil barely touching: thin lines,
// wide apart, the paper doing most of the work.
func (s *Sketch) hatch(d dress, h cells.Hit, field *noise.Perlin, u, v float64, paper palette.Color) palette.Color {
	cover := s.strokes(d, field, u, v, d.stroke*3.4, 0.06, 0.28)
	edge := (1 - mathx.Smoothstep(0, s.RimWide, h.Wall)) * 0.18
	return palette.Lerp(paper, d.deep, mathx.Clamp01(cover*0.85+edge))
}

// strokes is the coverage of a set of parallel strokes at the cell's angle:
// 1 along a stroke's spine, 0 in the paper between them. lo and hi are how
// much of the pitch the stroke covers and how softly it fades out.
func (s *Sketch) strokes(d dress, field *noise.Perlin, u, v, pitch, lo, hi float64) float64 {
	// Across the strokes, warped so no two runs are quite parallel. Without
	// this the fill is a printed screen rather than a hand.
	w := u*d.cos + v*d.sin
	w += pitch * s.Wobble * field.At(u/0.11, v/0.11)
	e := math.Abs(frac(w/pitch)-0.5) * 2
	return 1 - mathx.Smoothstep(lo, hi, e)
}

// bands are rings spaced off the *wall distance*, which is what makes them
// follow the cell's own outline whatever shape it is: in a crescent they
// bend around the waist, because the field they are cut from does.
func (s *Sketch) bands(d dress, h cells.Hit, u, v float64, field *noise.Perlin) palette.Color {
	if math.IsInf(h.Wall, 1) {
		return d.light
	}
	// A little wander so the rings are drawn rather than turned.
	w := h.Wall + d.pitch*0.12*field.At(u/0.08, v/0.08)
	e := math.Abs(frac(w/d.pitch)-0.5) * 2
	return palette.Lerp(d.light, d.deep, mathx.Smoothstep(0.3, 0.72, e))
}

// lay puts the ink line over whatever the cell was filled with.
//
// The taper and the swollen junction are one rule, not two: the half-width
// grows with Node, which is 1 where three cells meet and 0 halfway along a
// wall. That is why the fillets between three cells come out concave — Node
// falls off smoothly, so the line's edge is a curve.
func (s *Sketch) lay(col palette.Color, h cells.Hit, l levels, field *noise.Perlin, seed uint64, ink palette.Color, u, v float64) palette.Color {
	if math.IsInf(h.Wall, 1) {
		return col
	}
	half := l.ink / 2 * (1 + l.swell*h.Node)
	// The wander of a hand holding a pencil. In canvas units, so preview and
	// print wobble identically (invariant 2).
	half *= 1 + s.Wobble*field.At(u/0.035, v/0.035)
	// The softness of the edge is a fraction of the *unswollen* line, not of
	// the local half-width. Tied to the half-width it grows with the
	// swelling, so exactly where the line is heaviest — the junctions, the
	// part of the drawing that has to read as a knot of three walls — it
	// dissolves into a brown halo.
	soft := math.Max(l.ink*0.16, 0.0004)
	a := 1 - mathx.Smoothstep(half-soft, half+soft, h.Wall)
	if a <= 0 {
		return col
	}
	// Graphite is not solid: the tooth of the paper shows through the line
	// as well as through the fills.
	a *= mathx.Clamp01(0.92 + 2*s.grain(seed, u, v))
	return palette.Lerp(col, ink, a)
}

// grain is the paper's tooth: a signed nudge from a hash lattice, salted so
// the texture does not move when the cells do.
func (s *Sketch) grain(seed uint64, u, v float64) float64 {
	ix, iy := int64(math.Floor(u/tooth)), int64(math.Floor(v/tooth))
	return (noise.Hash01(seed^saltPaper, ix, iy) - 0.5) * s.Grain
}

func frac(x float64) float64 { return x - math.Floor(x) }
