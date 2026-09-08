package gradient

import (
	"math"
	"testing"

	"github.com/jaminalder/go-graphics/internal/palette"
)

func TestThroughHSLHitsTheStops(t *testing.T) {
	a := palette.MustHex("#4F51FE")
	b := palette.MustHex("#FF4E0B")
	g := ThroughHSL([]palette.Color{a, b})
	if got := g.At(0); !colorsClose(got, a) {
		t.Errorf("At(0) = %v, want %v", got, a)
	}
	if got := g.At(1); !colorsClose(got, b) {
		t.Errorf("At(1) = %v, want %v", got, b)
	}
}

func TestThroughHSLSingleColourIsConstant(t *testing.T) {
	c := palette.MustHex("#CD2019")
	g := ThroughHSL([]palette.Color{c})
	if g.At(0) != g.At(0.5) || g.At(0.5) != g.At(1) {
		t.Error("a one-stop gradient must be constant")
	}
}

func TestThroughRGBMidpointIsTheChannelMean(t *testing.T) {
	a := palette.Color{R: 0, G: 0, B: 1}
	b := palette.Color{R: 1, G: 1, B: 1}
	g := ThroughRGB([]palette.Color{a, b})
	mid := g.At(0.5)
	if math.Abs(mid.R-0.5) > 1e-9 || math.Abs(mid.G-0.5) > 1e-9 || math.Abs(mid.B-1) > 1e-9 {
		t.Fatalf("RGB midpoint %v, want (0.5, 0.5, 1)", mid)
	}
}

func TestThroughRGBCyanWhiteGoldDoesNotGoGreen(t *testing.T) {
	// The HSL short path from cyan to gold is green. Flames must not take it.
	g := ThroughRGB([]palette.Color{
		palette.MustHex("#5EB0D0"),
		palette.MustHex("#F0F0F0"),
		palette.MustHex("#F0D070"),
	})
	for i := 0; i <= 20; i++ {
		c := g.At(float64(i) / 20)
		if c.G > c.R+0.08 && c.G > c.B+0.08 && c.G > 0.4 {
			t.Fatalf("t=%g went green: %v", float64(i)/20, c)
		}
	}
}
