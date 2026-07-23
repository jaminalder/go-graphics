package palette

import (
	"image/color"
	"math"
	"testing"
)

func almostEqual(a, b Color) bool {
	const eps = 1e-9
	return math.Abs(a.R-b.R) < eps && math.Abs(a.G-b.G) < eps && math.Abs(a.B-b.B) < eps
}

func TestHexRoundTrip(t *testing.T) {
	for _, s := range []string{"#000000", "#FFFFFF", "#ED6A5A", "#5CA4A9", "#0F185B"} {
		c, err := Hex(s)
		if err != nil {
			t.Fatalf("Hex(%q): %v", s, err)
		}
		if got := c.Hex(); got != s {
			t.Errorf("Hex(%q).Hex() = %q", s, got)
		}
	}
}

func TestHexInvalid(t *testing.T) {
	for _, s := range []string{"", "#FFF", "ED6A5A", "#GGGGGG", "#ED6A5A00"} {
		if _, err := Hex(s); err == nil {
			t.Errorf("Hex(%q): expected error", s)
		}
	}
}

func TestNRGBA(t *testing.T) {
	tests := []struct {
		in   Color
		want color.NRGBA
	}{
		{Color{0, 0, 0}, color.NRGBA{0, 0, 0, 255}},
		{Color{1, 1, 1}, color.NRGBA{255, 255, 255, 255}},
		{Color{-0.5, 0.5, 1.5}, color.NRGBA{0, 128, 255, 255}}, // clamps
	}
	for _, tt := range tests {
		if got := tt.in.NRGBA(); got != tt.want {
			t.Errorf("%v.NRGBA() = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLerp(t *testing.T) {
	a, b := Color{0, 0, 0}, Color{1, 0.5, 0}
	if got := Lerp(a, b, 0); !almostEqual(got, a) {
		t.Errorf("Lerp t=0 = %v, want %v", got, a)
	}
	if got := Lerp(a, b, 1); !almostEqual(got, b) {
		t.Errorf("Lerp t=1 = %v, want %v", got, b)
	}
	mid := Color{0.5, 0.25, 0}
	if got := Lerp(a, b, 0.5); !almostEqual(got, mid) {
		t.Errorf("Lerp t=0.5 = %v, want %v", got, mid)
	}
}

func TestSRGBTransferRoundTrip(t *testing.T) {
	for _, v := range []float64{0, 0.001, 0.04, 0.2, 0.5, 0.7354, 1} {
		back := LinearToSRGB(SRGBToLinear(v))
		if math.Abs(back-v) > 1e-9 {
			t.Errorf("round trip %v -> %v", v, back)
		}
	}
	// 50% linear gray encodes to ~0.7354 sRGB.
	if got := LinearToSRGB(0.5); math.Abs(got-0.7354) > 5e-4 {
		t.Errorf("LinearToSRGB(0.5) = %v, want ≈0.7354", got)
	}
}

func TestLuminance(t *testing.T) {
	if got := (Color{1, 1, 1}).Luminance(); math.Abs(got-1) > 1e-9 {
		t.Errorf("white luminance = %v, want 1", got)
	}
	if got := (Color{}).Luminance(); got != 0 {
		t.Errorf("black luminance = %v, want 0", got)
	}
	g := (Color{G: 1}).Luminance()
	r := (Color{R: 1}).Luminance()
	b := (Color{B: 1}).Luminance()
	if !(g > r && r > b) {
		t.Errorf("expected G > R > B, got %v %v %v", g, r, b)
	}
}

func TestHSLRoundTrip(t *testing.T) {
	for _, s := range []string{"#ED6A5A", "#F4F1BB", "#9BC1BC", "#123456", "#808080"} {
		c := MustHex(s)
		h, sat, l := c.hsl()
		back := fromHSL(h, sat, l)
		const eps = 1e-6
		if math.Abs(back.R-c.R) > eps || math.Abs(back.G-c.G) > eps || math.Abs(back.B-c.B) > eps {
			t.Errorf("HSL round trip %s: got %v, want %v", s, back, c)
		}
	}
}

func TestLerpHSL(t *testing.T) {
	red := Color{R: 1}
	blue := Color{B: 1}
	if got := LerpHSL(red, blue, 0); !almostEqual(got, red) {
		t.Errorf("t=0 = %v, want %v", got, red)
	}
	if got := LerpHSL(red, blue, 1); !almostEqual(got, blue) {
		t.Errorf("t=1 = %v, want %v", got, blue)
	}
	// Shortest hue arc from red (0°) to blue (240°) passes through magenta
	// (300°), and the blend keeps full saturation — no graying out.
	h, s, _ := LerpHSL(red, blue, 0.5).HSL()
	if math.Abs(h-300) > 1e-6 || math.Abs(s-1) > 1e-6 {
		t.Errorf("midpoint = hue %v sat %v, want hue 300 sat 1", h, s)
	}
	// Achromatic endpoint adopts the other's hue: gray→red stays red-hued.
	h, _, _ = LerpHSL(Color{0.5, 0.5, 0.5}, red, 0.5).HSL()
	if h != 0 {
		t.Errorf("gray→red midpoint hue = %v, want 0", h)
	}
}

func TestFromHSLWrapsHue(t *testing.T) {
	if FromHSL(370, 1, 0.5) != FromHSL(10, 1, 0.5) {
		t.Error("hue should wrap mod 360")
	}
	if FromHSL(-30, 1, 0.5) != FromHSL(330, 1, 0.5) {
		t.Error("negative hue should wrap")
	}
}

func TestDesaturate(t *testing.T) {
	c := MustHex("#ED6A5A")
	if got := c.Desaturate(0); !almostEqual(got, c) {
		t.Errorf("Desaturate(0) changed color: %v -> %v", c, got)
	}
	gray := c.Desaturate(1)
	if math.Abs(gray.R-gray.G) > 1e-9 || math.Abs(gray.G-gray.B) > 1e-9 {
		t.Errorf("Desaturate(1) not gray: %v", gray)
	}
}

func TestContrastShade(t *testing.T) {
	dark := MustHex("#2A2A40")
	light := MustHex("#E8E0C8")
	_, _, ld := dark.HSL()
	_, _, ld2 := dark.ContrastShade(0.14).HSL()
	if ld2 <= ld {
		t.Errorf("dark color should lighten: %v -> %v", ld, ld2)
	}
	_, _, ll := light.HSL()
	_, _, ll2 := light.ContrastShade(0.14).HSL()
	if ll2 >= ll {
		t.Errorf("light color should darken: %v -> %v", ll, ll2)
	}
}

func TestLighten(t *testing.T) {
	c := MustHex("#5CA4A9")
	if got := c.Lighten(1); !almostEqual(got, Color{1, 1, 1}) {
		t.Errorf("Lighten(1) = %v, want white", got)
	}
	if got := c.Lighten(0); !almostEqual(got, c) {
		t.Errorf("Lighten(0) changed color: %v -> %v", c, got)
	}
}
