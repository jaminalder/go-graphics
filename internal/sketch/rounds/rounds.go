// Package rounds implements sketch 005: a brush prototype — a regular
// grid of large circles (donuts, groove discs, targets), one color per
// row on a dark ground, with a single "humanization" dial from
// machine-perfect flat shapes (0) to full dry-brush painting (1)
// (spec: docs/sketches/005-rounds.md).
package rounds

import (
	"image"
	"math"
	"math/rand/v2"
	"sort"

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
	Human      float64 // 0 = machine-perfect, 1 = full dry-brush

	opts cliOptions
}

// New returns the sketch with its defaults.
func New() *Sketch {
	return &Sketch{Rows: 5, Cols: 4, Human: 0.8}
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

	byLum := append([]palette.Color(nil), ctx.Palette.Colors...)
	sort.SliceStable(byLum, func(i, j int) bool {
		return byLum[i].Luminance() < byLum[j].Luminance()
	})
	bg := byLum[0]
	inks := byLum[1:]

	cv := paint.NewCanvas(ctx.Width, ctx.Height, bg)
	rng := ctx.RNG(streamPaint)

	sy := 1.0 / float64(s.Rows)
	sx := aspect / float64(s.Cols)
	r := 0.455 * math.Min(sx, sy)

	for row := 0; row < s.Rows; row++ {
		c1 := inks[row%len(inks)]
		c2 := inks[(row+1)%len(inks)]
		f := rowForms[row%len(rowForms)]
		for col := 0; col < s.Cols; col++ {
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

// donut paints a thick ring: one wide circular stroke plus dry-brush
// groove streaks in the background color.
func (s *Sketch) donut(cv *paint.Canvas, rng *rand.Rand, cx, cy, r float64, col, bg palette.Color, h float64) {
	cv.Stroke(paint.Loop(rng, cx, cy, r*0.73, h*r*0.05), r*0.54, col, 0.97)
	s.streaks(cv, rng, cx, cy, r, 0.5, 0.99, bg, h, int(h*15+0.5))
}

// grooves paints a solid disc engraved with concentric background-color
// rings all the way to the center.
func (s *Sketch) grooves(cv *paint.Canvas, rng *rand.Rand, cx, cy, r float64, col, bg palette.Color, h float64) {
	paint.FillDisc(cv, rng, cx, cy, r, col, 0.97, h*r*0.04)
	const k = 9
	for i := 1; i <= k; i++ {
		rr := r * (float64(i) - 0.15) / float64(k)
		ox := (rng.Float64() - 0.5) * h * r * 0.06
		oy := (rng.Float64() - 0.5) * h * r * 0.06
		cv.Stroke(paint.Loop(rng, cx+ox, cy+oy, rr, h*rr*0.09),
			r*0.018, bg, 0.55+0.35*rng.Float64())
	}
	s.streaks(cv, rng, cx, cy, r, 0.1, 0.99, bg, h, int(h*10+0.5))
	cv.Dab(cx, cy, r*0.035, bg, 0.9)
}

// target paints alternating color bands from the rim to the center.
func (s *Sketch) target(cv *paint.Canvas, rng *rand.Rand, cx, cy, r float64, c1, c2, bg palette.Color, h float64) {
	const k = 8
	bw := r / k
	for i := 0; i < k; i++ {
		rm := r - (float64(i)+0.5)*bw
		col := c1
		if i%2 == 1 {
			col = c2
		}
		ox := (rng.Float64() - 0.5) * h * r * 0.04
		oy := (rng.Float64() - 0.5) * h * r * 0.04
		cv.Stroke(paint.Loop(rng, cx+ox, cy+oy, rm, h*rm*0.08), bw*1.2, col, 0.96)
	}
	cv.Dab(cx, cy, bw*0.8, c1, 0.96)
	s.streaks(cv, rng, cx, cy, r, 0.15, 0.99, bg, h, int(h*12+0.5))
}

// streaks lays thin dry-brush lines in the background color at random
// radii — the "grooves" that make a mark read as hand-painted.
func (s *Sketch) streaks(cv *paint.Canvas, rng *rand.Rand, cx, cy, r, lo, hi float64, bg palette.Color, h float64, n int) {
	for range n {
		sr := (lo + (hi-lo)*rng.Float64()) * r
		ox := (rng.Float64() - 0.5) * h * r * 0.04
		oy := (rng.Float64() - 0.5) * h * r * 0.04
		cv.Stroke(paint.Loop(rng, cx+ox, cy+oy, sr, h*sr*0.06),
			r*(0.008+0.014*rng.Float64()), bg, 0.2+0.4*rng.Float64())
	}
}
