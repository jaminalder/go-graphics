// Package palette provides the Color type, color manipulation, and the
// ColorLisa artist palette data that all sketches draw from.
package palette

import (
	"fmt"
	"image/color"
	"math"
)

// Color is an sRGB color with float64 components in [0,1].
// Values outside [0,1] are permitted mid-computation; Clamp before display.
type Color struct {
	R, G, B float64
}

// Hex parses "#RRGGBB" (case-insensitive; leading '#' required).
func Hex(s string) (Color, error) {
	if len(s) != 7 || s[0] != '#' {
		return Color{}, fmt.Errorf("palette: invalid hex color %q", s)
	}
	var r, g, b uint8
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return Color{}, fmt.Errorf("palette: invalid hex color %q: %w", s, err)
	}
	return Color{float64(r) / 255, float64(g) / 255, float64(b) / 255}, nil
}

// MustHex is Hex but panics on invalid input. For use with literals only.
func MustHex(s string) Color {
	c, err := Hex(s)
	if err != nil {
		panic(err)
	}
	return c
}

// Hex formats the color as "#RRGGBB" (clamped).
func (c Color) Hex() string {
	n := c.NRGBA()
	return fmt.Sprintf("#%02X%02X%02X", n.R, n.G, n.B)
}

// Clamp limits all components to [0,1].
func (c Color) Clamp() Color {
	return Color{clamp01(c.R), clamp01(c.G), clamp01(c.B)}
}

// NRGBA converts to an 8-bit stdlib color (clamped, rounded, opaque).
func (c Color) NRGBA() color.NRGBA {
	return color.NRGBA{
		R: uint8(math.Round(clamp01(c.R) * 255)),
		G: uint8(math.Round(clamp01(c.G) * 255)),
		B: uint8(math.Round(clamp01(c.B) * 255)),
		A: 255,
	}
}

// Lerp linearly interpolates from a (t=0) to b (t=1). t is not clamped.
func Lerp(a, b Color, t float64) Color {
	return Color{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
	}
}

// Desaturate moves the color toward gray in HSL space.
// amount 0 leaves the color unchanged; 1 makes it fully gray.
func (c Color) Desaturate(amount float64) Color {
	h, s, l := c.hsl()
	return fromHSL(h, s*(1-clamp01(amount)), l)
}

// Saturate moves saturation toward fully saturated in HSL space.
// amount 0 leaves the color unchanged; 1 makes it fully saturated.
func (c Color) Saturate(amount float64) Color {
	h, s, l := c.hsl()
	return fromHSL(h, s+(1-s)*clamp01(amount), l)
}

// Lighten moves lightness toward white in HSL space.
// amount 0 leaves the color unchanged; 1 makes it white.
func (c Color) Lighten(amount float64) Color {
	h, s, l := c.hsl()
	return fromHSL(h, s, l+(1-l)*clamp01(amount))
}

// Luminance is the relative luminance approximation
// 0.2126·R + 0.7152·G + 0.0722·B on the clamped color.
func (c Color) Luminance() float64 {
	c = c.Clamp()
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

// HSL returns hue [0,360), saturation [0,1], lightness [0,1].
func (c Color) HSL() (h, s, l float64) { return c.hsl() }

// FromHSL builds a color from hue (any value, taken mod 360), saturation,
// and lightness (clamped to [0,1]).
func FromHSL(h, s, l float64) Color {
	h = math.Mod(math.Mod(h, 360)+360, 360)
	return fromHSL(h, clamp01(s), clamp01(l))
}

// LerpHSL interpolates from a (t=0) to b (t=1) in HSL space, taking the
// shortest hue arc. Unlike RGB-space interpolation, blends between distant
// hues stay saturated instead of graying out. Achromatic endpoints adopt
// the other endpoint's hue.
func LerpHSL(a, b Color, t float64) Color {
	ha, sa, la := a.hsl()
	hb, sb, lb := b.hsl()
	if sa == 0 {
		ha = hb
	}
	if sb == 0 {
		hb = ha
	}
	d := math.Mod(hb-ha+540, 360) - 180
	return FromHSL(ha+d*t, sa+(sb-sa)*t, la+(lb-la)*t)
}

// hsl converts to hue [0,360), saturation [0,1], lightness [0,1].
func (c Color) hsl() (h, s, l float64) {
	c = c.Clamp()
	maxC := math.Max(c.R, math.Max(c.G, c.B))
	minC := math.Min(c.R, math.Min(c.G, c.B))
	l = (maxC + minC) / 2
	d := maxC - minC
	if d == 0 {
		return 0, 0, l
	}
	if l > 0.5 {
		s = d / (2 - maxC - minC)
	} else {
		s = d / (maxC + minC)
	}
	switch maxC {
	case c.R:
		h = math.Mod((c.G-c.B)/d, 6)
	case c.G:
		h = (c.B-c.R)/d + 2
	default:
		h = (c.R-c.G)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

func fromHSL(h, s, l float64) Color {
	cc := (1 - math.Abs(2*l-1)) * s
	x := cc * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - cc/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = cc, x, 0
	case h < 120:
		r, g, b = x, cc, 0
	case h < 180:
		r, g, b = 0, cc, x
	case h < 240:
		r, g, b = 0, x, cc
	case h < 300:
		r, g, b = x, 0, cc
	default:
		r, g, b = cc, 0, x
	}
	return Color{r + m, g + m, b + m}
}

func clamp01(v float64) float64 {
	return math.Min(1, math.Max(0, v))
}
