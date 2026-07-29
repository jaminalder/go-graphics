package paint

import (
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

var black = palette.Color{}

// coverage sums how far every pixel moved from the background toward the
// ink colour — the painted area in pixels.
func (c *Canvas) coverage() float64 {
	var sum float64
	for _, p := range c.pix {
		sum += 1 - p.R
	}
	return sum
}

func (c *Canvas) at(x, y int) palette.Color { return c.pix[y*c.w+x] }

func TestRingCoverageMatchesItsArea(t *testing.T) {
	tests := []struct {
		name         string
		r, thickness float64
		tolerance    float64
	}{
		{"thin ring", 0.2, 0.01, 0.08},
		{"thick ring", 0.25, 0.06, 0.05},
		{"hairline", 0.3, 0.002, 0.15},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCanvas(400, 400, white)
			c.Ring(0.5, 0.5, tc.r, tc.r, tc.thickness, black, 1)

			scale := float64(400)
			want := 2 * math.Pi * tc.r * scale * tc.thickness * scale
			got := c.coverage()
			if math.Abs(got-want)/want > tc.tolerance {
				t.Errorf("coverage = %.1f px, want ≈%.1f px (%.1f%% off)",
					got, want, 100*math.Abs(got-want)/want)
			}
		})
	}
}

func TestRingLeavesItsHoleEmpty(t *testing.T) {
	c := NewCanvas(200, 200, white)
	c.Ring(0.5, 0.5, 0.3, 0.3, 0.02, black, 1)
	if got := c.at(100, 100); got.R != 1 {
		t.Errorf("centre pixel = %+v, want untouched white", got)
	}
	// Just inside the inner edge should also be clean.
	if got := c.at(100+int(0.3*200)-8, 100); got.R != 1 {
		t.Errorf("pixel inside the hole = %+v, want untouched white", got)
	}
}

// Once the band is as wide as the diameter the hole closes; the result is a
// filled disc of radius r + thickness/2. QQL leans on this for its smallest
// dots, where the innermost band swallows the centre.
func TestFatRingFillsTheDisc(t *testing.T) {
	c := NewCanvas(200, 200, white)
	r := 0.2
	c.Ring(0.5, 0.5, r, r, 2*r, black, 1)
	if got := c.at(100, 100); got.R > 0.001 {
		t.Errorf("centre pixel = %+v, want filled", got)
	}
	outer := 2 * r
	want := math.Pi * outer * 200 * outer * 200
	if got := c.coverage(); math.Abs(got-want)/want > 0.05 {
		t.Errorf("coverage = %.1f px, want ≈%.1f px (a filled disc)", got, want)
	}
}

// A ring thinner than a pixel must still deposit colour, proportionally —
// QQL draws a great many of them at preview size.
func TestSubPixelRingStillPaints(t *testing.T) {
	c := NewCanvas(200, 200, white)
	thickness := 0.002 // 0.4 px
	c.Ring(0.5, 0.5, 0.3, 0.3, thickness, black, 1)
	got := c.coverage()
	want := 2 * math.Pi * 0.3 * 200 * thickness * 200
	if got <= 0 {
		t.Fatal("nothing painted")
	}
	if math.Abs(got-want)/want > 0.2 {
		t.Errorf("coverage = %.2f px, want ≈%.2f px", got, want)
	}
}

func TestEllipticalRing(t *testing.T) {
	c := NewCanvas(400, 400, white)
	rx, ry, thickness := 0.3, 0.15, 0.01
	c.Ring(0.5, 0.5, rx, ry, thickness, black, 1)

	// Ramanujan's approximation of an ellipse's perimeter.
	a, b := rx*400, ry*400
	h := (a - b) * (a - b) / ((a + b) * (a + b))
	perim := math.Pi * (a + b) * (1 + 3*h/(10+math.Sqrt(4-3*h)))
	want := perim * thickness * 400
	if got := c.coverage(); math.Abs(got-want)/want > 0.1 {
		t.Errorf("coverage = %.1f px, want ≈%.1f px", got, want)
	}

	// The outline must pass through the semi-axis endpoints and nowhere near
	// the point where a circle of radius rx would have been.
	if got := c.at(200, 200-int(ry*400)); got.R > 0.5 {
		t.Error("no ink at the top of the minor axis")
	}
	if got := c.at(200, 200-int(rx*400)); got.R < 0.99 {
		t.Error("ink found where only a circular ring would reach")
	}
}

func TestRingClipsToTheCanvas(t *testing.T) {
	c := NewCanvas(100, 100, white)
	c.Ring(-0.2, -0.2, 0.3, 0.3, 0.05, black, 1) // mostly off-canvas
	c.Ring(1.5, 1.5, 0.4, 0.4, 0.05, black, 1)   // entirely off-canvas
	if c.coverage() <= 0 {
		t.Error("the partly visible ring painted nothing")
	}
}

func TestRingRespectsAlpha(t *testing.T) {
	full := NewCanvas(200, 200, white)
	full.Ring(0.5, 0.5, 0.3, 0.3, 0.04, black, 1)
	half := NewCanvas(200, 200, white)
	half.Ring(0.5, 0.5, 0.3, 0.3, 0.04, black, 0.5)

	ratio := half.coverage() / full.coverage()
	if math.Abs(ratio-0.5) > 0.02 {
		t.Errorf("half-alpha coverage ratio = %.3f, want ≈0.5", ratio)
	}
}

func TestRingIgnoresDegenerateInput(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		rx, ry, thickness, alph float64
	}{
		{"zero radius", 0, 0, 0.01, 1},
		{"negative radius", -0.1, -0.1, 0.01, 1},
		{"zero thickness", 0.2, 0.2, 0, 1},
		{"zero alpha", 0.2, 0.2, 0.01, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewCanvas(50, 50, white)
			c.Ring(0.5, 0.5, tc.rx, tc.ry, tc.thickness, black, tc.alph)
			if c.coverage() != 0 {
				t.Errorf("painted %v px, want nothing", c.coverage())
			}
		})
	}
}
