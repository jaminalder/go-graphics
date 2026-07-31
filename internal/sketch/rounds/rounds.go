// Package rounds implements sketch 005: a brush prototype — a regular
// grid of large circles (donuts, groove discs, targets), one color per
// row on a dark ground, with a single "humanization" dial from
// machine-perfect flat shapes (0) to a dry, streaky coat (1)
// (spec: docs/sketches/005-rounds.md).
//
// Every shape here is laid down by sweeping a bristle brush along
// concentric rings, so the texture is the record of the brush passing
// over the shape rather than marks drawn on top of it.
package rounds

import (
	"image"
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/opt"
	"github.com/jaminalder/go-graphics/internal/paint"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/sketch"
)

const streamPaint = 1

// form is a circle drawing style; rows cycle through the forms.
type form uint8

const (
	formDonut form = iota
	formGrooves
	formTarget
)

var rowForms = [5]form{formDonut, formGrooves, formDonut, formTarget, formDonut}

// Sketch holds the structural knobs.
type Sketch struct {
	Rows, Cols int
	Human      float64 // 0 = machine-perfect, 1 = dry and streaky

	knobs *opt.Set
}

// New returns the sketch with its defaults.
func New() *Sketch {
	s := &Sketch{Rows: 5, Cols: 4, Human: 0.8}
	s.declare()
	return s
}

// Name implements sketch.Sketch.
func (s *Sketch) Name() string { return "rounds" }

// Describe implements sketch.Sketch.
func (s *Sketch) Describe() string {
	return "brush prototype: grid of donut/groove/target circles with a humanization dial"
}

// Render implements sketch.Sketch. Stamp-painted: Context.AA/Deep unused.
func (s *Sketch) Render(ctx sketch.Context) (image.Image, error) {
	aspect := float64(ctx.Width) / float64(ctx.Height)
	h := s.Human

	byLum := palette.ByLuminance(ctx.Palette.Colors)
	bg := byLum[0]
	inks := byLum[1:]

	cv := paint.NewCanvas(ctx.Width, ctx.Height, bg)
	rng := ctx.RNG(streamPaint)

	sy := 1.0 / float64(s.Rows)
	sx := aspect / float64(s.Cols)
	r := 0.455 * math.Min(sx, sy)

	for row := range s.Rows {
		c1 := inks[row%len(inks)]
		c2 := inks[(row+1)%len(inks)]
		f := rowForms[row%len(rowForms)]
		for col := range s.Cols {
			cx := (float64(col) + 0.5) * sx
			cy := (float64(row) + 0.5) * sy
			// Hand placement: centers drift a touch off the grid.
			cx += (rng.Float64() - 0.5) * h * 0.012
			cy += (rng.Float64() - 0.5) * h * 0.012
			switch f {
			case formGrooves:
				s.grooves(cv, rng, cx, cy, r, c1, bg, h)
			case formTarget:
				s.target(cv, rng, cx, cy, r, c1, c2, bg, h)
			default:
				s.donut(cv, rng, cx, cy, r, c1, bg, h)
			}
		}
	}
	return cv.Image(), nil
}

// coat returns the wide brush that lays colour, sized to the shape. It
// runs nearly wet: the body of a shape wants to be flat, and a coat that
// leaves gaps everywhere reads as grit rather than as paint. What little
// dryness it has frays the silhouette.
func coat(rng *rand.Rand, r, h float64) paint.Brush {
	return paint.NewBrush(rng, r*0.3, 13, 0.35*h)
}

// liner returns the fine, dry brush that cuts dark arcs back into a
// finished coat. Running it dry is what breaks each arc into segments
// that fade in and out along their length.
func liner(rng *rand.Rand, r, h float64) paint.Brush {
	return paint.NewBrush(rng, r*0.03, 2, 0.2+0.5*h).Grain(rng, r*0.9)
}

// wob is the per-pass radius deviation as a fraction of radius. Kept
// small on purpose: the shapes stay recognisably circular and the
// disorder comes from the brush, not from warping the geometry.
func wob(h float64) float64 { return 0.012 * h }

// donut paints a thick ring: a coat swept around the annulus, then a few
// darker rings sunk into it near the rims where a brush loaded up.
func (s *Sketch) donut(cv *paint.Canvas, rng *rand.Rand, cx, cy, r float64, col, bg palette.Color, h float64) {
	paint.SweepRing(cv, rng, coat(rng, r, h), cx, cy, r*0.45, r, col, 0.97, wob(h))
	s.scuff(cv, rng, cx, cy, r, 0.47, 0.99, bg, h, int(7*h+0.5))
}

// grooves paints a solid disc engraved with concentric background-color
// rings all the way to the center.
func (s *Sketch) grooves(cv *paint.Canvas, rng *rand.Rand, cx, cy, r float64, col, bg palette.Color, h float64) {
	paint.SweepRing(cv, rng, coat(rng, r, h), cx, cy, 0, r, col, 0.97, wob(h))
	// The engraved rings are the design here, so they are drawn wetter
	// and closed; the dry scuff on top is what keeps them from looking
	// printed.
	const k = 8
	for i := 1; i <= k; i++ {
		rr := r * (float64(i) - 0.15) / float64(k)
		paint.NewBrush(rng, r*0.035, 2, 0.3*h).Grain(rng, r*1.2).
			Stroke(cv, paint.Ring(rng, cx, cy, rr, 1.05, wob(h)*rr), bg, 0.9)
	}
	cv.Dab(cx, cy, r*0.03, bg, 0.9)
	s.scuff(cv, rng, cx, cy, r, 0.1, 0.97, bg, h, int(6*h+0.5))
}

// target paints alternating color bands from the rim to the center, each
// band its own swept coat so the seams between them stay painterly.
func (s *Sketch) target(cv *paint.Canvas, rng *rand.Rand, cx, cy, r float64, c1, c2, bg palette.Color, h float64) {
	const k = 6
	bw := r / k
	for i := range k {
		col := c1
		if i%2 == 1 {
			col = c2
		}
		rOut := r - float64(i)*bw
		b := paint.NewBrush(rng, math.Min(bw*0.85, r*0.3), 9, 0.35*h)
		paint.SweepRing(cv, rng, b, cx, cy, rOut-bw, rOut, col, 0.97, wob(h))
	}
	s.scuff(cv, rng, cx, cy, r, 0.15, 0.97, bg, h, int(7*h+0.5))
}

// scuff drags a dry liner around n arcs at random radii between lo·r and
// hi·r, cutting back to the ground where the brush ran thin.
//
// This is the whole texture of a shape, and the reason it reads as a
// smear rather than a scribble is that every arc belongs to the same
// family of concentric circles the coat was swept along. Parallel marks
// that never cross each other are what the eye reads as one brush
// passing over a surface; marks at differing angles read as a second
// hand drawing on top. Arcs cluster toward the rims, where a real brush
// loads up and drags as it turns.
func (s *Sketch) scuff(cv *paint.Canvas, rng *rand.Rand, cx, cy, r, lo, hi float64, bg palette.Color, h float64, n int) {
	for range n {
		// Bias toward the band edges, but only half way — a full
		// smoothstep packs every arc into two dense rims and leaves an
		// obviously empty middle.
		t := rng.Float64()
		t += 0.5 * (t*t*(3-2*t) - t)
		rr := (lo + (hi-lo)*t) * r
		arc := 0.35 + 0.75*rng.Float64()
		liner(rng, r, h).
			Stroke(cv, paint.Ring(rng, cx, cy, rr, arc, wob(h)*rr), bg, 0.16+0.26*rng.Float64())
	}
}
