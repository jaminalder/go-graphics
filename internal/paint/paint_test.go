package paint

import (
	"bytes"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

var (
	white = palette.Color{R: 1, G: 1, B: 1}
	red   = palette.Color{R: 1}
)

func TestDabCoverageAndBlend(t *testing.T) {
	c := NewCanvas(64, 64, white)
	c.Dab(0.5, 0.5, 0.2, red, 1)
	// Center fully red, far corner untouched.
	center := c.pix[32*64+32]
	if math.Abs(center.R-1) > 1e-9 || center.G > 1e-9 {
		t.Errorf("center = %v, want pure red", center)
	}
	if corner := c.pix[0]; corner != white {
		t.Errorf("corner = %v, want untouched white", corner)
	}
	// Half alpha blends halfway.
	c2 := NewCanvas(8, 8, white)
	c2.Dab(0.5, 0.5, 0.4, red, 0.5)
	mid := c2.pix[4*8+4]
	if math.Abs(mid.G-0.5) > 0.02 {
		t.Errorf("half-alpha green channel = %v, want ≈0.5", mid.G)
	}
}

func TestStrokeIsSolid(t *testing.T) {
	c := NewCanvas(100, 100, white)
	c.Stroke([]Pt{{0.1, 0.5}, {0.9, 0.5}}, 0.06, red, 1)
	// Every pixel along the center line must be fully painted (no gaps
	// between dabs).
	for x := 15; x < 85; x++ {
		p := c.pix[50*100+x]
		if p.G > 0.02 {
			t.Fatalf("gap in stroke at x=%d: %v", x, p)
		}
	}
}

func TestSpiralDeterministicAndBounded(t *testing.T) {
	a := Spiral(rand.New(rand.NewPCG(1, 2)), 0.5, 0.5, 0.05, 0.2, 4, 0.01)
	b := Spiral(rand.New(rand.NewPCG(1, 2)), 0.5, 0.5, 0.05, 0.2, 4, 0.01)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same rng seed must give the same path")
		}
	}
	for _, p := range a {
		d := math.Hypot(p.X-0.5, p.Y-0.5)
		if d > 0.2+0.1 {
			t.Fatalf("spiral point %v far outside target radius", p)
		}
	}
}

func TestMarksDeterministic(t *testing.T) {
	paintAll := func() *Canvas {
		c := NewCanvas(96, 96, white)
		RingsDisc(c, rand.New(rand.NewPCG(3, 0)), 0.3, 0.3, 0.2, palette.Color{B: 0.5}, red)
		ScribbleDisc(c, rand.New(rand.NewPCG(4, 0)), 0.7, 0.3, 0.2, palette.Color{B: 0.5}, red)
		GouacheDisc(c, rand.New(rand.NewPCG(5, 0)), 0.5, 0.7, 0.2, red, palette.Color{B: 0.5})
		return c
	}
	a, b := paintAll().Image(), paintAll().Image()
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Error("marks are not deterministic")
	}
	// And they actually painted something.
	blank := NewCanvas(96, 96, white).Image()
	if bytes.Equal(a.Pix, blank.Pix) {
		t.Error("marks painted nothing")
	}
}
