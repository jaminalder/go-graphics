package flame

import (
	"math"

	"github.com/jaminalder/go-graphics/internal/mathx"
	"github.com/jaminalder/go-graphics/internal/palette"
)

type tap struct {
	dx, dy int
	w      float64
}

func (h *Hist) developEstimated(tone Tone, out []palette.Color, logMax, invG, vib, bright, gleam float64, bg palette.Color) []palette.Color {
	est := tone.Estimate
	curve := est.Curve
	if curve <= 0 {
		curve = 0.4
	}
	maxR := est.Radius
	minR := est.Min
	if minR < 0 {
		minR = 0
	}

	n := len(h.a)
	accR := make([]float64, n)
	accG := make([]float64, n)
	accB := make([]float64, n)
	accA := make([]float64, n)

	kernels := map[int][]tap{}
	kernel := func(radius float64) []tap {
		if radius < 0.5 {
			return []tap{{0, 0, 1}}
		}
		key := int(math.Round(radius * 2))
		if key < 1 {
			key = 1
		}
		if k, ok := kernels[key]; ok {
			return k
		}
		k := discKernel(float64(key) / 2)
		kernels[key] = k
		return k
	}

	for i, a := range h.a {
		if a == 0 {
			continue
		}
		hits := float64(a) / colorScale
		meanR := float64(h.r[i]) / float64(a)
		meanG := float64(h.g[i]) / float64(a)
		meanB := float64(h.b[i]) / float64(a)
		scale := math.Log(1+hits*bright) / logMax
		rad := maxR / math.Pow(math.Max(hits, 1), curve)
		if rad < minR {
			rad = minR
		}
		if rad > maxR {
			rad = maxR
		}
		x, y := i%h.w, i/h.w
		for _, t := range kernel(rad) {
			xx, yy := x+t.dx, y+t.dy
			if xx < 0 || yy < 0 || xx >= h.w || yy >= h.h {
				continue
			}
			j := yy*h.w + xx
			w := t.w * scale
			accR[j] += meanR * w
			accG[j] += meanG * w
			accB[j] += meanB * w
			accA[j] += w
		}
	}

	for i := range out {
		a := accA[i]
		if a <= 0 {
			out[i] = bg
			continue
		}
		meanR := accR[i] / a
		meanG := accG[i] / a
		meanB := accB[i] / a
		scale := mathx.Clamp01(a)
		alpha := math.Pow(scale, invG)
		if gleam > 0 {
			gw := mathx.Smoothstep(0.62, 1, scale) * gleam
			meanR += (1 - meanR) * gw
			meanG += (1 - meanG) * gw
			meanB += (1 - meanB) * gw
		}
		ch := func(m float64) float64 {
			ind := math.Pow(mathx.Clamp01(m*scale), invG)
			return vib*(m*alpha) + (1-vib)*ind
		}
		r, g, b := ch(meanR), ch(meanG), ch(meanB)
		out[i] = palette.Color{
			R: bg.R*(1-alpha) + r,
			G: bg.G*(1-alpha) + g,
			B: bg.B*(1-alpha) + b,
		}.Clamp()
	}
	return out
}

func discKernel(radius float64) []tap {
	r := int(math.Ceil(radius))
	if r < 1 {
		return []tap{{0, 0, 1}}
	}
	r2 := radius * radius
	taps := make([]tap, 0, (2*r+1)*(2*r+1))
	var sum float64
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			d2 := float64(dx*dx + dy*dy)
			if d2 > r2+1e-9 {
				continue
			}
			d := 0.0
			if radius > 0 {
				d = math.Sqrt(d2) / radius
			}
			w := math.Exp(-2 * d * d)
			taps = append(taps, tap{dx, dy, w})
			sum += w
		}
	}
	if sum <= 0 {
		return []tap{{0, 0, 1}}
	}
	inv := 1 / sum
	for i := range taps {
		taps[i].w *= inv
	}
	return taps
}
