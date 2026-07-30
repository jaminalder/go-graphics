// Package paint provides a stamp-based painting model for human-like
// marks: a float canvas painted with soft dabs and stroked paths, plus
// wobbly path generators and reusable disc marks (rings, scribble,
// gouache). Painting is sequential and order-dependent — a second
// rendering model beside render's pure per-pixel functions. Coordinates
// are canvas units (height = 1), so compositions stay resolution
// independent; anti-aliasing comes from the soft dab edges.
package paint

import (
	"image"
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
	"github.com/jaminalder/go-graphics/internal/render"
)

// Pt is a point in canvas units.
type Pt struct {
	X, Y float64
}

// Canvas is a float color buffer painted by stamping.
type Canvas struct {
	w, h  int
	scale float64 // pixels per canvas unit
	pix   []palette.Color
}

// NewCanvas creates a w×h pixel canvas filled with bg.
func NewCanvas(w, h int, bg palette.Color) *Canvas {
	c := &Canvas{w: w, h: h, scale: float64(h), pix: make([]palette.Color, w*h)}
	for i := range c.pix {
		c.pix[i] = bg
	}
	return c
}

// Scale is the canvas resolution in pixels per canvas unit. Compositions
// are written in canvas units and must stay resolution independent, so use
// it only to decide what is worth drawing at all — how finely to subdivide
// a mark, whether a detail can resolve — never to size one.
func (c *Canvas) Scale() float64 { return c.scale }

// Dab stamps a soft-edged disc of radius r at (u, v), blended source-over
// with the given alpha.
func (c *Canvas) Dab(u, v, r float64, col palette.Color, alpha float64) {
	px, py, pr := u*c.scale, v*c.scale, r*c.scale
	x0 := max(int(px-pr-1), 0)
	x1 := min(int(px+pr+1), c.w-1)
	y0 := max(int(py-pr-1), 0)
	y1 := min(int(py+pr+1), c.h-1)
	for y := y0; y <= y1; y++ {
		dy := float64(y) + 0.5 - py
		for x := x0; x <= x1; x++ {
			dx := float64(x) + 0.5 - px
			d := math.Sqrt(dx*dx + dy*dy)
			a := alpha * mathx.Clamp01(pr-d+0.5) // ~1px soft edge
			if a <= 0 {
				continue
			}
			i := y*c.w + x
			p := c.pix[i]
			c.pix[i] = palette.Color{
				R: p.R*(1-a) + col.R*a,
				G: p.G*(1-a) + col.G*a,
				B: p.B*(1-a) + col.B*a,
			}
		}
	}
}

// Stroke paints a dab train along the polyline: a line of the given width
// with soft edges. Dab spacing adapts to the output resolution, so
// strokes are solid at any size.
func (c *Canvas) Stroke(pts []Pt, width float64, col palette.Color, alpha float64) {
	if len(pts) == 0 {
		return
	}
	r := width / 2
	step := math.Max(r*0.4, 0.7/c.scale)
	c.Dab(pts[0].X, pts[0].Y, r, col, alpha)
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
			c.Dab(ax+(bx-ax)*t, ay+(by-ay)*t, r, col, alpha)
		}
		carry = math.Mod(carry+segLen, step)
	}
}

// Image quantizes the canvas to a dithered 8-bit image.
func (c *Canvas) Image() *image.NRGBA {
	return render.ImageFromColors(c.w, c.h, c.pix)
}
