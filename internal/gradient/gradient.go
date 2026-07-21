// Package gradient provides color gradients: continuous cosine gradients in
// the Iñigo Quílez form and discrete sampled gradients, including the
// shuffled variant that produces the contour-band effect of sketch 001.
package gradient

import (
	"math"
	"math/rand/v2"

	"github.com/jaminalder/go-graphics/internal/palette"
)

// Gradient maps t in [0,1] to a color. Implementations clamp t.
type Gradient interface {
	At(t float64) palette.Color
}

// Cosine is a per-channel palette of the form a + b*cos(2π(c*t + d))
// (https://iquilezles.org/articles/palettes/). Channel order: R, G, B.
type Cosine struct {
	A, B, C, D [3]float64
}

// CosineBetween builds the cosine gradient that eases from c1 (t=0) to
// c2 (t=1) along a half cosine, matching thi.ng's cosine-coefficients:
// a=(c1+c2)/2, b=(c1−c2)/2, c=−0.5, d=0.
func CosineBetween(c1, c2 palette.Color) Cosine {
	a1, a2 := [3]float64{c1.R, c1.G, c1.B}, [3]float64{c2.R, c2.G, c2.B}
	var g Cosine
	for i := range 3 {
		g.A[i] = (a1[i] + a2[i]) / 2
		g.B[i] = (a1[i] - a2[i]) / 2
		g.C[i] = -0.5
		g.D[i] = 0
	}
	return g
}

// At evaluates the gradient; t is clamped to [0,1] and the result to valid sRGB.
func (g Cosine) At(t float64) palette.Color {
	t = clamp01(t)
	eval := func(i int) float64 {
		return g.A[i] + g.B[i]*math.Cos(2*math.Pi*(g.C[i]*t+g.D[i]))
	}
	return palette.Color{R: eval(0), G: eval(1), B: eval(2)}.Clamp()
}

// Discrete is a gradient of equally sized color bands.
type Discrete []palette.Color

// Sample evaluates g at n evenly spaced points (t=0 … t=1 inclusive).
func Sample(g Gradient, n int) Discrete {
	d := make(Discrete, n)
	for i := range d {
		d[i] = g.At(float64(i) / float64(n-1))
	}
	return d
}

// At returns the band color for t; each of the n colors covers an equal
// 1/n-wide interval of [0,1].
func (d Discrete) At(t float64) palette.Color {
	idx := int(clamp01(t) * float64(len(d)))
	if idx >= len(d) {
		idx = len(d) - 1
	}
	return d[idx]
}

// Shuffled returns a copy with the colors in random order drawn from rng.
func (d Discrete) Shuffled(rng *rand.Rand) Discrete {
	s := make(Discrete, len(d))
	copy(s, d)
	rng.Shuffle(len(s), func(i, j int) { s[i], s[j] = s[j], s[i] })
	return s
}

func clamp01(v float64) float64 {
	return math.Min(1, math.Max(0, v))
}
