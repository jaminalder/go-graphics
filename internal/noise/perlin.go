// Package noise provides deterministic, seedable 2D gradient noise
// (classic improved Perlin) and fractal Brownian motion on top of it.
// Stdlib-only by design (see docs/ARCHITECTURE.md).
package noise

import (
	"math"
	"math/rand/v2"
)

// permStream is the fixed PCG stream used to build the permutation table,
// so noise seeded with S is independent of other RNG consumers of seed S.
const permStream = 0x6e6f697365 // "noise"

// Perlin is classic improved Perlin noise in 2D. A single octave returns
// values in about [-0.71, 0.71] (±√2/2) and is exactly 0 on integer lattice
// points. Safe for concurrent use after construction.
type Perlin struct {
	perm [512]uint8
}

// New builds a Perlin generator whose gradient permutation is derived
// deterministically from seed.
func New(seed uint64) *Perlin {
	rng := rand.New(rand.NewPCG(seed, permStream))
	var p [256]uint8
	for i := range p {
		p[i] = uint8(i)
	}
	rng.Shuffle(256, func(i, j int) { p[i], p[j] = p[j], p[i] })
	n := &Perlin{}
	for i := range n.perm {
		n.perm[i] = p[i&255]
	}
	return n
}

// gradients are the 8 unit-length gradient directions.
var gradients = [8][2]float64{
	{1, 0},
	{-1, 0},
	{0, 1},
	{0, -1},
	{math.Sqrt2 / 2, math.Sqrt2 / 2},
	{-math.Sqrt2 / 2, math.Sqrt2 / 2},
	{math.Sqrt2 / 2, -math.Sqrt2 / 2},
	{-math.Sqrt2 / 2, -math.Sqrt2 / 2},
}

// At evaluates one octave of noise at (x, y).
func (p *Perlin) At(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	xi, yi := int(x0)&255, int(y0)&255
	xf, yf := x-x0, y-y0

	u, v := fade(xf), fade(yf)

	g00 := p.grad(xi, yi, xf, yf)
	g10 := p.grad(xi+1, yi, xf-1, yf)
	g01 := p.grad(xi, yi+1, xf, yf-1)
	g11 := p.grad(xi+1, yi+1, xf-1, yf-1)

	return lerp(lerp(g00, g10, u), lerp(g01, g11, u), v)
}

// FBM sums octaves 0..maxOctave of noise: octave o contributes
// At(x·2^o, y·2^o) / 2^o. maxOctave 0 is a single octave. The caller applies
// the base frequency to x and y beforehand (cycles per canvas unit).
func (p *Perlin) FBM(x, y float64, maxOctave int) float64 {
	return p.FBMP(x, y, maxOctave, 0.5)
}

// FBMP is FBM with an explicit persistence: octave o contributes
// At(x·2^o, y·2^o) · persistence^o. Lower persistence damps the
// high-frequency octaves — same macro shapes, smoother contour lines.
func (p *Perlin) FBMP(x, y float64, maxOctave int, persistence float64) float64 {
	sum := 0.0
	freq, amp := 1.0, 1.0
	for o := 0; o <= maxOctave; o++ {
		sum += p.At(x*freq, y*freq) * amp
		freq *= 2
		amp *= persistence
	}
	return sum
}

func (p *Perlin) grad(xi, yi int, dx, dy float64) float64 {
	g := gradients[p.perm[int(p.perm[xi])+yi]&7]
	return g[0]*dx + g[1]*dy
}

// fade is Perlin's quintic smoothstep 6t⁵−15t⁴+10t³.
func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

func lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}
