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

func TestSaturate(t *testing.T) {
	c := MustHex("#8A6E5A") // muted brown
	if got := c.Saturate(0); !almostEqual(got, c) {
		t.Errorf("Saturate(0) changed color: %v -> %v", c, got)
	}
	_, s0, _ := c.hsl()
	_, s1, _ := c.Saturate(0.5).hsl()
	if s1 <= s0 {
		t.Errorf("Saturate(0.5) did not increase saturation: %v -> %v", s0, s1)
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
